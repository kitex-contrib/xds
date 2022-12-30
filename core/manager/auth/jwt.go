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
	jwtTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	jwtTokenKey  = "Authorization"

	clusterIDMetadataKey = "clusterid"
	clusterIDEnvKey      = "ISTIO_META_CLUSTER_ID"
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
		klog.Errorf("[XDS] Auth, getJWTToken error=%s\n", err.Error())
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
