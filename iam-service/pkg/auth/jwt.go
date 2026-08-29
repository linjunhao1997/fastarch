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
	jwt.RegisteredClaims
}

type ClaimsParam struct {
	UserId         string    `json:"userId"`
	Username       string    `json:"username"`
	ExpirationTime time.Time `json:"expirationTime"`
	IsAdmin        bool      `json:"isAdmin"`
}

func GenerateJWT(param ClaimsParam) (string, error) {
	claims := &CustomClaims{
		UserId:   param.UserId,
		Username: param.Username,
		IsAdmin:  param.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
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

func GetAccountIdFromJwt(tokenString string) (string, error) {
	if len(tokenString) == 0 {
		return "", errors.New("invalid token")
	}
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		return "", err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims.UserId, nil
	}
	return "", errors.New("invalid token")
}

func IsAdminFromJwt(tokenString string) (bool, error) {
	if len(tokenString) == 0 {
		return false, errors.New("invalid token")
	}
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		return false, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims.IsAdmin, nil
	}
	return false, errors.New("invalid token")
}
