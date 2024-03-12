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

	"github.com/golang/protobuf/ptypes/any"
	"github.com/stretchr/testify/assert"
)

func TestUnmarshalCDSError(t *testing.T) {
	tests := []struct {
		name         string
		rawResources []*any.Any
		want         map[string]Resource
		wantErr      bool
	}{
		{
			name:         "resource is nil",
			rawResources: nil,
			want:         map[string]Resource{},
			wantErr:      false,
		},
		{
			name: "incorrect resource type url",
			rawResources: []*any.Any{
				{TypeUrl: ListenerTypeURL, Value: []byte{}},
			},
			want:    map[string]Resource{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalCDS(tt.rawResources)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalCDS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalCDS() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalCDSSuccess(t *testing.T) {
	rawResources := []*any.Any{
		MarshalAny(Cluster1),
	}
	got, err := UnmarshalCDS(rawResources)
	assert.Nil(t, err)
	assert.Equal(t, len(got), 1)
	cluster := got[ClusterName1].(*ClusterResource)
	assert.NotNil(t, cluster)
	assert.Equal(t, cluster.EndpointName, EndpointName1)
	assert.Equal(t, cluster.DiscoveryType, ClusterDiscoveryTypeEDS)
	assert.Equal(t, cluster.LbPolicy, ClusterLbRoundRobin)
	assert.Equal(t, cluster.OutlierDetection, &OutlierDetection{
		FailurePercentageThreshold:     10,
		FailurePercentageRequestVolume: 100,
	})
	assert.Nil(t, cluster.InlineEndpoints)
}
