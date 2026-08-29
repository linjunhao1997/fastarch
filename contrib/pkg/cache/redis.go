// Package cache 提供服务通用的缓存客户端初始化助手。
package cache

import "github.com/redis/go-redis/v9"

// NewRedis 创建 Redis 客户端（懒连接，首次命令时才真正建连）。
func NewRedis(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}
