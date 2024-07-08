/*
 * Copyright 2024 CloudWeGo Authors
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
	"sync/atomic"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

type retrySuit struct {
	lastPolicies atomic.Value
	*retry.Container
}

func updateRetryPolicy(rc *retrySuit, res map[string]xdsresource.Resource) {
	lastPolicies := make(map[string]struct{})
	val := rc.lastPolicies.Load()
	if val != nil {
		lastPolicies = val.(map[string]struct{})
	}

	thisPolicies := make(map[string]struct{})
	defer rc.lastPolicies.Store(thisPolicies)
	for _, resource := range res {
		r, ok := resource.(*xdsresource.RouteConfigResource)
		if !ok {
			continue
		}
		if r.HTTPRouteConfig == nil {
			continue
		}
		for _, vhs := range r.HTTPRouteConfig.VirtualHosts {
			for _, route := range vhs.Routes {
				for _, cluster := range route.WeightedClusters {
					thisPolicies[cluster.Name] = struct{}{}
					retryPolicy := retry.Policy{
						Enable: true,
						Type:   retry.FailureType,
						FailurePolicy: &retry.FailurePolicy{
							RetrySameNode: false,
							StopPolicy: retry.StopPolicy{
								MaxRetryTimes: route.RetryPolicy.NumRetries,
								MaxDurationMS: uint32(route.RetryPolicy.PerTryTimeout.Milliseconds()),
							},
						},
					}
					bop := &retry.BackOffPolicy{}
					if route.RetryPolicy.RetryBackOff == nil {
						bop.BackOffType = retry.NoneBackOffType
					} else {
						if route.RetryPolicy.RetryBackOff.MaxInterval > route.RetryPolicy.RetryBackOff.BaseInterval {
							bop.BackOffType = retry.RandomBackOffType
							bop.CfgItems = map[retry.BackOffCfgKey]float64{
								retry.MaxMSBackOffCfgKey: float64(route.RetryPolicy.RetryBackOff.MaxInterval.Milliseconds()),
								retry.MinMSBackOffCfgKey: float64(route.RetryPolicy.RetryBackOff.BaseInterval.Milliseconds()),
							}
						} else {
							bop.BackOffType = retry.FixedBackOffType
							bop.CfgItems = map[retry.BackOffCfgKey]float64{
								retry.FixMSBackOffCfgKey: float64(route.RetryPolicy.RetryBackOff.BaseInterval.Milliseconds()),
							}
						}
					}
					retryPolicy.FailurePolicy.BackOffPolicy = bop
					rc.Container.NotifyPolicyChange(cluster.Name, retryPolicy)
				}
			}
		}
	}

	for key := range lastPolicies {
		if _, ok := thisPolicies[key]; !ok {
			rc.Container.DeletePolicy(key)
		}
	}
}

// NewRetryPolicy integrate xds config and kitex circuitbreaker
func NewRetryPolicy() client.Option {
	m := xdsResourceManager.getManager()
	if m == nil {
		panic("xds resource manager has not been initialized")
	}

	retry := &retrySuit{
		Container: retry.NewRetryContainer(retry.WithCustomizeKeyFunc(genRetryServiceKey)),
	}

	m.RegisterXDSUpdateHandler(xdsresource.RouteConfigType, func(res map[string]xdsresource.Resource) {
		updateRetryPolicy(retry, res)
	})
	return client.WithRetryContainer(retry.Container)
}

// keep consistent when initialising the circuit breaker suit and updating
// the retry policy.
func genRetryServiceKey(ctx context.Context, ri rpcinfo.RPCInfo) string {
	if ri == nil {
		return ""
	}
	// the value of RouterClusterKey is stored in route process.
	key, _ := ri.To().Tag(RouterClusterKey)
	return key
}
