package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/linjunhao1997/contrib/pkg/authz"
	"github.com/linjunhao1997/contrib/pkg/database"
	"github.com/linjunhao1997/contrib/pkg/server/grpcserver"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	home_v1 "github.com/linjunhao1997/idl/pb/home"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"home-service/conf"
	"home-service/pkg/migration"
	"home-service/pkg/service"
)

const errorDomain = "home-service"

// RunGrpcServer 组装 home-service 的 gRPC 服务：迁移、DB、判权客户端、
// 拦截器链、服务注册后阻塞运行，ctx 取消时优雅退出。
func RunGrpcServer(ctx context.Context, cfg *conf.Config) error {
	// 表创建与种子由内嵌迁移承担，失败 fail-fast。
	if err := migration.Run(cfg.Database.DSN); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	db, err := database.NewMySQL(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}

	appId := cfg.Home.ApplicationId
	if appId <= 0 {
		return fmt.Errorf("home.application_id must be positive")
	}
	if cfg.Home.IamAddr == "" {
		return fmt.Errorf("home.iam_addr is required for inbound authorization")
	}
	// 归属应用摘要（宿主固定值，来自配置；与 IAM 种子 000021 一致）。
	appSummary := &common_v1.ApplicationSummary{
		Id:     cast.ToString(appId),
		Code:   cfg.Home.ApplicationCode,
		Name:   cfg.Home.ApplicationName,
		Status: "enabled",
	}

	// 入站判权薄客户端：透传 JWT 调 IAM CheckPermission（ADR 0005）。
	authzClient, err := authz.New(cfg.Home.IamAddr)
	if err != nil {
		return fmt.Errorf("init authz client: %w", err)
	}
	defer func() { _ = authzClient.Close() }()

	authInterceptor := authz.Interceptor(authzClient, authz.Config{
		ApplicationId:   appId,
		AuthOnlyMethods: service.AuthOnlyMethods(),
		MethodPerms:     service.MethodPermsFor,
		ErrorDomain:     errorDomain,
	})

	srv, err := grpcserver.New(cfg.GRpc.AddrWithPort(),
		grpcserver.WithDefaultInterceptors(),
		grpcserver.WithErrorDomain(errorDomain),
		grpcserver.WithGormErrorMapping(),
		grpcserver.WithServerOptions(grpc.ChainUnaryInterceptor(authInterceptor)),
	)
	if err != nil {
		return fmt.Errorf("init grpc server: %w", err)
	}

	if err := registerServices(srv.Grpc(), db, appId, appSummary); err != nil {
		return err
	}
	slog.Info("home-service grpc ready", "addr", cfg.GRpc.AddrWithPort(), "application_id", appId)
	return srv.Run(ctx)
}

// registerServices 注册首页配置服务与 contrib 菜单能力（只暴露 ListMyMenuTree）。
func registerServices(s *grpc.Server, db *gorm.DB, appId int64, appSummary *common_v1.ApplicationSummary) error {
	home_v1.RegisterHomepageServiceServer(s, service.NewHomepageService(db, appId, appSummary))

	menuSvc, err := service.NewMenuService(db, appId)
	if err != nil {
		return fmt.Errorf("build menu service: %w", err)
	}
	home_v1.RegisterMenuServiceServer(s, menuSvc)
	return nil
}
