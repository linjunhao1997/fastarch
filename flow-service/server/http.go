package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linjunhao1997/contrib/pkg/server/httpserver"
	flow_v1 "github.com/linjunhao1997/idl/pb/flow"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"flow-service/conf"
	"flow-service/pkg/mcpserver"
)

// RunHttpServer 组装 grpc-gateway 反向代理；业务服务由 grpc 子命令提供，需先启动。
func RunHttpServer(ctx context.Context, cfg *conf.Config) error {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
	}
	mux := httpserver.NewMux()

	if err := flow_v1.RegisterMemberServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register member service handler: %w", err)
	}
	if err := flow_v1.RegisterTaskServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register task service handler: %w", err)
	}
	if err := flow_v1.RegisterQuestionServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register question service handler: %w", err)
	}
	if err := flow_v1.RegisterLogServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register log service handler: %w", err)
	}
	if err := flow_v1.RegisterMenuServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register menu service handler: %w", err)
	}
	if err := flow_v1.RegisterDictServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register dict service handler: %w", err)
	}

	// flow 自己的 MCP 接入点：与 grpc-gateway 同端口，挂在 /flow/mcp 上。
	// 身份由每个请求自带的个人 token 决定，服务端不保存任何人的 token。
	mcpHandler, err := mcpserver.HTTPHandler(cfg)
	if err != nil {
		return err
	}
	root := http.NewServeMux()
	root.Handle("/flow/mcp", mcpHandler)
	root.Handle("/", mux)

	return httpserver.Run(ctx, cfg.Http.AddrWithPort(), root)
}
