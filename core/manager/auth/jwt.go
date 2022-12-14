package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

// ClientHTTP2JwtHandler is used to set jwt token to http2 header
var ClientHTTP2JwtHandler = &clientHTTP2JwtHandler{}

type clientHTTP2JwtHandler struct{}

var (
	_ remote.MetaHandler          = ClientHTTP2JwtHandler
	_ remote.StreamingMetaHandler = ClientHTTP2JwtHandler
)

const (
	jwtTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	jwtTokenKey  = "Authorization"
)

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

func setJWTToken(md metadata.MD) error {
	jwtToken, err := getJWTToken()
	if err != nil {
		return fmt.Errorf("getJWTToken error: %v\n", err)
	}
	md.Set(jwtTokenKey, jwtTokenValueFmt(jwtToken))
	return nil
}

func (*clientHTTP2JwtHandler) OnConnectStream(ctx context.Context) (context.Context, error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	if !isGRPC(ri) {
		return ctx, nil
	}
	var md metadata.MD
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}
	err := setJWTToken(md)
	if err != nil {
		return nil, err
	}
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
