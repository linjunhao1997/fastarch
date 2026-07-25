package server

import (
	"context"
	"log"
	"log/slog"
	"net/http"

	"iam-service/conf"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/linjunhao1997/idl/lib/gwlib"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func RunHttpServer(cfg *conf.Config) {
	dialOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
	}

	mux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(gwlib.IncomingHeaderMatcher),
		runtime.WithOutgoingHeaderMatcher(gwlib.OutgoingHeaderMatcher),
		runtime.WithMetadata(gwlib.WithMetadata))

	if err := iam_v1.RegisterAccountServiceHandlerFromEndpoint(context.Background(), mux, cfg.Http.Endpoint, dialOptions); err != nil {
		slog.Error("register account service handler from endpoint", "error", err)
	}
	if err := iam_v1.RegisterAuthenticationServiceHandlerFromEndpoint(context.Background(), mux, cfg.Http.Endpoint, dialOptions); err != nil {
		slog.Error("register authentication service handler from endpoint", "error", err)
	}
	if err := iam_v1.RegisterAuthorizationServiceHandlerFromEndpoint(context.Background(), mux, cfg.Http.Endpoint, dialOptions); err != nil {
		slog.Error("register authorization service handler from endpoint", "error", err)
	}
	if err := iam_v1.RegisterApplicationServiceHandlerFromEndpoint(context.Background(), mux, cfg.Http.Endpoint, dialOptions); err != nil {
		slog.Error("register application service handler from endpoint", "error", err)
	}
	slog.Info("http gateway server listening", "endpoint", cfg.Http.AddrWithPort())
	log.Fatalf("failed serve: %s", http.ListenAndServe(cfg.Http.AddrWithPort(), mux))
}
