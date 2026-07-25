package server

import (
	"iam-service/conf"
	"iam-service/pkg/auth"
	"iam-service/pkg/model"
	"iam-service/pkg/service"
	"log"
	"net"

	"github.com/linjunhao1997/idl/lib/grpclib"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func RunGrpcServer(cfg *conf.Config) {
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.Redis.Addr,
	})

	var opts = []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(grpclib.RecoveryUnaryServerInterceptor, grpclib.ValidationUnaryServerInterceptor, auth.Interceptor(rdb)),
		grpc.ChainStreamInterceptor(grpclib.RecoveryStreamServerInterceptor),
		grpc.StatsHandler(otelgrpc.NewClientHandler()),
	}
	s := grpc.NewServer(opts...)

	db, err := gorm.Open(mysql.Open(cfg.Database.GetDSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed open mysql: %s", err)
	}

	err = db.AutoMigrate(model.Models...)
	if err != nil {
		log.Fatalf("failed auto migrate: %s", err)
	}

	iam_v1.RegisterAuthenticationServiceServer(s, service.NewAuthenticationService(db, rdb))
	iam_v1.RegisterAccountServiceServer(s, service.NewAccountService(db))
	iam_v1.RegisterAuthorizationServiceServer(s, service.NewAuthorizationService(db, rdb))
	iam_v1.RegisterApplicationServiceServer(s, service.NewApplicationService(db))
	lis, err := net.Listen("tcp", cfg.GRpc.AddrWithPort())
	if err != nil {
		log.Fatalf("failed listening: %s", err)
	}
	log.Fatalf("failed serve: %s", s.Serve(lis))
}
