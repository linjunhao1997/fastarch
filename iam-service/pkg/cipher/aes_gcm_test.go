package cipher

import (
	"strings"
	"testing"
)

const testKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestAESGCMRoundTrip(t *testing.T) {
	a, err := NewAESGCMFromEncodedKey(testKeyHex)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	plaintext := []byte("secret-key-示例-1234567890")
	ciphertext, err := a.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if strings.Contains(ciphertext, string(plaintext)) {
		t.Fatal("ciphertext must not contain plaintext")
	}
	got, err := a.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("round trip mismatch: got %q", got)
	}
}

func TestAESGCMTamperedCiphertextFails(t *testing.T) {
	a, _ := NewAESGCMFromEncodedKey(testKeyHex)
	ciphertext, _ := a.Encrypt([]byte("secret"))
	// 篡改密文末尾一个 base64 字符
	tampered := flipLastChar(ciphertext)
	if _, err := a.Decrypt(tampered); err == nil {
		t.Fatal("tampered ciphertext must fail")
	}
}

func TestAESGCMWrongKeyFails(t *testing.T) {
	a1, _ := NewAESGCMFromEncodedKey(testKeyHex)
	otherKey := strings.Repeat("f", 64)
	a2, err := NewAESGCMFromEncodedKey(otherKey)
	if err != nil {
		t.Fatalf("new cipher with other key: %v", err)
	}
	ciphertext, _ := a1.Encrypt([]byte("secret"))
	if _, err := a2.Decrypt(ciphertext); err == nil {
		t.Fatal("decrypt with wrong key must fail")
	}
}

func TestAESGCMKeyFormats(t *testing.T) {
	// hex
	if _, err := NewAESGCMFromEncodedKey(testKeyHex); err != nil {
		t.Fatalf("hex key: %v", err)
	}
	// raw 32 bytes
	if _, err := NewAESGCMFromEncodedKey(strings.Repeat("a", 32)); err != nil {
		t.Fatalf("raw key: %v", err)
	}
	// 非法长度
	if _, err := NewAESGCMFromEncodedKey("short"); err == nil {
		t.Fatal("invalid key must fail")
	}
	if _, err := NewAESGCMFromEncodedKey(""); err == nil {
		t.Fatal("empty key must fail")
	}
}

func flipLastChar(s string) string {
	last := s[len(s)-1]
	replacement := byte('A')
	if last == 'A' {
		replacement = 'B'
	}
	return s[:len(s)-1] + string(replacement)
}
