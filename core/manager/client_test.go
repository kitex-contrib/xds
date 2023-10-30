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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/kitex-contrib/xds/core/manager/mock"
	"github.com/kitex-contrib/xds/core/xdsresource"

	"github.com/stretchr/testify/assert"

	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	"github.com/cloudwego/kitex/client/callopt"
)

var (
	mockUpdater = &xdsResourceManager{
		cache:       map[xdsresource.ResourceType]map[string]xdsresource.Resource{},
		meta:        make(map[xdsresource.ResourceType]map[string]*xdsresource.ResourceMeta),
		notifierMap: make(map[xdsresource.ResourceType]map[string]*notifier),
		mu:          sync.RWMutex{},
		opts:        NewOptions(nil),
	}
	mockBootstrapConfig = &BootstrapConfig{
		node:      NodeProto,
		xdsSvrCfg: XdsServerConfig,
	}
)

type mockADSClient struct {
	ADSClient
	opt *streamOpt
}

func (sc *mockADSClient) StreamAggregatedResources(ctx context.Context, callOptions ...callopt.Option) (stream ADSStream, err error) {
	return &mockADSStream{opt: sc.opt}, nil
}

type mockADSStream struct {
	ADSStream
	opt *streamOpt
}

type streamOpt struct {
	sendFunc  func(*discoveryv3.DiscoveryRequest) error
	recvFunc  func() (*discoveryv3.DiscoveryResponse, error)
	closeFunc func() error
}

func (sc *mockADSStream) Send(req *discoveryv3.DiscoveryRequest) error {
	if sc.opt != nil && sc.opt.sendFunc != nil {
		return sc.opt.sendFunc(req)
	}
	return nil
}

func (sc *mockADSStream) Recv() (*discoveryv3.DiscoveryResponse, error) {
	if sc.opt != nil && sc.opt.recvFunc != nil {
		return sc.opt.recvFunc()
	}
	return nil, nil
}

func (sc *mockADSStream) Close() error {
	if sc.opt != nil && sc.opt.closeFunc != nil {
		return sc.opt.closeFunc()
	}
	return nil
}

func Test_newXdsClient(t *testing.T) {
	address := ":8889"
	svr := mock.StartXDSServer(address)
	defer func() {
		if svr != nil {
			_ = svr.Stop()
		}
	}()

	c, err := initXDSClient(&BootstrapConfig{
		node:      &v3core.Node{},
		xdsSvrCfg: &XDSServerConfig{SvrAddr: address, SvrName: IstiodSvrName},
	}, nil)
	defer c.close()
	assert.Nil(t, err)
}

func Test_xdsClient_handleResponse(t *testing.T) {
	// inject mock
	c := &xdsClient{
		config:          mockBootstrapConfig,
		adsClient:       &mockADSClient{},
		connectBackoff:  backoff.NewExponentialBackOff(),
		watchedResource: make(map[xdsresource.ResourceType]map[string]bool),
		cipResolver:     newNdsResolver(),
		versionMap:      make(map[xdsresource.ResourceType]string),
		nonceMap:        make(map[xdsresource.ResourceType]string),
		resourceUpdater: mockUpdater,
		closeCh:         make(chan struct{}),
		streamCh:        make(chan ADSStream, 1),
		reqCh:           make(chan *discoveryv3.DiscoveryRequest, 1024),
	}
	defer c.close()

	// handle before watch
	err := c.handleResponse(mock.LdsResp1)
	assert.Nil(t, err)
	assert.Equal(t, c.version(xdsresource.ListenerType), "")
	assert.Equal(t, c.nonce(xdsresource.ListenerType), "")

	// handle after watch
	c.watchedResource[xdsresource.ListenerType] = make(map[string]bool)
	err = c.handleResponse(mock.LdsResp1)
	assert.Nil(t, err)
	assert.Equal(t, c.version(xdsresource.ListenerType), mock.LDSVersion1)
	assert.Equal(t, c.nonce(xdsresource.ListenerType), mock.LDSNonce1)

	c.watchedResource[xdsresource.RouteConfigType] = make(map[string]bool)
	err = c.handleResponse(mock.RdsResp1)
	assert.Nil(t, err)
	assert.Equal(t, c.version(xdsresource.RouteConfigType), mock.RDSVersion1)
	assert.Equal(t, c.nonce(xdsresource.RouteConfigType), mock.RDSNonce1)
}

func TestReconnect(t *testing.T) {
	// used to control the func
	type mockStatus struct {
		err error
	}
	sendCh := make(chan *mockStatus)
	recvCh := make(chan *mockStatus)
	sendCnt, recvCnt := int64(0), int64(0)
	closed := int32(0)
	defer func() {
		close(sendCh)
		close(recvCh)
	}()

	ac := &mockADSClient{
		opt: &streamOpt{
			sendFunc: func(req *discoveryv3.DiscoveryRequest) error {
				atomic.AddInt64(&sendCnt, 1)
				return nil
			},
			recvFunc: func() (response *discoveryv3.DiscoveryResponse, err error) {
				s := <-recvCh
				atomic.AddInt64(&recvCnt, 1)
				if s.err != nil {
					return nil, s.err
				}
				// handle eds will not trigger new send
				return mock.EdsResp1, nil
			},
			closeFunc: func() error {
				atomic.StoreInt32(&closed, 1)
				return nil
			},
		},
	}

	cli, err := newXdsClient(&BootstrapConfig{
		node: NodeProto,
		xdsSvrCfg: &XDSServerConfig{
			SvrAddr:        XdsServerAddress,
			NDSNotRequired: true,
		},
	}, ac, mockUpdater)
	assert.Nil(t, err)

	assert.Equal(t, cli.version(xdsresource.EndpointsType), "")
	cli.Watch(xdsresource.EndpointsType, xdsresource.EndpointName1, false)
	// mock recv succeed
	recvCh <- &mockStatus{err: nil}
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, cli.nonce(xdsresource.EndpointsType), mock.EdsResp1.Nonce)
	// mock recv failed, reconnect
	recvCh <- &mockStatus{err: fmt.Errorf("recv failed")}
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 2, int(atomic.LoadInt64(&recvCnt)))
	assert.Equal(t, 1, int(atomic.LoadInt32(&closed)))
	assert.Equal(t, 3, int(atomic.LoadInt64(&sendCnt))) // watch&ack and reconnect
	// without resp, the nonce should be reset to empty. the version should not be reset.
	assert.Equal(t, cli.nonce(xdsresource.ListenerType), "")
	assert.Equal(t, cli.version(xdsresource.EndpointsType), mock.EdsResp1.VersionInfo)
	_ = cli
}

func TestClearCh(t *testing.T) {
	ch := make(chan *discoveryv3.DiscoveryRequest, 1024)
	for i := 0; i < 10; i++ {
		ch <- &discoveryv3.DiscoveryRequest{}
	}
	assert.Equal(t, 10, len(ch))
	clearRequestCh(ch, 10)
	assert.Equal(t, 0, len(ch))
}
