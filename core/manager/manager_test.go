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
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kitex-contrib/xds/core/manager/mock"
	"github.com/kitex-contrib/xds/core/xdsresource"

	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
)

// for test use
var (
	XdsServerAddress = ":8889"
	NodeProto        = &v3core.Node{
		Id: "sidecar~kitex-test-node",
	}
	XdsServerConfig = &XDSServerConfig{
		SvrAddr: XdsServerAddress,
	}
	XdsBootstrapConfig = &BootstrapConfig{
		node:      NodeProto,
		xdsSvrCfg: XdsServerConfig,
	}
)

func Test_xdsResourceManager_Get(t *testing.T) {
	// Init
	svr := mock.StartXDSServer(XdsServerAddress)
	defer func() {
		if svr != nil {
			_ = svr.Stop()
		}
	}()
	m, err := NewXDSResourceManager(XdsBootstrapConfig)
	assert.Nil(t, err)

	type args struct {
		ctx          context.Context
		resourceType xdsresource.ResourceType
		resourceName string
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "UnknownResourceType",
			args: args{
				ctx:          context.Background(),
				resourceType: 10,
				resourceName: "",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "GetListenerSuccess",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.ListenerType,
				resourceName: xdsresource.ListenerName1,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "GetRouteConfigSuccess",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.RouteConfigType,
				resourceName: xdsresource.RouteConfigName1,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "GetClusterSuccess",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.ClusterType,
				resourceName: xdsresource.ClusterName1,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "GetEndpointsSuccess",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.EndpointsType,
				resourceName: xdsresource.EndpointName1,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "ListenerNotExist",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.ListenerType,
				resourceName: "random",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "RouteConfigNotExist",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.RouteConfigType,
				resourceName: "random",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "ClusterNotExist",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.ClusterType,
				resourceName: "random",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "EndpointsNotExist",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.EndpointsType,
				resourceName: "random",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.Get(tt.args.ctx, tt.args.resourceType, tt.args.resourceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_xdsResourceManager_Get_Resource_Update(t *testing.T) {
	// Init
	svr := mock.StartXDSServer(XdsServerAddress)
	defer func() {
		if svr != nil {
			_ = svr.Stop()
		}
	}()
	m, initErr := NewXDSResourceManager(XdsBootstrapConfig)
	assert.Nil(t, initErr)

	var res xdsresource.Resource
	var err error
	// Listener
	res, err = m.Get(context.Background(), xdsresource.ListenerType, xdsresource.ListenerName1)
	assert.Nil(t, err)
	assert.NotNil(t, res)
	listener, ok := res.(*xdsresource.ListenerResource)
	assert.True(t, ok)
	assert.Equal(t, len(listener.NetworkFilters), 2)
	// push the new resource that does not include listener1 and check if the resourceManager can update the resource
	svr.PushResourceUpdate(mock.LdsResp2)
	time.Sleep(time.Millisecond * 100)
	// should return nil resource
	res, err = m.Get(context.Background(), xdsresource.ListenerType, xdsresource.ListenerName1)
	assert.NotNil(t, err)
	assert.Nil(t, res)

	// RouteConfig
	res, err = m.Get(context.Background(), xdsresource.RouteConfigType, xdsresource.RouteConfigName1)
	assert.Nil(t, err)
	assert.NotNil(t, res)
	_, ok = res.(*xdsresource.RouteConfigResource)
	assert.True(t, ok)
	// push the new resource and check if the resourceManager can update the resource
	svr.PushResourceUpdate(mock.RdsResp2)
	time.Sleep(time.Millisecond * 100)
	res, err = m.Get(context.Background(), xdsresource.RouteConfigType, xdsresource.RouteConfigName1)
	assert.NotNil(t, err)
	assert.Nil(t, res)

	// Cluster
	res, err = m.Get(context.Background(), xdsresource.ClusterType, xdsresource.ClusterName1)
	assert.Nil(t, err)
	assert.NotNil(t, res)
	_, ok = res.(*xdsresource.ClusterResource)
	assert.True(t, ok)
	// push the new resource and check if the resourceManager can update the resource
	svr.PushResourceUpdate(mock.CdsResp2)
	time.Sleep(time.Millisecond * 100)
	res, err = m.Get(context.Background(), xdsresource.ClusterType, xdsresource.ClusterName1)
	assert.NotNil(t, err)
	assert.Nil(t, res)

	// Endpoint
	res, err = m.Get(context.Background(), xdsresource.EndpointsType, xdsresource.EndpointName1)
	assert.Nil(t, err)
	assert.NotNil(t, res)
	_, ok = res.(*xdsresource.EndpointsResource)
	assert.True(t, ok)
	// push the new resource and check if the resourceManager can update the resource
	// TODO: endpoint will not be updated because of the incremental push feature of Istio
	// svr.PushResourceUpdate(mock.EdsResp2)
	// time.Sleep(time.Millisecond * 100)
	// res, err = m.Get(context.Background(), xdsresource.EndpointsType, xdsresource.EndpointName1)
	// test.Assert(t, err != nil)
	// test.Assert(t, m.cache[xdsresource.EndpointsType][xdsresource.EndpointName1] == nil)
}

func Test_xdsResourceManager_getFromCache(t *testing.T) {
	m := &xdsResourceManager{
		cache: map[xdsresource.ResourceType]map[string]xdsresource.Resource{
			xdsresource.ListenerType: {
				xdsresource.ListenerName1: xdsresource.Listener1,
			},
		},
		meta: map[xdsresource.ResourceType]map[string]*xdsresource.ResourceMeta{
			xdsresource.ListenerType: {
				xdsresource.ListenerName1: &xdsresource.ResourceMeta{},
			},
		},
	}

	// succeed
	res, ok := m.getFromCache(xdsresource.ListenerType, xdsresource.ListenerName1)
	assert.True(t, ok)
	assert.Equal(t, xdsresource.Listener1, res)

	// failed
	_, ok = m.getFromCache(xdsresource.ListenerType, "randomListener")
	assert.False(t, ok)
	_, ok = m.getFromCache(xdsresource.ClusterType, "randomCluster")
	assert.False(t, ok)
}

func Test_xdsResourceManager_ConcurrentGet(t *testing.T) {
	svr := mock.StartXDSServer(XdsServerAddress)
	defer func() {
		if svr != nil {
			_ = svr.Stop()
		}
	}()
	m, initErr := NewXDSResourceManager(XdsBootstrapConfig)
	assert.Nil(t, initErr)

	g := func(t2 *testing.T) {
		_, err := m.Get(context.Background(), xdsresource.ListenerType, xdsresource.ListenerName1)
		assert.Nil(t2, err)
		_, err = m.Get(context.Background(), xdsresource.ListenerType, "randomListener")
		assert.NotNil(t2, err)

		_, err = m.Get(context.Background(), xdsresource.RouteConfigType, xdsresource.RouteConfigName1)
		assert.Nil(t2, err)
		_, err = m.Get(context.Background(), xdsresource.RouteConfigType, "randomRouteConfig")
		assert.NotNil(t2, err)

		_, err = m.Get(context.Background(), xdsresource.ClusterType, xdsresource.ClusterName1)
		assert.Nil(t2, err)
		_, err = m.Get(context.Background(), xdsresource.ClusterType, "randomCluster")
		assert.NotNil(t2, err)

		_, err = m.Get(context.Background(), xdsresource.EndpointsType, xdsresource.EndpointName1)
		assert.Nil(t2, err)
		_, err = m.Get(context.Background(), xdsresource.EndpointsType, "randomEndpoints")
		assert.NotNil(t2, err)
	}

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			g(t)
			wg.Done()
		}()
	}
	wg.Wait()
}
