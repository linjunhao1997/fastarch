package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunAndShutdown(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, "127.0.0.1:0", handler) }()

	// 等待服务起来后取消 context，模拟收到退出信号
	time.Sleep(200 * time.Millisecond)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error after shutdown: %v", err)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestNewMuxRegistersHandlers(t *testing.T) {
	mux := NewMux()
	if mux == nil {
		t.Fatal("NewMux returned nil")
	}
	// mux 实现 http.Handler，可直接服务请求
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	mux.ServeHTTP(rec, req) // 未注册路由返回 404，证明 handler 链正常工作
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unregistered route, got %d", rec.Code)
	}
}
