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

	udpatypev1 "github.com/cncf/xds/go/udpa/type/v1"
	v3listenerpb "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	v3routepb "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	ratelimitv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/local_ratelimit/v3"
	v3httppb "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	v3thrift_proxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/thrift_proxy/v3"
	"google.golang.org/protobuf/types/known/anypb"

	"google.golang.org/protobuf/proto"
)

// ListenerResource is the resource parsed from the lds response.
// Only includes NetworkFilters now
type ListenerResource struct {
	NetworkFilters []*NetworkFilter
}

func (r *ListenerResource) ResourceType() ResourceType {
	return ListenerType
}

type NetworkFilterType int

const (
	NetworkFilterTypeHTTP NetworkFilterType = iota
	NetworkFilterTypeThrift
)

// NetworkFilter contains the network filter config we need got from Listener.FilterChains.
// Only support HttpConnectionManager and ThriftProxy.
// If InlineRouteConfig is not nil, use it to perform routing.
// Or, use RouteConfigName as the resourceName to get the RouteConfigResource.
type NetworkFilter struct {
	FilterType        NetworkFilterType
	RouteConfigName   string
	RoutePort         uint32
	InlineRouteConfig *RouteConfigResource
}

// UnmarshalLDS unmarshalls the LDS Response.
// Only focus on OutboundListener now.
// Get the InlineRouteConfig or RouteConfigName from the listener.
func UnmarshalLDS(rawResources []*anypb.Any) (map[string]*ListenerResource, error) {
	ret := make(map[string]*ListenerResource, len(rawResources))
	errMap := make(map[string]error)
	var errSlice []error

	for _, r := range rawResources {
		if r.GetTypeUrl() != ListenerTypeURL {
			err := fmt.Errorf("invalid listener resource type: %s", r.GetTypeUrl())
			errSlice = append(errSlice, err)
			continue
		}
		lis := &v3listenerpb.Listener{}
		if err := proto.Unmarshal(r.GetValue(), lis); err != nil {
			err = fmt.Errorf("unmarshal Listener failed, error=%s", err)
			errSlice = append(errSlice, fmt.Errorf("unmarshal Listener failed, error=%s", err))
			continue
		}
		// Process Network Filters
		// Process both FilterChains and DefaultFilterChain
		var filtersErr []error
		var nfs []*NetworkFilter
		if fcs := lis.FilterChains; fcs != nil {
			for _, fc := range fcs {
				res, err := unmarshalFilterChain(fc)
				if err != nil {
					filtersErr = append(filtersErr, err)
					continue
				}
				nfs = append(nfs, res...)
			}
		}

		if fc := lis.DefaultFilterChain; fc != nil {
			res, err := unmarshalFilterChain(fc)
			if err != nil {
				filtersErr = append(filtersErr, err)
			}
			nfs = append(nfs, res...)
		}
		ret[lis.Name] = &ListenerResource{
			NetworkFilters: nfs,
		}
		if len(filtersErr) > 0 {
			errMap[lis.Name] = combineErrors(filtersErr)
		}
	}
	if len(errMap) == 0 && len(errSlice) == 0 {
		return ret, nil
	}
	return ret, processUnmarshalErrors(errSlice, errMap)
}

// unmarshalFilterChain unmarshalls the filter chain.
// Only process HttpConnectionManager and ThriftProxy now.
func unmarshalFilterChain(fc *v3listenerpb.FilterChain) ([]*NetworkFilter, error) {
	matchPort := fc.GetFilterChainMatch().GetDestinationPort().GetValue()
	var filters []*NetworkFilter
	var errSlice []error
	for _, f := range fc.Filters {
		switch cfgType := f.GetConfigType().(type) {
		case *v3listenerpb.Filter_TypedConfig:
			if cfgType.TypedConfig.TypeUrl == ThriftProxyTypeURL {
				r, err := unmarshalThriftProxy(cfgType.TypedConfig)
				if err != nil {
					errSlice = append(errSlice, err)
					continue
				}
				filters = append(filters, &NetworkFilter{
					FilterType:        NetworkFilterTypeThrift,
					InlineRouteConfig: r,
				})
			}
			if cfgType.TypedConfig.TypeUrl == HTTPConnManagerTypeURL {
				n, r, err := unmarshallHTTPConnectionManager(cfgType.TypedConfig)
				if err != nil {
					errSlice = append(errSlice, err)
					continue
				}
				filters = append(filters, &NetworkFilter{
					FilterType:        NetworkFilterTypeHTTP,
					RouteConfigName:   n,
					RoutePort:         matchPort,
					InlineRouteConfig: r,
				})
			}
		}
	}
	return filters, combineErrors(errSlice)
}

func unmarshalThriftProxy(rawResources *anypb.Any) (*RouteConfigResource, error) {
	tp := &v3thrift_proxy.ThriftProxy{}
	if err := proto.Unmarshal(rawResources.GetValue(), tp); err != nil {
		return nil, fmt.Errorf("unmarshal HttpConnectionManager failed: %s", err)
	}
	rs := tp.RouteConfig.GetRoutes()
	routes := make([]*Route, len(rs))
	for i, r := range rs {
		route := &Route{}
		routeMatch := &ThriftRouteMatch{}
		match := r.GetMatch()
		if match == nil {
			return nil, fmt.Errorf("no match in routeConfig:%s", tp.RouteConfig.Name)
		}
		// method or service match
		switch t := match.GetMatchSpecifier().(type) {
		case *v3thrift_proxy.RouteMatch_MethodName:
			routeMatch.Method = t.MethodName
		case *v3thrift_proxy.RouteMatch_ServiceName:
			routeMatch.ServiceName = t.ServiceName
		}
		routeMatch.Tags = BuildMatchers(match.GetHeaders())
		route.Match = routeMatch
		// action
		action := r.Route
		if action == nil {
			return nil, fmt.Errorf("no action in routeConfig:%s", tp.RouteConfig.Name)
		}
		switch cs := action.GetClusterSpecifier().(type) {
		case *v3thrift_proxy.RouteAction_Cluster:
			route.WeightedClusters = []*WeightedCluster{
				{Name: cs.Cluster, Weight: 1},
			}
		case *v3thrift_proxy.RouteAction_WeightedClusters:
			wcs := cs.WeightedClusters
			clusters := make([]*WeightedCluster, len(wcs.Clusters))
			for i, wc := range wcs.GetClusters() {
				clusters[i] = &WeightedCluster{
					Name:   wc.GetName(),
					Weight: wc.GetWeight().GetValue(),
				}
			}
			route.WeightedClusters = clusters
		}
		routes[i] = route
	}
	return &RouteConfigResource{
		ThriftRouteConfig: &ThriftRouteConfig{Routes: routes},
	}, nil
}

func unmarshallHTTPConnectionManager(rawResources *anypb.Any) (string, *RouteConfigResource, error) {
	httpConnMng := &v3httppb.HttpConnectionManager{}

	if err := proto.Unmarshal(rawResources.GetValue(), httpConnMng); err != nil {
		return "", nil, fmt.Errorf("unmarshal HttpConnectionManager failed: %s", err)
	}
	maxTokens, tokensPerfill, err := getLocalRateLimitFromHttpConnectionManager(httpConnMng)
	if err != nil {
		return "", nil, err
	}
	// convert listener
	// 1. RDS
	// 2. inline route config
	switch httpConnMng.RouteSpecifier.(type) {
	case *v3httppb.HttpConnectionManager_Rds:
		if httpConnMng.GetRds() == nil {
			return "", nil, fmt.Errorf("no Rds in the apiListener")
		}
		if httpConnMng.GetRds().GetRouteConfigName() == "" {
			return "", nil, fmt.Errorf("no route config Name")
		}
		return httpConnMng.GetRds().GetRouteConfigName(), &RouteConfigResource{
			MaxTokens:     maxTokens,
			TokensPerFill: tokensPerfill,
		}, nil
	case *v3httppb.HttpConnectionManager_RouteConfig:
		var rcfg *v3routepb.RouteConfiguration
		if rcfg = httpConnMng.GetRouteConfig(); rcfg == nil {
			return "", nil, fmt.Errorf("no inline route config")
		}
		inlineRouteConfig, err := unmarshalRouteConfig(rcfg)
		if err != nil {
			return "", nil, err
		}
		inlineRouteConfig.MaxTokens = maxTokens
		inlineRouteConfig.TokensPerFill = tokensPerfill
		return httpConnMng.GetRouteConfig().GetName(), inlineRouteConfig, nil
	}
	return "", nil, nil
}

func getLocalRateLimitFromHttpConnectionManager(hcm *v3httppb.HttpConnectionManager) (uint32, uint32, error) {
	for _, filter := range hcm.HttpFilters {
		switch filter.ConfigType.(type) {
		case *v3httppb.HttpFilter_TypedConfig:
			if filter.GetTypedConfig() == nil {
				continue
			}
			typedConfig := filter.GetTypedConfig().GetValue()
			switch filter.GetTypedConfig().TypeUrl {
			case RateLimitTypeURL:
				lrl := &ratelimitv3.LocalRateLimit{}
				if err := proto.Unmarshal(typedConfig, lrl); err != nil {
					return 0, 0, fmt.Errorf("unmarshal LocalRateLimit failed: %s", err)
				}
				if lrl.TokenBucket != nil {
					return lrl.TokenBucket.MaxTokens, lrl.TokenBucket.TokensPerFill.GetValue(), nil
				}
			case TypedStructTypeURL:
				// ratelimit may be configured with udpa struct.
				ts := &udpatypev1.TypedStruct{}
				if err := proto.Unmarshal(typedConfig, ts); err != nil {
					return 0, 0, fmt.Errorf("unmarshal TypedStruct failed: %s", err)
				}
				tokenBucket, ok := ts.GetValue().GetFields()["token_bucket"]
				if !ok {
					continue
				}
				maxTokens, ok := tokenBucket.GetStructValue().GetFields()["max_tokens"]
				if !ok {
					continue
				}
				tokensPerfill, ok := tokenBucket.GetStructValue().GetFields()["tokens_per_fill"]
				if !ok {
					continue
				}
				return uint32(maxTokens.GetNumberValue()), uint32(tokensPerfill.GetNumberValue()), nil
			}
		}
		return 0, 0, nil
	}
	return 0, 0, nil
}
