/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

// xdsResourceManager manages all the xds resources in the cache and export Get function for resource retrieve.
// It uses client to fetch the resources from the control plane.
// It cleans the expired resource periodically.
type xdsResourceManager struct {
	// client communicates with the control plane
	client *xdsClient

	// cache stores all the resources
	cache map[xdsresource.ResourceType]map[string]xdsresource.Resource
	meta  map[xdsresource.ResourceType]map[string]*xdsresource.ResourceMeta
	// notifierMap maintains the channel for notifying resource update
	notifierMap map[xdsresource.ResourceType]map[string]*notifier
	mu          sync.RWMutex
	closeCh     chan struct{}

	// options
	opts *Options
}

// notifier is used to notify the resource update along with error
type notifier struct {
	ch  chan struct{}
	err error
}

func (n *notifier) notify(err error) {
	n.err = err
	close(n.ch)
}

// NewXDSResourceManager creates a new xds resource manager
func NewXDSResourceManager(bootstrapConfig *BootstrapConfig, opts ...Option) (*xdsResourceManager, error) {
	// load bootstrap config
	var err error
	m := &xdsResourceManager{
		cache:       map[xdsresource.ResourceType]map[string]xdsresource.Resource{},
		meta:        make(map[xdsresource.ResourceType]map[string]*xdsresource.ResourceMeta),
		notifierMap: make(map[xdsresource.ResourceType]map[string]*notifier),
		mu:          sync.RWMutex{},
		opts:        NewOptions(opts),
		closeCh:     make(chan struct{}),
	}
	// Initial xds client
	if bootstrapConfig == nil {
		bootstrapConfig, err = newBootstrapConfig(m.opts.XDSSvrConfig)
		if err != nil {
			return nil, err
		}
	}
	cli, err := initXDSClient(bootstrapConfig, m)
	if err != nil {
		return nil, err
	}
	m.client = cli

	// start the cache cleaner
	go m.cleaner()
	return m, nil
}

func initXDSClient(bootstrapConfig *BootstrapConfig, m *xdsResourceManager) (*xdsClient, error) {
	// build ads client that communicates with the xds server
	ac, err := newADSClient(bootstrapConfig.xdsSvrCfg)
	if err != nil {
		return nil, fmt.Errorf("[XDS] client: construct ads client failed, %s", err.Error())
	}
	cli, err := newXdsClient(bootstrapConfig, ac, m)
	return cli, err
}

// getFromCache returns the resource from cache and update the access time in the meta
func (m *xdsResourceManager) getFromCache(rType xdsresource.ResourceType, rName string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if ms, ok1 := m.meta[rType]; ok1 {
		if mt, ok2 := ms[rName]; ok2 {
			mt.LastAccessTime.Store(time.Now())
		}
	}
	if c, ok1 := m.cache[rType]; ok1 {
		if r, ok2 := c[rName]; ok2 {
			return r, true
		}
	}
	return nil, false
}

// Get gets the specified resource from cache or from the control plane.
// If the resource is not in the cache, it will be fetched from the control plane via client.
// This will be a synchronous call. It uses the notifier to notify the resource update and return the resource.
func (m *xdsResourceManager) Get(ctx context.Context, rType xdsresource.ResourceType, rName string) (interface{}, error) {
	if _, ok := xdsresource.ResourceTypeToURL[rType]; !ok {
		return nil, fmt.Errorf("[XDS] manager, invalid resource type: %d", rType)
	}
	// Get from cache first
	res, ok := m.getFromCache(rType, rName)
	if ok {
		return res, nil
	}

	// Fetch resource via client and wait for the update
	m.mu.Lock()
	// Setup channel for this resource
	if _, ok := m.notifierMap[rType]; !ok {
		m.notifierMap[rType] = make(map[string]*notifier)
	}
	nf, ok := m.notifierMap[rType][rName]
	if !ok {
		nf = &notifier{ch: make(chan struct{})}
		m.notifierMap[rType][rName] = nf
		// only send one request for this resource
		m.client.Watch(rType, rName, false)
	}
	m.mu.Unlock()
	// Set fetch timeout
	// TODO: timeout should be specified in the config of xdsResourceManager
	ctx, cancel := context.WithTimeout(ctx, defaultXDSFetchTimeout)
	defer cancel()

	select {
	case <-nf.ch:
		// error in the notifier
		if nf.err != nil {
			return nil, fmt.Errorf("[XDS] manager, fetch %s resource[%s] failed, error=%s",
				xdsresource.ResourceTypeToName[rType], rName, nf.err.Error())
		}
		res, _ = m.getFromCache(rType, rName)
		return res, nil
	case <-ctx.Done():
		// remove the notifier if timeout.
		m.mu.Lock()
		delete(m.notifierMap[rType], rName)
		m.mu.Unlock()
		return nil, fmt.Errorf("[XDS] manager, fetch %s resource[%s] timeout",
			xdsresource.ResourceTypeToName[rType], rName)
	}
}

// cleaner cleans the expired cache periodically
// TODO: optimize the cache
func (m *xdsResourceManager) cleaner() {
	ticker := time.NewTicker(defaultCacheExpireTime)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.mu.Lock()
			for rt := range m.meta {
				// when the cache is cleared, a remove watch request should be sent to Istiod.
				for rName, meta := range m.meta[rt] {
					t, ok := meta.LastAccessTime.Load().(time.Time)
					if !ok {
						continue
					}
					if time.Since(t) > defaultCacheExpireTime {
						delete(m.meta[rt], rName)
						if m.cache[rt] != nil {
							delete(m.cache[rt], rName)
						}
						m.client.Watch(rt, rName, true)
					}
				}
			}
			m.mu.Unlock()
		case <-m.closeCh:
			return
		}
	}
}

// Dump dumps the cache to local file when the cache is updated
func (m *xdsResourceManager) Dump() {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.opts.DumpPath
	dumpResource := make(map[string]interface{})
	for rType, n := range xdsresource.ResourceTypeToName {
		if res, ok := m.cache[rType]; ok {
			dumpResource[n] = res
		}
	}
	data, err := json.MarshalIndent(dumpResource, "", "    ")
	if err != nil {
		klog.Warnf("[XDS] manager, marshal xds resource failed when dumping, error=%s", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		klog.Warnf("[XDS] manager, dump xds resource failed\n")
	}
}

// Close closes the xds manager
func (m *xdsResourceManager) Close() {
	// close xds client
	m.client.close()
	close(m.closeCh)
}

// updateMeta updates the meta (version, updateTime) of the resource
func (m *xdsResourceManager) updateMeta(rType xdsresource.ResourceType, version string) {
	updateTime := time.Now()

	if _, ok := m.meta[rType]; !ok {
		m.meta[rType] = make(map[string]*xdsresource.ResourceMeta)
	}
	for name := range m.cache[rType] {
		if mt, ok := m.meta[rType][name]; ok {
			mt.Version = version
			mt.UpdateTime = updateTime
		} else {
			m.meta[rType][name] = &xdsresource.ResourceMeta{
				Version:        version,
				UpdateTime:     updateTime,
				LastAccessTime: atomic.Value{},
			}
		}
	}
}

// UpdateResource is invoked by client to update the cache
func (m *xdsResourceManager) UpdateResource(rt xdsresource.ResourceType, up map[string]xdsresource.Resource, version string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, res := range up {
		if _, ok := m.cache[rt]; !ok {
			m.cache[rt] = make(map[string]xdsresource.Resource)
		}
		m.cache[rt][name] = res
		if _, ok := m.notifierMap[rt]; !ok {
			continue
		}
		if nf, exist := m.notifierMap[rt][name]; exist {
			nf.notify(nil)
			delete(m.notifierMap[rt], name)
		}
	}

	if rt.RequireFullADSResponse() {
		// remove all resources that are not in the new update
		for name := range m.cache[rt] {
			if _, ok := up[name]; !ok {
				delete(m.cache[rt], name)
			}
		}
	}
	// update meta
	m.updateMeta(rt, version)
}
