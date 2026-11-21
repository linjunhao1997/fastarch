package cmd

import (
	"github.com/linjunhao1997/contrib/pkg/app"

	"home-service/conf"
	"home-service/server"
)

// Execute 是服务入口，组装通用启动引导与 home-service 子命令。
func Execute() {
	a := app.New[conf.Config]("home-service",
		app.WithConfigDefaults(map[string]any{
			"grpc.addr":             "127.0.0.1",
			"grpc.port":             9093,
			"http.addr":             "127.0.0.1",
			"http.port":             8083,
			"home.application_id":   4,
			"home.application_code": "home",
			"home.application_name": "企业官网",
		}),
	)
	a.AddCommand("grpc", "启动gRPC服务器", server.RunGrpcServer)
	a.AddCommand("http", "启动HTTP服务器", server.RunHttpServer)
	a.Execute()
}
