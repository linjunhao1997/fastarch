package conf

import (
	"fmt"
	"os"

	"github.com/linjunhao1997/contrib/pkg/config"
)

// Config 是 iam-service 的配置结构，通用配置段复用 contrib/pkg/config，
// 服务特有配置段按需在此追加。
type Config struct {
	App        config.App      `mapstructure:"app"`
	Database   config.Database `mapstructure:"database"`
	Redis      config.Redis    `mapstructure:"redis"`
	GRpc       config.GRpc     `mapstructure:"grpc"`
	Http       config.Http     `mapstructure:"http"`
	Jwt        config.Jwt      `mapstructure:"jwt"`
	Credential Credential      `mapstructure:"credential"`
}

// Credential 是访问凭证（AK/SK 签名鉴权，specs/009-ak-sk-auth）的配置段。
type Credential struct {
	// TimestampWindowSeconds 是签名时间戳允许偏差窗口（±秒），默认 300。
	TimestampWindowSeconds int `mapstructure:"timestamp_window_seconds"`
	// MaxCredentialsPerAccount 是单账号凭证数量上限，默认 20。
	MaxCredentialsPerAccount int `mapstructure:"max_credentials_per_account"`
	// EncryptKey 是 SK 静态加密密钥（32 字节，hex/base64/原始字节编码），
	// 生产环境必须经环境变量 IAM_CREDENTIAL_ENCRYPT_KEY 注入，禁止写入配置文件入库入仓。
	EncryptKey string `mapstructure:"encrypt_key"`
}

// CredentialEncryptKeyEnv 是 SK 静态加密密钥的环境变量名。
const CredentialEncryptKeyEnv = "IAM_CREDENTIAL_ENCRYPT_KEY"

// ResolveCredentialConfig 补全凭证配置默认值并解析加密密钥：
// EncryptKey 为空时回退读取环境变量；密钥缺失返回错误（fail-fast，禁止明文回退）。
func (c *Config) ResolveCredentialConfig() (timestampWindow int, maxPerAccount int, encryptKey string, err error) {
	timestampWindow = c.Credential.TimestampWindowSeconds
	if timestampWindow <= 0 {
		timestampWindow = 300
	}
	maxPerAccount = c.Credential.MaxCredentialsPerAccount
	if maxPerAccount <= 0 {
		maxPerAccount = 20
	}
	encryptKey = c.Credential.EncryptKey
	if encryptKey == "" {
		encryptKey = os.Getenv(CredentialEncryptKeyEnv)
	}
	if encryptKey == "" {
		return 0, 0, "", fmt.Errorf("credential encrypt key is required: set %s environment variable or credential.encrypt_key config", CredentialEncryptKeyEnv)
	}
	return timestampWindow, maxPerAccount, encryptKey, nil
}
