package helper

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

func HashPasswordWithSalt(password string, saltStr string) string {
	saltBytes, _ := base64.StdEncoding.DecodeString(saltStr)

	// 将密码转为字节，与盐拼接
	hashInput := append([]byte(password), saltBytes...)

	// 计算 SHA256 哈希
	hash := sha256.Sum256(hashInput)
	return hex.EncodeToString(hash[:]) // 返回十六进制字符串
}
