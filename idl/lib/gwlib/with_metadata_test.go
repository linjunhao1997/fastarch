package gwlib

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithMetadataPassthroughSignatureHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/authentication/access-credentials?b=2", nil)
	req.Header.Set("X-Fa-Access-Key", "FAKtest")
	req.Header.Set("X-Fa-Timestamp", "1760000000")
	req.Header.Set("X-Fa-Nonce", "nonce-1")
	req.Header.Set("X-Fa-Signature", "abcdef")
	req.Header.Set("X-Fa-Signature-Method", "HMAC-SHA256")
	req.Header.Set("X-Ignored-Header", "no")

	md := WithMetadata(context.Background(), req)

	if got := md.Get("x-fa-access-key"); len(got) != 1 || got[0] != "FAKtest" {
		t.Fatalf("x-fa-access-key not passed through: %v", got)
	}
	if got := md.Get("x-fa-timestamp"); len(got) != 1 || got[0] != "1760000000" {
		t.Fatalf("x-fa-timestamp not passed through: %v", got)
	}
	if got := md.Get("x-fa-nonce"); len(got) != 1 || got[0] != "nonce-1" {
		t.Fatalf("x-fa-nonce not passed through: %v", got)
	}
	if got := md.Get("x-fa-signature"); len(got) != 1 || got[0] != "abcdef" {
		t.Fatalf("x-fa-signature not passed through: %v", got)
	}
	if got := md.Get("x-fa-signature-method"); len(got) != 1 || got[0] != "HMAC-SHA256" {
		t.Fatalf("x-fa-signature-method not passed through: %v", got)
	}
	if got := md.Get("x-ignored-header"); len(got) != 0 {
		t.Fatalf("unregistered header must not pass through: %v", got)
	}
	if got := md.Get(HeaderGatewayOrigin); len(got) != 1 || got[0] != GatewayOriginHTTP {
		t.Fatalf("gateway origin marker missing: %v", got)
	}
	// 签名请求的内部透传要素：原始方法/路径/查询串/请求体摘要
	if got := md.Get(MetadataOriginalMethod); len(got) != 1 || got[0] != http.MethodPost {
		t.Fatalf("original method not passed: %v", got)
	}
	if got := md.Get(MetadataOriginalURI); len(got) != 1 || got[0] != "/authentication/access-credentials" {
		t.Fatalf("original uri not passed: %v", got)
	}
	if got := md.Get(MetadataOriginalQuery); len(got) != 1 || got[0] != "b=2" {
		t.Fatalf("original query not passed: %v", got)
	}
	// nil body → 空体摘要
	if got := md.Get(MetadataBodyDigest); len(got) != 1 || got[0] != EmptyBodyDigestHex {
		t.Fatalf("body digest not passed: %v", got)
	}
}

// 带请求体时：摘要为 body 的 SHA-256，且 body 原样放回供下游读取。
func TestWithMetadataBodyDigestWithBody(t *testing.T) {
	body := `{"remark":"ci-deploy"}`
	req := httptest.NewRequest(http.MethodPost, "/authentication/access-credentials?a=1", strings.NewReader(body))
	req.Header.Set("X-Fa-Access-Key", "FAKtest")

	md := WithMetadata(context.Background(), req)

	sum := sha256.Sum256([]byte(body))
	want := hex.EncodeToString(sum[:])
	if got := md.Get(MetadataBodyDigest); len(got) != 1 || got[0] != want {
		t.Fatalf("body digest = %v, want %s", got, want)
	}
	rest, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read restored body: %v", err)
	}
	if string(rest) != body {
		t.Fatalf("restored body = %q, want %q", rest, body)
	}
}

// 非签名请求不注入内部透传要素。
func TestWithMetadataNoInternalFieldsWithoutSignature(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/authentication/access-credentials", nil)
	md := WithMetadata(context.Background(), req)
	for _, key := range []string{MetadataOriginalMethod, MetadataOriginalURI, MetadataOriginalQuery, MetadataBodyDigest} {
		if got := md.Get(key); len(got) != 0 {
			t.Fatalf("%s must be absent without signature headers, got %v", key, got)
		}
	}
}

func TestWithMetadataMissingSignatureHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/authentication/access-credentials", nil)
	md := WithMetadata(context.Background(), req)
	for _, header := range SignatureHeaders {
		if got := md.Get(header); len(got) != 0 {
			t.Fatalf("%s must be absent when not sent, got %v", header, got)
		}
	}
}
