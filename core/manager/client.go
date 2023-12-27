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
	"fmt"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/cenkalti/backoff/v4"

	"github.com/kitex-contrib/xds/core/api/kitex_gen/envoy/service/discovery/v3/aggregateddiscoveryservice"
	"github.com/kitex-contrib/xds/core/manager/auth"
	"github.com/kitex-contrib/xds/core/xdsresource"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/genproto/googleapis/rpc/status"
)

type (
	ADSClient = aggregateddiscoveryservice.Client
	ADSStream = aggregateddiscoveryservice.AggregatedDiscoveryService_StreamAggregatedResourcesClient
)

// newADSClient constructs a new stream client that communicates with the xds server
func newADSClient(xdsSvrCfg *XDSServerConfig) (ADSClient, error) {
	var opts []client.Option
	opts = append(opts, client.WithHostPorts(xdsSvrCfg.SvrAddr))

	if xdsSvrCfg.XDSAuth {
		if tlsConfig, err := auth.GetTLSConfig(xdsSvrCfg.SvrAddr); err != nil {
			return nil, err
		} else {
			opts = append(opts, client.WithGRPCTLSConfig(tlsConfig))
		}
		opts = append(opts, client.WithMetaHandler(auth.ClientHTTP2JwtHandler))
	}

	cli, err := aggregateddiscoveryservice.NewClient(xdsSvrCfg.SvrName, opts...)
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
	adsClient      ADSClient
	streamCh       chan ADSStream
	reqCh          chan *discoveryv3.DiscoveryRequest
	connectBackoff backoff.BackOff

	// resourceUpdater is used to update the resource update to the cache.
	resourceUpdater *xdsResourceManager

	// channel for stop
	closeCh chan struct{}

	mu sync.RWMutex
}

// newXdsClient constructs a new xdsClient, which is used to get xds resources from the xds server.
func newXdsClient(bCfg *BootstrapConfig, ac ADSClient, updater *xdsResourceManager) (*xdsClient, error) {
	cli := &xdsClient{
		config:          bCfg,
		adsClient:       ac,
		connectBackoff:  backoff.NewExponentialBackOff(),
		watchedResource: make(map[xdsresource.ResourceType]map[string]bool),
		cipResolver:     newNdsResolver(),
		versionMap:      make(map[xdsresource.ResourceType]string),
		nonceMap:        make(map[xdsresource.ResourceType]string),
		resourceUpdater: updater,
		closeCh:         make(chan struct{}),
		streamCh:        make(chan ADSStream, 1),
		reqCh:           make(chan *discoveryv3.DiscoveryRequest, 1024),
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

func (c *xdsClient) sender(as ADSStream) {
	// 1. make sure no concurrent send
	// 2. construct a new stream when getting errors (EOF?)
	currStream := as
	for {
		select {
		case <-c.closeCh:
			klog.Infof("KITEX: [XDS] client, stop ads client sender")
			return
		case s := <-c.streamCh:
			// new stream, send request with non version and nonce
			currStream = s
			if err := c.reqWhenReconnect(currStream); err != nil {
				currStream = nil
				continue
			}
		case req := <-c.reqCh:
			if currStream != nil {
				err := currStream.Send(req)
				if err != nil {
					klog.Errorf("KITEX: [XDS] client, send failed, error=%s", err)
					currStream = nil
				}
			}
		}
	}
}

// receiver receives and handle response from the xds server.
// xds server may proactively push the update.
func (c *xdsClient) receiver(as ADSStream) {
	// receiver
	defer func() {
		if err := recover(); err != nil {
			klog.Errorf("KITEX: [XDS] client, run receiver panic, error=%s, stack=%s", err, string(debug.Stack()))
		}
	}()

	currStream := as
	for {
		select {
		case <-c.closeCh:
			klog.Infof("KITEX: [XDS] client, stop ads client receiver")
			return
		default:
		}
		if currStream != nil {
			resp, err := currStream.Recv()
			if err != nil {
				klog.Errorf("KITEX: [XDS] client, receive failed, error=%s", err)
				currStream.Close()
				if auth.IsAuthError(err) {
					// if it is auth error, return directly.
					klog.Errorf("KITEX: [XDS] client, authentication of the control plane failed, close the xDS client. Please check the error log in control plane for more details.")
					c.close()
					return
				}
				if s, e := c.reconnect(); e == nil {
					currStream = s
				}
				continue
			}
			err = c.handleResponse(resp)
			if err != nil {
				klog.Errorf("KITEX: [XDS] client, handle response failed, error=%s", err)
			}
		}
	}
}

// ndsRequired returns if nds is required before lds.
func (c *xdsClient) ndsRequired() bool {
	return !c.config.xdsSvrCfg.NDSNotRequired
}

// ndsWarmup sends the requests (NDS) to the xds server and waits for the response to set the lookup table.
func (c *xdsClient) ndsWarmup() {
	// watch the NameTable when init the xds client
	c.Watch(xdsresource.NameTableType, "", false)
	<-c.cipResolver.initRequestCh
}

// warmup sends the requests (NDS) to the xds server and waits for the response to set the lookup table.
func (c *xdsClient) warmup() {
	if c.ndsRequired() {
		c.ndsWarmup()
	}
	// TODO: maybe need to watch the listener
	klog.Infof("KITEX: [XDS] client, warmup done")
}

func (c *xdsClient) run() {
	// run receiver
	as, err := c.connect()
	if err != nil {
		klog.Errorf("[XDS] client, failed to connect the xDS Server, error=%s", err.Error())
		return
	}

	// start sender and receiver
	go c.sender(as)
	go c.receiver(as)
	c.warmup()
}

// close the xdsClient
func (c *xdsClient) close() {
	select {
	case <-c.closeCh:
	default:
		close(c.closeCh)
	}
}

// connect construct a new stream that connects to the xds server
func (c *xdsClient) connect() (as ADSStream, err error) {
	// backoff retry to connect
	err = backoff.Retry(func() error {
		stream, retryErr := c.adsClient.StreamAggregatedResources(context.Background())
		if retryErr != nil {
			return retryErr
		}
		as = stream
		return nil
	}, c.connectBackoff)
	c.connectBackoff.Reset()

	if err != nil {
		return nil, err
	}
	return as, nil
}

// reconnect constructs a new stream to the server and reset the nonce map and request channel.
// It will only be called in receiver and use the streamCh to notify the sender.
func (c *xdsClient) reconnect() (ADSStream, error) {
	// create new stream
	as, err := c.connect()
	if err != nil {
		klog.Errorf("KITEX: [XDS] client, reconnect failed, error=%s", err)
		return nil, err
	}
	// reset nonce map and request
	c.mu.Lock()
	c.nonceMap = make(map[xdsresource.ResourceType]string)
	clearRequestCh(c.reqCh, len(c.reqCh))
	c.mu.Unlock()

	// notify others to use the new stream
	c.streamCh <- as
	return as, nil
}

// reqWhenReconnect construct a new stream and send all the watched resources
func (c *xdsClient) reqWhenReconnect(as ADSStream) error {
	// send new requests for all watched resources when reqWhenReconnect
	c.mu.Lock()
	defer c.mu.Unlock()
	for rType, res := range c.watchedResource {
		req := c.prepareRequest(rType, c.versionMap[rType], c.nonceMap[rType], res)
		if err := as.Send(req); err != nil {
			// return err if send failed
			return err
		}
	}
	return nil
}

func (c *xdsClient) sendRequest(req *discoveryv3.DiscoveryRequest) {
	// put the req to the channel
	c.reqCh <- req
}

func (c *xdsClient) resolveAddr(host string) string {
	// In the worst case, lookupHost is called twice, try to reduce it.
	// May exists three kind host:
	// 1. fqdn host in Kubernetes, such as example.default.svc.cluster.local, invoke once always.
	// 2. short name in Kubernetes, such as example, invoke once when the host exists in cipResolver, and twice when the host does not exist in cipResolver.
	// 3. service outside Kubernetes, such as www.example.com, invoke twice always.
	// FIXME: format as <serviceName>.<namespace> is not supported.
	fqdn := c.config.tryExpandFQDN(host)
	cip, ok := c.cipResolver.lookupHost(fqdn)
	if ok && len(cip) > 0 {
		return cip[0]
	}
	if fqdn != host {
		cip, ok := c.cipResolver.lookupHost(host)
		if ok && len(cip) > 0 {
			return cip[0]
		}
	}
	return ""
}

// getListenerName returns the listener name in this format: ${clusterIP}_${port}
// lookup the clusterIP using the cipResolver and return the listenerName
func (c *xdsClient) getListenerName(rName string) (string, error) {
	tmp := strings.Split(rName, ":")
	if len(tmp) != 2 {
		return "", fmt.Errorf("invalid listener name: %s", rName)
	}
	addr, port := tmp[0], tmp[1]
	cip := c.resolveAddr(addr)
	if len(cip) > 0 {
		return cip + "_" + port, nil
	}
	return "", fmt.Errorf("failed to convert listener name for %s", rName)
}

// handleLDS handles the lds response
func (c *xdsClient) handleLDS(resp *discoveryv3.DiscoveryResponse) error {
	res, err := xdsresource.UnmarshalLDS(resp.GetResources())
	c.updateAndACK(xdsresource.ListenerType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		klog.Warnf("KITEX: [XDS] unmarshal lds response error:%v", err)
		return err
	}

	// returned listener name is in the format of ${clusterIP}_${port}
	// which should be converted into to the listener name, in the form of ${fqdn}_${port}, watched by the xds client.
	// we need to filter the response.
	c.mu.RLock()
	filteredRes := make(map[string]xdsresource.Resource)
	for n := range c.watchedResource[xdsresource.ListenerType] {
		if c.ndsRequired() {
			ln, err := c.getListenerName(n)
			if err != nil || ln == "" {
				klog.Warnf("KITEX: [XDS] get listener name %s failed, err: %v", n, err)
				continue
			}
			if lis, ok := res[ln]; ok {
				filteredRes[n] = lis
			}
		} else {
			filteredRes[n] = res[n]
		}
	}
	c.mu.RUnlock()
	// update to cache
	c.resourceUpdater.UpdateResource(xdsresource.ListenerType, filteredRes, resp.GetVersionInfo())
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
	c.resourceUpdater.UpdateResource(xdsresource.RouteConfigType, res, resp.GetVersionInfo())
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
	c.resourceUpdater.UpdateResource(xdsresource.ClusterType, res, resp.GetVersionInfo())
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
	c.resourceUpdater.UpdateResource(xdsresource.EndpointsType, res, resp.GetVersionInfo())
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

func (c *xdsClient) version(t xdsresource.ResourceType) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.versionMap[t]
}

func (c *xdsClient) nonce(t xdsresource.ResourceType) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nonceMap[t]
}

func clearRequestCh(ch chan *discoveryv3.DiscoveryRequest, length int) {
	for i := 0; i < length; i++ {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		default:
			return
		}
	}
}
