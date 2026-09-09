package gwlib

import (
	"context"
	"google.golang.org/grpc/metadata"
	"net/http"
)

const (
	HeaderCookieKeyAuthorization = "Authorization"
	// HeaderGatewayOrigin 标记请求经 grpc-gateway 转发（网关 → gRPC 方向），
	// 鉴权拦截器据此区分 HTTP 接口域与服务间 gRPC 域（specs/005-typed-permission-model D3）。
	HeaderGatewayOrigin = "x-gateway-origin"
	// GatewayOriginHTTP 是 HeaderGatewayOrigin 的取值：来源为 HTTP 网关。
	GatewayOriginHTTP = "http"
)

func WithMetadata(ctx context.Context, req *http.Request) metadata.MD {
	md := metadata.New(map[string]string{})
	pair, _ := req.Cookie(HeaderCookieKeyAuthorization)
	if pair != nil {
		md.Append(HeaderCookieKeyAuthorization, pair.Value)
	}
	md.Set(HeaderGatewayOrigin, GatewayOriginHTTP)
	return md
}
