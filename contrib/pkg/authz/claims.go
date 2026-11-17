// Package authz 提供"第二宿主"服务把权限判定委托给 IAM 的薄客户端：
// 无验签解析 JWT 取身份（仅供审计）、透传 JWT 调 IAM CheckPermission、自动判权拦截器。
//
// 安全纪律（务必遵守）：
//   - 本包**不验签** JWT。解出的身份字段（AccountId/IsAdmin/ApplicationId）一律
//     只用于写审计字段或展示，**绝不可作为授权依据**。
//   - 授权的唯一来源是 IAM CheckPermission 返回的 ok。调用方必须在放行前
//     对每个受保护方法完成一次 CheckPermission；admin 无本地旁路（否则伪造
//     isAdmin=true 即可绕过判权），同样由 IAM 判定（IAM 对 admin 返回 ok=true）。
package authz

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// Claims 是 JWT payload 中的身份字段子集。注意这些字段未经签名验证，
// 只可用于审计归责，不可用于任何授权决策。
type Claims struct {
	AccountId     int64  `json:"-"` // 由 UserId 转换而来，见 ParseUnverified
	UserId        string `json:"userId"`
	Username      string `json:"username"`
	IsAdmin       bool   `json:"isAdmin"`
	ApplicationId int64  `json:"applicationId"`
	Jti           string `json:"jti"`
}

var (
	ErrEmptyToken    = errors.New("authz: empty token")
	ErrMalformedJWT  = errors.New("authz: malformed jwt")
	ErrInvalidClaims = errors.New("authz: invalid claims payload")
)

// ParseUnverified 从 JWT 的 payload 段解出身份字段，**不校验签名**。
// 调用方必须已经在别处（IAM CheckPermission）验证过同一 token 的有效性。
// 解析出的字段只用于审计，不得据此放行任何请求。
func ParseUnverified(token string) (*Claims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrEmptyToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrMalformedJWT
	}
	// 三段都必须是合法 base64url，拒绝畸形 token（即便本包不验签，结构也要合法）。
	if _, err := base64.RawURLEncoding.DecodeString(parts[0]); err != nil {
		return nil, ErrMalformedJWT
	}
	if _, err := base64.RawURLEncoding.DecodeString(parts[2]); err != nil {
		return nil, ErrMalformedJWT
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidClaims
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, ErrInvalidClaims
	}
	if c.UserId == "" {
		return nil, ErrInvalidClaims
	}
	c.AccountId = parseAccountId(c.UserId)
	return &c, nil
}

// parseAccountId 把 userId 字符串转数字。JWT 的 userId 是十进制字符串；
// 非数字（如外部机器令牌的非数字 id）返回 0，审计字段记为 0 但不影响授权。
func parseAccountId(userId string) int64 {
	var id int64
	for _, ch := range userId {
		if ch < '0' || ch > '9' {
			return 0
		}
		id = id*10 + int64(ch-'0')
	}
	return id
}
