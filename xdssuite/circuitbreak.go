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
	"sync/atomic"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/rpcinfo"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

type circuitBreaker struct {
	lastPolicies atomic.Value
	cb           *circuitbreak.CBSuite
}

func (cb *circuitBreaker) updateAllCircuitConfigs(configs map[string]circuitbreak.CBConfig) {
	if cb.cb == nil {
		return
	}
	newPolicies := make(map[string]struct{})
	for k, v := range configs {
		cb.cb.UpdateServiceCBConfig(k, v)
		newPolicies[k] = struct{}{}
	}

	defer cb.lastPolicies.Store(newPolicies)

	val := cb.lastPolicies.Load()
	if val == nil {
		return
	}
	lastPolicies, ok := val.(map[string]struct{})
	if !ok {
		return
	}
	// disable the old policies that are not in the new configs.
	for k := range lastPolicies {
		if _, ok := newPolicies[k]; !ok {
			// TODO: the CBSuite not provides the deleting interface, that
			// may lead to memory leak.
			cb.cb.UpdateServiceCBConfig(k, circuitbreak.CBConfig{
				Enable: false,
			})
		}
	}
}

func updateCircuitPolicy(res map[string]xdsresource.Resource, handler func(map[string]circuitbreak.CBConfig)) {
	// update circuit break policy
	policies := make(map[string]circuitbreak.CBConfig)
	for key, resource := range res {
		cluster, ok := resource.(*xdsresource.ClusterResource)
		if !ok {
			continue
		}
		if cluster.OutlierDetection == nil {
			continue
		}
		cbconfig := circuitbreak.CBConfig{}
		if cluster.OutlierDetection.FailurePercentageRequestVolume != 0 && cluster.OutlierDetection.FailurePercentageThreshold != 0 {
			cbconfig.Enable = true
			cbconfig.ErrRate = float64(cluster.OutlierDetection.FailurePercentageThreshold) / 100
			cbconfig.MinSample = int64(cluster.OutlierDetection.FailurePercentageRequestVolume)
		}
		policies[key] = cbconfig
	}
	handler(policies)
}

// NewCircuitBreaker integrate xds config and kitex circuitbreaker
func NewCircuitBreaker(opts ...Option) client.Option {
	opt := NewOptions(opts)
	m := xdsResourceManager.getManager()
	if m == nil {
		panic("xds resource manager has not been initialized")
	}

	cb := &circuitBreaker{
		cb: circuitbreak.NewCBSuite(genCBServiceKey),
	}
	m.RegisterXDSUpdateHandler(xdsresource.ClusterType, func(res map[string]xdsresource.Resource) {
		updateCircuitPolicy(res, cb.updateAllCircuitConfigs)
	})
	if opt.enableServiceCircuitBreak {
		return client.WithCircuitBreaker(cb.cb)
	}
	return client.WithInstanceMW(cb.cb.ServiceCBMW())
}

// keep consistent when initialising the circuit breaker suit and updating
// the circuit breaker policy.
func genCBServiceKey(ri rpcinfo.RPCInfo) string {
	if ri == nil {
		return ""
	}
	// the value of RouterClusterKey is stored in route process.
	key, _ := ri.To().Tag(RouterClusterKey)
	return key
}
