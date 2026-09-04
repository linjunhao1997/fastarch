package server

import (
	"context"
	"fmt"

	"github.com/linjunhao1997/contrib/pkg/cache"
	"github.com/linjunhao1997/contrib/pkg/database"
	"github.com/linjunhao1997/contrib/pkg/server/grpcserver"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	"iam-service/conf"
	"iam-service/pkg/auth"
	"iam-service/pkg/model"
	"iam-service/pkg/service"
)

// RunGrpcServer 组装 iam-service 的 gRPC 服务：资源初始化、服务注册后阻塞运行，
// ctx 取消时优雅退出。
func RunGrpcServer(ctx context.Context, cfg *conf.Config) error {
	rdb := cache.NewRedis(cfg.Redis.Addr)

	db, err := database.NewMySQL(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("connect mysql: %w", err)
	}
	if err := db.AutoMigrate(model.Models...); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	srv, err := grpcserver.New(cfg.GRpc.AddrWithPort(),
		grpcserver.WithDefaultInterceptors(),
		grpcserver.WithErrorDomain(cfg.App.Name),
		grpcserver.WithGormErrorMapping(),
		grpcserver.WithServerOptions(
			// 权限快照读穿回源函数注入鉴权拦截器（research.md D6-②）
			grpc.ChainUnaryInterceptor(auth.Interceptor(rdb, service.NewPermissionSnapshotLoader(db))),
			grpc.StatsHandler(otelgrpc.NewClientHandler()),
		),
	)
	if err != nil {
		return fmt.Errorf("init grpc server: %w", err)
	}

	s := srv.Grpc()
	iam_v1.RegisterAuthenticationServiceServer(s, service.NewAuthenticationService(db, rdb))
	iam_v1.RegisterAccountServiceServer(s, service.NewAccountService(db, rdb))
	iam_v1.RegisterAuthorizationServiceServer(s, service.NewAuthorizationService(db, rdb))
	iam_v1.RegisterApplicationServiceServer(s, service.NewApplicationService(db))

	return srv.Run(ctx)
}
