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
	"iam-service/pkg/migration"
	"iam-service/pkg/service"
)

// RunGrpcServer 组装 iam-service 的 gRPC 服务：资源初始化、服务注册后阻塞运行，
// ctx 取消时优雅退出。
func RunGrpcServer(ctx context.Context, cfg *conf.Config) error {
	rdb := cache.NewRedis(cfg.Redis.Addr)

	// 表创建、字段变更与数据录入全部由内嵌的 golang-migrate 迁移承担
	// （pkg/migration/migrations），失败 fail-fast，禁止以未迁移 Schema 运行。
	if err := migration.Run(cfg.Database.DSN); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	db, err := database.NewMySQL(cfg.Database.DSN)

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
	signatureStore := auth.NewSignatureStore(db, rdb, credentialAES, credentialWindow)

	srv, err := grpcserver.New(cfg.GRpc.AddrWithPort(),
		grpcserver.WithDefaultInterceptors(),
		grpcserver.WithErrorDomain(cfg.App.Name),
		grpcserver.WithGormErrorMapping(),
		grpcserver.WithServerOptions(
			// 权限快照读穿回源函数注入鉴权拦截器（research.md D6-②）；
			// 签名验签器注入 AK/SK 双轨分支（specs/009-ak-sk-auth T017）；
			// appCode 解析器注入数据归属校验（appCode→appId + 接入集合校验）
			grpc.ChainUnaryInterceptor(auth.Interceptor(rdb, service.NewPermissionSnapshotLoader(db), signatureStore, service.ResolveDataAppByCode(db), cfg.Iam.ApplicationId)),
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
	iam_v1.RegisterMenuServiceServer(s, service.NewMenuService(db))
	iam_v1.RegisterDictServiceServer(s, service.NewDictService(db))

	return srv.Run(ctx)
}
