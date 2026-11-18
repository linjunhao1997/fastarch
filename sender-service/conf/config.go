package conf

import (
	"fmt"

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
	// IamAddr 是 IAM gRPC 地址，供入站判权薄客户端透传 JWT 调 CheckPermission。
	IamAddr string `mapstructure:"iam_addr"`
	// IAMAccessKey / IAMSecretKey 是代登录用的机机账号凭证。
	// IAM 验签后从该账号所属应用推导目标应用，调用方不再自报应用编码。
	IAMAccessKey string `mapstructure:"iam_access_key"`
	IAMSecretKey string `mapstructure:"iam_secret_key"`
}

// ResolveIAMServiceAuth 校验并返回配置中的机机账号凭证，缺失则 fail-fast。
func (s *Sender) ResolveIAMServiceAuth() (accessKey, secretKey string, err error) {
	if s.IAMAccessKey == "" || s.IAMSecretKey == "" {
		return "", "", fmt.Errorf("sender.iam_access_key and sender.iam_secret_key are required for login delegation")
	}
	return s.IAMAccessKey, s.IAMSecretKey, nil
}
