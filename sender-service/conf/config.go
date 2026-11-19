package conf

import (
	"github.com/linjunhao1997/contrib/pkg/config"
)

// Config 是 sender-service 的配置结构，通用段复用 contrib/pkg/config。
// Sender 特有段：固定应用 id、IAM 判权地址。
type Config struct {
	App      config.App      `mapstructure:"app"`
	Database config.Database `mapstructure:"database"`
	Redis    config.Redis    `mapstructure:"redis"`
	GRpc     config.GRpc     `mapstructure:"grpc"`
	Http     config.Http     `mapstructure:"http"`
	Sender   Sender          `mapstructure:"sender"`
}

// Sender 宿主配置。
type Sender struct {
	// ApplicationId 是 sender 应用在 IAM 注册表中的固定 id（迁移 000010 为 2）。
	ApplicationId int64 `mapstructure:"application_id"`
	// ApplicationCode / ApplicationName 是 sender 应用的展示标识（与 IAM 种子 000010 一致），
	// 用于消息模板/发送记录响应回填"归属应用"摘要，避免跨服务查询。
	ApplicationCode string `mapstructure:"application_code"`
	ApplicationName string `mapstructure:"application_name"`
	// IamAddr 是 IAM gRPC 地址，供入站判权薄客户端透传 JWT 调 CheckPermission。
	// 统一登录后 sender 不再代登录，无需 IAM 服务账号 AK/SK。
	IamAddr string `mapstructure:"iam_addr"`
}
