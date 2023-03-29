/*
 * Copyright 2023 CloudWeGo Authors
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

package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

const (
	jwtTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token" // token path in the pod.

	jwtTokenKey = "Authorization" // Istiod gets the jwt from meta using this key.

	// If Istio is deployed in a different cluster, set this env as the cluster id of this service.
	// Usually, we can get the value using "kubectl config get-clusters".
	clusterIDEnvKey = "ISTIO_META_CLUSTER_ID"

	clusterIDMetadataKey = "clusterid" // Istiod retrieves clusterid and use it for auth of JWT.
)

var (
	clusterID string
	jwtToken  string
)

func init() {
	// init clusterID
	clusterID = os.Getenv(clusterIDEnvKey)
	// watch jwtToken file
	if token, err := getJWTToken(); err != nil {
		klog.Warnf("[XDS] Auth, getJWTToken error=%s. Ignore this log if not deploying on multiple clusters.\n", err.Error())
	} else {
		jwtToken = token
	}
}

// ClientHTTP2JwtHandler is used to set jwt token to http2 header
var ClientHTTP2JwtHandler = &clientHTTP2JwtHandler{}

type clientHTTP2JwtHandler struct{}

var (
	_ remote.MetaHandler          = ClientHTTP2JwtHandler
	_ remote.StreamingMetaHandler = ClientHTTP2JwtHandler
)

func (*clientHTTP2JwtHandler) OnConnectStream(ctx context.Context) (context.Context, error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	if !isGRPC(ri) {
		return ctx, nil
	}
	var md metadata.MD
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}
	// Set JWT and clusterID for auth
	md.Set(jwtTokenKey, jwtTokenValueFmt(jwtToken))
	md.Set(clusterIDMetadataKey, clusterID)
	return metadata.NewOutgoingContext(ctx, md), nil
}

func (*clientHTTP2JwtHandler) OnReadStream(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (ch *clientHTTP2JwtHandler) WriteMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	return ctx, nil
}

func (ch *clientHTTP2JwtHandler) ReadMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	return ctx, nil
}

func isGRPC(ri rpcinfo.RPCInfo) bool {
	return ri.Config().TransportProtocol()&transport.GRPC == transport.GRPC
}

var jwtTokenValueFmt = func(jwtToken string) string {
	return fmt.Sprintf("Bearer %s", jwtToken)
}

func getJWTToken() (string, error) {
	saToken := jwtTokenPath

	token, err := os.ReadFile(saToken)
	if err != nil {
		return "", err
	}

	return string(token), nil
}
