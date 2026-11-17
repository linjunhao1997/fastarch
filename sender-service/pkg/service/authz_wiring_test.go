package service

import (
	"context"
	"encoding/base64"
	"net"
	"testing"

	"github.com/linjunhao1997/contrib/pkg/authz"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeIAM 实现 IAM 的 CheckPermission，用真实 gRPC 连接走完整的 authz 链路。
type fakeIAM struct {
	iam_v1.UnimplementedAuthorizationServiceServer
	ok      bool
	err     error
	gotCode string
}

func (f *fakeIAM) CheckPermission(_ context.Context, in *iam_v1.CheckPermissionRequest) (*iam_v1.CheckPermissionResponse, error) {
	f.gotCode = in.GetPermissionCode()
	if f.err != nil {
		return nil, f.err
	}
	return &iam_v1.CheckPermissionResponse{Ok: f.ok}, nil
}

func jwt(payload string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte("fakesig"))
}

func ctxWithJWT(token string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
}

// startFakeIAM 起一个真实 gRPC server，返回地址与关闭函数。
func startFakeIAM(t *testing.T, fake *fakeIAM) (string, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := grpc.NewServer()
	iam_v1.RegisterAuthorizationServiceServer(s, fake)
	go func() { _ = s.Serve(lis) }()
	return lis.Addr().String(), func() { s.Stop(); _ = lis.Close() }
}

const realSendMethod = "/sender.v1.MessageService/SendMessage"

func newRealInterceptor(t *testing.T, fake *fakeIAM) (grpc.UnaryServerInterceptor, func()) {
	t.Helper()
	addr, stop := startFakeIAM(t, fake)
	client, err := authz.New(addr)
	if err != nil {
		stop()
		t.Fatalf("authz.New: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	interceptor := authz.Interceptor(client, authz.Config{
		ApplicationId:   testApplicationId,
		AuthOnlyMethods: AuthOnlyMethods(),
		MethodPerms:     MethodPermsFor,
		ErrorDomain:     "sender-service",
	})
	return interceptor, stop
}

// 用真实 full method + 真实 gRPC 连接验证：JWT 透传、权限码映射、放行/拒绝/fail-closed。
func TestAuthzWiringWithRealMethod(t *testing.T) {
	fake := &fakeIAM{ok: true}
	interceptor, stop := newRealInterceptor(t, fake)
	defer stop()

	handlerCalled := false
	handler := func(ctx context.Context, _ any) (any, error) {
		handlerCalled = true
		if got := authz.GetApplicationId(ctx); got != testApplicationId {
			t.Fatalf("application_id in ctx = %d, want %d", got, testApplicationId)
		}
		return "ok", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: realSendMethod}
	token := jwt(`{"userId":"42","jti":"j-1","isAdmin":false}`)

	_, err := interceptor(ctxWithJWT(token), nil, info, handler)
	if err != nil {
		t.Fatalf("allowed request err = %v", err)
	}
	if !handlerCalled {
		t.Fatal("handler not called on allowed request")
	}
	if fake.gotCode != "sender.record.send" {
		t.Fatalf("permission code sent to IAM = %q, want sender.record.send", fake.gotCode)
	}
}

func TestAuthzWiringDeniedAndFailClosed(t *testing.T) {
	// IAM 拒绝 → PermissionDenied，handler 不执行
	fakeDeny := &fakeIAM{ok: false}
	interceptorDeny, stopDeny := newRealInterceptor(t, fakeDeny)
	defer stopDeny()
	called := false
	h := func(context.Context, any) (any, error) { called = true; return nil, nil }
	_, err := interceptorDeny(ctxWithJWT(jwt(`{"userId":"42"}`)), nil,
		&grpc.UnaryServerInfo{FullMethod: realSendMethod}, h)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("deny code = %v, want PermissionDenied", status.Code(err))
	}
	if called {
		t.Fatal("handler called on denied request")
	}

	// 未登记方法 → fail-closed，不调 IAM
	fakeAny := &fakeIAM{ok: true}
	interceptorAny, stopAny := newRealInterceptor(t, fakeAny)
	defer stopAny()
	_, err = interceptorAny(ctxWithJWT(jwt(`{"userId":"42"}`)), nil,
		&grpc.UnaryServerInfo{FullMethod: "/sender.v1.MessageService/Unknown"}, h)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unregistered method code = %v, want PermissionDenied", status.Code(err))
	}
	if fakeAny.gotCode != "" {
		t.Fatalf("IAM called for unregistered method, code=%q", fakeAny.gotCode)
	}
}
