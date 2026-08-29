// Package config 提供服务通用的配置段定义和基于 viper 的泛型配置加载器。
//
// 每个服务定义自己的 Config 结构体，按需嵌入本包提供的通用配置段：
//
//	type Config struct {
//	    App      config.App      `mapstructure:"app"`
//	    GRpc     config.GRpc     `mapstructure:"grpc"`
//	    Http     config.Http     `mapstructure:"http"`
//	    Database config.Database `mapstructure:"database"`
//	    Redis    config.Redis    `mapstructure:"redis"`
//	    // ...服务自己的配置段
//	}
//
// 然后调用 Load[Config] 加载即可。
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// App 服务自身信息。
type App struct {
	Name string `mapstructure:"name"`
	Mode string `mapstructure:"mode"` // debug / release
}

// GRpc gRPC 服务监听配置。
type GRpc struct {
	Addr string `mapstructure:"addr"`
	Port int    `mapstructure:"port"`
}

// AddrWithPort 返回 host:port 形式的监听地址。
func (c GRpc) AddrWithPort() string {
	return fmt.Sprintf("%s:%d", c.Addr, c.Port)
}

// Http HTTP（grpc-gateway）监听与上游配置。
type Http struct {
	Addr     string `mapstructure:"addr"`
	Port     int    `mapstructure:"port"`
	Endpoint string `mapstructure:"endpoint"` // 反向代理的 gRPC 地址
}

// AddrWithPort 返回 host:port 形式的监听地址。
func (c Http) AddrWithPort() string {
	return fmt.Sprintf("%s:%d", c.Addr, c.Port)
}

// Database 数据库连接配置。
type Database struct {
	DSN string `mapstructure:"dsn"`
}

// Redis Redis 连接配置。
type Redis struct {
	Addr string `mapstructure:"addr"`
}

// Jwt JWT 签发配置。
type Jwt struct {
	Secret          string `mapstructure:"secret"`
	ExpirationHours int    `mapstructure:"expiration_hours"`
}

type options struct {
	defaults    map[string]any
	searchPaths []string
}

// Option 配置加载选项。
type Option func(*options)

// WithDefaults 设置 viper 默认值（yaml 中缺省时生效），key 为点分路径，如 "grpc.port"。
func WithDefaults(defaults map[string]any) Option {
	return func(o *options) {
		o.defaults = defaults
	}
}

// WithSearchPaths 增加额外的配置文件兜底查找路径。
func WithSearchPaths(paths ...string) Option {
	return func(o *options) {
		o.searchPaths = append(o.searchPaths, paths...)
	}
}

// Load 从 path 读取 yaml 配置并反序列化到 T。
//
// 路径解析按以下顺序取第一个存在的文件（都不存在时仍使用 path，由 viper 报错）：
//  1. path：-c 显式传入，或工作目录恰好在服务目录时的 conf/config.yaml
//  2. <serviceName>/conf/config.yaml：在 monorepo 根目录启动（如 IDE 默认工作目录）
//  3. WithSearchPaths 指定的额外路径
//  4. 可执行文件同目录的 conf/config.yaml：部署时二进制旁放置配置
func Load[T any](serviceName, path string, opts ...Option) (*T, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(resolvePath(serviceName, path, o.searchPaths))
	for key, val := range o.defaults {
		v.SetDefault(key, val)
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := new(T)
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return cfg, nil
}

func resolvePath(serviceName, path string, extra []string) string {
	candidates := []string{path, filepath.Join(serviceName, "conf", "config.yaml")}
	candidates = append(candidates, extra...)
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "conf", "config.yaml"))
	}
	for _, p := range candidates {
		if fileExists(p) {
			return p
		}
	}
	return path
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
