package grpcserver

import (
	"context"
	"testing"
	"time"
)

func TestRunAndGracefulStop(t *testing.T) {
	srv, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	// 等待服务起来后取消 context，模拟收到退出信号
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error after graceful stop: %v", err)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestNewListenFailure(t *testing.T) {
	// 端口 0 两次绑定：先占用一个端口，再用同一端口创建应报错
	first, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	addr := first.lis.Addr().String()
	defer first.srv.Stop()

	if _, err := New(addr); err == nil {
		t.Fatal("expected listen error for occupied port")
	}
}
