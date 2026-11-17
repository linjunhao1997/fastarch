package authz

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ---- 测试替身 ----

type mockIAM struct {
	ok      bool
	err     error
	gotCode string
	gotMD   metadata.MD // 记录收到的出站 metadata，验证 JWT 透传
}

func (m *mockIAM) CheckPermission(ctx context.Context, in *iam_v1.CheckPermissionRequest, _ ...grpc.CallOption) (*iam_v1.CheckPermissionResponse, error) {
	m.gotCode = in.GetPermissionCode()
	m.gotMD, _ = metadata.FromOutgoingContext(ctx)
	if m.err != nil {
		return nil, m.err
	}
	return &iam_v1.CheckPermissionResponse{Ok: m.ok}, nil
}

func jwtFor(payload string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte(`fakesig`))
}

func ctxWithJWT(token string) context.Context {
	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewIncomingContext(context.Background(), md)
}

const testMethod = "sender.v1.RecordService/SendMessage"

func testConfig() Config {
	return Config{
		ApplicationId: 2,
		ErrorDomain:   "sender-service",
		MethodPerms: func(m string) []string {
			if m == testMethod {
				return []string{"sender.record.send"}
			}
			return nil
		},
	}
}

// handler 记录是否被调用，并在 ctx 中读出注入的 accountId/appId。
func testHandler(captured *[2]int64) grpc.UnaryHandler {
	return func(ctx context.Context, _ any) (any, error) {
		captured[0] = GetAccountId(ctx)
		captured[1] = GetApplicationId(ctx)
		return "ok", nil
	}
}

// ---- ParseUnverified ----

func TestParseUnverified(t *testing.T) {
	token := jwtFor(`{"userId":"7","username":"zhangsan","isAdmin":true,"applicationId":0,"jti":"j-1"}`)
	c, err := ParseUnverified(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.AccountId != 7 || c.UserId != "7" || !c.IsAdmin || c.Jti != "j-1" {
		t.Fatalf("unexpected claims: %+v", c)
	}
}

func TestParseUnverifiedNoSignatureCheck(t *testing.T) {
	// 任意假签名都能解——证明本函数不验签（身份不可作授权依据）。
	token := jwtFor(`{"userId":"99"}`)
	if _, err := ParseUnverified(token); err != nil {
		t.Fatalf("unverified parse should succeed: %v", err)
	}
}

func TestParseUnverifiedRejectsMalformed(t *testing.T) {
	for _, token := range []string{"", "abc", "a.b", "a.b.c", "!!!.eyJ1c2VySWQiOiI3In0.x"} {
		if _, err := ParseUnverified(token); err == nil {
			t.Fatalf("expected error for %q", token)
		}
	}
}

// ---- Client 透传 ----

func TestCheckPermissionForwardsJWT(t *testing.T) {
	m := &mockIAM{ok: true}
	c := newWithIamClient(m)
	token := jwtFor(`{"userId":"7"}`)
	ctx := ctxWithJWT(token)

	ok, err := c.CheckPermission(ctx, "sender.record.send")
	if err != nil || !ok {
		t.Fatalf("check: ok=%v err=%v", ok, err)
	}
	if m.gotCode != "sender.record.send" {
		t.Fatalf("code not forwarded: %q", m.gotCode)
	}
	got := m.gotMD.Get("authorization")
	if len(got) == 0 || got[0] != "Bearer "+token {
		t.Fatalf("jwt not forwarded: %v", got)
	}
}

// ---- Interceptor ----

func TestInterceptorAllowsWhenIAMOk(t *testing.T) {
	m := &mockIAM{ok: true}
	c := newWithIamClient(m)
	var captured [2]int64
	interceptor := Interceptor(c, testConfig())
	info := &grpc.UnaryServerInfo{FullMethod: testMethod}
	ctx := ctxWithJWT(jwtFor(`{"userId":"7","jti":"j-1"}`))

	resp, err := interceptor(ctx, nil, info, testHandler(&captured))
	if err != nil || resp != "ok" {
		t.Fatalf("expected allow, got resp=%v err=%v", resp, err)
	}
	if captured[0] != 7 || captured[1] != 2 {
		t.Fatalf("identity not injected: %v", captured)
	}
}

func TestInterceptorDeniesWhenIAMDenies(t *testing.T) {
	m := &mockIAM{ok: false}
	c := newWithIamClient(m)
	called := false
	interceptor := Interceptor(c, testConfig())
	info := &grpc.UnaryServerInfo{FullMethod: testMethod}

	_, err := interceptor(ctxWithJWT(jwtFor(`{"userId":"7"}`)), nil, info,
		func(context.Context, any) (any, error) { called = true; return nil, nil })
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
	if called {
		t.Fatal("handler must not run when denied")
	}
}

func TestInterceptorFailClosedOnIAMError(t *testing.T) {
	m := &mockIAM{err: errors.New("iam down")}
	c := newWithIamClient(m)
	interceptor := Interceptor(c, testConfig())
	info := &grpc.UnaryServerInfo{FullMethod: testMethod}

	_, err := interceptor(ctxWithJWT(jwtFor(`{"userId":"7"}`)), nil, info,
		func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func TestInterceptorNoAdminBypass(t *testing.T) {
	// 伪造 admin token：拦截器不得因 isAdmin=true 跳过 IAM 判权。
	m := &mockIAM{ok: false}
	c := newWithIamClient(m)
	interceptor := Interceptor(c, testConfig())
	info := &grpc.UnaryServerInfo{FullMethod: testMethod}
	adminToken := jwtFor(`{"userId":"7","isAdmin":true}`)

	_, err := interceptor(ctxWithJWT(adminToken), nil, info,
		func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("admin must not bypass IAM: %v", err)
	}
	if m.gotCode == "" {
		t.Fatal("IAM must be called even for admin")
	}
}

func TestInterceptorUnregisteredMethodDenied(t *testing.T) {
	m := &mockIAM{ok: true}
	c := newWithIamClient(m)
	interceptor := Interceptor(c, testConfig())
	info := &grpc.UnaryServerInfo{FullMethod: "sender.v1.Other/Unknown"}

	_, err := interceptor(ctxWithJWT(jwtFor(`{"userId":"7"}`)), nil, info,
		func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for unregistered method, got %v", err)
	}
	if m.gotCode != "" {
		t.Fatal("IAM must not be called for unregistered method")
	}
}

func TestInterceptorAuthOnlySkipsPermission(t *testing.T) {
	m := &mockIAM{ok: false}
	c := newWithIamClient(m)
	cfg := testConfig()
	cfg.AuthOnlyMethods = map[string]struct{}{"sender.v1.MenuService/ListMyMenuTree": {}}
	interceptor := Interceptor(c, cfg)
	info := &grpc.UnaryServerInfo{FullMethod: "sender.v1.MenuService/ListMyMenuTree"}
	called := false

	_, err := interceptor(ctxWithJWT(jwtFor(`{"userId":"7"}`)), nil, info,
		func(context.Context, any) (any, error) { called = true; return nil, nil })
	if err != nil || !called {
		t.Fatalf("auth-only should pass without IAM check: err=%v called=%v", err, called)
	}
	if m.gotCode != "" {
		t.Fatal("IAM must not be called for auth-only method")
	}
}
