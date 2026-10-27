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
	"iam-service/pkg/cipher"
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
	if err := model.DedupRolePermissions(db); err != nil {
		return fmt.Errorf("dedup role permissions: %w", err)
	}
	if err := model.DropLegacyForeignKeys(db); err != nil {
		return fmt.Errorf("drop legacy foreign keys: %w", err)
	}
	// 权限体系基础数据初始化（specs/005-typed-permission-model US3）：
	// fail-closed——seed 失败不可带残缺权限表运行
	if err := model.Seed(ctx, db); err != nil {
		return fmt.Errorf("seed permission data: %w", err)
	}
	if err := model.MigrateResourceTree(db); err != nil {
		return fmt.Errorf("migrate resource tree: %w", err)
	}

	// 访问凭证域配置解析（specs/009-ak-sk-auth）：密钥缺失 fail-fast，
	// 禁止以明文/无加密状态运行（data-model.md 配置项）
	credentialWindow, credentialMaxPerAccount, credentialEncryptKey, err := cfg.ResolveCredentialConfig()
	if err != nil {
		return err
	}
	credentialAES, err := cipher.NewAESGCMFromEncodedKey(credentialEncryptKey)
	if err != nil {
		return fmt.Errorf("init credential cipher: %w", err)
	}

	srv, err := grpcserver.New(cfg.GRpc.AddrWithPort(),
		grpcserver.WithDefaultInterceptors(),
		grpcserver.WithErrorDomain(cfg.App.Name),
		grpcserver.WithGormErrorMapping(),
		grpcserver.WithServerOptions(
			// 权限快照读穿回源函数注入鉴权拦截器（research.md D6-②）；
			// 签名验签器注入 AK/SK 双轨分支（specs/009-ak-sk-auth T017）
			grpc.ChainUnaryInterceptor(auth.Interceptor(rdb, service.NewPermissionSnapshotLoader(db), auth.NewSignatureStore(db, rdb, credentialAES, credentialWindow))),
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
	iam_v1.RegisterAccessCredentialServiceServer(s, service.NewAccessCredentialService(db, rdb, credentialAES, credentialMaxPerAccount))

	return srv.Run(ctx)
}
