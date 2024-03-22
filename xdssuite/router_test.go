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
	"fmt"
	"testing"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/stretchr/testify/assert"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

var (
	pkgName      = "pkg"
	svcName      = "svc"
	method       = "method"
	clusterName1 = "cluster1"
	clusterName2 = "cluster2"
	route0       = &xdsresource.Route{
		Match: &xdsresource.HTTPRouteMatch{
			Prefix: "/prefix",
		},
	}
	route1 = &xdsresource.Route{
		Match: &xdsresource.HTTPRouteMatch{
			Prefix: "/",
		},
		WeightedClusters: []*xdsresource.WeightedCluster{
			{
				Name:   clusterName1,
				Weight: 100,
			},
		},
		Timeout: 0,
	}
	route2 = &xdsresource.Route{
		Match: &xdsresource.HTTPRouteMatch{
			Path: fmt.Sprintf("/%s.%s/%s", pkgName, svcName, method),
		},
		WeightedClusters: []*xdsresource.WeightedCluster{
			{
				Name:   clusterName2,
				Weight: 100,
			},
		},
		Timeout: 0,
	}
	routeConfigNil = &xdsresource.RouteConfigResource{
		HTTPRouteConfig: &xdsresource.HTTPRouteConfig{
			VirtualHosts: nil,
		},
	}
	routeConfigPrefixNotMatched = &xdsresource.RouteConfigResource{
		HTTPRouteConfig: &xdsresource.HTTPRouteConfig{
			VirtualHosts: []*xdsresource.VirtualHost{
				{
					Routes: []*xdsresource.Route{
						route0,
					},
				},
			},
		},
	}
	routeConfigDefaultPrefix = &xdsresource.RouteConfigResource{
		HTTPRouteConfig: &xdsresource.HTTPRouteConfig{
			VirtualHosts: []*xdsresource.VirtualHost{
				{
					Routes: []*xdsresource.Route{
						route1,
					},
				},
			},
		},
	}
	routeConfigPathMatched = &xdsresource.RouteConfigResource{
		HTTPRouteConfig: &xdsresource.HTTPRouteConfig{
			VirtualHosts: []*xdsresource.VirtualHost{
				{
					Routes: []*xdsresource.Route{
						route2,
					},
				},
			},
		},
	}
	routeConfigInOrder = &xdsresource.RouteConfigResource{
		HTTPRouteConfig: &xdsresource.HTTPRouteConfig{
			VirtualHosts: []*xdsresource.VirtualHost{
				{
					Routes: []*xdsresource.Route{
						route1,
						route2,
					},
				},
			},
		},
	}
)

func Test_matchHTTPRoute(t *testing.T) {
	to := rpcinfo.NewEndpointInfo(svcName, method, nil, nil)
	ri := rpcinfo.NewRPCInfo(nil, to, rpcinfo.NewInvocation(svcName, method, pkgName), nil, nil)
	md := map[string]string{}

	var r *xdsresource.Route

	// not matched
	r = matchHTTPRoute(md, ri, routeConfigNil)
	assert.Nil(t, r)
	r = matchHTTPRoute(md, ri, routeConfigPrefixNotMatched)
	assert.Nil(t, r)

	// default prefix
	r = matchHTTPRoute(md, ri, routeConfigDefaultPrefix)
	assert.Equal(t, clusterName1, r.WeightedClusters[0].Name)
	// path
	r = matchHTTPRoute(md, ri, routeConfigPathMatched)
	assert.NotNil(t, r)
	assert.Equal(t, clusterName2, r.WeightedClusters[0].Name)
	// test the order, return the first matched
	r = matchHTTPRoute(md, ri, routeConfigInOrder)
	assert.NotNil(t, r)
	assert.Equal(t, clusterName1, r.WeightedClusters[0].Name)
}

func TestPickCluster(t *testing.T) {
	testFunc := func(route *xdsresource.Route, valid bool) {
		cntMap := make(map[string]int)
		num := 100
		for i := 0; i < num; i++ {
			c, err := pickCluster(route)
			if !valid {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			cntMap[c]++
		}
		var total int
		for _, c := range route.WeightedClusters {
			total += cntMap[c.Name]
		}
		if valid {
			assert.Equal(t, cntMap[""], 0)
			assert.Equal(t, num, total)
		}
	}

	// total weight more than 100
	routeWithWeight200 := &xdsresource.Route{
		WeightedClusters: []*xdsresource.WeightedCluster{
			{
				Name:   clusterName2,
				Weight: 100,
			},
			{
				Name:   clusterName1,
				Weight: 100,
			},
		},
		Timeout: 0,
	}
	assert.Equal(t, uint32(200), calTotalWeight(routeWithWeight200.WeightedClusters))
	testFunc(routeWithWeight200, true)

	// weight less than 100
	routeWithWeight50 := &xdsresource.Route{
		WeightedClusters: []*xdsresource.WeightedCluster{
			{
				Name:   clusterName2,
				Weight: 25,
			},
			{
				Name:   clusterName1,
				Weight: 25,
			},
		},
		Timeout: 0,
	}
	assert.Equal(t, uint32(50), calTotalWeight(routeWithWeight50.WeightedClusters))
	testFunc(routeWithWeight50, true)

	// total weight
	routeWithWeight0 := &xdsresource.Route{
		WeightedClusters: []*xdsresource.WeightedCluster{
			{
				Name:   clusterName2,
				Weight: 0,
			},
			{
				Name:   clusterName1,
				Weight: 0,
			},
		},
		Timeout: 0,
	}
	assert.Equal(t, uint32(0), calTotalWeight(routeWithWeight0.WeightedClusters))
	testFunc(routeWithWeight0, false)
}
