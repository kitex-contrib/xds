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

package manager

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestTryExpandFQDN(t *testing.T) {
	testCases := []struct {
		desc string
		host string
		want string
		bsc  *BootstrapConfig
	}{
		{
			desc: "success",
			host: "servicea",
			want: "servicea.default.svc.cluster.local",
			bsc: &BootstrapConfig{
				nodeDomain:      "cluster.local",
				configNamespace: "default",
			},
		},
		{
			desc: "success",
			host: "servicea.bookinfo",
			want: "servicea.bookinfo.svc.cluster.local",
			bsc: &BootstrapConfig{
				nodeDomain:      "cluster.local",
				configNamespace: "default",
			},
		},
		{
			desc: "fqdn",
			host: "servicea.bookinfo.svc.cluster.local",
			want: "servicea.bookinfo.svc.cluster.local",
			bsc: &BootstrapConfig{
				nodeDomain:      "cluster.local",
				configNamespace: "default",
			},
		},
		{
			desc: "error",
			host: "servicea.bookinfo.svc",
			want: "servicea.bookinfo.svc.cluster.local",
			bsc: &BootstrapConfig{
				nodeDomain:      "cluster.local",
				configNamespace: "default",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := tc.bsc.tryExpandFQDN(tc.host)
			if got != tc.want {
				t.Errorf("tryExpandFQDN() got = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseMetaEnvs(t *testing.T) {
	testCases := []struct {
		desc         string
		envs         string
		istioVersion string
		want         *structpb.Struct
	}{
		{
			desc:         "success",
			envs:         `{"cluster": "c1", "domain": "d1", "ISTIO_VERSION": "1.16.5"}`,
			istioVersion: "1.16.3",
			want: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					IstioVersion: {
						Kind: &structpb.Value_StringValue{StringValue: "1.16.5"},
					},
					"cluster": {
						Kind: &structpb.Value_StringValue{StringValue: "c1"},
					},
					"domain": {
						Kind: &structpb.Value_StringValue{StringValue: "d1"},
					},
				},
			},
		},
		{
			desc:         "default",
			envs:         ``,
			istioVersion: "1.16.3",
			want: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					IstioVersion: {
						Kind: &structpb.Value_StringValue{StringValue: "1.16.3"},
					},
				},
			},
		},
		{
			desc:         "fault",
			envs:         `{ISTIO_VERSION: 1.16.3}`,
			istioVersion: "1.16.3",
			want: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					IstioVersion: {
						Kind: &structpb.Value_StringValue{StringValue: "1.16.3"},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := parseMetaEnvs(tc.envs, tc.istioVersion)
			if diff := cmp.Diff(got, tc.want, protocmp.Transform()); diff != "" {
				t.Fatalf("the result %s is diff(-got,+want): %s", tc.desc, diff)
			}
		})
	}
}
