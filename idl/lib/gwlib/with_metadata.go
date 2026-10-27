package gwlib

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"
)

const (
	HeaderCookieKeyAuthorization = "Authorization"
	// HeaderGatewayOrigin 标记请求经 grpc-gateway 转发（网关 → gRPC 方向），
	// 鉴权拦截器据此区分 HTTP 接口域与服务间 gRPC 域（specs/005-typed-permission-model D3）。
	HeaderGatewayOrigin = "x-gateway-origin"
	// GatewayOriginHTTP 是 HeaderGatewayOrigin 的取值：来源为 HTTP 网关。
	GatewayOriginHTTP = "http"
)

// 签名验签内部透传元数据键（specs/009-ak-sk-auth，非公开规范头）：
// gRPC 侧拿到的是解码后的请求对象，无法还原 HTTP 原始要素，由网关计算注入。
const (
	// MetadataOriginalMethod 透传原始 HTTP 方法（CanonicalURI 要素）。
	MetadataOriginalMethod = "x-fa-original-method"
	// MetadataOriginalURI 透传原始请求路径（与实际请求路径逐字节一致）。
	MetadataOriginalURI = "x-fa-original-uri"
	// MetadataOriginalQuery 透传原始查询串（服务端按规范重新规范化）。
	MetadataOriginalQuery = "x-fa-original-query"
	// MetadataBodyDigest 透传请求体 SHA-256 小写十六进制摘要（网关侧计算）。
	MetadataBodyDigest = "x-fa-body-digest"
)

// SignatureHeaders 是 AK/SK 签名鉴权请求头（specs/009-ak-sk-auth
// contracts/ak-sk-signature-spec.md）。IncomingHeaderMatcher 不放行未登记头，
// 网关必须在 WithMetadata 中显式透传为 gRPC 元数据供鉴权拦截器验签。
var SignatureHeaders = []string{
	"X-Fa-Access-Key",
	"X-Fa-Timestamp",
	"X-Fa-Nonce",
	"X-Fa-Signature",
	"X-Fa-Signature-Method",
}

func WithMetadata(ctx context.Context, req *http.Request) metadata.MD {
	md := metadata.New(map[string]string{})
	pair, _ := req.Cookie(HeaderCookieKeyAuthorization)
	if pair != nil {
		md.Append(HeaderCookieKeyAuthorization, pair.Value)
	}
	for _, header := range SignatureHeaders {
		if values := req.Header.Values(header); len(values) > 0 {
			md.Append(strings.ToLower(header), values[0])
		}
	}
	if len(req.Header.Values("X-Fa-Access-Key")) > 0 {
		// 签名请求：网关计算原始 HTTP 要素与请求体摘要供服务端重算规范化串
		md.Set(MetadataOriginalMethod, req.Method)
		md.Set(MetadataOriginalURI, req.URL.EscapedPath())
		md.Set(MetadataOriginalQuery, req.URL.RawQuery)
		md.Set(MetadataBodyDigest, requestSHA256Hex(req))
	}
	md.Set(HeaderGatewayOrigin, GatewayOriginHTTP)
	return md
}

// requestSHA256Hex 读取请求体计算 SHA-256 摘要后原样放回，保证下游仍可读取 body。
func requestSHA256Hex(req *http.Request) string {
	if req.Body == nil {
		return EmptyBodyDigestHex
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return EmptyBodyDigestHex
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// EmptyBodyDigestHex 是空请求体的 SHA-256 摘要（规范第 3.1 节）。
const EmptyBodyDigestHex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
