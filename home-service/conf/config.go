package conf

import (
	"github.com/linjunhao1997/contrib/pkg/config"
)

// Config 是 home-service 的配置结构，通用段复用 contrib/pkg/config。
// Home 特有段：固定应用 id、IAM 判权地址。
type Config struct {
	App      config.App      `mapstructure:"app"`
	Database config.Database `mapstructure:"database"`
	Redis    config.Redis    `mapstructure:"redis"`
	GRpc     config.GRpc     `mapstructure:"grpc"`
	Http     config.Http     `mapstructure:"http"`
	Home     Home            `mapstructure:"home"`
}

// Home 宿主配置。
type Home struct {
	// ApplicationId 是 home 应用在 IAM 注册表中的固定 id（迁移 000021 为 4）。
	ApplicationId int64 `mapstructure:"application_id"`
	// ApplicationCode / ApplicationName 是 home 应用的展示标识，用于响应回填归属应用摘要。
	ApplicationCode string `mapstructure:"application_code"`
	ApplicationName string `mapstructure:"application_name"`
	// IamAddr 是 IAM gRPC 地址，供入站判权薄客户端透传 JWT 调 CheckPermission。
	IamAddr string `mapstructure:"iam_addr"`
}
