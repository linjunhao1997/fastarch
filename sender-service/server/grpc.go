package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/linjunhao1997/contrib/pkg/authz"
	"github.com/linjunhao1997/contrib/pkg/database"
	"github.com/linjunhao1997/contrib/pkg/server/grpcserver"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	sender_v1 "github.com/linjunhao1997/idl/pb/sender"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"sender-service/conf"
	"sender-service/pkg/migration"
	"sender-service/pkg/service"
)

const errorDomain = "sender-service"

// RunGrpcServer 组装 sender-service 的 gRPC 服务：迁移、DB、判权客户端、
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

	appId := cfg.Sender.ApplicationId
	if appId <= 0 {
		return fmt.Errorf("sender.application_id must be positive")
	}
	if cfg.Sender.IamAddr == "" {
		return fmt.Errorf("sender.iam_addr is required for inbound authorization")
	}
	// 归属应用摘要（宿主固定值，来自配置；与 IAM 种子 000010 一致）。
	// 只读回填到模板/记录响应，避免为每条记录跨服务查应用目录。
	appSummary := &common_v1.ApplicationSummary{
		Id:     cast.ToString(appId),
		Code:   cfg.Sender.ApplicationCode,
		Name:   cfg.Sender.ApplicationName,
		Status: "enabled",
	}
	// 入站判权薄客户端：透传 JWT 调 IAM CheckPermission（ADR 0005）。
	// 统一登录后 sender 不再有登录端点，也不需要 IAM 服务账号 AK/SK。
	authzClient, err := authz.New(cfg.Sender.IamAddr)
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
	slog.Info("sender-service grpc ready", "addr", cfg.GRpc.AddrWithPort(), "application_id", appId)
	return srv.Run(ctx)
}

// registerServices 注册消息/模板服务与 contrib 菜单/字典能力。
// contrib 实现的方法签名与 sender.v1 接口一致（require_unimplemented_servers=false），
// 可直接注册为 sender.v1 的服务（ADR 0003）。
func registerServices(s *grpc.Server, db *gorm.DB, appId int64, appSummary *common_v1.ApplicationSummary) error {
	sender_v1.RegisterMessageServiceServer(s, service.NewMessageService(db, appId, appSummary))
	sender_v1.RegisterTemplateServiceServer(s, service.NewTemplateService(db, appId, appSummary))

	menuSvc, err := service.NewMenuService(db, appId)
	if err != nil {
		return fmt.Errorf("build menu service: %w", err)
	}
	dictSvc, err := service.NewDictService(db, appId)
	if err != nil {
		return fmt.Errorf("build dict service: %w", err)
	}
	sender_v1.RegisterMenuServiceServer(s, menuSvc)
	sender_v1.RegisterDictServiceServer(s, dictSvc)
	return nil
}
