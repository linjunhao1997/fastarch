package conf

import "github.com/linjunhao1997/contrib/pkg/config"

// Config 是 sender-service 的配置结构，通用段复用 contrib/pkg/config。
// Sender 特有段：固定应用 id、IAM 判权地址。
type Config struct {
	App        config.App      `mapstructure:"app"`
	Database   config.Database `mapstructure:"database"`
	Redis      config.Redis    `mapstructure:"redis"`
	GRpc       config.GRpc     `mapstructure:"grpc"`
	Http       config.Http     `mapstructure:"http"`
	Sender     Sender          `mapstructure:"sender"`
}

// Sender 宿主配置。
type Sender struct {
	// ApplicationId 是 sender 应用在 IAM 注册表中的固定 id（迁移 000010 为 2）。
	ApplicationId int64 `mapstructure:"application_id"`
	// IamAddr 是 IAM gRPC 地址，供入站判权薄客户端透传 JWT 调 CheckPermission。
	IamAddr string `mapstructure:"iam_addr"`
}
