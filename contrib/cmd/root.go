package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "contrib",
	Short: "contrib",
	Long:  ``,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {

	genGormMysqlRepositoryCmd.Flags().StringVarP(&interfaceFilePath, "interface_file_path", "i", "", "repository接口文件路径")
	genGormMysqlRepositoryCmd.Flags().StringVarP(&modelFilePath, "model_file_path", "m", "", "模型文件路径")

	rootCmd.AddCommand(genGormMysqlRepositoryCmd)
}
