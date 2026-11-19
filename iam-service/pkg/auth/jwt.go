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
	// AdmittedAppIds 为签发时快照的用户接入应用集合（home + t_r_user_application）。
	// 运行时校验 x-fa-app-code 是否为其中之一：授权来自签名 claim，header 仅为选择信号。
	// 复制切片入 claim，避免调用方后续修改影响令牌。
	AdmittedAppIds []int64 `json:"admittedAppIds"`
	jwt.RegisteredClaims
}

type ClaimsParam struct {
	UserId         string    `json:"userId"`
	Username       string    `json:"username"`
	ExpirationTime time.Time `json:"expirationTime"`
	IsAdmin        bool      `json:"isAdmin"`
	// Jti 令牌唯一标识：作为服务端 app 上下文映射（app:ctx:{jti}）的键。
	Jti string `json:"jti"`
	// AdmittedAppIds 用户可切换的数据归属应用集合；admin 可为空（旁路校验）。
	AdmittedAppIds []int64 `json:"admittedAppIds"`
}

func GenerateJWT(param ClaimsParam) (string, error) {
	admitted := make([]int64, len(param.AdmittedAppIds))
	copy(admitted, param.AdmittedAppIds)
	claims := &CustomClaims{
		UserId:         param.UserId,
		Username:       param.Username,
		IsAdmin:        param.IsAdmin,
		AdmittedAppIds: admitted,
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
// accountId/isAdmin/jti/admittedAppIds，避免多次解析。
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
