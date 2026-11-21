package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linjunhao1997/contrib/pkg/server/httpserver"
	home_v1 "github.com/linjunhao1997/idl/pb/home"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"home-service/conf"
)

// RequireLogin 在网关层区分「未登录」与「无权限」：缺少 Bearer/Authorization Cookie
// 时直接返回 401，避免把请求透传给 IAM 后得到 400。已发布首页是公开读接口，
// 无需登录即可访问。携带凭证的请求继续由 gRPC 判权拦截器委托 IAM 校验，
// 权限不足返回 403。
func RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || isPublicRead(r) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") == "" {
			if c, err := r.Cookie("authorization"); err != nil || c.Value == "" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprint(w, `{"code":401,"message":"未登录或登录已过期"}`)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isPublicRead 是官网公开页面需要读取的接口白名单。
func isPublicRead(r *http.Request) bool {
	return r.Method == http.MethodGet && r.URL.Path == "/home/homepage/published"
}

// RunHttpServer 组装 grpc-gateway 反向代理；业务服务由 grpc 子命令提供，需先启动。
func RunHttpServer(ctx context.Context, cfg *conf.Config) error {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
	}
	mux := httpserver.NewMux()

	// 统一登录：home 不暴露登录端点，JWT 由 IAM 签发、入站拦截层透传判权。
	if err := home_v1.RegisterHomepageServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register homepage service handler: %w", err)
	}
	if err := home_v1.RegisterMenuServiceHandlerFromEndpoint(ctx, mux, cfg.Http.Endpoint, dialOptions); err != nil {
		return fmt.Errorf("register menu service handler: %w", err)
	}

	return httpserver.Run(ctx, cfg.Http.AddrWithPort(), RequireLogin(mux))
}
