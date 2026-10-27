// Package auth 的签名验签纯函数（specs/009-ak-sk-auth T015）。
// 规范见 specs/009-ak-sk-auth/contracts/ak-sk-signature-spec.md：
// StringToSign = method \n URI \n canonicalQuery \n AK \n timestamp \n nonce \n bodyDigest，
// Signature = HexLower(HMAC-SHA256(StringToSign, SK))，比较使用常数时间。
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 签名相关元数据键（gwlib 小写透传）。
const (
	MetadataAccessKey       = "x-fa-access-key"
	MetadataTimestamp       = "x-fa-timestamp"
	MetadataNonce           = "x-fa-nonce"
	MetadataSignature       = "x-fa-signature"
	MetadataSignatureMethod = "x-fa-signature-method"
)

// 网关内部透传元数据键（gwlib WithMetadata 注入，非公开规范头）：
// gRPC 侧拿到的是解码后的请求对象，无法还原 HTTP 原始要素（方法/路径/查询串/请求体），
// 由网关在转发时计算并附带，供服务端重算规范化串。
const (
	MetadataOriginalMethod = "x-fa-original-method"
	MetadataOriginalURI    = "x-fa-original-uri"
	MetadataOriginalQuery  = "x-fa-original-query"
	MetadataBodyDigest     = "x-fa-body-digest"
)

// SignatureMethodHMACSHA256 是唯一支持的签名算法标识（规范第 2 节）。
const SignatureMethodHMACSHA256 = "HMAC-SHA256"

// EmptyBodyDigestHex 是空请求体的 SHA-256 小写十六进制摘要（规范第 3.1 节）。
const EmptyBodyDigestHex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// SHA256Hex 返回字节的 SHA-256 小写十六进制摘要。
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// BuildStringToSign 构造规范化请求串（规范第 3.1 节）。
func BuildStringToSign(method, canonicalURI, canonicalQuery, accessKey, timestamp, nonce, bodyDigestHex string) string {
	return strings.Join([]string{
		method, canonicalURI, canonicalQuery, accessKey, timestamp, nonce, bodyDigestHex,
	}, "\n")
}

// ComputeSignatureHex 计算 HMAC-SHA256 签名（小写十六进制），密钥为 SK 原始字符串字节。
func ComputeSignatureHex(stringToSign, secretKey string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature 常数时间比较签名是否一致。
func VerifySignature(stringToSign, secretKey, providedSignature string) bool {
	expected := ComputeSignatureHex(stringToSign, secretKey)
	return hmac.Equal([]byte(expected), []byte(providedSignature))
}

// CanonicalizeQuery 将原始查询串规范化（规范第 3.1 节）：
// key 与 value 先解码再按 RFC 3986 编码一次（空格 → %20），按 key 升序排序，
// key=value 以 & 连接；空查询串为空字符串。
func CanonicalizeQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	type queryPair struct{ key, value string }
	pairs := make([]queryPair, 0, 8)
	for _, part := range strings.Split(rawQuery, "&") {
		if part == "" {
			continue
		}
		key, value, _ := strings.Cut(part, "=")
		decodedKey, err := url.QueryUnescape(key)
		if err != nil {
			decodedKey = key
		}
		decodedValue, err := url.QueryUnescape(value)
		if err != nil {
			decodedValue = value
		}
		pairs = append(pairs, queryPair{escapeRFC3986(decodedKey), escapeRFC3986(decodedValue)})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].key != pairs[j].key {
			return pairs[i].key < pairs[j].key
		}
		return pairs[i].value < pairs[j].value
	})
	segments := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		segments = append(segments, pair.key+"="+pair.value)
	}
	return strings.Join(segments, "&")
}

// escapeRFC3986 按 RFC 3986 编码：仅保留 unreserved（A-Za-z0-9-_.~），空格编码为 %20
// （url.QueryEscape 会把空格编为 +，不符合规范，故自行实现）。
func escapeRFC3986(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		default:
			const upperhex = "0123456789ABCDEF"
			b.WriteByte('%')
			b.WriteByte(upperhex[c>>4])
			b.WriteByte(upperhex[c&0x0f])
		}
	}
	return b.String()
}

// IsTimestampInWindow 判断秒级时间戳是否在 ±windowSeconds 窗口内（规范第 2 节）。
func IsTimestampInWindow(timestamp string, now time.Time, windowSeconds int) bool {
	seconds, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return false
	}
	diff := now.Unix() - seconds
	if diff < 0 {
		diff = -diff
	}
	return diff <= int64(windowSeconds)
}
