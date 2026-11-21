package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/linjunhao1997/contrib/pkg/authz"
	"github.com/linjunhao1997/contrib/pkg/database"
	"github.com/linjunhao1997/contrib/pkg/server/grpcserver"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	flow_v1 "github.com/linjunhao1997/idl/pb/flow"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"flow-service/conf"
	"flow-service/pkg/migration"
	"flow-service/pkg/service"
)

const errorDomain = "flow-service"

// RunGrpcServer 组装 flow-service：迁移 → DB → 判权客户端 → 拦截器 → 服务注册。
func RunGrpcServer(ctx context.Context, cfg *conf.Config) error {
	if err := migration.Run(cfg.Database.DSN); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	db, err := database.NewMySQL(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}

	appId := cfg.Flow.ApplicationId
	if appId <= 0 {
		return fmt.Errorf("flow.application_id must be positive")
	}
	if cfg.Flow.IamAddr == "" {
		return fmt.Errorf("flow.iam_addr is required for inbound authorization")
	}
	appSummary := &common_v1.ApplicationSummary{
		Id:     cast.ToString(appId),
		Code:   cfg.Flow.ApplicationCode,
		Name:   cfg.Flow.ApplicationName,
		Status: "enabled",
	}
	// 入站判权：只认 JWT，TH 透传调 IAM CheckPermission（ADR 0005）。
	authzClient, err := authz.New(cfg.Flow.IamAddr)
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

	if err := registerServices(srv.Grpc(), db, appId, appSummary, cfg); err != nil {
		return err
	}
	slog.Info("flow-service grpc ready", "addr", cfg.GRpc.AddrWithPort(), "application_id", appId)
	return srv.Run(ctx)
}

// registerServices 注册成员/任务/问题/留痕四个业务服务 + contrib 菜单/字典。
func registerServices(s *grpc.Server, db *gorm.DB, appId int64, summary *common_v1.ApplicationSummary, cfg *conf.Config) error {
	host := service.NewHost(db, appId, summary, cfg)

	flow_v1.RegisterMemberServiceServer(s, service.NewMemberService(host))
	flow_v1.RegisterTaskServiceServer(s, service.NewTaskService(host))
	flow_v1.RegisterQuestionServiceServer(s, service.NewQuestionService(host))
	flow_v1.RegisterLogServiceServer(s, service.NewLogService(host))

	menuSvc, err := service.NewMenuService(db, appId)
	if err != nil {
		return fmt.Errorf("build menu service: %w", err)
	}
	dictSvc, err := service.NewDictService(db, appId)
	if err != nil {
		return fmt.Errorf("build dict service: %w", err)
	}
	flow_v1.RegisterMenuServiceServer(s, menuSvc)
	flow_v1.RegisterDictServiceServer(s, dictSvc)
	return nil
}
