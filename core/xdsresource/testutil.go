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

package xdsresource

import (
	v3clusterpb "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3endpointpb "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	v3listenerpb "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	v3routepb "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	ratelimitv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/local_ratelimit/v3"
	v3httppb "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	v3thrift_proxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/thrift_proxy/v3"
	v3matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	typedv3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	dnsProto "github.com/kitex-contrib/xds/core/api/kitex_gen/istio.io/istio/pkg/dns/proto/istio_networking_nds_v1"
)

func MarshalAny(message proto.Message) *anypb.Any {
	a, _ := anypb.New(message)
	return a
}

var (
	thriftProxy = &v3thrift_proxy.ThriftProxy{
		RouteConfig: &v3thrift_proxy.RouteConfiguration{
			Routes: []*v3thrift_proxy.Route{
				{
					Match: &v3thrift_proxy.RouteMatch{
						MatchSpecifier: &v3thrift_proxy.RouteMatch_MethodName{
							MethodName: "method",
						},
						Headers: []*v3routepb.HeaderMatcher{
							{
								Name: "k1",
								HeaderMatchSpecifier: &v3routepb.HeaderMatcher_StringMatch{
									StringMatch: &v3matcher.StringMatcher{
										MatchPattern: &v3matcher.StringMatcher_Exact{
											Exact: "v1",
										},
									},
								},
							},
							{
								Name: "k2",
								HeaderMatchSpecifier: &v3routepb.HeaderMatcher_StringMatch{
									StringMatch: &v3matcher.StringMatcher{
										MatchPattern: &v3matcher.StringMatcher_Exact{
											Exact: "v2",
										},
									},
								},
							},
							{
								Name: "k3",
								HeaderMatchSpecifier: &v3routepb.HeaderMatcher_StringMatch{
									StringMatch: &v3matcher.StringMatcher{
										MatchPattern: &v3matcher.StringMatcher_Prefix{
											Prefix: "pre",
										},
									},
								},
							},
							{
								Name: "k4",
								HeaderMatchSpecifier: &v3routepb.HeaderMatcher_StringMatch{
									StringMatch: &v3matcher.StringMatcher{
										MatchPattern: &v3matcher.StringMatcher_SafeRegex{
											SafeRegex: &v3matcher.RegexMatcher{
												Regex: "[a-z][1-9]",
											},
										},
									},
								},
							},
						},
					},
					Route: &v3thrift_proxy.RouteAction{
						ClusterSpecifier: &v3thrift_proxy.RouteAction_Cluster{
							Cluster: "cluster",
						},
					},
				},
			},
		},
	}
	rateLimit = &ratelimitv3.LocalRateLimit{
		TokenBucket: &typedv3.TokenBucket{
			MaxTokens: 10,
			TokensPerFill: &wrapperspb.UInt32Value{
				Value: 101,
			},
		},
	}
	// Rds
	httpConnectionManager1 = &v3httppb.HttpConnectionManager{
		RouteSpecifier: &v3httppb.HttpConnectionManager_Rds{
			Rds: &v3httppb.Rds{
				ConfigSource:    nil,
				RouteConfigName: RouteConfigName1,
			},
		},
		HttpFilters: []*v3httppb.HttpFilter{
			{
				ConfigType: &v3httppb.HttpFilter_TypedConfig{
					TypedConfig: MarshalAny(rateLimit),
				},
			},
		},
	}
	// Inline RouteConfig
	httpConnectionManager2 = &v3httppb.HttpConnectionManager{
		RouteSpecifier: &v3httppb.HttpConnectionManager_RouteConfig{
			RouteConfig: RouteConfig1,
		},
	}
)

var (
	HostName1        = "host1"
	Port1            = "80"
	ClusterIP1       = "0.0.0.0"
	ListenerName1    = HostName1 + ":" + Port1
	ReturnedLisName1 = ClusterIP1 + "_" + Port1
	ReturnedInbound  = "virtualInbound"

	HostName2        = "host2"
	Port2            = "8080"
	ClusterIP2       = "10.0.0.1"
	ListenerName2    = HostName2 + ":" + Port2
	ReturnedLisName2 = ClusterIP2 + "_" + Port2
	ReturnedLisName3 = "lis3"

	InboundListener1 = &v3listenerpb.Listener{
		Name: ReturnedInbound,
		FilterChains: []*v3listenerpb.FilterChain{
			{
				FilterChainMatch: &v3listenerpb.FilterChainMatch{
					DestinationPort: wrapperspb.UInt32(80),
				},
				Filters: []*v3listenerpb.Filter{
					{
						ConfigType: &v3listenerpb.Filter_TypedConfig{
							TypedConfig: MarshalAny(httpConnectionManager1),
						},
					},
				},
			},
		},
	}
	Listener1 = &v3listenerpb.Listener{
		Name: ReturnedLisName1,
		FilterChains: []*v3listenerpb.FilterChain{
			{
				FilterChainMatch: &v3listenerpb.FilterChainMatch{
					DestinationPort: wrapperspb.UInt32(80),
				},
				Filters: []*v3listenerpb.Filter{
					{
						ConfigType: &v3listenerpb.Filter_TypedConfig{
							TypedConfig: MarshalAny(httpConnectionManager1),
						},
					},
					{
						ConfigType: &v3listenerpb.Filter_TypedConfig{
							TypedConfig: MarshalAny(thriftProxy),
						},
					},
				},
			},
		},
	}

	Listener2 = &v3listenerpb.Listener{
		Name: ReturnedLisName2,
		FilterChains: []*v3listenerpb.FilterChain{
			{
				Filters: []*v3listenerpb.Filter{
					{
						ConfigType: &v3listenerpb.Filter_TypedConfig{
							TypedConfig: MarshalAny(httpConnectionManager2),
						},
					},
					{
						ConfigType: &v3listenerpb.Filter_TypedConfig{
							TypedConfig: MarshalAny(thriftProxy),
						},
					},
				},
			},
		},
		DefaultFilterChain: &v3listenerpb.FilterChain{
			Filters: []*v3listenerpb.Filter{
				{
					ConfigType: &v3listenerpb.Filter_TypedConfig{
						TypedConfig: MarshalAny(thriftProxy),
					},
				},
			},
		},
	}
	// Listener3 with thrift proxy in FilterChain and DefaultFilterChain
	Listener3 = &v3listenerpb.Listener{
		Name: ReturnedLisName3,
		FilterChains: []*v3listenerpb.FilterChain{
			{
				Filters: []*v3listenerpb.Filter{
					{
						ConfigType: &v3listenerpb.Filter_TypedConfig{
							TypedConfig: MarshalAny(thriftProxy),
						},
					},
				},
			},
		},
		DefaultFilterChain: &v3listenerpb.FilterChain{
			Filters: []*v3listenerpb.Filter{
				{
					ConfigType: &v3listenerpb.Filter_TypedConfig{
						TypedConfig: MarshalAny(thriftProxy),
					},
				},
			},
		},
	}
)

var (
	RouteConfigName1 = "routeConfig1"
	RouteConfigName2 = "routeConfig2"

	RouteConfig1 = &v3routepb.RouteConfiguration{
		Name: RouteConfigName1,
		VirtualHosts: []*v3routepb.VirtualHost{
			{
				Name: "vhName",
				Routes: []*v3routepb.Route{
					{
						Match: &v3routepb.RouteMatch{
							PathSpecifier: &v3routepb.RouteMatch_Path{
								Path: "path",
							},
						},
						Action: &v3routepb.Route_Route{
							Route: &v3routepb.RouteAction{
								ClusterSpecifier: &v3routepb.RouteAction_WeightedClusters{
									WeightedClusters: &v3routepb.WeightedCluster{
										Clusters: []*v3routepb.WeightedCluster_ClusterWeight{
											{
												Name: ClusterName1,
												Weight: &wrapperspb.UInt32Value{
													Value: uint32(50),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	RouteConfig2 = &v3routepb.RouteConfiguration{
		Name: RouteConfigName2,
		VirtualHosts: []*v3routepb.VirtualHost{
			{
				Name: "vhName",
				Routes: []*v3routepb.Route{
					{
						Match: &v3routepb.RouteMatch{
							PathSpecifier: &v3routepb.RouteMatch_Path{
								Path: "path",
							},
						},
						Action: &v3routepb.Route_Route{
							Route: &v3routepb.RouteAction{
								ClusterSpecifier: &v3routepb.RouteAction_WeightedClusters{
									WeightedClusters: &v3routepb.WeightedCluster{
										Clusters: []*v3routepb.WeightedCluster_ClusterWeight{
											{
												Name: ClusterName1,
												Weight: &wrapperspb.UInt32Value{
													Value: uint32(50),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
)

// Cluster Resource
var (
	ClusterName1 = "cluster1"
	ClusterName2 = "cluster2"

	Cluster1 = &v3clusterpb.Cluster{
		Name:                 ClusterName1,
		ClusterDiscoveryType: &v3clusterpb.Cluster_Type{Type: v3clusterpb.Cluster_EDS},
		EdsClusterConfig: &v3clusterpb.Cluster_EdsClusterConfig{
			ServiceName: EndpointName1,
		},
		LbPolicy: v3clusterpb.Cluster_ROUND_ROBIN,
		OutlierDetection: &v3clusterpb.OutlierDetection{
			FailurePercentageThreshold: &wrapperspb.UInt32Value{
				Value: 10,
			},
			FailurePercentageRequestVolume: &wrapperspb.UInt32Value{
				Value: 100,
			},
		},
		LoadAssignment: nil,
	}
	Cluster2 = &v3clusterpb.Cluster{
		Name:                 ClusterName2,
		ClusterDiscoveryType: &v3clusterpb.Cluster_Type{Type: v3clusterpb.Cluster_EDS},
		EdsClusterConfig: &v3clusterpb.Cluster_EdsClusterConfig{
			ServiceName: EndpointName1,
		},
		OutlierDetection: &v3clusterpb.OutlierDetection{
			FailurePercentageThreshold: &wrapperspb.UInt32Value{
				Value: 10,
			},
			FailurePercentageRequestVolume: &wrapperspb.UInt32Value{
				Value: 0,
			},
		},
		LbPolicy:       v3clusterpb.Cluster_ROUND_ROBIN,
		LoadAssignment: nil,
	}
)

var (
	EndpointName1 = "endpoint1"
	EndpointName2 = "endpoint2"

	edsAddr                = "127.0.0.1"
	edsPort1, edsPort2     = 8080, 8081
	edsWeight1, edsWeight2 = 50, 50
	Endpoints1             = &v3endpointpb.ClusterLoadAssignment{
		ClusterName: EndpointName1,
		Endpoints: []*v3endpointpb.LocalityLbEndpoints{
			{
				LbEndpoints: []*v3endpointpb.LbEndpoint{
					{
						HostIdentifier: &v3endpointpb.LbEndpoint_Endpoint{
							Endpoint: &v3endpointpb.Endpoint{
								Address: &v3.Address{
									Address: &v3.Address_SocketAddress{
										SocketAddress: &v3.SocketAddress{
											Protocol: v3.SocketAddress_TCP,
											Address:  edsAddr,
											PortSpecifier: &v3.SocketAddress_PortValue{
												PortValue: uint32(edsPort1),
											},
										},
									},
								},
							},
						},
						LoadBalancingWeight: &wrapperspb.UInt32Value{
							Value: uint32(edsWeight1),
						},
					},
					{
						HostIdentifier: &v3endpointpb.LbEndpoint_Endpoint{
							Endpoint: &v3endpointpb.Endpoint{
								Address: &v3.Address{
									Address: &v3.Address_SocketAddress{
										SocketAddress: &v3.SocketAddress{
											Protocol: v3.SocketAddress_TCP,
											Address:  edsAddr,
											PortSpecifier: &v3.SocketAddress_PortValue{
												PortValue: uint32(edsPort2),
											},
										},
									},
								},
							},
						},
						LoadBalancingWeight: &wrapperspb.UInt32Value{
							Value: uint32(edsWeight2),
						},
					},
				},
			},
		},
	}
	Endpoints2 = &v3endpointpb.ClusterLoadAssignment{
		ClusterName: EndpointName2,
	}
)

var NameTable1 = &dnsProto.NameTable{
	Table: map[string]*dnsProto.NameTable_NameInfo{
		HostName1: {
			Ips: []string{ClusterIP1},
		},
		HostName2: {
			Ips: []string{ClusterIP2},
		},
	},
}
