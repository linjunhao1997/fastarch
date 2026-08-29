// Package grpcserver 提供 gRPC 服务的组装与启停助手：
// 默认拦截器链（Recovery + Validation）、监听、Serve 与信号驱动的优雅退出。
package grpcserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/linjunhao1997/idl/lib/grpclib"
	"google.golang.org/grpc"
)

// shutdownTimeout 是 GracefulStop 的最长等待时间，超时后强制 Stop。
const shutdownTimeout = 10 * time.Second

// Server 封装 grpc.Server 及其监听器。
type Server struct {
	srv *grpc.Server
	lis net.Listener
}

type options struct {
	serverOptions       []grpc.ServerOption
	listener            net.Listener
	defaultInterceptors bool
	errorDomain         string
}

// Option 服务选项。
type Option func(*options)

// WithServerOptions 追加业务自定义 grpc.ServerOption
// （如业务鉴权拦截器、StatsHandler 等）。默认拦截器在最外层，先于业务拦截器执行。
func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(o *options) {
		o.serverOptions = append(o.serverOptions, opts...)
	}
}

// WithListener 使用自定义监听器（默认 net.Listen("tcp", addr)）。
func WithListener(lis net.Listener) Option {
	return func(o *options) {
		o.listener = lis
	}
}

// WithDefaultInterceptors 启用默认拦截器链：
// unary: Recovery -> Validation -> ErrorDetails -> 业务拦截器；stream: Recovery。
func WithDefaultInterceptors() Option {
	return func(o *options) {
		o.defaultInterceptors = true
	}
}

// WithGormErrorMapping 启用 gorm 错误映射（记录不存在 → codes.NotFound）。
func WithGormErrorMapping() Option {
	return func(o *options) {
		o.serverOptions = append(o.serverOptions, grpc.ChainUnaryInterceptor(GormErrorInterceptor))
	}
}

// WithErrorDomain 在默认链中启用业务错误详情拦截器：业务错误会被转换为带
// ErrorInfo（reason=业务码，domain=服务名）的 gRPC 状态，grpc-gateway 输出到
// 错误响应 JSON 的 details 数组。domain 通常传服务名，如 "iam-service"。
func WithErrorDomain(domain string) Option {
	return func(o *options) {
		o.errorDomain = domain
	}
}

// New 创建 gRPC 服务并监听 addr。
func New(addr string, opts ...Option) (*Server, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	var serverOpts []grpc.ServerOption
	if o.defaultInterceptors {
		unaryChain := []grpc.UnaryServerInterceptor{
			grpclib.RecoveryUnaryServerInterceptor,
			grpclib.ValidationUnaryServerInterceptor,
		}
		if o.errorDomain != "" {
			unaryChain = append(unaryChain, grpclib.ErrorInterceptor(o.errorDomain))
		}
		serverOpts = append(serverOpts,
			grpc.ChainUnaryInterceptor(unaryChain...),
			grpc.ChainStreamInterceptor(grpclib.RecoveryStreamServerInterceptor),
		)
	}
	serverOpts = append(serverOpts, o.serverOptions...)

	lis := o.listener
	if lis == nil {
		var err error
		lis, err = net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("listen %s: %w", addr, err)
		}
	}

	return &Server{
		srv: grpc.NewServer(serverOpts...),
		lis: lis,
	}, nil
}

// Grpc 返回底层 grpc.Server，用于注册业务服务。
func (s *Server) Grpc() *grpc.Server {
	return s.srv
}

// Run 阻塞提供服务，直到 ctx 取消（收到信号）或 Serve 异常返回。
// ctx 取消后先 GracefulStop，超过 shutdownTimeout 未结束则强制 Stop。
func (s *Server) Run(ctx context.Context) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.srv.Serve(s.lis)
	}()

	slog.Info("grpc server listening", "addr", s.lis.Addr().String())

	select {
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("grpc serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		slog.Info("grpc server shutting down")
		done := make(chan struct{})
		go func() {
			s.srv.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(shutdownTimeout):
			slog.Warn("grpc graceful shutdown timeout, force stop")
			s.srv.Stop()
			<-done
		}
		return nil
	}
}
