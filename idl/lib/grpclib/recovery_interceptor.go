package grpclib

import (
	"runtime/debug"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	RecoveryUnaryServerInterceptor  grpc.UnaryServerInterceptor
	RecoveryStreamServerInterceptor grpc.StreamServerInterceptor
)

// Initialization shows an initialization sequence with a custom recovery handler func.
func init() {
	// Define customfunc to handle panic
	customFunc := func(p any) (err error) {
		debug.PrintStack()
		return status.Errorf(codes.Unknown, "panic triggered: %v", p)
	}
	// Shared options for the logger, with a custom gRPC code to log level function.
	opts := []recovery.Option{
		recovery.WithRecoveryHandler(customFunc),
	}
	// Create a server. Recovery handlers should typically be last in the chain so that other middleware
	// (e.g. logging) can operate on the recovered state instead of being directly affected by any panic
	RecoveryUnaryServerInterceptor = recovery.UnaryServerInterceptor(opts...)
	RecoveryStreamServerInterceptor = recovery.StreamServerInterceptor(opts...)
}
