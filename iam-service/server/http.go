package server

import (
	"context"
	"fmt"

	"github.com/linjunhao1997/contrib/pkg/server/httpserver"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"iam-service/conf"
)

// RunHttpServer 组装 grpc-gateway 反向代理并阻塞运行，ctx 取消时优雅退出。
// 注意：gateway 只负责协议转换，业务服务由 grpc 子命令提供，需先启动 gRPC 服务。
func RunHttpServer(ctx context.Context, cfg *conf.Config) error {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
	}

	mux := httpserver.NewMux()

	if err := iam_v1.RegisterAccountServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register account service handler: %w", err)
	}
	if err := iam_v1.RegisterAuthenticationServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register authentication service handler: %w", err)
	}
	if err := iam_v1.RegisterAuthorizationServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register authorization service handler: %w", err)
	}
	if err := iam_v1.RegisterApplicationServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register application service handler: %w", err)
	}

	return httpserver.Run(ctx, cfg.Http.AddrWithPort(), mux)
}
