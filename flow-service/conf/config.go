package conf

import (
	"github.com/linjunhao1997/contrib/pkg/config"
)

// Config 是 flow-service 的配置结构，通用段复用 contrib/pkg/config。
// Flow 特有段：固定应用 id、IAM 判权地址、成员建档策略、MCP token。
type Config struct {
	App      config.App      `mapstructure:"app"`
	Database config.Database `mapstructure:"database"`
	Redis    config.Redis    `mapstructure:"redis"`
	GRpc     config.GRpc     `mapstructure:"grpc"`
	Http     config.Http     `mapstructure:"http"`
	Flow     Flow            `mapstructure:"flow"`
}

// Flow 宿主配置。
type Flow struct {
	// ApplicationId 是 flow 应用在 IAM 注册表中的固定 id（迁移 000017 为 3）。
	ApplicationId int64 `mapstructure:"application_id"`
	// ApplicationCode / ApplicationName 仅用于响应回填归属应用摘要。
	ApplicationCode string `mapstructure:"application_code"`
	ApplicationName string `mapstructure:"application_name"`
	// IamAddr 是 IAM gRPC 地址，供入站判权薄客户端透传 JWT 调 CheckPermission。
	IamAddr string `mapstructure:"iam_addr"`
	// BootstrapAllRoles：首次登录自动建档时是否授予全部工作角色。
	// 单人为验证场景置 true；正式环境应改为管理员分配（false）。
	BootstrapAllRoles bool `mapstructure:"bootstrap_all_roles"`
	// DefaultRoleCode 是首次建档的默认当前角色。
	DefaultRoleCode string `mapstructure:"default_role_code"`
}
