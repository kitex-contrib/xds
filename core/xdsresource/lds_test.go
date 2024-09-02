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
	"reflect"
	"testing"

	udpatypev1 "github.com/cncf/udpa/go/udpa/type/v1"
	v3routepb "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	ratelimitv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/local_ratelimit/v3"
	v3httppb "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/golang/protobuf/ptypes/any"
	_struct "github.com/golang/protobuf/ptypes/struct"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestUnmarshalLDSError(t *testing.T) {
	tests := []struct {
		name         string
		rawResources []*any.Any
		want         map[string]*ListenerResource
		wantErr      bool
	}{
		{
			name:         "resource is nil",
			rawResources: nil,
			want:         map[string]*ListenerResource{},
			wantErr:      false,
		},
		{
			name: "incorrect resource type url",
			rawResources: []*any.Any{
				{TypeUrl: EndpointTypeURL, Value: []byte{}},
			},
			want:    map[string]*ListenerResource{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalLDS(tt.rawResources)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalLDS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalLDS() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalLDSHttpConnectionManager(t *testing.T) {
	rawResources := []*any.Any{
		MarshalAny(Listener1),
		MarshalAny(Listener2),
	}
	res, err := UnmarshalLDS(rawResources)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res))
	// rds
	lis1 := res[ReturnedLisName1]
	assert.NotNil(t, lis1)
	assert.NotNil(t, lis1.NetworkFilters)
	assert.Equal(t, RouteConfigName1, lis1.NetworkFilters[0].RouteConfigName)
	assert.Equal(t, uint32(80), lis1.NetworkFilters[0].RoutePort)
	assert.Equal(t, uint32(10), lis1.NetworkFilters[0].InlineRouteConfig.MaxTokens)
	assert.Equal(t, uint32(101), lis1.NetworkFilters[0].InlineRouteConfig.TokensPerFill)

	// inline route config
	lis2 := res[ReturnedLisName2]
	assert.NotNil(t, lis2)
	assert.NotNil(t, lis1.NetworkFilters)
	assert.Equal(t, uint32(0), lis2.NetworkFilters[0].RoutePort)
	inlineRcfg := lis2.NetworkFilters[0].InlineRouteConfig
	assert.NotNil(t, inlineRcfg)
	assert.NotNil(t, inlineRcfg.HTTPRouteConfig)
}

func TestUnmarshallLDSThriftProxy(t *testing.T) {
	rawResources := []*any.Any{
		MarshalAny(Listener3),
	}
	res, err := UnmarshalLDS(rawResources)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res))

	lis := res[ReturnedLisName3]
	assert.NotNil(t, lis)
	assert.Equal(t, 2, len(lis.NetworkFilters))

	f := func(filter *NetworkFilter) {
		assert.Equal(t, NetworkFilterTypeThrift, filter.FilterType)
		tp := filter.InlineRouteConfig.ThriftRouteConfig
		assert.NotNil(t, tp)
		assert.Equal(t, 1, len(tp.Routes))
		r := tp.Routes[0]
		assert.NotNil(t, r.Match)
		assert.True(t, r.Match.MatchPath("method"))
		assert.True(t, r.Match.MatchMeta(map[string]string{
			"k1": "v1",
			"k2": "v2",
			"k3": "prehello",
			"k4": "a4",
		}))
		assert.True(t, r.Match.MatchMeta(map[string]string{
			"k1": "v1",
			"k2": "v2",
			"k3": "pre",
			"k4": "a4",
		}))
		assert.False(t, r.Match.MatchMeta(map[string]string{
			"k3": "prehello",
			"k4": "a4",
		}))
		assert.True(t, r.Match.MatchMeta(map[string]string{
			"k1": "v1",
			"k2": "v2",
			"k3": "prehello",
			"k4": "z9",
		}))
		assert.NotNil(t, r.WeightedClusters)
	}
	f(lis.NetworkFilters[0])
	f(lis.NetworkFilters[1])
}

func TestGetLocalRateLimitFromHttpConnectionManager(t *testing.T) {
	rateLimit := &ratelimitv3.LocalRateLimit{
		TokenBucket: &v3.TokenBucket{
			MaxTokens: 10,
			TokensPerFill: &wrappers.UInt32Value{
				Value: 10,
			},
		},
	}
	hcm := &v3httppb.HttpConnectionManager{
		RouteSpecifier: &v3httppb.HttpConnectionManager_RouteConfig{
			RouteConfig: &v3routepb.RouteConfiguration{
				Name: "InboundPassthroughClusterIpv4",
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
	token, tokensPerfill, err := getLocalRateLimitFromHttpConnectionManager(hcm)
	assert.Equal(t, err, nil)
	assert.Equal(t, token, uint32(10))
	assert.Equal(t, tokensPerfill, uint32(10))

	// ---------------------------------- struct ratelimit ------------------------------------
	structLimit := &udpatypev1.TypedStruct{
		TypeUrl: RateLimitTypeURL,
		Value: &_struct.Struct{
			Fields: map[string]*structpb.Value{
				"token_bucket": {
					Kind: &structpb.Value_StructValue{
						StructValue: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"max_tokens": {
									Kind: &structpb.Value_NumberValue{
										NumberValue: 100,
									},
								},
								"tokens_per_fill": {
									Kind: &structpb.Value_NumberValue{
										NumberValue: 100,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	hcm1 := &v3httppb.HttpConnectionManager{
		RouteSpecifier: &v3httppb.HttpConnectionManager_RouteConfig{
			RouteConfig: &v3routepb.RouteConfiguration{
				Name: "InboundPassthroughClusterIpv4",
			},
		},
		HttpFilters: []*v3httppb.HttpFilter{
			{
				ConfigType: &v3httppb.HttpFilter_TypedConfig{
					TypedConfig: MarshalAny(structLimit),
				},
			},
		},
	}
	token, tokensPerfill, err = getLocalRateLimitFromHttpConnectionManager(hcm1)
	assert.Equal(t, err, nil)
	assert.Equal(t, token, uint32(100))
	assert.Equal(t, tokensPerfill, uint32(100))
}
