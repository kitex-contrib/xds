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

// Code generated by Kitex v0.3.4. DO NOT EDIT.

package aggregateddiscoveryservice

import (
	"context"
	"fmt"

	"github.com/kitex-contrib/xds/core/api/kitex_gen/envoy/service/discovery/v3"

	v3discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	client "github.com/cloudwego/kitex/client"
	kitex "github.com/cloudwego/kitex/pkg/serviceinfo"
	streaming "github.com/cloudwego/kitex/pkg/streaming"
	proto "google.golang.org/protobuf/proto"
)

func serviceInfo() *kitex.ServiceInfo {
	return aggregatedDiscoveryServiceServiceInfo
}

var aggregatedDiscoveryServiceServiceInfo = NewServiceInfo()

func NewServiceInfo() *kitex.ServiceInfo {
	serviceName := "AggregatedDiscoveryService"
	handlerType := (*v3.AggregatedDiscoveryService)(nil)
	methods := map[string]kitex.MethodInfo{
		"StreamAggregatedResources": kitex.NewMethodInfo(streamAggregatedResourcesHandler, newStreamAggregatedResourcesArgs, newStreamAggregatedResourcesResult, false),
		"DeltaAggregatedResources":  kitex.NewMethodInfo(deltaAggregatedResourcesHandler, newDeltaAggregatedResourcesArgs, newDeltaAggregatedResourcesResult, false),
	}
	extra := map[string]interface{}{
		"PackageName": "envoy.service.discovery.v3",
	}
	extra["streaming"] = true
	svcInfo := &kitex.ServiceInfo{
		ServiceName:     serviceName,
		HandlerType:     handlerType,
		Methods:         methods,
		PayloadCodec:    kitex.Protobuf,
		KiteXGenVersion: "v0.3.4",
		Extra:           extra,
	}
	return svcInfo
}

func streamAggregatedResourcesHandler(ctx context.Context, handler, arg, result interface{}) error {
	st := arg.(*streaming.Args).Stream
	stream := &aggregatedDiscoveryServiceStreamAggregatedResourcesServer{st}
	return handler.(v3.AggregatedDiscoveryService).StreamAggregatedResources(stream)
}

type aggregatedDiscoveryServiceStreamAggregatedResourcesClient struct {
	streaming.Stream
}

func (x *aggregatedDiscoveryServiceStreamAggregatedResourcesClient) Send(m *v3discovery.DiscoveryRequest) error {
	return x.Stream.SendMsg(m)
}

func (x *aggregatedDiscoveryServiceStreamAggregatedResourcesClient) Recv() (*v3discovery.DiscoveryResponse, error) {
	m := new(v3discovery.DiscoveryResponse)
	return m, x.Stream.RecvMsg(m)
}

type aggregatedDiscoveryServiceStreamAggregatedResourcesServer struct {
	streaming.Stream
}

func (x *aggregatedDiscoveryServiceStreamAggregatedResourcesServer) Send(m *v3discovery.DiscoveryResponse) error {
	return x.Stream.SendMsg(m)
}

func (x *aggregatedDiscoveryServiceStreamAggregatedResourcesServer) Recv() (*v3discovery.DiscoveryRequest, error) {
	m := new(v3discovery.DiscoveryRequest)
	return m, x.Stream.RecvMsg(m)
}

func newStreamAggregatedResourcesArgs() interface{} {
	return &StreamAggregatedResourcesArgs{}
}

func newStreamAggregatedResourcesResult() interface{} {
	return &StreamAggregatedResourcesResult{}
}

type StreamAggregatedResourcesArgs struct {
	Req *v3discovery.DiscoveryRequest
}

func (p *StreamAggregatedResourcesArgs) Marshal(out []byte) ([]byte, error) {
	if !p.IsSetReq() {
		return out, fmt.Errorf("no req in StreamAggregatedResourcesArgs")
	}
	return proto.Marshal(p.Req)
}

func (p *StreamAggregatedResourcesArgs) Unmarshal(in []byte) error {
	msg := new(v3discovery.DiscoveryRequest)
	if err := proto.Unmarshal(in, msg); err != nil {
		return err
	}
	p.Req = msg
	return nil
}

var StreamAggregatedResourcesArgs_Req_DEFAULT *v3discovery.DiscoveryRequest

func (p *StreamAggregatedResourcesArgs) GetReq() *v3discovery.DiscoveryRequest {
	if !p.IsSetReq() {
		return StreamAggregatedResourcesArgs_Req_DEFAULT
	}
	return p.Req
}

func (p *StreamAggregatedResourcesArgs) IsSetReq() bool {
	return p.Req != nil
}

type StreamAggregatedResourcesResult struct {
	Success *v3discovery.DiscoveryResponse
}

var StreamAggregatedResourcesResult_Success_DEFAULT *v3discovery.DiscoveryResponse

func (p *StreamAggregatedResourcesResult) Marshal(out []byte) ([]byte, error) {
	if !p.IsSetSuccess() {
		return out, fmt.Errorf("no req in StreamAggregatedResourcesResult")
	}
	return proto.Marshal(p.Success)
}

func (p *StreamAggregatedResourcesResult) Unmarshal(in []byte) error {
	msg := new(v3discovery.DiscoveryResponse)
	if err := proto.Unmarshal(in, msg); err != nil {
		return err
	}
	p.Success = msg
	return nil
}

func (p *StreamAggregatedResourcesResult) GetSuccess() *v3discovery.DiscoveryResponse {
	if !p.IsSetSuccess() {
		return StreamAggregatedResourcesResult_Success_DEFAULT
	}
	return p.Success
}

func (p *StreamAggregatedResourcesResult) SetSuccess(x interface{}) {
	p.Success = x.(*v3discovery.DiscoveryResponse)
}

func (p *StreamAggregatedResourcesResult) IsSetSuccess() bool {
	return p.Success != nil
}

func deltaAggregatedResourcesHandler(ctx context.Context, handler, arg, result interface{}) error {
	st := arg.(*streaming.Args).Stream
	stream := &aggregatedDiscoveryServiceDeltaAggregatedResourcesServer{st}
	return handler.(v3.AggregatedDiscoveryService).DeltaAggregatedResources(stream)
}

type aggregatedDiscoveryServiceDeltaAggregatedResourcesClient struct {
	streaming.Stream
}

func (x *aggregatedDiscoveryServiceDeltaAggregatedResourcesClient) Send(m *v3discovery.DeltaDiscoveryRequest) error {
	return x.Stream.SendMsg(m)
}

func (x *aggregatedDiscoveryServiceDeltaAggregatedResourcesClient) Recv() (*v3discovery.DeltaDiscoveryResponse, error) {
	m := new(v3discovery.DeltaDiscoveryResponse)
	return m, x.Stream.RecvMsg(m)
}

type aggregatedDiscoveryServiceDeltaAggregatedResourcesServer struct {
	streaming.Stream
}

func (x *aggregatedDiscoveryServiceDeltaAggregatedResourcesServer) Send(m *v3discovery.DeltaDiscoveryResponse) error {
	return x.Stream.SendMsg(m)
}

func (x *aggregatedDiscoveryServiceDeltaAggregatedResourcesServer) Recv() (*v3discovery.DeltaDiscoveryRequest, error) {
	m := new(v3discovery.DeltaDiscoveryRequest)
	return m, x.Stream.RecvMsg(m)
}

func newDeltaAggregatedResourcesArgs() interface{} {
	return &DeltaAggregatedResourcesArgs{}
}

func newDeltaAggregatedResourcesResult() interface{} {
	return &DeltaAggregatedResourcesResult{}
}

type DeltaAggregatedResourcesArgs struct {
	Req *v3discovery.DeltaDiscoveryRequest
}

func (p *DeltaAggregatedResourcesArgs) Marshal(out []byte) ([]byte, error) {
	if !p.IsSetReq() {
		return out, fmt.Errorf("no req in DeltaAggregatedResourcesArgs")
	}
	return proto.Marshal(p.Req)
}

func (p *DeltaAggregatedResourcesArgs) Unmarshal(in []byte) error {
	msg := new(v3discovery.DeltaDiscoveryRequest)
	if err := proto.Unmarshal(in, msg); err != nil {
		return err
	}
	p.Req = msg
	return nil
}

var DeltaAggregatedResourcesArgs_Req_DEFAULT *v3discovery.DeltaDiscoveryRequest

func (p *DeltaAggregatedResourcesArgs) GetReq() *v3discovery.DeltaDiscoveryRequest {
	if !p.IsSetReq() {
		return DeltaAggregatedResourcesArgs_Req_DEFAULT
	}
	return p.Req
}

func (p *DeltaAggregatedResourcesArgs) IsSetReq() bool {
	return p.Req != nil
}

type DeltaAggregatedResourcesResult struct {
	Success *v3discovery.DeltaDiscoveryResponse
}

var DeltaAggregatedResourcesResult_Success_DEFAULT *v3discovery.DeltaDiscoveryResponse

func (p *DeltaAggregatedResourcesResult) Marshal(out []byte) ([]byte, error) {
	if !p.IsSetSuccess() {
		return out, fmt.Errorf("no req in DeltaAggregatedResourcesResult")
	}
	return proto.Marshal(p.Success)
}

func (p *DeltaAggregatedResourcesResult) Unmarshal(in []byte) error {
	msg := new(v3discovery.DeltaDiscoveryResponse)
	if err := proto.Unmarshal(in, msg); err != nil {
		return err
	}
	p.Success = msg
	return nil
}

func (p *DeltaAggregatedResourcesResult) GetSuccess() *v3discovery.DeltaDiscoveryResponse {
	if !p.IsSetSuccess() {
		return DeltaAggregatedResourcesResult_Success_DEFAULT
	}
	return p.Success
}

func (p *DeltaAggregatedResourcesResult) SetSuccess(x interface{}) {
	p.Success = x.(*v3discovery.DeltaDiscoveryResponse)
}

func (p *DeltaAggregatedResourcesResult) IsSetSuccess() bool {
	return p.Success != nil
}

type kClient struct {
	c client.Client
}

func newServiceClient(c client.Client) *kClient {
	return &kClient{
		c: c,
	}
}

func (p *kClient) StreamAggregatedResources(ctx context.Context) (AggregatedDiscoveryService_StreamAggregatedResourcesClient, error) {
	streamClient, ok := p.c.(client.Streaming)
	if !ok {
		return nil, fmt.Errorf("client not support streaming")
	}
	res := new(streaming.Result)
	err := streamClient.Stream(ctx, "StreamAggregatedResources", nil, res)
	if err != nil {
		return nil, err
	}
	stream := &aggregatedDiscoveryServiceStreamAggregatedResourcesClient{res.Stream}
	return stream, nil
}

func (p *kClient) DeltaAggregatedResources(ctx context.Context) (AggregatedDiscoveryService_DeltaAggregatedResourcesClient, error) {
	streamClient, ok := p.c.(client.Streaming)
	if !ok {
		return nil, fmt.Errorf("client not support streaming")
	}
	res := new(streaming.Result)
	err := streamClient.Stream(ctx, "DeltaAggregatedResources", nil, res)
	if err != nil {
		return nil, err
	}
	stream := &aggregatedDiscoveryServiceDeltaAggregatedResourcesClient{res.Stream}
	return stream, nil
}
