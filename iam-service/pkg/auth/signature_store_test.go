package auth

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"

	"iam-service/pkg/cipher"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"
)

// newStoreTestEnv 构造 SignatureStore 直测环境（内存 SQLite + miniredis）。
func newStoreTestEnv(t *testing.T) *SignatureStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AccessCredential{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	aes, err := cipher.NewAESGCM([]byte(testAESKey))
	if err != nil {
		t.Fatalf("new aes: %v", err)
	}
	_, rdb := newTestSnapshotEnv(t, nil)
	return NewSignatureStore(db, rdb, aes, 300)
}

// seedStoreCredential 创建凭证行并返回明文 SK。
func seedStoreCredential(t *testing.T, store *SignatureStore, ak, sk string, status string) {
	t.Helper()
	aes, err := cipher.NewAESGCM([]byte(testAESKey))
	if err != nil {
		t.Fatalf("new aes: %v", err)
	}
	cipherText, err := aes.Encrypt([]byte(sk))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if err := store.db.Create(&model.AccessCredential{
		Id:              700,
		AccessKey:       ak,
		SecretKeyCipher: cipherText,
		AccountId:       7,
		Status:          status,
		Version:         1,
	}).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
}

func storeMD(ak, sk, nonce string) metadata.MD {
	md := signedMetadata(ak, sk, "GET", "/authentication/access-credentials", "", "", strconv.FormatInt(time.Now().Unix(), 10), nonce)
	md.Set(constant.MetadataGatewayOrigin, constant.GatewayOriginHTTP)
	return md
}

// T016：Authenticate 直测——重放拒绝、新 nonce 放行。
func TestSignatureStoreReplay(t *testing.T) {
	store := newStoreTestEnv(t)
	seedStoreCredential(t, store, "FAKstore0000000000000001", "sk-store", constant.AccessCredentialStatusEnabled)
	ctx := context.Background()

	if _, err := store.Authenticate(ctx, storeMD("FAKstore0000000000000001", "sk-store", "nonce-a")); err != nil {
		t.Fatalf("first authenticate should pass: %v", err)
	}
	if _, err := store.Authenticate(ctx, storeMD("FAKstore0000000000000001", "sk-store", "nonce-a")); err != ErrSignatureNonceReplay {
		t.Fatalf("replay error = %v, want ErrSignatureNonceReplay", err)
	}
	if _, err := store.Authenticate(ctx, storeMD("FAKstore0000000000000001", "sk-store", "nonce-b")); err != nil {
		t.Fatalf("new nonce should pass: %v", err)
	}
}

// T016：last_used_time 节流——窗口内首次验签写库，第二次跳过。
func TestSignatureStoreLastUsedThrottle(t *testing.T) {
	store := newStoreTestEnv(t)
	seedStoreCredential(t, store, "FAKlastused00000000000001", "sk-store", constant.AccessCredentialStatusEnabled)
	ctx := context.Background()

	if _, err := store.Authenticate(ctx, storeMD("FAKlastused00000000000001", "sk-store", "nonce-1")); err != nil {
		t.Fatalf("authenticate #1: %v", err)
	}
	// touchLastUsed 为异步 best-effort，轮询等待落库
	deadline := time.Now().Add(2 * time.Second)
	for {
		var row model.AccessCredential
		if err := store.db.Where(model.AccessCredentialColumnId, 700).First(&row).Error; err != nil {
			t.Fatalf("load credential: %v", err)
		}
		if row.LastUsedTime != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("last_used_time not updated within deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// 节流键已占位：清空 last_used_time 后再次验签不应立即写库
	if err := store.db.Model(&model.AccessCredential{}).Where(model.AccessCredentialColumnId, 700).
		Update(model.AccessCredentialColumnLastUsedTime, nil).Error; err != nil {
		t.Fatalf("clear last used: %v", err)
	}
	if _, err := store.Authenticate(ctx, storeMD("FAKlastused00000000000001", "sk-store", "nonce-2")); err != nil {
		t.Fatalf("authenticate #2: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	var row model.AccessCredential
	if err := store.db.Where(model.AccessCredentialColumnId, 700).First(&row).Error; err != nil {
		t.Fatalf("load credential: %v", err)
	}
	if row.LastUsedTime != nil {
		t.Fatal("last_used_time should be throttled within window")
	}
}
