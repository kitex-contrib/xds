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
			cb.cb.UpdateServiceCBConfig(k, circuitbreak.CBConfig{
				Enable: false,
			})
		}
	}
}

// NewCircuitBreaker integrate xds config and kitex circuitbreaker
func NewCircuitBreaker() client.Option {
	m := xdsResourceManager.getManager()
	if m == nil {
		panic("xds resource manager has not been initialized")
	}

	cb := &circuitBreaker{
		cb: circuitbreak.NewCBSuite(genServiceCBKey),
	}
	m.RegisterCircuitBreaker(cb.updateAllCircuitConfigs)
	return client.WithCircuitBreaker(cb.cb)
}

// keep consistent when initialising the circuit breaker suit and updating
// the circuit breaker policy.
func genServiceCBKey(ri rpcinfo.RPCInfo) string {
	if ri == nil {
		return ""
	}
	// the value of RouterClusterKey is stored in route process.
	key, _ := ri.To().Tag(RouterClusterKey)
	return key
}
