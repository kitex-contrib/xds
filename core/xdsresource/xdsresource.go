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
	"sync/atomic"
	"time"
)

type Resource interface{}

// ResourceMeta records the meta information of a resource.
type ResourceMeta struct {
	Version        string
	UpdateTime     time.Time
	LastAccessTime atomic.Value
}

const (
	// ReservedLdsResourceName the virtualInbound is used for server side configuration, should
	// initialize it in the first and reserved for the all lifecycle.
	ReservedLdsResourceName = "virtualInbound"

	// DefaultServerPort if not set port for the server, it will use this.
	DefaultServerPort = 0
)

type ResourceType int

const (
	UnknownResource ResourceType = iota
	ListenerType
	RouteConfigType
	ClusterType
	EndpointsType

	NameTableType
)

// RequireFullADSResponse returns if a full response is required for every update.
func (rt ResourceType) RequireFullADSResponse() bool {
	// the RDS/EDS responses are allowed to only include part of the resources
	return rt == ListenerType || rt == ClusterType
}

// Resource types in xDS v3.
const (
	apiTypePrefix          = "type.googleapis.com/"
	ListenerTypeURL        = apiTypePrefix + "envoy.config.listener.v3.Listener"
	RouteTypeURL           = apiTypePrefix + "envoy.config.route.v3.RouteConfiguration"
	ClusterTypeURL         = apiTypePrefix + "envoy.config.cluster.v3.Cluster"
	EndpointTypeURL        = apiTypePrefix + "envoy.config.endpoint.v3.ClusterLoadAssignment"
	SecretTypeURL          = apiTypePrefix + "envoy.extensions.transport_sockets.tls.v3.Secret"
	ExtensionConfigTypeURL = apiTypePrefix + "envoy.config.core.v3.TypedExtensionConfig"
	RuntimeTypeURL         = apiTypePrefix + "envoy.service.runtime.v3.Runtime"
	HTTPConnManagerTypeURL = apiTypePrefix + "envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager"
	ThriftProxyTypeURL     = apiTypePrefix + "envoy.extensions.filters.network.thrift_proxy.v3.ThriftProxy"

	NameTableTypeURL = apiTypePrefix + "istio.networking.nds.v1.NameTable"
	// AnyType is used only by ADS
	AnyType = ""
)

var ResourceTypeToURL = map[ResourceType]string{
	ListenerType:    ListenerTypeURL,
	RouteConfigType: RouteTypeURL,
	ClusterType:     ClusterTypeURL,
	EndpointsType:   EndpointTypeURL,
	NameTableType:   NameTableTypeURL,
}

var ResourceURLToType = map[string]ResourceType{
	ListenerTypeURL:  ListenerType,
	RouteTypeURL:     RouteConfigType,
	ClusterTypeURL:   ClusterType,
	EndpointTypeURL:  EndpointsType,
	NameTableTypeURL: NameTableType,
}

var ResourceTypeToName = map[ResourceType]string{
	ListenerType:    "Listener",
	RouteConfigType: "RouteConfig",
	ClusterType:     "Cluster",
	EndpointsType:   "ClusterLoadAssignment",
	NameTableType:   "NameTable",
}

// XDSUpdateHandler is the callback function when one type resource is updated.
type XDSUpdateHandler func(map[string]Resource)
