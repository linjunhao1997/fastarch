package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// shutdownTimeout 是 HTTP 服务优雅退出的最长等待时间。
const shutdownTimeout = 10 * time.Second

// Run 在 addr 上启动 HTTP 服务并阻塞，直到 ctx 取消（收到信号）或服务异常返回。
// ctx 取消后调用 Shutdown 等待在途请求处理完成，超时返回错误。
func Run(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	slog.Info("http server listening", "addr", addr)

	select {
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("http serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		slog.Info("http server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
		return nil
	}
}
