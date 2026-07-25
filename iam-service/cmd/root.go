package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"iam-service/conf"
)

var rootCmd = &cobra.Command{
	Use:   "iam-service",
	Short: "iam-service",
	Long:  ``,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	cfg, err := conf.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %s\n", err)
		os.Exit(1)
	}
	grpcCfg = cfg
	httpCfg = cfg
}