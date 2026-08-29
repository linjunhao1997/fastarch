package cmd

import (
	"github.com/linjunhao1997/contrib/pkg/app"

	"iam-service/conf"
	"iam-service/server"
)

// Execute 是服务入口，组装通用启动引导与 iam-service 自己的子命令。
func Execute() {
	a := app.New[conf.Config]("iam-service",
		app.WithConfigDefaults(map[string]any{
			"grpc.addr":            "127.0.0.1",
			"grpc.port":            9090,
			"http.addr":            "127.0.0.1",
			"http.port":            8080,
			"jwt.expiration_hours": 24,
		}),
	)
	a.AddCommand("grpc", "启动gRPC服务器", server.RunGrpcServer)
	a.AddCommand("http", "启动HTTP服务器", server.RunHttpServer)
	a.Execute()
}
