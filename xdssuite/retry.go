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
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

type retrySuit struct {
	lastPolicies atomic.Value
	*retry.Container
	router      *XDSRouter
	matchMethod bool
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
								MaxDurationMS: uint32(route.RetryPolicy.PerTryTimeout.Milliseconds()) * uint32(route.RetryPolicy.NumRetries),
								CBPolicy: retry.CBPolicy{
									ErrorRate: route.RetryPolicy.CBErrorRate,
								},
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
					for _, method := range route.RetryPolicy.Methods {
						key := retryPolicyKey(cluster.Name, method)
						thisPolicies[key] = struct{}{}
						rc.Container.NotifyPolicyChange(key, *retryPolicy.DeepCopy())
					}
				}
			}
		}
	}

	klog.Debugf("[XDS] update retry policy: %v", thisPolicies)
	for key := range lastPolicies {
		if _, ok := thisPolicies[key]; !ok {
			rc.Container.DeletePolicy(key)
		}
	}
}

func retryPolicyKey(cluster, method string) string {
	return cluster + "|" + method
}

// NewRetryPolicy integrate xds config and kitex circuitbreaker
func NewRetryPolicy(opts ...Option) client.Option {
	opt := NewOptions(opts)
	m := xdsResourceManager.getManager()
	if m == nil {
		panic("xds resource manager has not been initialized")
	}

	rs := &retrySuit{
		router:      NewXDSRouter(opts...),
		matchMethod: opt.matchRetryMethod,
	}
	rs.Container = retry.NewRetryContainer(retry.WithCustomizeKeyFunc(rs.genRetryServiceKey))

	m.RegisterXDSUpdateHandler(xdsresource.RouteConfigType, func(res map[string]xdsresource.Resource) {
		updateRetryPolicy(rs, res)
	})
	return client.WithRetryContainer(rs.Container)
}

// keep consistent when initialising the circuit breaker suit and updating
// the retry policy.
func (rs *retrySuit) genRetryServiceKey(ctx context.Context, ri rpcinfo.RPCInfo) string {
	if ri == nil {
		return ""
	}
	dest := ri.To()
	if dest == nil {
		return ""
	}

	// the value of RouterClusterKey is stored in route process.
	key, exist := ri.To().Tag(RouterClusterKey)
	if exist {
		return key
	}
	res, err := rs.router.Route(ctx, ri)
	if err != nil {
		klog.Warnf("[XDS] get router key failed err: %v", err)
		return ""
	}
	// set destination
	_ = remoteinfo.AsRemoteInfo(dest).SetTag(RouterClusterKey, res.ClusterPicked)
	remoteinfo.AsRemoteInfo(dest).SetTagLock(RouterClusterKey)
	// set timeout
	_ = rpcinfo.AsMutableRPCConfig(ri.Config()).SetRPCTimeout(res.RPCTimeout)
	return rs.retryPolicyKey(res.ClusterPicked, dest.Method())
}

func (rs *retrySuit) retryPolicyKey(cluster, method string) string {
	if !rs.matchMethod {
		return cluster
	}
	return retryPolicyKey(cluster, method)
}
