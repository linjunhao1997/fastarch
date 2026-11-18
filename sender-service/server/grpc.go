package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/linjunhao1997/contrib/pkg/authz"
	"github.com/linjunhao1997/contrib/pkg/database"
	"github.com/linjunhao1997/contrib/pkg/server/grpcserver"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	sender_v1 "github.com/linjunhao1997/idl/pb/sender"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	iamAccessKey, iamSecretKey, err := cfg.Sender.ResolveIAMServiceAuth()
	if err != nil {
		return fmt.Errorf("resolve iam service credentials: %w", err)
	}

	// 登录委托 IAM 的 gRPC 连接（凭据校验唯一权威，ADR 0007）。
	iamAuthConn, err := grpc.NewClient(cfg.Sender.IamAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("dial iam for authentication: %w", err)
	}
	defer func() { _ = iamAuthConn.Close() }()
	iamAuth := iam_v1.NewAuthenticationServiceClient(iamAuthConn)

	// 入站判权薄客户端：透传 JWT 调 IAM CheckPermission（ADR 0005）。
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

	if err := registerServices(srv.Grpc(), db, appId, service.NewSenderAuthenticationService(iamAuth, iamAccessKey, iamSecretKey)); err != nil {
		return err
	}
	slog.Info("sender-service grpc ready", "addr", cfg.GRpc.AddrWithPort(), "application_id", appId)
	return srv.Run(ctx)
}

// registerServices 注册认证/消息/模板服务与 contrib 菜单/字典能力。
// contrib 实现的方法签名与 sender.v1 接口一致（require_unimplemented_servers=false），
// 可直接注册为 sender.v1 的服务（ADR 0003）。
func registerServices(s *grpc.Server, db *gorm.DB, appId int64, authSvc *service.AuthenticationService) error {
	sender_v1.RegisterAuthenticationServiceServer(s, authSvc)
	sender_v1.RegisterMessageServiceServer(s, service.NewMessageService(db, appId))
	sender_v1.RegisterTemplateServiceServer(s, service.NewTemplateService(db, appId))

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
