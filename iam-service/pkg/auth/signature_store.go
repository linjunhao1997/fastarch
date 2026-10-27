package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"

	"iam-service/pkg/cipher"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"
)

const (
	// nonceKeyPrefix 防重放键前缀（data-model.md：aksk:nonce:<ak>:<nonce>，TTL = 时间戳窗口）。
	nonceKeyPrefix = "aksk:nonce:"
	// lastUsedKeyPrefix last_used_time 节流标记键前缀（R6：aksk:lastused:<ak>，EX 60s）。
	lastUsedKeyPrefix = "aksk:lastused:"
	// lastUsedThrottleWindow 是 last_used_time 的节流窗口（秒）。
	lastUsedThrottleWindow = 60
)

// SignatureStore 是签名验签的凭证读取与状态判定（specs/009-ak-sk-auth T016）：
//   - AK→凭证每请求查库点查（唯一索引），保证禁用/过期/删除即时生效（R7）；
//   - SK 以 AES-256-GCM 解密还原后重算 HMAC；
//   - nonce 防重放走共享 Redis SETNX（R4，多实例一致）；
//   - last_used_time 节流更新（R6，best-effort 失败仅记日志）。
type SignatureStore struct {
	db            *gorm.DB
	rdb           *redis.Client
	aes           *cipher.AESGCM
	windowSeconds int
}

func NewSignatureStore(db *gorm.DB, rdb *redis.Client, aes *cipher.AESGCM, windowSeconds int) *SignatureStore {
	if windowSeconds <= 0 {
		windowSeconds = 300
	}
	return &SignatureStore{db: db, rdb: rdb, aes: aes, windowSeconds: windowSeconds}
}

// Authenticate 按规范第 4 节顺序完成验签（失败即拒绝 fail-closed），通过后返回绑定账号 id。
// 返回的 *SignatureError 为语义化验签错误（Unauthenticated + ErrorInfo reason）；
// 其余 error 为内部故障（Redis/DB/解密异常），由调用方统一按 fail-closed 处理。
func (s *SignatureStore) Authenticate(ctx context.Context, md metadata.MD) (int64, error) {
	// 1. 四要素头齐全（signature-method 可省，出现则必须为 HMAC-SHA256）
	accessKey := firstMetadataValue(md, MetadataAccessKey)
	timestamp := firstMetadataValue(md, MetadataTimestamp)
	nonce := firstMetadataValue(md, MetadataNonce)
	signature := firstMetadataValue(md, MetadataSignature)
	if accessKey == "" || timestamp == "" || nonce == "" || signature == "" {
		return 0, ErrSignatureMissing
	}
	if method := firstMetadataValue(md, MetadataSignatureMethod); method != "" && method != SignatureMethodHMACSHA256 {
		return 0, ErrSignatureMissing
	}
	// 网关内部透传要素：规范化串所需（gRPC 侧无法还原 HTTP 原始请求）
	originalMethod := firstMetadataValue(md, MetadataOriginalMethod)
	originalURI := firstMetadataValue(md, MetadataOriginalURI)
	bodyDigest := firstMetadataValue(md, MetadataBodyDigest)
	if originalMethod == "" || originalURI == "" || bodyDigest == "" {
		return 0, ErrSignatureMissing
	}

	// 2. AK 存在（唯一索引点查）
	var credential model.AccessCredential
	err := s.db.WithContext(ctx).
		Select(model.AccessCredentialFields).
		Where(model.AccessCredentialColumnAccessKey, accessKey).
		First(&credential).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrSignatureKeyUnknown
	}
	if err != nil {
		slog.Error("load access credential failed", "access_key", accessKey, "error", err)
		return 0, errors.New("load access credential failed")
	}

	// 3. 凭证启用
	if credential.Status != constant.AccessCredentialStatusEnabled {
		return 0, ErrSignatureKeyDisabled
	}

	// 4. 未过 expire_time
	if credential.ExpireTime != nil && credential.ExpireTime.Before(time.Now()) {
		return 0, ErrSignatureKeyExpired
	}

	// 5. 时间戳窗口
	if !IsTimestampInWindow(timestamp, time.Now(), s.windowSeconds) {
		return 0, ErrSignatureTimestampExpired
	}

	// 6. nonce 防重放：SETNX 原子占位（多实例一致，R4）；Redis 故障 fail-closed
	nonceKey := nonceKeyPrefix + accessKey + ":" + nonce
	ok, err := s.rdb.SetNX(ctx, nonceKey, "1", time.Duration(s.windowSeconds)*time.Second).Result()
	if err != nil {
		slog.Error("nonce replay check failed", "access_key", accessKey, "error", err)
		return 0, errors.New("nonce replay check unavailable")
	}
	if !ok {
		return 0, ErrSignatureNonceReplay
	}

	// 7. HMAC 签名一致（常数时间比较）；解密失败为配置/数据异常，不透出细节
	secretKey, err := s.aes.Decrypt(credential.SecretKeyCipher)
	if err != nil {
		slog.Error("decrypt secret key failed", "access_key", accessKey, "error", err)
		return 0, errors.New("credential secret decrypt failed")
	}
	canonicalQuery := CanonicalizeQuery(firstMetadataValue(md, MetadataOriginalQuery))
	stringToSign := BuildStringToSign(originalMethod, originalURI, canonicalQuery, accessKey, timestamp, nonce, bodyDigest)
	if !VerifySignature(stringToSign, string(secretKey), signature) {
		return 0, ErrSignatureMismatch
	}

	s.touchLastUsed(credential)

	return credential.AccountId, nil
}

// touchLastUsed 节流更新 last_used_time（R6）：Redis SETNX EX 60 控制每凭证每分钟
// 最多一次写库；异步 best-effort，失败仅记日志不影响请求。
func (s *SignatureStore) touchLastUsed(credential model.AccessCredential) {
	ok, err := s.rdb.SetNX(context.WithoutCancel(context.Background()),
		lastUsedKeyPrefix+credential.AccessKey, "1", lastUsedThrottleWindow*time.Second).Result()
	if err != nil {
		slog.Warn("last used throttle mark failed", "access_key", credential.AccessKey, "error", err)
		return
	}
	if !ok {
		return // 节流窗口内，跳过写库
	}
	now := time.Now()
	err = s.db.WithContext(context.WithoutCancel(context.Background())).Model(model.AccessCredential{}).
		Where(model.AccessCredentialColumnId, credential.Id).
		Updates(map[string]interface{}{
			model.AccessCredentialColumnLastUsedTime: now,
			model.AccessCredentialColumnUpdateTime:   now,
			model.AccessCredentialColumnVersion:      credential.Version + 1,
		}).Error
	if err != nil {
		slog.Warn("update last used time failed", "access_key", credential.AccessKey, "error", err)
	}
}

func firstMetadataValue(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
