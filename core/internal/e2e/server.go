/*
 * Copyright 2023 CloudWeGo Authors
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

package e2e

import (
	"context"
	"net"
	"reflect"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	v3resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
)

type XDSServer struct {
	grpcSvc  *grpc.Server
	xdsSvr   server.Server
	resCache cache.SnapshotCache
}

func (s *XDSServer) Stop() {
	s.grpcSvc.Stop()
}

func (s *XDSServer) SetResourceCache(ctx context.Context, node string, resourceCache *ResourceCache) {
	res := map[v3resource.Type][]types.Resource{
		v3resource.ListenerType: convert(resourceCache.Listeners),
		v3resource.RouteType:    convert(resourceCache.Routes),
		v3resource.ClusterType:  convert(resourceCache.Clusters),
		v3resource.EndpointType: convert(resourceCache.Endpoints),
	}
	snapshot, err := cache.NewSnapshot("v1", res)
	if err != nil {
		panic(err)
	}
	s.resCache.SetSnapshot(ctx, node, snapshot)
}

func NewXDSServer(address string, callbackFuncs server.CallbackFuncs) *XDSServer {
	c := cache.NewSnapshotCache(false, cache.IDHash{}, nil)
	svr := server.NewServer(context.Background(), c, callbackFuncs)
	gs := grpc.NewServer()
	discovery.RegisterAggregatedDiscoveryServiceServer(gs, svr)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}
	go gs.Serve(lis)
	return &XDSServer{xdsSvr: svr, grpcSvc: gs, resCache: c}
}

func convert(s interface{}) []types.Resource {
	v := reflect.ValueOf(s)
	ret := make([]types.Resource, v.Len())
	for i := 0; i < v.Len(); i++ {
		ret[i] = v.Index(i).Interface().(types.Resource)
	}
	return ret
}
