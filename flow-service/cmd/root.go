package cmd

import (
	"github.com/linjunhao1997/contrib/pkg/app"

	"flow-service/conf"
	"flow-service/pkg/mcpserver"
	"flow-service/server"
)

// Execute 是服务入口，组装通用启动引导与 flow-service 子命令。
func Execute() {
	a := app.New[conf.Config]("flow-service",
		app.WithConfigDefaults(map[string]any{
			"grpc.addr":                "127.0.0.1",
			"grpc.port":                9092,
			"http.addr":                "127.0.0.1",
			"http.port":                8082,
			"flow.application_id":      3,
			"flow.application_code":    "flow",
			"flow.application_name":    "任务工作台",
			"flow.bootstrap_all_roles": true,
			"flow.default_role_code":   "dev",
		}),
	)
	a.AddCommand("grpc", "启动gRPC服务器", server.RunGrpcServer)
	a.AddCommand("http", "启动HTTP服务器", server.RunHttpServer)
	a.AddCommand("mcp", "启动 MCP stdio 服务（给 agent 用）", mcpserver.Run)
	a.Execute()
}
