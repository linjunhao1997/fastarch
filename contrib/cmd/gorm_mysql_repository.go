package cmd

import (
	"fmt"
	"github.com/linjunhao1997/contrib/pkg/gen"
	"github.com/spf13/cobra"
	"log"
)

var (
	interfaceFilePath string
	modelFilePath     string
)

var genGormMysqlRepositoryCmd = &cobra.Command{
	Use:   "gen_gorm_mysql_repository",
	Short: "生成gorm的repository",
	Long:  "生成gorm的repository",
	Run: func(cmd *cobra.Command, args []string) {
		// 启动HTTP服务器的逻辑
		fmt.Printf("repositry接口文件路径:%s,模型文件路径 %s\n", interfaceFilePath, modelFilePath)
		if err := gen.GenerateGormMysqlRepository(interfaceFilePath, modelFilePath); err != nil {
			log.Fatal(err)
		}
	},
}
