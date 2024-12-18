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
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	dnsProto "github.com/kitex-contrib/xds/core/api/kitex_gen/istio.io/istio/pkg/dns/proto/istio_networking_nds_v1"
)

type NDSResource struct {
	NameTable map[string][]string
}

func UnmarshalNDS(rawResources []*anypb.Any) (*NDSResource, error) {
	if len(rawResources) < 1 {
		return nil, fmt.Errorf("no NDS resource found in the response")
	}
	r := rawResources[0]
	if r.GetTypeUrl() != NameTableTypeURL {
		return nil, fmt.Errorf("invalid nameTable resource type: %s", r.GetTypeUrl())
	}
	nt := &dnsProto.NameTable{}
	if err := proto.Unmarshal(r.GetValue(), nt); err != nil {
		return nil, fmt.Errorf("unmarshal NameTable failed: %s", err)
	}
	res := make(map[string][]string)
	for k, v := range nt.Table {
		res[k] = v.Ips
	}

	return &NDSResource{
		NameTable: res,
	}, nil
}
