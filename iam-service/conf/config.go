package conf

import "github.com/linjunhao1997/contrib/pkg/config"

// Config 是 iam-service 的配置结构，通用配置段复用 contrib/pkg/config，
// 服务特有配置段按需在此追加。
type Config struct {
	App      config.App      `mapstructure:"app"`
	Database config.Database `mapstructure:"database"`
	Redis    config.Redis    `mapstructure:"redis"`
	GRpc     config.GRpc     `mapstructure:"grpc"`
	Http     config.Http     `mapstructure:"http"`
	Jwt      config.Jwt      `mapstructure:"jwt"`
}
