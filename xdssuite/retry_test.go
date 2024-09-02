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
	"time"

	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/stretchr/testify/assert"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

func newContainer(in map[string]retry.Policy) interface{} {
	r := retry.NewRetryContainer()
	for k, v := range in {
		r.NotifyPolicyChange(k, v)
	}
	return dump(r.Dump())
}

func dump(in interface{}) interface{} {
	m := in.(map[string]interface{})
	delete(m, "msg")
	return m
}

func TestRetry(t *testing.T) {
	r := &retrySuit{
		Container: retry.NewRetryContainer(),
	}

	updateRetryPolicy(r, map[string]xdsresource.Resource{
		"1": &xdsresource.RouteConfigResource{
			HTTPRouteConfig: &xdsresource.HTTPRouteConfig{
				VirtualHosts: []*xdsresource.VirtualHost{
					{
						Routes: []*xdsresource.Route{
							{
								RetryPolicy: xdsresource.RetryPolicy{
									NumRetries:    1,
									PerTryTimeout: time.Second,
								},
								WeightedClusters: []*xdsresource.WeightedCluster{
									{
										Name: "cluster1",
									},
								},
							},
							{
								RetryPolicy: xdsresource.RetryPolicy{
									NumRetries:    2,
									PerTryTimeout: time.Second,
									RetryBackOff: &xdsresource.RetryBackOff{
										BaseInterval: time.Second,
									},
								},
								WeightedClusters: []*xdsresource.WeightedCluster{
									{
										Name: "cluster2",
									},
								},
							},
							{
								RetryPolicy: xdsresource.RetryPolicy{
									NumRetries:    3,
									PerTryTimeout: time.Second,
									RetryBackOff: &xdsresource.RetryBackOff{
										BaseInterval: time.Second,
										MaxInterval:  time.Second * 2,
									},
								},
								WeightedClusters: []*xdsresource.WeightedCluster{
									{
										Name: "cluster3",
									},
								},
							},
						},
					},
				},
			},
		},
	})

	assert.Equal(t, newContainer(map[string]retry.Policy{
		"cluster1": {
			Enable: true,
			Type:   retry.FailureType,
			FailurePolicy: &retry.FailurePolicy{
				RetrySameNode: false,
				StopPolicy: retry.StopPolicy{
					MaxRetryTimes: 1,
					MaxDurationMS: 1000,
				},
				BackOffPolicy: &retry.BackOffPolicy{
					BackOffType: retry.NoneBackOffType,
				},
			},
		},
		"cluster2": {
			Enable: true,
			Type:   retry.FailureType,
			FailurePolicy: &retry.FailurePolicy{
				RetrySameNode: false,
				StopPolicy: retry.StopPolicy{
					MaxRetryTimes: 2,
					MaxDurationMS: 2000,
				},
				BackOffPolicy: &retry.BackOffPolicy{
					BackOffType: retry.FixedBackOffType,
					CfgItems: map[retry.BackOffCfgKey]float64{
						retry.FixMSBackOffCfgKey: 1000,
					},
				},
			},
		},
		"cluster3": {
			Enable: true,
			Type:   retry.FailureType,
			FailurePolicy: &retry.FailurePolicy{
				RetrySameNode: false,
				StopPolicy: retry.StopPolicy{
					MaxRetryTimes: 3,
					MaxDurationMS: 3000,
				},
				BackOffPolicy: &retry.BackOffPolicy{
					BackOffType: retry.RandomBackOffType,
					CfgItems: map[retry.BackOffCfgKey]float64{
						retry.MaxMSBackOffCfgKey: 2000,
						retry.MinMSBackOffCfgKey: 1000,
					},
				},
			},
		},
	}), dump(r.Dump()))

	updateRetryPolicy(r, map[string]xdsresource.Resource{
		"1": &xdsresource.RouteConfigResource{
			HTTPRouteConfig: &xdsresource.HTTPRouteConfig{
				VirtualHosts: []*xdsresource.VirtualHost{
					{
						Routes: []*xdsresource.Route{
							{
								RetryPolicy: xdsresource.RetryPolicy{
									NumRetries:    2,
									PerTryTimeout: time.Second * 5,
								},
								WeightedClusters: []*xdsresource.WeightedCluster{
									{
										Name: "cluster1",
									},
								},
							},
						},
					},
				},
			},
		},
	})

	assert.Equal(t, newContainer(map[string]retry.Policy{
		"cluster1": {
			Enable: true,
			Type:   retry.FailureType,
			FailurePolicy: &retry.FailurePolicy{
				RetrySameNode: false,
				StopPolicy: retry.StopPolicy{
					MaxRetryTimes: 2,
					MaxDurationMS: 10000,
				},
				BackOffPolicy: &retry.BackOffPolicy{
					BackOffType: retry.NoneBackOffType,
				},
			},
		},
	}), dump(r.Dump()))
}
