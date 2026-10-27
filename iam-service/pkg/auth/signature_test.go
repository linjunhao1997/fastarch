package auth

import (
	"strings"
	"testing"
	"time"
)

const (
	testVectorSK        = "SkExampleKeyForUnitTest0123456789abcdef"
	testVectorAK        = "FAKtest0000000000000000"
	testVectorTimestamp = "1760000000"
	testVectorNonce     = "d3f1c2a4-nonce"
)

// 空请求体摘要向量（规范第 3.1 节给出）。
func TestSHA256HexEmptyBody(t *testing.T) {
	if got := SHA256Hex(nil); got != EmptyBodyDigestHex {
		t.Fatalf("SHA256Hex(nil) = %s, want %s", got, EmptyBodyDigestHex)
	}
}

// 规范示例复现向量：POST + body（独立用 PowerShell HMACSHA256 计算期望值）。
func TestSignatureVectorPOSTWithBody(t *testing.T) {
	body := `{"remark":"ci-deploy"}`
	bodyDigest := SHA256Hex([]byte(body))
	if bodyDigest != "dd750ff57f3323b9992e9b6b8b096ff7529ff1a60297635d1ecfda6ced39a8fc" {
		t.Fatalf("body digest = %s, want spec vector", bodyDigest)
	}
	sts := BuildStringToSign("POST", "/authentication/access-credentials", "", testVectorAK, testVectorTimestamp, testVectorNonce, bodyDigest)
	wantSts := strings.Join([]string{
		"POST", "/authentication/access-credentials", "", testVectorAK, testVectorTimestamp, testVectorNonce,
		"dd750ff57f3323b9992e9b6b8b096ff7529ff1a60297635d1ecfda6ced39a8fc",
	}, "\n")
	if sts != wantSts {
		t.Fatalf("string to sign = %q, want %q", sts, wantSts)
	}
	got := ComputeSignatureHex(sts, testVectorSK)
	want := "a5d910c9623e639cdc1029e6f38283ac2bab990d138985bda7a63608e1122a39"
	if got != want {
		t.Fatalf("signature = %s, want %s", got, want)
	}
	if !VerifySignature(sts, testVectorSK, got) {
		t.Fatal("VerifySignature should accept the correct signature")
	}
}

// GET + 查询串向量：CanonicalizeQuery 升序排序 + 空体摘要入签。
func TestSignatureVectorGETWithQuery(t *testing.T) {
	query := CanonicalizeQuery("b=2&a=1")
	if query != "a=1&b=2" {
		t.Fatalf("canonical query = %q, want a=1&b=2", query)
	}
	sts := BuildStringToSign("GET", "/authentication/access-credentials", query, testVectorAK, testVectorTimestamp, testVectorNonce, EmptyBodyDigestHex)
	got := ComputeSignatureHex(sts, testVectorSK)
	want := "0452f32ca08cad5f1e0f1bb91082fb14fd3119d73bc2239a6a11f2e22e3a097e"
	if got != want {
		t.Fatalf("signature = %s, want %s", got, want)
	}
}

// 篡改必败：签名、StringToSign 任一部分被改动均验证失败。
func TestVerifySignatureTamperRejected(t *testing.T) {
	sts := BuildStringToSign("POST", "/authentication/access-credentials", "", testVectorAK, testVectorTimestamp, testVectorNonce, EmptyBodyDigestHex)
	signature := ComputeSignatureHex(sts, testVectorSK)

	if VerifySignature(sts, testVectorSK, "0"+signature[1:]) {
		t.Fatal("tampered signature should be rejected")
	}
	if VerifySignature(sts, "wrong-secret-key", signature) {
		t.Fatal("wrong secret key should be rejected")
	}
	tamperedSts := strings.Replace(sts, "POST", "PUT", 1)
	if VerifySignature(tamperedSts, testVectorSK, signature) {
		t.Fatal("tampered string to sign should be rejected")
	}
}

// 查询串规范化：RFC 3986 编码（空格 → %20）、key 升序、重复 key 稳定排序。
func TestCanonicalizeQuery(t *testing.T) {
	tests := []struct {
		name     string
		rawQuery string
		want     string
	}{
		{name: "empty", rawQuery: "", want: ""},
		{name: "sorted", rawQuery: "b=2&a=1", want: "a=1&b=2"},
		{name: "space encoded rfc3986", rawQuery: "k=hello world", want: "k=hello%20world"},
		{name: "path separator escaped", rawQuery: "p=a/b", want: "p=a%2Fb"},
		{name: "pre-encoded once not twice", rawQuery: "k=a%20b", want: "k=a%20b"},
		{name: "duplicate keys sorted", rawQuery: "b=2&a=2&a=1", want: "a=1&a=2&b=2"},
		{name: "valueless key", rawQuery: "flag", want: "flag="},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanonicalizeQuery(test.rawQuery); got != test.want {
				t.Fatalf("CanonicalizeQuery(%q) = %q, want %q", test.rawQuery, got, test.want)
			}
		})
	}
}

// 时间戳窗口：窗口内通过，超出/非法拒绝。
func TestIsTimestampInWindow(t *testing.T) {
	now := time.Unix(1760000000, 0)
	tests := []struct {
		name      string
		timestamp string
		want      bool
	}{
		{name: "exact", timestamp: "1760000000", want: true},
		{name: "within future window", timestamp: "1760000299", want: true},
		{name: "beyond future window", timestamp: "1760000301", want: false},
		{name: "within past window", timestamp: "1759999701", want: true},
		{name: "beyond past window", timestamp: "1759999699", want: false},
		{name: "not a number", timestamp: "abc", want: false},
		{name: "empty", timestamp: "", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsTimestampInWindow(test.timestamp, now, 300); got != test.want {
				t.Fatalf("IsTimestampInWindow(%q) = %v, want %v", test.timestamp, got, test.want)
			}
		})
	}
}
