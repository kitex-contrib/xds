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

	"github.com/stretchr/testify/assert"

	"github.com/golang/protobuf/ptypes/any"
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
				{TypeUrl: EndpointTypeUrl, Value: []byte{}},
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

	// inline route config
	lis2 := res[ReturnedLisName2]
	assert.NotNil(t, lis2)
	assert.NotNil(t, lis1.NetworkFilters)
	inlineRcfg := lis2.NetworkFilters[0].InlineRouteConfig
	assert.NotNil(t, inlineRcfg)
	assert.NotNil(t, inlineRcfg.HttpRouteConfig)
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
		for k, v := range map[string]string{"k1": "v1", "k2": "v2"} {
			assert.Equal(t, v, r.Match.GetTags()[k])
		}
		assert.NotNil(t, r.WeightedClusters)
	}
	f(lis.NetworkFilters[0])
	f(lis.NetworkFilters[1])
}
