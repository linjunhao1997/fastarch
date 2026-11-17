package cmd

import (
	"github.com/linjunhao1997/contrib/pkg/app"

	"sender-service/conf"
	"sender-service/server"
)

// Execute 是服务入口，组装通用启动引导与 sender-service 子命令。
func Execute() {
	a := app.New[conf.Config]("sender-service",
		app.WithConfigDefaults(map[string]any{
			"grpc.addr":            "127.0.0.1",
			"grpc.port":            9091,
			"http.addr":            "127.0.0.1",
			"http.port":            8081,
			"sender.application_id": 2,
		}),
	)
	a.AddCommand("grpc", "启动gRPC服务器", server.RunGrpcServer)
	a.AddCommand("http", "启动HTTP服务器", server.RunHttpServer)
	a.Execute()
}
