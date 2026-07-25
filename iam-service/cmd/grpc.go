package cmd

import (
	"github.com/spf13/cobra"
	"iam-service/conf"
	"iam-service/server"
)

var (
	configPath string
	grpcCfg    *conf.Config
)

var grpcCmd = &cobra.Command{
	Use:   "grpc",
	Short: "启动gRPC服务器",
	Long:  "启动gRPC服务器，监听指定的地址和端口",
	Run: func(cmd *cobra.Command, args []string) {
		server.RunGrpcServer(grpcCfg)
	},
}

func init() {
	rootCmd.AddCommand(grpcCmd)
	grpcCmd.Flags().StringVarP(&configPath, "config", "c", "conf/config.yaml", "配置文件路径")
}