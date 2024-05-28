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
	"testing"

	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/stretchr/testify/assert"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

func cbConifg(conf interface{}) interface{} {
	m := conf.(map[string]interface{})
	m = m["cb_config"].(map[string]interface{})
	return m["service"]
}

func TestCircuitBreaker(t *testing.T) {
	cb := &circuitBreaker{
		cb: circuitbreak.NewCBSuite(genServiceKey),
	}

	updater := func(res map[string]xdsresource.Resource) {
		updateCircuitPolicy(res, cb.updateAllCircuitConfigs)
	}

	updater(map[string]xdsresource.Resource{
		"c1": &xdsresource.ClusterResource{
			OutlierDetection: &xdsresource.OutlierDetection{
				FailurePercentageThreshold:     10,
				FailurePercentageRequestVolume: 100,
			},
		},
	})
	assert.Equal(t, cbConifg(cb.cb.Dump()), map[string]interface{}{
		"c1": circuitbreak.CBConfig{
			Enable:    true,
			MinSample: 100,
			ErrRate:   float64(0.1),
		},
	})

	updater(map[string]xdsresource.Resource{
		"c2": &xdsresource.ClusterResource{
			OutlierDetection: &xdsresource.OutlierDetection{
				FailurePercentageThreshold:     0,
				FailurePercentageRequestVolume: 100,
			},
		},
	})
	assert.Equal(t, cbConifg(cb.cb.Dump()), map[string]interface{}{
		"c1": circuitbreak.CBConfig{
			Enable: false,
		},
		"c2": circuitbreak.CBConfig{
			Enable: false,
		},
	})

	updater(map[string]xdsresource.Resource{
		"c2": &xdsresource.ClusterResource{
			OutlierDetection: &xdsresource.OutlierDetection{
				FailurePercentageThreshold:     10,
				FailurePercentageRequestVolume: 0,
			},
		},
	})
	assert.Equal(t, cbConifg(cb.cb.Dump()), map[string]interface{}{
		"c1": circuitbreak.CBConfig{
			Enable: false,
		},
		"c2": circuitbreak.CBConfig{
			Enable: false,
		},
	})

	updater(map[string]xdsresource.Resource{
		"c2": &xdsresource.ClusterResource{
			OutlierDetection: &xdsresource.OutlierDetection{
				FailurePercentageThreshold:     10,
				FailurePercentageRequestVolume: 50,
			},
		},
	})
	assert.Equal(t, cbConifg(cb.cb.Dump()), map[string]interface{}{
		"c1": circuitbreak.CBConfig{
			Enable: false,
		},
		"c2": circuitbreak.CBConfig{
			Enable:    true,
			MinSample: 50,
			ErrRate:   float64(0.1),
		},
	})
}
