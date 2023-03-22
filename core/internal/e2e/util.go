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
	v3clusterpb "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	v3endpointpb "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	v3listenerpb "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	v3routepb "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	dnsProto "github.com/kitex-contrib/xds/core/api/kitex_gen/istio.io/istio/pkg/dns/proto/istio_networking_nds_v1"
	"github.com/kitex-contrib/xds/core/xdsresource"
)

type ResourceCache struct {
	Listeners  []*v3listenerpb.Listener
	Routes     []*v3routepb.RouteConfiguration
	Clusters   []*v3clusterpb.Cluster
	Endpoints  []*v3endpointpb.ClusterLoadAssignment
	NameTables []*dnsProto.NameTable
}

var (
	DefaultResourceCache = &ResourceCache{
		Listeners: []*v3listenerpb.Listener{xdsresource.Listener1, xdsresource.Listener2, xdsresource.Listener3},
		Routes:    []*v3routepb.RouteConfiguration{xdsresource.RouteConfig1, xdsresource.RouteConfig2},
		Clusters:  []*v3clusterpb.Cluster{xdsresource.Cluster1, xdsresource.Cluster2},
		Endpoints: []*v3endpointpb.ClusterLoadAssignment{xdsresource.Endpoints1, xdsresource.Endpoints2},
	}
)
