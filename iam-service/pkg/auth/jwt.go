package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("12345678")

type CustomClaims struct {
	UserId   string `json:"userId"`
	Username string `json:"username"`
	Roles    string `json:"roles"`
	IsAdmin  bool   `json:"isAdmin"`
	// ApplicationId 为令牌签发时锁定的目标应用（外部平台/机器令牌直接带入）；
	// 人用登录令牌通常为 0，当前应用改由 Redis app:ctx:{jti} 映射决定（切换不重签）。
	ApplicationId int64 `json:"applicationId"`
	jwt.RegisteredClaims
}

type ClaimsParam struct {
	UserId         string    `json:"userId"`
	Username       string    `json:"username"`
	ExpirationTime time.Time `json:"expirationTime"`
	IsAdmin        bool      `json:"isAdmin"`
	// Jti 令牌唯一标识：作为服务端 app 上下文映射（app:ctx:{jti}）的键。
	Jti string `json:"jti"`
	// ApplicationId 签发即锁定的应用（机器/外部令牌用；人用登录传 0）。
	ApplicationId int64 `json:"applicationId"`
}

func GenerateJWT(param ClaimsParam) (string, error) {
	claims := &CustomClaims{
		UserId:        param.UserId,
		Username:      param.Username,
		IsAdmin:       param.IsAdmin,
		ApplicationId: param.ApplicationId,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        param.Jti,
			ExpiresAt: jwt.NewNumericDate(param.ExpirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ParseClaims 单次解析并校验令牌，返回完整 claims。拦截器据此一次取全
// accountId/isAdmin/jti/applicationId，避免多次解析。
func ParseClaims(tokenString string) (*CustomClaims, error) {
	if len(tokenString) == 0 {
		return nil, errors.New("invalid token")
	}
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// GetAccountIdFromJwt 解析令牌返回用户 id（兼容既有调用方）。
func GetAccountIdFromJwt(tokenString string) (string, error) {
	claims, err := ParseClaims(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserId, nil
}

// IsAdminFromJwt 解析令牌返回是否管理员（兼容既有调用方）。
func IsAdminFromJwt(tokenString string) (bool, error) {
	claims, err := ParseClaims(tokenString)
	if err != nil {
		return false, err
	}
	return claims.IsAdmin, nil
}
