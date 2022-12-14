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
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/kitex-contrib/xds/core/api/kitex_gen/envoy/service/discovery/v3/aggregateddiscoveryservice"
	"github.com/kitex-contrib/xds/core/xdsresource"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/genproto/googleapis/rpc/status"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
)

type (
	ADSClient = aggregateddiscoveryservice.Client
	ADSStream = aggregateddiscoveryservice.AggregatedDiscoveryService_StreamAggregatedResourcesClient
)

// newADSClient constructs a new stream client that communicates with the xds server
func newADSClient(addr string) (ADSClient, error) {
	cli, err := aggregateddiscoveryservice.NewClient("xds_servers",
		client.WithHostPorts(addr),
	)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

// ndsResolver is used to resolve the clusterIP, using NDS.
type ndsResolver struct {
	lookupTable   map[string][]string
	mu            sync.Mutex
	initRequestCh chan struct{}
	closed        bool
}

func newNdsResolver() *ndsResolver {
	return &ndsResolver{
		mu:            sync.Mutex{},
		lookupTable:   make(map[string][]string),
		initRequestCh: make(chan struct{}),
	}
}

// lookupHost returns the clusterIP of the given hostname.
func (r *ndsResolver) lookupHost(host string) ([]string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ips, ok := r.lookupTable[host]
	return ips, ok
}

// updateResource updates the lookup table based on the NDS response.
func (r *ndsResolver) updateLookupTable(up map[string][]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		close(r.initRequestCh)
		r.closed = true
	}
	r.lookupTable = up
}

// xdsClient communicates with the control plane to perform xds resource discovery.
// It maintains the connection and all the resources being watched.
// It processes the responses from the xds server and update the resources to the cache in xdsResourceManager.
type xdsClient struct {
	// config is the bootstrap config read from the config file.
	config *BootstrapConfig

	// watchedResource is the map of resources that are watched by the client.
	// every discovery request will contain all the resources of one type in the map.
	watchedResource map[xdsresource.ResourceType]map[string]bool

	// cipResolver is used to resolve the clusterIP, using NDS.
	// listenerName (we use fqdn for listener name) to clusterIP.
	// "kitex-server.default.svc.cluster.local" -> "10.0.0.1"
	cipResolver *ndsResolver

	// versionMap stores the versions of different resource type.
	versionMap map[xdsresource.ResourceType]string
	// nonceMap stores the nonce of the recent response.
	nonceMap map[xdsresource.ResourceType]string

	// adsClient is a kitex client using grpc protocol that can communicate with xds server.
	adsClient        ADSClient
	adsStream        ADSStream
	streamClientLock sync.Mutex

	// resourceUpdater is used to update the resource update to the cache.
	resourceUpdater *xdsResourceManager

	// channel for stop
	closeCh chan struct{}

	mu sync.RWMutex
}

// newXdsClient constructs a new xdsClient, which is used to get xds resources from the xds server.
func newXdsClient(bCfg *BootstrapConfig, updater *xdsResourceManager) (*xdsClient, error) {
	// build stream client that communicates with the xds server
	ac, err := newADSClient(bCfg.xdsSvrCfg.SvrAddr)
	if err != nil {
		return nil, fmt.Errorf("[XDS] client: construct stream client failed, %s", err.Error())
	}

	cli := &xdsClient{
		config:          bCfg,
		adsClient:       ac,
		watchedResource: make(map[xdsresource.ResourceType]map[string]bool),
		cipResolver:     newNdsResolver(),
		versionMap:      make(map[xdsresource.ResourceType]string),
		nonceMap:        make(map[xdsresource.ResourceType]string),
		resourceUpdater: updater,
		closeCh:         make(chan struct{}),
	}
	cli.run()
	return cli, nil
}

// Watch adds a resource to the watch map and send a discovery request.
func (c *xdsClient) Watch(rType xdsresource.ResourceType, rName string, remove bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// New resource type
	if r := c.watchedResource[rType]; r == nil {
		c.watchedResource[rType] = make(map[string]bool)
	}
	// subscribe new resource
	if remove {
		delete(c.watchedResource[rType], rName)
	} else {
		c.watchedResource[rType][rName] = true
	}
	// prepare resource name
	req := c.prepareRequest(rType, c.versionMap[rType], c.nonceMap[rType], c.watchedResource[rType])
	// send request for this resource
	c.sendRequest(req)
}

// prepareRequest prepares a new request for the specified resource type with input version and nonce
func (c *xdsClient) prepareRequest(rType xdsresource.ResourceType, version, nonce string, res map[string]bool) *discoveryv3.DiscoveryRequest {
	rNames := make([]string, 0, len(res))
	for name := range res {
		rNames = append(rNames, name)
	}
	return &discoveryv3.DiscoveryRequest{
		VersionInfo:   version,
		Node:          c.config.node,
		TypeUrl:       xdsresource.ResourceTypeToURL[rType],
		ResourceNames: rNames,
		ResponseNonce: nonce,
	}
}

// updateAndACK update versionMap, nonceMap and send ack to xds server
func (c *xdsClient) updateAndACK(rType xdsresource.ResourceType, nonce, version string, err error) {
	// update nonce and version
	// update and ACK only if error is nil, or NACK should be sent
	c.mu.Lock()
	defer c.mu.Unlock()
	var errMsg string
	if err == nil {
		c.versionMap[rType] = version
	} else {
		errMsg = err.Error()
	}
	c.nonceMap[rType] = nonce

	// prepare resource name
	req := c.prepareRequest(rType, c.versionMap[rType], c.nonceMap[rType], c.watchedResource[rType])
	// NACK with error message
	if errMsg != "" {
		req.ErrorDetail = &status.Status{
			Code: int32(codes.InvalidArgument), Message: errMsg,
		}
	}
	c.sendRequest(req)
}

// receiver receives and handle response from the xds server.
// xds server may proactively push the update.
func (c *xdsClient) receiver() {
	// receiver
	defer func() {
		if err := recover(); err != nil {
			klog.Errorf("KITEX: [XDS] client, run receiver panic, error=%s, stack=%s", err, string(debug.Stack()))
		}
	}()

	for {
		select {
		case <-c.closeCh:
			klog.Infof("KITEX: [XDS] client, stop ads client receiver")
			return
		default:
		}
		resp, err := c.recv()
		if err != nil {
			klog.Errorf("KITEX: [XDS] client, receive failed, error=%s", err)
			c.reconnect()
			continue
		}
		err = c.handleResponse(resp)
		if err != nil {
			klog.Errorf("KITEX: [XDS] client, handle response failed, error=%s", err)
		}
	}
}

// warmup sends the requests (NDS) to the xds server and waits for the response to set the lookup table.
func (c *xdsClient) warmup() {
	// watch the NameTable when init the xds client
	c.Watch(xdsresource.NameTableType, "", false)
	<-c.cipResolver.initRequestCh
	klog.Infof("KITEX: [XDS] client, warmup done")
	// TODO: maybe need to watch the listener
}

func (c *xdsClient) run() {
	// run receiver
	go c.receiver()
	c.warmup()
}

// close the xdsClient
func (c *xdsClient) close() {
	close(c.closeCh)
	if c.adsStream != nil {
		c.adsStream.Close()
	}
}

// getStreamClient returns the adsClient of xdsClient
func (c *xdsClient) getStreamClient() (ADSStream, error) {
	c.streamClientLock.Lock()
	defer c.streamClientLock.Unlock()
	// get stream client
	if c.adsStream != nil {
		return c.adsStream, nil
	}
	// connect
	as, err := c.connect()
	if err != nil {
		return nil, err
	}
	c.adsStream = as
	return c.adsStream, nil
}

// connect construct a new stream that connects to the xds server
func (c *xdsClient) connect() (ADSStream, error) {
	as, err := c.adsClient.StreamAggregatedResources(context.Background())
	if err != nil {
		// TODO: optimize the retry of create stream
		if errors.Is(err, kerrors.ErrGetConnection) {
			time.Sleep(time.Second)
		}
		return nil, err
	}
	return as, nil
}

// reconnect construct a new stream and send all the watched resources
func (c *xdsClient) reconnect() error {
	c.streamClientLock.Lock()
	defer c.streamClientLock.Unlock()
	// close old stream
	if c.adsStream != nil {
		c.adsStream.Close()
	}
	// create new stream
	as, err := c.connect()
	if err != nil {
		return err
	}
	c.adsStream = as

	// reset the version map, nonce map and send new requests when reconnect
	c.mu.Lock()
	defer c.mu.Unlock()
	c.versionMap = make(map[xdsresource.ResourceType]string)
	c.nonceMap = make(map[xdsresource.ResourceType]string)
	for rType, res := range c.watchedResource {
		req := c.prepareRequest(rType, c.versionMap[rType], c.nonceMap[rType], res)
		_ = c.adsStream.Send(req)
	}
	return nil
}

func (c *xdsClient) sendRequest(req *discoveryv3.DiscoveryRequest) {
	// get stream client
	sc, err := c.getStreamClient()
	if err != nil {
		klog.Errorf("KITEX: [XDS] client, get stream client failed, error=%s", err)
		return
	}
	err = sc.Send(req)
	if err != nil {
		klog.Errorf("KITEX: [XDS] client, send failed, error=%s", err)
	}
}

// recv uses stream client to receive the response from the xds server
func (c *xdsClient) recv() (resp *discoveryv3.DiscoveryResponse, err error) {
	sc, err := c.getStreamClient()
	if err != nil {
		return nil, err
	}
	resp, err = sc.Recv()
	return resp, err
}

// getListenerName returns the listener name in this format: ${clusterIP}_${port}
// lookup the clusterIP using the cipResolver and return the listenerName
func (c *xdsClient) getListenerName(rName string) (string, error) {
	tmp := strings.Split(rName, ":")
	if len(tmp) != 2 {
		return "", fmt.Errorf("invalid listener name: %s", rName)
	}
	addr, port := tmp[0], tmp[1]
	cip, ok := c.cipResolver.lookupHost(addr)
	if ok && len(cip) > 0 {
		clusterIPPort := cip[0] + "_" + port
		return clusterIPPort, nil
	}
	return "", fmt.Errorf("failed to convert listener name for %s", rName)
}

// handleLDS handles the lds response
func (c *xdsClient) handleLDS(resp *discoveryv3.DiscoveryResponse) error {
	res, err := xdsresource.UnmarshalLDS(resp.GetResources())
	c.updateAndACK(xdsresource.ListenerType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		return err
	}

	// returned listener name is in the format of ${clusterIP}_${port}
	// which should be converted into to the listener name, in the form of ${fqdn}_${port}, watched by the xds client.
	// we need to filter the response.
	c.mu.RLock()
	filteredRes := make(map[string]*xdsresource.ListenerResource)
	for n := range c.watchedResource[xdsresource.ListenerType] {
		ln, err := c.getListenerName(n)
		if err != nil || ln == "" {
			continue
		}
		if lis, ok := res[ln]; ok {
			filteredRes[n] = lis
		}
	}
	c.mu.RUnlock()
	// update to cache
	c.resourceUpdater.UpdateListenerResource(filteredRes, resp.GetVersionInfo())
	return nil
}

// handleRDS handles the rds response
func (c *xdsClient) handleRDS(resp *discoveryv3.DiscoveryResponse) error {
	res, err := xdsresource.UnmarshalRDS(resp.GetResources())
	c.updateAndACK(xdsresource.RouteConfigType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		return err
	}

	// filter the resources that are not in the watched list
	c.mu.RLock()
	for name := range res {
		// only accept the routeConfig that is subscribed
		if _, ok := c.watchedResource[xdsresource.RouteConfigType][name]; !ok {
			delete(res, name)
		}
	}
	c.mu.RUnlock()
	// update to cache
	c.resourceUpdater.UpdateRouteConfigResource(res, resp.GetVersionInfo())
	return nil
}

// handleCDS handles the cds response
func (c *xdsClient) handleCDS(resp *discoveryv3.DiscoveryResponse) error {
	res, err := xdsresource.UnmarshalCDS(resp.GetResources())
	c.updateAndACK(xdsresource.ClusterType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		return fmt.Errorf("handle cluster failed: %s", err)
	}

	// filter the resources that are not in the watched list
	c.mu.RLock()
	for name := range res {
		if _, ok := c.watchedResource[xdsresource.ClusterType][name]; !ok {
			delete(res, name)
		}
	}
	c.mu.RUnlock()
	// update to cache
	c.resourceUpdater.UpdateClusterResource(res, resp.GetVersionInfo())
	return nil
}

// handleEDS handles the eds response
func (c *xdsClient) handleEDS(resp *discoveryv3.DiscoveryResponse) error {
	res, err := xdsresource.UnmarshalEDS(resp.GetResources())
	c.updateAndACK(xdsresource.EndpointsType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		return fmt.Errorf("handle endpoint failed: %s", err)
	}

	// filter the resources that are not in the watched list
	c.mu.RLock()
	for name := range res {
		if _, ok := c.watchedResource[xdsresource.EndpointsType][name]; !ok {
			delete(res, name)
		}
	}
	c.mu.RUnlock()
	// update to cache
	c.resourceUpdater.UpdateEndpointsResource(res, resp.GetVersionInfo())
	return nil
}

func (c *xdsClient) handleNDS(resp *discoveryv3.DiscoveryResponse) error {
	nt, err := xdsresource.UnmarshalNDS(resp.GetResources())
	c.updateAndACK(xdsresource.NameTableType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		return err
	}
	c.cipResolver.updateLookupTable(nt.NameTable)
	return err
}

// handleResponse handles the response from xDS server
func (c *xdsClient) handleResponse(msg interface{}) error {
	if msg == nil {
		return nil
	}
	// check the type of response
	resp, ok := msg.(*discoveryv3.DiscoveryResponse)
	if !ok {
		return fmt.Errorf("invalid discovery response")
	}

	url := resp.GetTypeUrl()
	rType, ok := xdsresource.ResourceURLToType[url]
	if !ok {
		klog.Warnf("KITEX: [XDS] client handleResponse, unknown type of resource, url: %s", url)
		return nil
	}

	// check if this resource type is watched
	c.mu.RLock()
	if _, ok := c.watchedResource[rType]; !ok {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()

	/*
		handle different resources:
		unmarshal resources and ack, update to cache if no error
	*/
	var err error
	switch rType {
	case xdsresource.ListenerType:
		err = c.handleLDS(resp)
	case xdsresource.RouteConfigType:
		err = c.handleRDS(resp)
	case xdsresource.ClusterType:
		err = c.handleCDS(resp)
	case xdsresource.EndpointsType:
		err = c.handleEDS(resp)
	case xdsresource.NameTableType:
		err = c.handleNDS(resp)
	}
	return err
}
