package cmd

import (
	"github.com/spf13/cobra"
	"iam-service/conf"
	"iam-service/server"
)

var httpCfg *conf.Config

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "启动HTTP服务器",
	Long:  "启动HTTP服务器，监听指定的地址和端口",
	Run: func(cmd *cobra.Command, args []string) {
		server.RunHttpServer(httpCfg)
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)
	httpCmd.Flags().StringVarP(&configPath, "config", "c", "conf/config.yaml", "配置文件路径")
}