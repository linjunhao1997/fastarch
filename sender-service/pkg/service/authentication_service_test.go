package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	sender_v1 "github.com/linjunhao1997/idl/pb/sender"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// stubServerTransportStream 使 PasswordAuth 的 SetHeader 在单测上下文可用
// （与 iam-service 同款处理）。
type stubServerTransportStream struct {
	grpc.ServerTransportStream
}

func (stubServerTransportStream) SetHeader(metadata.MD) error  { return nil }
func (stubServerTransportStream) SendHeader(metadata.MD) error { return nil }
func (stubServerTransportStream) SetTrailer(metadata.MD) error { return nil }

// fakeIAMAuth 是 IAM AuthenticationService 的测试替身：捕获出站 gRPC metadata
// 并可配置返回令牌或错误，用于验证机机签名而不依赖真实 IAM。
type fakeIAMAuth struct {
	gotMD       metadata.MD
	returnToken string
	err         error
}

func (f *fakeIAMAuth) PasswordAuth(ctx context.Context, _ *iam_v1.PasswordAuthRequest, _ ...grpc.CallOption) (*iam_v1.PasswordAuthResponse, error) {
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		f.gotMD = md.Copy()
	}
	if f.err != nil {
		return nil, f.err
	}
	return &iam_v1.PasswordAuthResponse{AccessToken: f.returnToken}, nil
}

func firstMD(md metadata.MD, key string) string {
	if values := md.Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}

// TestPasswordAuthDelegatesToIAM 验证 sender 登录委托 IAM（ADR 0007 补充裁决）：
//  1. 出站 metadata 携带 AK/SK 签名，且不再自报 application_code；
//  2. IAM 返回的 token 原样转发；
//  3. IAM 的业务错误原样透传（不二次包装）。
func TestPasswordAuthDelegatesToIAM(t *testing.T) {
	const accessKey = "FAKSVCKEY00000000000000001"
	const secretKey = "svc-secret"

	t.Run("signs request with service credentials", func(t *testing.T) {
		fake := &fakeIAMAuth{returnToken: "jwt-abc"}
		svc := NewSenderAuthenticationService(fake, accessKey, secretKey)

		ctx := grpc.NewContextWithServerTransportStream(context.Background(), stubServerTransportStream{})
		resp, err := svc.PasswordAuth(ctx, &sender_v1.PasswordAuthRequest{
			Identifier: "admin",
			Password:   "password123",
		})
		if err != nil {
			t.Fatalf("PasswordAuth() error = %v", err)
		}
		if resp.GetAccessToken() != "jwt-abc" {
			t.Fatalf("access token = %q, want jwt-abc", resp.GetAccessToken())
		}
		if got := firstMD(fake.gotMD, "x-fa-access-key"); got != accessKey {
			t.Fatalf("outgoing access key = %q, want %q", got, accessKey)
		}
		if got := firstMD(fake.gotMD, "application_code"); got != "" {
			t.Fatalf("caller-supplied application_code must not be sent, got %q", got)
		}
		timestamp := firstMD(fake.gotMD, "x-fa-timestamp")
		nonce := firstMD(fake.gotMD, "x-fa-nonce")
		if timestamp == "" || nonce == "" {
			t.Fatal("missing timestamp or nonce in signature metadata")
		}
		stringToSign := strings.Join([]string{
			iamServiceMethod, iamServiceURI, iamServiceQuery, accessKey, timestamp, nonce, iamEmptyBodyDigest,
		}, "\n")
		mac := hmac.New(sha256.New, []byte(secretKey))
		mac.Write([]byte(stringToSign))
		want := hex.EncodeToString(mac.Sum(nil))
		if got := firstMD(fake.gotMD, "x-fa-signature"); got != want {
			t.Fatalf("signature = %q, want %q", got, want)
		}
	})

	t.Run("propagates iam error unchanged", func(t *testing.T) {
		want := errors.New("invalid account password")
		fake := &fakeIAMAuth{err: want}
		svc := NewSenderAuthenticationService(fake, accessKey, secretKey)
		_, err := svc.PasswordAuth(context.Background(), &sender_v1.PasswordAuthRequest{
			Identifier: "nobody",
			Password:   "wrong",
		})
		if !errors.Is(err, want) {
			t.Fatalf("delegated error = %v, want propagated %v", err, want)
		}
		if got := firstMD(fake.gotMD, "x-fa-access-key"); got != accessKey {
			t.Fatalf("err path access key = %q, want %q", got, accessKey)
		}
	})
}
