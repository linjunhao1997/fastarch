// Package cipher 提供 AES-256-GCM 静态加解密能力（specs/009-ak-sk-auth R2）：
// SK 等需可还原的敏感数据以密文落库，密钥经环境变量注入。
package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	keySize    = 32 // AES-256
	nonceSize  = 12 // GCM 标准 nonce 长度
	hexKeyLen = keySize * 2
)

// AESGCM 是 AES-256-GCM 加解密器。密文格式：nonce(12B) || ciphertext，随机 nonce 前置拼接。
type AESGCM struct {
	aead cipher.AEAD
}

// NewAESGCM 用 32 字节原始密钥构造。
func NewAESGCM(key []byte) (*AESGCM, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("aes key must be %d bytes, got %d", keySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &AESGCM{aead: aead}, nil
}

// NewAESGCMFromEncodedKey 从编码密钥构造，依次尝试：64 字符 hex、base64、32 字节原始字符串。
// 密钥来源为环境变量/配置，编码形式不约定死，三者均支持。
func NewAESGCMFromEncodedKey(encoded string) (*AESGCM, error) {
	if encoded == "" {
		return nil, errors.New("encrypt key is empty")
	}
	if len(encoded) == hexKeyLen {
		if key, err := hex.DecodeString(encoded); err == nil {
			return NewAESGCM(key)
		}
	}
	if key, err := base64.StdEncoding.DecodeString(encoded); err == nil && len(key) == keySize {
		return NewAESGCM(key)
	}
	if len(encoded) == keySize {
		return NewAESGCM([]byte(encoded))
	}
	return nil, errors.New("encrypt key must be 32 raw bytes, 64-char hex, or base64 of 32 bytes")
}

// Encrypt 加密并返回 base64(nonce || ciphertext)。
func (a *AESGCM) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}
	sealed := a.aead.Seal(nil, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

// Decrypt 解密 base64(nonce || ciphertext) 密文。
func (a *AESGCM) Decrypt(ciphertext string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	if len(raw) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	plaintext, err := a.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return plaintext, nil
}
