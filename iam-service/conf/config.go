package conf

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	GRpc     GRpcConfig     `mapstructure:"grpc"`
	Http     HttpConfig     `mapstructure:"http"`
	Jwt      JwtConfig      `mapstructure:"jwt"`
	App      AppConfig      `mapstructure:"app"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr string `mapstructure:"addr"`
}

type GRpcConfig struct {
	Addr string `mapstructure:"addr"`
	Port int    `mapstructure:"port"`
}

type HttpConfig struct {
	Addr     string `mapstructure:"addr"`
	Port     int    `mapstructure:"port"`
	Endpoint string `mapstructure:"endpoint"`
}

type JwtConfig struct {
	Secret          string `mapstructure:"secret"`
	ExpirationHours int    `mapstructure:"expiration_hours"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Mode string `mapstructure:"mode"`
}

var cfg *Config

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	viper.SetDefault("grpc.addr", "127.0.0.1")
	viper.SetDefault("grpc.port", 9090)
	viper.SetDefault("http.addr", "127.0.0.1")
	viper.SetDefault("http.port", 8080)
	viper.SetDefault("jwt.expiration_hours", 24)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

func Get() *Config {
	return cfg
}

func (c *DatabaseConfig) GetDSN() string {
	return c.DSN
}

func (c *GRpcConfig) AddrWithPort() string {
	return fmt.Sprintf("%s:%d", c.Addr, c.Port)
}

func (c *HttpConfig) AddrWithPort() string {
	return fmt.Sprintf("%s:%d", c.Addr, c.Port)
}
