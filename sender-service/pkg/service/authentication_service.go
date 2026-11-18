package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	sender_v1 "github.com/linjunhao1997/idl/pb/sender"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// senderRepresentation 是 sender 登录所依赖的 IAM 认证客户端接口（便于测试替身）。
type senderRepresentation interface {
	PasswordAuth(ctx context.Context, in *iam_v1.PasswordAuthRequest, opts ...grpc.CallOption) (*iam_v1.PasswordAuthResponse, error)
}

// AK/SK 签名要素（与 IAM SignatureStore 的验签规范一致）。
// gRPC 内部调用无法还原 HTTP 原始请求，method/URI/body digest 由调用方与服务端
// 按同一常量构造，仅用于证明机机身份，不承载业务数据。
const (
	iamServiceMethod   = "POST"
	iamServiceURI      = "/authentication/password-auth"
	iamServiceQuery    = ""
	iamEmptyBodyDigest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	iamSignatureMethod = "HMAC-SHA256"
)

// withIAMServiceSignature 为出站 gRPC 调用注入 AK/SK 签名元数据。
// IAM 验签通过后从机机账号所属应用推导目标应用，不信任调用方自报 code。
func withIAMServiceSignature(ctx context.Context, accessKey, secretKey string) (context.Context, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := hex.EncodeToString(nonceBytes)
	stringToSign := strings.Join([]string{
		iamServiceMethod, iamServiceURI, iamServiceQuery, accessKey, timestamp, nonce, iamEmptyBodyDigest,
	}, "\n")
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(stringToSign))
	signature := hex.EncodeToString(mac.Sum(nil))

	md := metadata.Pairs(
		"x-fa-access-key", accessKey,
		"x-fa-timestamp", timestamp,
		"x-fa-nonce", nonce,
		"x-fa-signature", signature,
		"x-fa-signature-method", iamSignatureMethod,
		"x-fa-original-method", iamServiceMethod,
		"x-fa-original-uri", iamServiceURI,
		"x-fa-original-query", iamServiceQuery,
		"x-fa-body-digest", iamEmptyBodyDigest,
	)
	return metadata.NewOutgoingContext(ctx, md), nil
}

// AuthenticationService 是 sender 独立登录端点（ADR 0007）。
// sender 无自有用户库：凭据校验委托 IAM PasswordAuth；调用携带 AK/SK 机机账号签名，
// 由 IAM 从服务账号所属应用推导目标应用（而非调用方自报 code），登录即锁定该应用。
// 签发令牌由 IAM 完成，本服务仅转发并回写 cookie。
type AuthenticationService struct {
	iam       senderRepresentation
	accessKey string
	secretKey string
}

// NewSenderAuthenticationService 构造 sender 登录服务（机机账号凭证由配置注入）。
func NewSenderAuthenticationService(iam senderRepresentation, accessKey, secretKey string) *AuthenticationService {
	return &AuthenticationService{iam: iam, accessKey: accessKey, secretKey: secretKey}
}

// PasswordAuth 密码认证：转发 IAM 校验并返回令牌。
// grpc.SetHeader 回写同 IAM 一致的 Set-Cookie，经 grpc-gateway 出站头透传给浏览器。
func (s *AuthenticationService) PasswordAuth(ctx context.Context, request *sender_v1.PasswordAuthRequest) (*sender_v1.PasswordAuthResponse, error) {
	// 机机代登录：AK/SK 签名证明调用方是 sender 服务，IAM 从服务账号推导应用（ADR 0007 补充裁决）。
	outCtx, err := withIAMServiceSignature(ctx, s.accessKey, s.secretKey)
	if err != nil {
		return nil, err
	}
	resp, err := s.iam.PasswordAuth(outCtx, &iam_v1.PasswordAuthRequest{
		Identifier: request.Identifier,
		Password:   request.Password,
	})
	if err != nil {
		return nil, err // IAM 的业务错误语义（1001001/1001063…）原样透传
	}
	token := resp.GetAccessToken()
	// 与 IAM 登录一致地回写 HttpOnly cookie（凭据源仍为 IAM 签发的 token）。
	accessTokenCookie := http.Cookie{
		Name:     "access_token",
		Value:    token,
		Expires:  time.Now().Add(30 * time.Minute),
		Path:     "/",
		HttpOnly: true,
	}
	if err := grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", accessTokenCookie.String())); err != nil {
		return nil, err
	}
	return &sender_v1.PasswordAuthResponse{AccessToken: token}, nil
}
