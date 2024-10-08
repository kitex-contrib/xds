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

	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/rpcinfo"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

// XDSResolver uses xdsResourceManager to get endpoints.
type XDSResolver struct {
	manager XDSResourceManager
}

func NewXDSResolver() *XDSResolver {
	m := xdsResourceManager.getManager()
	if m == nil {
		panic("xds resource manager has not been initialized")
	}
	return &XDSResolver{
		manager: m,
	}
}

// Target should return a description for the given target that is suitable for being a key for cache.
func (r *XDSResolver) Target(ctx context.Context, target rpcinfo.EndpointInfo) (description string) {
	dest, ok := target.Tag(RouterClusterKey)
	// TODO: if no cluster is specified, the discovery will fail
	if !ok {
		return target.ServiceName()
	}
	return dest
}

// Resolve returns a list of instances for the given description of a target.
func (r *XDSResolver) Resolve(ctx context.Context, desc string) (discovery.Result, error) {
	eps, err := r.getEndpoints(ctx, desc)
	if err != nil {
		return discovery.Result{}, err
	}
	instances := make([]discovery.Instance, len(eps))
	for i, e := range eps {
		instances[i] = discovery.NewInstance(e.Addr().Network(), e.Addr().String(), int(e.Weight()), e.Meta())
	}
	res := discovery.Result{
		Cacheable: true,
		CacheKey:  desc,
		Instances: instances,
	}
	return res, nil
}

// getEndpoints gets endpoints for this desc from xdsResourceManager
func (r *XDSResolver) getEndpoints(ctx context.Context, desc string) ([]*xdsresource.Endpoint, error) {
	res, err := r.manager.Get(ctx, xdsresource.ClusterType, desc)
	if err != nil {
		return nil, err
	}
	cluster := res.(*xdsresource.ClusterResource)
	var endpoints *xdsresource.EndpointsResource
	if cluster.InlineEndpoints != nil {
		endpoints = cluster.InlineEndpoints
	} else {
		resource, err := r.manager.Get(ctx, xdsresource.EndpointsType, cluster.EndpointName)
		if err != nil {
			return nil, err
		}
		cla := resource.(*xdsresource.EndpointsResource)
		endpoints = cla
	}

	if endpoints == nil || len(endpoints.Localities) == 0 {
		return nil, fmt.Errorf("no endpoints for cluster: %s", desc)
	}
	// TODO: filter localities
	eps := make([]*xdsresource.Endpoint, 0, len(endpoints.Localities)*3)
	for _, locality := range endpoints.Localities {
		eps = append(eps, locality.Endpoints...)
	}
	return eps, nil
}

// Diff implements the Resolver interface. Use DefaultDiff.
func (r *XDSResolver) Diff(cacheKey string, prev, next discovery.Result) (discovery.Change, bool) {
	return discovery.DefaultDiff(cacheKey, prev, next)
}

// Name returns the name of the resolver.
func (r *XDSResolver) Name() string {
	return "xdsResolver"
}
