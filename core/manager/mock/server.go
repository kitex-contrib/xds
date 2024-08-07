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

package mock

import (
	"fmt"
	"net"
	"sync"
	"time"

	v3 "github.com/kitex-contrib/xds/core/api/kitex_gen/envoy/service/discovery/v3"
	"github.com/kitex-contrib/xds/core/api/kitex_gen/envoy/service/discovery/v3/aggregateddiscoveryservice"
	"github.com/kitex-contrib/xds/core/xdsresource"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/server"
)

type testXDSServer struct {
	mu sync.Mutex
	server.Server
	respCh chan *discoveryv3.DiscoveryResponse
}

func (svr *testXDSServer) PushResourceUpdate(resp *discoveryv3.DiscoveryResponse) {
	svr.respCh <- resp
}

func StartXDSServer(address string) *testXDSServer {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	respCh := make(chan *discoveryv3.DiscoveryResponse)

	svr := aggregateddiscoveryservice.NewServer(
		&testAdsService{
			respCh: respCh,
			resourceCache: map[xdsresource.ResourceType]map[string]*discoveryv3.DiscoveryResponse{
				xdsresource.ListenerType: {
					"":                   LdsInbound,
					LdsResp1.VersionInfo: LdsResp1,
					LdsResp2.VersionInfo: LdsResp2,
					LdsResp3.VersionInfo: LdsResp3,
				},
				xdsresource.RouteConfigType: {
					"":                   RdsResp1,
					RdsResp1.VersionInfo: RdsResp1,
				},
				xdsresource.ClusterType: {
					"":                   CdsResp1,
					CdsResp1.VersionInfo: CdsResp1,
				},
				xdsresource.EndpointsType: {
					"":                   EdsResp1,
					EdsResp1.VersionInfo: EdsResp1,
				},
				xdsresource.NameTableType: {
					"": NdsResp1,
				},
			},
		},
		server.WithServiceAddr(addr),
	)

	xdsServer := &testXDSServer{
		sync.Mutex{},
		svr,
		respCh,
	}
	go func() {
		_ = xdsServer.Run()
	}()
	time.Sleep(time.Millisecond * 100)
	return xdsServer
}

type testAdsService struct {
	respCh chan *discoveryv3.DiscoveryResponse
	// resourceCache: version -> response
	resourceCache map[xdsresource.ResourceType]map[string]*discoveryv3.DiscoveryResponse
}

func (svr *testAdsService) StreamAggregatedResources(stream v3.AggregatedDiscoveryService_StreamAggregatedResourcesServer) (err error) {
	errCh := make(chan error, 2)
	stopCh := make(chan struct{})
	defer close(stopCh)
	// receive the request
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
			}

			msg, err := stream.Recv()
			if err != nil {
				errCh <- err
				return
			}
			svr.handleRequest(msg)
		}
	}()

	// send the response
	go func() {
		for {
			select {
			case <-stopCh:
				return
			case resp := <-svr.respCh:
				err := stream.Send(resp)
				if err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	err = <-errCh
	return err
}

func (svr *testAdsService) handleRequest(msg interface{}) {
	req, ok := msg.(*discoveryv3.DiscoveryRequest)
	if !ok {
		// ignore msgs other than DiscoveryRequest
		return
	}
	rType, ok := xdsresource.ResourceURLToType[req.TypeUrl]
	if !ok {
		return
	}
	if _, ok := svr.resourceCache[rType]; !ok {
		return
	}

	klog.Infof("resc type %v version [%v]", rType, req.VersionInfo)
	cache, ok := svr.resourceCache[rType][req.VersionInfo]
	// ignore ack
	if !ok || req.ResponseNonce == cache.Nonce {
		klog.Infof("resourceCache info.. %v", svr.resourceCache[rType][req.VersionInfo])
		if cache == nil {
			klog.Infof("res nonce %s ", req.ResponseNonce)
		} else {
			klog.Infof("cache nonce [%s] res nonce %s ", cache.Nonce, req.ResponseNonce)
		}
		return
	}
	svr.respCh <- cache
}

func (svr *testAdsService) DeltaAggregatedResources(stream v3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) (err error) {
	return fmt.Errorf("DeltaAggregatedResources has not been implemented")
}
