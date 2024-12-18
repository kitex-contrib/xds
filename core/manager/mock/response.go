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

package mock

import (
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

// LDS Response
var (
	LDSVersionInit = "init"
	LDSVersion1    = "v1"
	LDSVersion2    = "v2"
	LDSVersion3    = "v3"
	LDSNonceInit   = "init"
	LDSNonce1      = "nonce1"
	LDSNonce2      = "nonce2"
	LDSNonce3      = "nonce3"

	LdsInbound = &discoveryv3.DiscoveryResponse{
		VersionInfo: LDSVersion1,
		Resources: []*anypb.Any{
			xdsresource.MarshalAny(xdsresource.InboundListener1),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ListenerTypeURL,
		ControlPlane: nil,
		Nonce:        LDSNonceInit,
	}

	LdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: LDSVersion1,
		Resources: []*anypb.Any{
			xdsresource.MarshalAny(xdsresource.Listener1),
			xdsresource.MarshalAny(xdsresource.Listener2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ListenerTypeURL,
		Nonce:        LDSNonce1,
		ControlPlane: nil,
	}
	LdsResp2 = &discoveryv3.DiscoveryResponse{
		VersionInfo: LDSVersion2,
		Resources: []*anypb.Any{
			xdsresource.MarshalAny(xdsresource.Listener2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ListenerTypeURL,
		Nonce:        LDSNonce2,
		ControlPlane: nil,
	}
	LdsResp3 = &discoveryv3.DiscoveryResponse{
		VersionInfo:  LDSVersion3,
		Resources:    nil,
		Canary:       false,
		TypeUrl:      xdsresource.ListenerTypeURL,
		Nonce:        LDSNonce3,
		ControlPlane: nil,
	}
)

var (
	RDSVersion1 = "v1"
	RDSVersion2 = "v2"
	RDSNonce1   = "nonce1"
	RDSNonce2   = "nonce2"

	RdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: RDSVersion1,
		Resources: []*anypb.Any{
			xdsresource.MarshalAny(xdsresource.RouteConfig1),
			xdsresource.MarshalAny(xdsresource.RouteConfig2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.RouteTypeURL,
		Nonce:        RDSNonce1,
		ControlPlane: nil,
	}
	RdsResp2 = &discoveryv3.DiscoveryResponse{
		VersionInfo: RDSVersion2,
		Resources: []*anypb.Any{
			xdsresource.MarshalAny(xdsresource.RouteConfig2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.RouteTypeURL,
		Nonce:        RDSNonce2,
		ControlPlane: nil,
	}
)

// CDS Response
var (
	CDSVersion1 = "v1"
	CDSVersion2 = "v2"
	CDSNonce1   = "nonce1"
	CDSNonce2   = "nonce2"

	CdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: CDSVersion1,
		Resources: []*anypb.Any{
			xdsresource.MarshalAny(xdsresource.Cluster1),
			xdsresource.MarshalAny(xdsresource.Cluster2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ClusterTypeURL,
		Nonce:        CDSNonce1,
		ControlPlane: nil,
	}
	CdsResp2 = &discoveryv3.DiscoveryResponse{
		VersionInfo: CDSVersion2,
		Resources: []*anypb.Any{
			xdsresource.MarshalAny(xdsresource.Cluster2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ClusterTypeURL,
		Nonce:        CDSNonce2,
		ControlPlane: nil,
	}
)

// EDS Response
var (
	EDSVersion1 = "v1"
	EDSVersion2 = "v2"
	EDSNonce1   = "nonce1"
	EDSNonce2   = "nonce2"

	EdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: EDSVersion1,
		Resources: []*anypb.Any{
			xdsresource.MarshalAny(xdsresource.Endpoints1),
			xdsresource.MarshalAny(xdsresource.Endpoints2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.EndpointTypeURL,
		Nonce:        EDSNonce1,
		ControlPlane: nil,
	}
	EdsResp2 = &discoveryv3.DiscoveryResponse{
		VersionInfo: EDSVersion2,
		Resources: []*anypb.Any{
			xdsresource.MarshalAny(xdsresource.Endpoints2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.EndpointTypeURL,
		Nonce:        EDSNonce2,
		ControlPlane: nil,
	}
)

// NDS response
var (
	NDSVersion1 = "v1"
	NDSNonce1   = "nonce1"
	NdsResp1    = &discoveryv3.DiscoveryResponse{
		VersionInfo: NDSVersion1,
		Resources: []*anypb.Any{
			xdsresource.MarshalAny(xdsresource.NameTable1),
		},
		Canary:       false,
		TypeUrl:      xdsresource.NameTableTypeURL,
		Nonce:        NDSNonce1,
		ControlPlane: nil,
	}
)
