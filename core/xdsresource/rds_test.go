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

	v3routepb "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestUnmarshalRDSError(t *testing.T) {
	tests := []struct {
		name         string
		rawResources []*any.Any
		want         map[string]*RouteConfigResource
		wantErr      bool
	}{
		{
			name:         "resource is nil",
			rawResources: nil,
			want:         map[string]*RouteConfigResource{},
			wantErr:      false,
		},
		{
			name: "incorrect resource type url",
			rawResources: []*any.Any{
				{TypeUrl: EndpointTypeURL, Value: []byte{}},
			},
			want:    map[string]*RouteConfigResource{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalRDS(tt.rawResources)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalRDS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalRDS() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalRDSSuccess(t *testing.T) {
	var (
		routeConfigName = "route_config"
		vhName          = "vh"
		path            = "test"
	)
	rawResources := []*any.Any{
		MarshalAny(&v3routepb.RouteConfiguration{
			Name: routeConfigName,
			VirtualHosts: []*v3routepb.VirtualHost{
				{
					Name: vhName,
					Routes: []*v3routepb.Route{
						{
							Match: &v3routepb.RouteMatch{
								PathSpecifier: &v3routepb.RouteMatch_Path{
									Path: path,
								},
							},
							Action: &v3routepb.Route_Route{
								Route: &v3routepb.RouteAction{
									ClusterSpecifier: &v3routepb.RouteAction_WeightedClusters{
										WeightedClusters: &v3routepb.WeightedCluster{
											Clusters: []*v3routepb.WeightedCluster_ClusterWeight{
												{
													Name: "cluster1",
													Weight: &wrapperspb.UInt32Value{
														Value: uint32(50),
													},
												},
												{
													Name: "cluster2",
													Weight: &wrapperspb.UInt32Value{
														Value: uint32(50),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}),
	}
	got, err := UnmarshalRDS(rawResources)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(got))
	routeConfig := got[routeConfigName]
	assert.NotNil(t, routeConfig)
	assert.Equal(t, 1, len(routeConfig.HTTPRouteConfig.VirtualHosts))
	vh := routeConfig.HTTPRouteConfig.VirtualHosts[0]
	assert.Equal(t, vhName, vh.Name)
	assert.NotNil(t, vh.Routes)
	assert.True(t, vh.Routes[0].Match.MatchPath(path))
	wcs := vh.Routes[0].WeightedClusters
	assert.NotNil(t, wcs)
	assert.Equal(t, 2, len(wcs))
	assert.Equal(t, 50, int(wcs[0].Weight))
	assert.Equal(t, 50, int(wcs[1].Weight))
}

func TestHTTPRouteMatch_MatchPath(t *testing.T) {
	type fields struct {
		Path   string
		Prefix string
		Tags   map[string]string
	}
	type args struct {
		path string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "empty route, empty input path",
			fields: fields{
				Path:   "",
				Prefix: "/",
			},
			args: args{
				path: "",
			},
			want: true,
		},
		{
			name: "empty route, non-empty input path",
			fields: fields{
				Path:   "",
				Prefix: "/",
			},
			args: args{
				path: "p",
			},
			want: true,
		},
		{
			name: "matched route path",
			fields: fields{
				Path: "p1",
			},
			args: args{
				path: "p1",
			},
			want: true,
		},
		{
			name: "not matched",
			fields: fields{
				Path: "p2",
			},
			args: args{
				path: "p1",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := &HTTPRouteMatch{
				Path:   tt.fields.Path,
				Prefix: tt.fields.Prefix,
				Tags:   tt.fields.Tags,
			}
			if got := rm.MatchPath(tt.args.path); got != tt.want {
				t.Errorf("MatchPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
