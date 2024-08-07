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

package xdssuite

import (
	"context"
	"sync"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

var xdsResourceManager = &singletonManager{}

type singletonManager struct {
	manager XDSResourceManager
	sync.RWMutex
}

func (m *singletonManager) getManager() XDSResourceManager {
	m.RLock()
	defer m.RUnlock()
	return m.manager
}

// XDSResourceManager is the interface for the xds resource manager.
// Get() returns the resources according to the input resourceType and resourceName.
// Get() returns error when the fetching fails or the resource is not found in the latest update.
type XDSResourceManager interface {
	Get(ctx context.Context, resourceType xdsresource.ResourceType, resourceName string) (interface{}, error)
	// RegisterXDSUpdateHandler registers the callback function for the specified resource
	RegisterXDSUpdateHandler(resourceType xdsresource.ResourceType, handler xdsresource.XDSUpdateHandler)
}

func XDSInited() bool {
	xdsResourceManager.RLock()
	inited := xdsResourceManager.manager != nil
	xdsResourceManager.RUnlock()
	return inited
}

// SetXDSResourceManager builds the XDSResourceManager using the input function.
func SetXDSResourceManager(m XDSResourceManager) error {
	xdsResourceManager.Lock()
	defer xdsResourceManager.Unlock()

	if xdsResourceManager.manager == nil {
		xdsResourceManager.manager = m
	}
	return nil
}
