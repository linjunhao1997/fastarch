package auth

import (
	"errors"

	"google.golang.org/grpc/codes"
)

// SignatureError 是验签域业务错误（specs/009-ak-sk-auth FR-014）：语义化区分失败原因，
// 经 grpclib ErrorInterceptor 转成 gRPC ErrorInfo（reason=Code）透传；
// 错误信息不泄露 SK 或其派生信息。
type SignatureError struct {
	Code     string
	grpcCode codes.Code
	Err      error
}

func (e *SignatureError) Error() string {
	return e.Err.Error()
}

func (e *SignatureError) Unwrap() error {
	return e.Err
}

// BusinessCode 实现 grpclib.BusinessError，业务码随 ErrorInfo.Reason 下发。
func (e *SignatureError) BusinessCode() string {
	return e.Code
}

// GRPCCode 实现 grpclib 的可选接口。
func (e *SignatureError) GRPCCode() codes.Code {
	if e.grpcCode == 0 {
		return codes.Unauthenticated
	}
	return e.grpcCode
}

func newSignatureError(code string, err error) *SignatureError {
	return &SignatureError{Code: code, grpcCode: codes.Unauthenticated, Err: err}
}

// 验签失败语义（specs/009-ak-sk-auth contracts/ak-sk-signature-spec.md 第 4 节校验顺序）：
// 服务端按序校验，失败即拒绝（fail-closed）。
var (
	ErrSignatureMissing          = newSignatureError("SIGNATURE_MISSING", errors.New("签名要素缺失"))
	ErrSignatureKeyUnknown       = newSignatureError("KEY_UNKNOWN", errors.New("访问密钥不存在"))
	ErrSignatureKeyDisabled      = newSignatureError("KEY_DISABLED", errors.New("访问密钥已禁用"))
	ErrSignatureKeyExpired       = newSignatureError("KEY_EXPIRED", errors.New("访问密钥已过期"))
	ErrSignatureTimestampExpired = newSignatureError("TIMESTAMP_EXPIRED", errors.New("签名时间戳超出允许窗口，请校准系统时间"))
	ErrSignatureNonceReplay      = newSignatureError("NONCE_REPLAY", errors.New("签名随机数已被使用，请重新签名"))
	ErrSignatureMismatch         = newSignatureError("SIGNATURE_MISMATCH", errors.New("签名错误"))
)
