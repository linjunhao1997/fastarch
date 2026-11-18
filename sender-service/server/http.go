package server

import (
	"context"
	"fmt"

	"github.com/linjunhao1997/contrib/pkg/server/httpserver"
	sender_v1 "github.com/linjunhao1997/idl/pb/sender"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"sender-service/conf"
)

// RunHttpServer 组装 grpc-gateway 反向代理；业务服务由 grpc 子命令提供，需先启动。
func RunHttpServer(ctx context.Context, cfg *conf.Config) error {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
	}
	mux := httpserver.NewMux()

	if err := sender_v1.RegisterAuthenticationServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register authentication service handler: %w", err)
	}
	if err := sender_v1.RegisterMessageServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register message service handler: %w", err)
	}
	if err := sender_v1.RegisterTemplateServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register template service handler: %w", err)
	}
	if err := sender_v1.RegisterMenuServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register menu service handler: %w", err)
	}
	if err := sender_v1.RegisterDictServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register dict service handler: %w", err)
	}

	return httpserver.Run(ctx, cfg.Http.AddrWithPort(), mux)
}
