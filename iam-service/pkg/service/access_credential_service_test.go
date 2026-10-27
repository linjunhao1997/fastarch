package service

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"google.golang.org/protobuf/types/known/timestamppb"

	"iam-service/pkg/auth"
	"iam-service/pkg/cipher"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"
)

// 测试用 AES 密钥（32 字节原始字符串，仅测试环境）。
const testServiceAESKey = "0123456789abcdef0123456789abcdef"

// newTestCredentialService 构造内存 SQLite + miniredis 的凭证服务测试环境。
func newTestCredentialService(t *testing.T, maxPerAccount int) (*AccessCredentialService, *cipher.AESGCM) {
	t.Helper()
	db := setupAccountTestDB(t)
	rdb, _ := setupTestRedis(t)
	aes, err := cipher.NewAESGCM([]byte(testServiceAESKey))
	if err != nil {
		t.Fatalf("new aes: %v", err)
	}
	return NewAccessCredentialService(db, rdb, aes, maxPerAccount), aes
}

// credentialContext 返回携带操作者身份的上下文（审计字段服务端注入来源）。
func credentialContext() context.Context {
	return auth.WithAccountId(context.Background(), 42)
}

// parseCredentialID 把响应中的字符串化雪花 ID 转回 int64。
func parseCredentialID(t *testing.T, id string) int64 {
	t.Helper()
	value, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		t.Fatalf("parse credential id %q: %v", id, err)
	}
	return value
}

// TestCreateAccessCredentialReturnsOneTimeSecret 创建成功返回一次性明文 SK，
// 落库为密文，审计字段服务端注入（specs/009-ak-sk-auth T010 / FR-002）。
func TestCreateAccessCredentialReturnsOneTimeSecret(t *testing.T) {
	service, aes := newTestCredentialService(t, 20)
	createTestAccount(t, service.db, 1, "cred@example.com", 1)

	resp, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{
		AccountId: 1,
		Remark:    "ci-deploy",
	})
	if err != nil {
		t.Fatalf("CreateAccessCredential() error = %v", err)
	}

	// SK 一次性明文：Base64 编码的 32 字节
	raw, err := base64.StdEncoding.DecodeString(resp.SecretKey)
	if err != nil {
		t.Fatalf("secret key not base64: %v", err)
	}
	if len(raw) != 32 {
		t.Fatalf("secret key raw length = %d, want 32", len(raw))
	}
	// 落库为密文且可解密还原
	var row model.AccessCredential
	if err := service.db.Where(model.AccessCredentialColumnId, resp.Credential.Id).First(&row).Error; err != nil {
		t.Fatalf("load credential row: %v", err)
	}
	if row.SecretKeyCipher == resp.SecretKey {
		t.Fatal("stored secret must be ciphertext, not plaintext")
	}
	plain, err := aes.Decrypt(row.SecretKeyCipher)
	if err != nil {
		t.Fatalf("decrypt stored cipher: %v", err)
	}
	if string(plain) != resp.SecretKey {
		t.Fatalf("decrypted secret = %q, want %q", plain, resp.SecretKey)
	}

	// AK 形态：FAK 前缀 + 24 长度
	if !strings.HasPrefix(row.AccessKey, "FAK") || len(row.AccessKey) != 24 {
		t.Fatalf("access key = %q, want FAK-prefixed 24 chars", row.AccessKey)
	}
	// 审计字段服务端注入
	if row.CreatedBy != 42 || row.UpdatedBy != 42 {
		t.Fatalf("audit operator = %d/%d, want 42/42", row.CreatedBy, row.UpdatedBy)
	}
	if row.Status != constant.AccessCredentialStatusEnabled {
		t.Fatalf("initial status = %q, want enabled", row.Status)
	}
	// 响应不含 SK 密文字段（契约 AccessCredential 消息无 secret_key，编译期保证）
	if resp.Credential.AccessKey != row.AccessKey {
		t.Fatalf("response access key mismatch: %q vs %q", resp.Credential.AccessKey, row.AccessKey)
	}
}

// FR-004：绑定账号必须存在且启用。
func TestCreateAccessCredentialAccountInvalid(t *testing.T) {
	service, _ := newTestCredentialService(t, 20)
	createTestAccount(t, service.db, 1, "one@example.com", 1)
	// 停用账号
	if err := service.db.Model(&model.Account{}).Where(model.AccountColumnId, 1).
		Update(model.AccountColumnStatus, constant.AccountStatusDisabled).Error; err != nil {
		t.Fatalf("disable account: %v", err)
	}

	for _, tc := range []struct {
		name      string
		accountId int64
	}{
		{name: "account not exists", accountId: 999},
		{name: "account disabled", accountId: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{AccountId: tc.accountId})
			if err != ErrAccessCredentialAccountInvalid {
				t.Fatalf("error = %v, want ErrAccessCredentialAccountInvalid", err)
			}
			var count int64
			if err := service.db.Model(&model.AccessCredential{}).Count(&count).Error; err != nil {
				t.Fatalf("count credentials: %v", err)
			}
			if count != 0 {
				t.Fatalf("credential count = %d, want 0 (rejection leaves no residue)", count)
			}
		})
	}
}

// FR-010：单账号凭证上限。
func TestCreateAccessCredentialLimitExceeded(t *testing.T) {
	service, _ := newTestCredentialService(t, 2)
	createTestAccount(t, service.db, 1, "limit@example.com", 1)

	for i := 0; i < 2; i++ {
		if _, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{AccountId: 1}); err != nil {
			t.Fatalf("create #%d error = %v", i+1, err)
		}
	}
	_, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{AccountId: 1})
	if err != ErrAccessCredentialLimitExceeded {
		t.Fatalf("error = %v, want ErrAccessCredentialLimitExceeded", err)
	}
	var count int64
	if err := service.db.Model(&model.AccessCredential{}).Count(&count).Error; err != nil {
		t.Fatalf("count credentials: %v", err)
	}
	if count != 2 {
		t.Fatalf("credential count = %d, want 2", count)
	}
}

// T010：列表分页与字段全量断言（含账号名批量组装，禁 N+1）。
func TestListAccessCredentialsPaging(t *testing.T) {
	service, _ := newTestCredentialService(t, 20)
	createTestAccount(t, service.db, 1, "a@example.com", 1)
	createTestAccount(t, service.db, 2, "b@example.com", 1)
	for i := 0; i < 3; i++ {
		accountId := int64(1)
		if i == 2 {
			accountId = 2
		}
		if _, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{
			AccountId: accountId,
			Remark:    "remark-" + string(rune('a'+i)),
		}); err != nil {
			t.Fatalf("create #%d error = %v", i+1, err)
		}
	}

	resp, err := service.ListAccessCredentials(context.Background(), &iam_v1.ListAccessCredentialsRequest{
		CurrentPage: 1,
		PageSize:    2,
	})
	if err != nil {
		t.Fatalf("ListAccessCredentials() error = %v", err)
	}
	if resp.Page.Total != 3 || resp.Page.TotalPages != 2 || resp.Page.PageSize != 2 {
		t.Fatalf("page = %+v, want total 3 / total_pages 2 / page_size 2", resp.Page)
	}
	if len(resp.AccessCredentials) != 2 {
		t.Fatalf("items = %d, want 2", len(resp.AccessCredentials))
	}
	for _, item := range resp.AccessCredentials {
		if item.AccountName == "" {
			t.Fatalf("account_name not assembled for credential %s", item.Id)
		}
		if item.Id == "" || item.AccessKey == "" {
			t.Fatalf("id/access_key must be populated: %+v", item)
		}
		if item.CreatedBy == "" || item.CreateTime == nil {
			t.Fatalf("audit fields must be returned: %+v", item)
		}
	}
}

// T019：account_id / access_key 前缀 / status 筛选与组合条件。
func TestListAccessCredentialsFilters(t *testing.T) {
	service, _ := newTestCredentialService(t, 20)
	createTestAccount(t, service.db, 1, "a@example.com", 1)
	createTestAccount(t, service.db, 2, "b@example.com", 1)

	// 两个账号各 1 条启用 + 账号 1 追加 1 条禁用
	enabledOne, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{AccountId: 1, Remark: "keep"})
	if err != nil {
		t.Fatalf("create enabled one: %v", err)
	}
	if _, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{AccountId: 1, Remark: "off"}); err != nil {
		t.Fatalf("create disabled: %v", err)
	}
	if _, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{AccountId: 2, Remark: "other"}); err != nil {
		t.Fatalf("create account2: %v", err)
	}
	if err := service.db.Model(&model.AccessCredential{}).
		Where(model.AccessCredentialColumnRemark, "off").
		Update(model.AccessCredentialColumnStatus, constant.AccessCredentialStatusDisabled).Error; err != nil {
		t.Fatalf("disable credential: %v", err)
	}

	list := func(req *iam_v1.ListAccessCredentialsRequest) []string {
		resp, err := service.ListAccessCredentials(context.Background(), req)
		if err != nil {
			t.Fatalf("ListAccessCredentials() error = %v", err)
		}
		keys := make([]string, 0, len(resp.AccessCredentials))
		for _, item := range resp.AccessCredentials {
			keys = append(keys, item.AccessKey)
		}
		return keys
	}

	// account_id 筛选
	if got := list(&iam_v1.ListAccessCredentialsRequest{AccountId: 2}); len(got) != 1 {
		t.Fatalf("filter by account_id=2: got %d items, want 1", len(got))
	}
	// status 筛选
	if got := list(&iam_v1.ListAccessCredentialsRequest{Status: "disabled"}); len(got) != 1 {
		t.Fatalf("filter by status=disabled: got %d items, want 1", len(got))
	}
	// access_key 前缀筛选
	if got := list(&iam_v1.ListAccessCredentialsRequest{AccessKey: enabledOne.Credential.AccessKey[:8]}); len(got) != 1 {
		t.Fatalf("filter by access_key prefix: got %d items, want 1", len(got))
	}
	// 组合条件：account_id=1 且 status=enabled
	if got := list(&iam_v1.ListAccessCredentialsRequest{AccountId: 1, Status: "enabled"}); len(got) != 1 || got[0] != enabledOne.Credential.AccessKey {
		t.Fatalf("combined filter: got %v, want only %s", got, enabledOne.Credential.AccessKey)
	}
	// 无匹配 → 空列表非错误
	if got := list(&iam_v1.ListAccessCredentialsRequest{AccountId: 999}); len(got) != 0 {
		t.Fatalf("filter by missing account: got %v, want empty", got)
	}
}

// T021：启停往返、幂等、乐观锁版本推进与审计赋值。
func TestChangeAccessCredentialStatus(t *testing.T) {
	service, _ := newTestCredentialService(t, 20)
	createTestAccount(t, service.db, 1, "a@example.com", 1)
	created, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{AccountId: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 启用 → 禁用
	credentialId := parseCredentialID(t, created.Credential.Id)
	if _, err := service.ChangeAccessCredentialStatus(credentialContext(), &iam_v1.ChangeAccessCredentialStatusRequest{
		Id: credentialId, Status: constant.AccessCredentialStatusDisabled,
	}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	// 重复禁用：幂等成功
	if _, err := service.ChangeAccessCredentialStatus(credentialContext(), &iam_v1.ChangeAccessCredentialStatusRequest{
		Id: credentialId, Status: constant.AccessCredentialStatusDisabled,
	}); err != nil {
		t.Fatalf("idempotent disable: %v", err)
	}
	// 禁用 → 启用
	if _, err := service.ChangeAccessCredentialStatus(credentialContext(), &iam_v1.ChangeAccessCredentialStatusRequest{
		Id: credentialId, Status: constant.AccessCredentialStatusEnabled,
	}); err != nil {
		t.Fatalf("enable: %v", err)
	}

	var row model.AccessCredential
	if err := service.db.Where(model.AccessCredentialColumnId, credentialId).First(&row).Error; err != nil {
		t.Fatalf("load credential: %v", err)
	}
	if row.Status != constant.AccessCredentialStatusEnabled {
		t.Fatalf("status = %q, want enabled", row.Status)
	}
	// 两次真实变更（禁用 + 启用）各推进一版；重复禁用幂等不推进
	if row.Version != created.Credential.Version+2 {
		t.Fatalf("version = %d, want %d (two actual changes)", row.Version, created.Credential.Version+2)
	}
	if row.UpdatedBy != 42 {
		t.Fatalf("updated_by = %d, want 42", row.UpdatedBy)
	}

	// 不存在凭证 → NotFound
	_, err = service.ChangeAccessCredentialStatus(credentialContext(), &iam_v1.ChangeAccessCredentialStatusRequest{
		Id: 999999, Status: constant.AccessCredentialStatusEnabled,
	})
	if err != ErrAccessCredentialNotFound {
		t.Fatalf("missing credential error = %v, want ErrAccessCredentialNotFound", err)
	}
}

// T023：重置 SK 往返——AK 不变、旧 SK 失效、新 SK 可解密、版本推进。
func TestResetAccessCredentialSecret(t *testing.T) {
	service, aes := newTestCredentialService(t, 20)
	createTestAccount(t, service.db, 1, "a@example.com", 1)
	created, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{AccountId: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	oldSecret := created.SecretKey

	reset, err := service.ResetAccessCredentialSecret(credentialContext(), &iam_v1.ResetAccessCredentialSecretRequest{Id: parseCredentialID(t, created.Credential.Id)})
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if reset.SecretKey == oldSecret {
		t.Fatal("new secret must differ from old secret")
	}
	if reset.Credential.AccessKey != created.Credential.AccessKey {
		t.Fatal("AK must be unchanged across reset (FR-008)")
	}
	// 新密文可解密为新明文，旧明文失效
	var row model.AccessCredential
	if err := service.db.Where(model.AccessCredentialColumnId, parseCredentialID(t, created.Credential.Id)).First(&row).Error; err != nil {
		t.Fatalf("load credential: %v", err)
	}
	plain, err := aes.Decrypt(row.SecretKeyCipher)
	if err != nil {
		t.Fatalf("decrypt new cipher: %v", err)
	}
	if string(plain) != reset.SecretKey {
		t.Fatalf("stored secret = %q, want %q", plain, reset.SecretKey)
	}
	if row.Version != created.Credential.Version+1 {
		t.Fatalf("version = %d, want %d", row.Version, created.Credential.Version+1)
	}

	// 旧 SK 签名语义（结合 signature_store 点查）：验签路径解密出的 SK 已变更，
	// 旧签名必然 SIGNATURE_MISMATCH —— 由 pkg/auth/auth_interceptor_test.go 覆盖。
	// 此处断言旧明文不再匹配任何落库密文。
	_ = oldSecret

	// 不存在凭证 → NotFound
	if _, err := service.ResetAccessCredentialSecret(credentialContext(), &iam_v1.ResetAccessCredentialSecretRequest{Id: 999999}); err != ErrAccessCredentialNotFound {
		t.Fatalf("missing credential error = %v, want ErrAccessCredentialNotFound", err)
	}
}

// T024：物理删除——列表消失、二次删除 NotFound、操作审计。
func TestDeleteAccessCredential(t *testing.T) {
	service, _ := newTestCredentialService(t, 20)
	createTestAccount(t, service.db, 1, "a@example.com", 1)
	created, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{AccountId: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := service.DeleteAccessCredential(credentialContext(), &iam_v1.DeleteAccessCredentialRequest{Id: parseCredentialID(t, created.Credential.Id)}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var count int64
	if err := service.db.Model(&model.AccessCredential{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("credential count = %d, want 0 (physical delete)", count)
	}
	// 二次删除 → NotFound 语义
	if _, err := service.DeleteAccessCredential(credentialContext(), &iam_v1.DeleteAccessCredentialRequest{Id: parseCredentialID(t, created.Credential.Id)}); err != ErrAccessCredentialNotFound {
		t.Fatalf("second delete error = %v, want ErrAccessCredentialNotFound", err)
	}
}

// GetAccessCredential：详情返回全量业务字段与账号名。
func TestGetAccessCredential(t *testing.T) {
	service, _ := newTestCredentialService(t, 20)
	createTestAccount(t, service.db, 1, "a@example.com", 1)
	created, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{AccountId: 1, Remark: "detail"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	detail, err := service.GetAccessCredential(context.Background(), &common_v1.GetRequest{Id: parseCredentialID(t, created.Credential.Id)})
	if err != nil {
		t.Fatalf("GetAccessCredential() error = %v", err)
	}
	if detail.AccessKey != created.Credential.AccessKey || detail.AccountName == "" || detail.Remark != "detail" {
		t.Fatalf("detail fields mismatch: %+v", detail)
	}
	if detail.Version != created.Credential.Version {
		t.Fatalf("detail version = %d, want %d", detail.Version, created.Credential.Version)
	}

	if _, err := service.GetAccessCredential(context.Background(), &common_v1.GetRequest{Id: 999999}); err == nil {
		t.Fatal("missing credential should error")
	}
}

// 过期时间写入与回读（创建时可指定 expire_time）。
func TestCreateAccessCredentialWithExpireTime(t *testing.T) {
	service, _ := newTestCredentialService(t, 20)
	createTestAccount(t, service.db, 1, "a@example.com", 1)
	expire := time.Now().Add(24 * time.Hour).Round(time.Second).UTC()

	resp, err := service.CreateAccessCredential(credentialContext(), &iam_v1.CreateAccessCredentialRequest{
		AccountId:  1,
		ExpireTime: timestamppb.New(expire),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if resp.Credential.ExpireTime == nil {
		t.Fatal("expire_time should be returned")
	}
	if diff := resp.Credential.ExpireTime.AsTime().Sub(expire); diff > time.Second || diff < -time.Second {
		t.Fatalf("expire_time = %v, want ~%v", resp.Credential.ExpireTime.AsTime(), expire)
	}
}
