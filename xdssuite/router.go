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
	"fmt"
	"time"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"

	"github.com/bytedance/gopkg/lang/fastrand"

	"github.com/kitex-contrib/xds/core/xdsresource"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

const (
	// RouterClusterKey picked cluster in rpcInfo tag key
	RouterClusterKey = "XDS_Route_Picked_Cluster"
)

// NewXDSRouterMiddleware creates a middleware for request routing via XDS.
// This middleware picks one upstream cluster and sets RPCTimeout based on route config retrieved from XDS.
func NewXDSRouterMiddleware(opts ...Option) endpoint.Middleware {
	router := NewXDSRouter(opts...)
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request, response interface{}) error {
			ri := rpcinfo.GetRPCInfo(ctx)
			dest := ri.To()
			if dest == nil {
				return kerrors.ErrNoDestService
			}
			res, err := router.Route(ctx, ri)
			if err != nil {
				return kerrors.ErrRoute.WithCause(err)
			}
			// set destination
			_ = remoteinfo.AsRemoteInfo(dest).SetTag(RouterClusterKey, res.ClusterPicked)
			remoteinfo.AsRemoteInfo(dest).SetTagLock(RouterClusterKey)
			// set timeout
			_ = rpcinfo.AsMutableRPCConfig(ri.Config()).SetRPCTimeout(res.RPCTimeout)
			return next(ctx, request, response)
		}
	}
}

// RouteResult stores the result of route
type RouteResult struct {
	RPCTimeout    time.Duration
	ClusterPicked string
	// TODO: retry policy also in RDS
}

// XDSRouter is a router that uses xds to route the rpc call
type XDSRouter struct {
	manager XDSResourceManager
	opts    *Options
}

func NewXDSRouter(opts ...Option) *XDSRouter {
	m := xdsResourceManager.getManager()
	if m == nil {
		panic("xds resource manager has not been initialized")
	}
	return &XDSRouter{
		manager: m,
		opts:    NewOptions(opts),
	}
}

// Route routes the rpc call to a cluster based on the RPCInfo
func (r *XDSRouter) Route(ctx context.Context, ri rpcinfo.RPCInfo) (*RouteResult, error) {
	route, err := r.matchRoute(ctx, ri)
	// no matched route
	if err != nil {
		return nil, fmt.Errorf("[XDS] Router, no matched route for service %s, err=%s", ri.To().ServiceName(), err)
	}
	// pick cluster from the matched route
	cluster, err := pickCluster(route)
	if err != nil {
		return nil, fmt.Errorf("[XDS] Router, no cluster selected, err=%s", err)
	}
	return &RouteResult{
		RPCTimeout:    route.Timeout,
		ClusterPicked: cluster,
	}, nil
}

// matchRoute gets the route config from xdsResourceManager
func (r *XDSRouter) matchRoute(ctx context.Context, ri rpcinfo.RPCInfo) (*xdsresource.Route, error) {
	// use serviceName as the listener name
	ln := ri.To().ServiceName()
	lds, err := r.manager.Get(ctx, xdsresource.ListenerType, ln)
	if err != nil {
		return nil, fmt.Errorf("get listener failed: %v", err)
	}
	listener := lds.(*xdsresource.ListenerResource)

	var thriftFilter, httpFilter *xdsresource.NetworkFilter
	for _, f := range listener.NetworkFilters {
		if f.FilterType == xdsresource.NetworkFilterTypeHTTP {
			httpFilter = f
		}
		if f.FilterType == xdsresource.NetworkFilterTypeThrift {
			thriftFilter = f
		}
	}

	// get metadata
	md := r.opts.routerMetaExtractor(ctx)

	// match thriftFilter route first, only inline route is supported
	if ri.Config().TransportProtocol() != transport.GRPC {
		if thriftFilter != nil && thriftFilter.InlineRouteConfig != nil {
			r := matchThriftRoute(md, ri, thriftFilter.InlineRouteConfig)
			if r != nil {
				return r, nil
			}
		}
	}
	if httpFilter == nil {
		return nil, fmt.Errorf("no http filter found in listener %s", ln)
	}
	// inline route config
	if httpFilter.InlineRouteConfig != nil {
		r := matchHTTPRoute(md, ri, httpFilter.InlineRouteConfig)
		if r != nil {
			return r, nil
		}
	}
	// Get the route config
	rds, err := r.manager.Get(ctx, xdsresource.RouteConfigType, httpFilter.RouteConfigName)
	if err != nil {
		return nil, fmt.Errorf("get route failed: %v", err)
	}
	rcfg := rds.(*xdsresource.RouteConfigResource)
	rt := matchHTTPRoute(md, ri, rcfg)
	if rt != nil {
		return rt, nil
	}
	return nil, fmt.Errorf("no matched route")
}

// matchHTTPRoute matches one http route
func matchHTTPRoute(md map[string]string, ri rpcinfo.RPCInfo, routeConfig *xdsresource.RouteConfigResource) *xdsresource.Route {
	if rcfg := routeConfig.HTTPRouteConfig; rcfg != nil {
		var svcName string
		if ri.Invocation().PackageName() == "" {
			svcName = ri.Invocation().ServiceName()
		} else {
			svcName = fmt.Sprintf("%s.%s", ri.Invocation().PackageName(), ri.Invocation().ServiceName())
		}
		// path defined in the virtual services should be ${serviceName}/${methodName}
		path := fmt.Sprintf("/%s/%s", svcName, ri.Invocation().MethodName())

		// match
		for _, vh := range rcfg.VirtualHosts {
			// skip the domain match now.

			// use the first matched route.
			for _, r := range vh.Routes {
				if routeMatched(path, md, r) {
					return r
				}
			}
		}
	}
	return nil
}

// matchThriftRoute matches one thrift route
func matchThriftRoute(md map[string]string, ri rpcinfo.RPCInfo, routeConfig *xdsresource.RouteConfigResource) *xdsresource.Route {
	if rcfg := routeConfig.ThriftRouteConfig; rcfg != nil {
		for _, r := range rcfg.Routes {
			if routeMatched(ri.To().Method(), md, r) {
				return r
			}
		}
	}
	return nil
}

// routeMatched checks if the route matches the info provided in the RPCInfo
func routeMatched(path string, md map[string]string, r *xdsresource.Route) bool {
	if r.Match != nil && r.Match.MatchPath(path) {
		tagMatched := true
		for mk, mv := range r.Match.GetTags() {
			if v, ok := md[mk]; !ok || v != mv {
				tagMatched = false
				break
			}
		}
		if tagMatched {
			return true
		}
	}
	return false
}

// pickCluster selects cluster based on the weight
func pickCluster(route *xdsresource.Route) (string, error) {
	// handle weighted cluster
	wcs := route.WeightedClusters
	if len(wcs) == 0 {
		return "", fmt.Errorf("no weighted clusters in route")
	}
	if len(wcs) == 1 {
		return wcs[0].Name, nil
	}
	currWeight := uint32(0)
	totalWeight := calTotalWeight(wcs)
	if totalWeight <= 0 {
		js, _ := route.MarshalJSON()
		return "", fmt.Errorf("total weight of route is invalid (<= 0), route: %s", js)
	}
	targetWeight := uint32(fastrand.Int31n(int32(totalWeight)))
	for _, wc := range wcs {
		currWeight += wc.Weight
		if currWeight >= targetWeight {
			return wc.Name, nil
		}
	}
	// should not reach here
	return "", fmt.Errorf("random pick failed")
}

func calTotalWeight(wcs []*xdsresource.WeightedCluster) uint32 {
	var total uint32
	for i := range wcs {
		total += wcs[i].Weight
	}
	return total
}
