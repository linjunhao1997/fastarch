package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/model"
)

func setupAccountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := model.AutoMigrateModels(db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

// setupTestRedis 以 miniredis 构造测试用 Redis 客户端（research.md D8 v2）。
func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	return rdb, mr
}

func createTestUser(t *testing.T, db *gorm.DB, id int64, username string, version int64) model.User {
	t.Helper()
	now := time.Now()
	user := model.User{
		Id:         id,
		Username:   username,
		Nickname:   username,
		Status:     model.StatusEnabled,
		CreateTime: now,
		UpdateTime: now,
		Version:    version,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

// 用户名唯一：重复创建返回 ErrUsernameDuplicated（011 模型：username 唯一索引）。
func TestAccountUsernameUnique(t *testing.T) {
	db := setupAccountTestDB(t)
	createTestUser(t, db, 1, "alice", 0)
	service := NewAccountService(db, nil)

	_, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Username: "alice",
		Password: "password123",
	})
	if err != ErrUsernameDuplicated {
		t.Fatalf("CreateAccount() error = %v, want %v", err, ErrUsernameDuplicated)
	}
}

// 创建用户初始密码必填：缺省直接拒绝（创建路径无旧密码可校验）。
func TestCreateAccountPasswordRequired(t *testing.T) {
	db := setupAccountTestDB(t)
	service := NewAccountService(db, nil)

	_, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Username: "no-password",
	})
	if err != ErrPasswordRequired {
		t.Fatalf("CreateAccount() error = %v, want %v", err, ErrPasswordRequired)
	}
	var count int64
	if err := db.Model(model.User{}).Where(model.UserColumnUsername, "no-password").Count(&count).Error; err != nil {
		t.Fatalf("count user: %v", err)
	}
	if count != 0 {
		t.Fatal("user must not be created without initial password")
	}
}

// 创建用户：bcrypt 摘要落 password_hash，明文不落盘。
func TestCreateAccountHashesPassword(t *testing.T) {
	db := setupAccountTestDB(t)
	service := NewAccountService(db, nil)

	created, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Username: "hashed",
		Password: "password123",
		Nickname: "Hashed User",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if created.Username != "hashed" || created.Nickname != "Hashed User" {
		t.Fatalf("created account = %+v", created)
	}
	var user model.User
	if err := db.Where(model.UserColumnUsername, "hashed").First(&user).Error; err != nil {
		t.Fatalf("get user: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("password123")); err != nil {
		t.Fatalf("bcrypt verify password: %v", err)
	}
}

// email 可选唯一：重复 email 返回 ErrEmailDuplicated；不传 email（NULL）不冲突。
func TestCreateAccountEmailUnique(t *testing.T) {
	db := setupAccountTestDB(t)
	service := NewAccountService(db, nil)

	first, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Username: "first",
		Password: "password123",
		Email:    "first@example.com",
	})
	if err != nil {
		t.Fatalf("CreateAccount() #1 error = %v", err)
	}
	if first.Email != "first@example.com" {
		t.Fatalf("created email = %q, want first@example.com", first.Email)
	}

	// 重复 email 被拒绝
	_, err = service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Username: "second",
		Password: "password123",
		Email:    "first@example.com",
	})
	if err != ErrEmailDuplicated {
		t.Fatalf("CreateAccount() #2 error = %v, want %v", err, ErrEmailDuplicated)
	}

	// 不传 email（NULL）的多个用户互不冲突
	for _, username := range []string{"third", "fourth"} {
		if _, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
			Username: username,
			Password: "password123",
		}); err != nil {
			t.Fatalf("CreateAccount() %s error = %v", username, err)
		}
	}
}

// 创建用户：role_ids 初始角色绑定生效。
func TestCreateAccountWithRoleIds(t *testing.T) {
	db := setupAccountTestDB(t)
	role := model.Role{Id: 11, Name: "role", Code: "role", Status: model.StatusEnabled}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	service := NewAccountService(db, nil)

	created, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Username: "with-roles",
		Password: "password123",
		RoleIds:  []string{cast.ToString(role.Id)},
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if len(created.RoleCodeList) != 1 || created.RoleCodeList[0] != "role" {
		t.Fatalf("RoleCodeList = %v, want [role]", created.RoleCodeList)
	}
}

func TestListAccountsKeyword(t *testing.T) {
	db := setupAccountTestDB(t)
	createTestUser(t, db, 1, "alpha", 0)
	createTestUser(t, db, 2, "beta", 0)
	service := NewAccountService(db, nil)

	response, err := service.ListAccounts(context.Background(), &iam_v1.ListAccountsRequest{Keyword: "alpha"})
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}
	if len(response.Accounts) != 1 || response.Accounts[0].Username != "alpha" {
		t.Fatalf("ListAccounts() = %v, want alpha", response.Accounts)
	}
}

// keyword 命中 email（双标识词汇全覆盖）。
func TestListAccountsKeywordMatchesEmail(t *testing.T) {
	db := setupAccountTestDB(t)
	email := "alpha@example.com"
	user := createTestUser(t, db, 1, "alpha", 0)
	user.Email = &email
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("save user email: %v", err)
	}
	createTestUser(t, db, 2, "beta", 0)
	service := NewAccountService(db, nil)

	response, err := service.ListAccounts(context.Background(), &iam_v1.ListAccountsRequest{Keyword: "alpha@example"})
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}
	if len(response.Accounts) != 1 || response.Accounts[0].Username != "alpha" {
		t.Fatalf("ListAccounts() = %v, want alpha", response.Accounts)
	}
}

func TestUpdateAccount(t *testing.T) {
	tests := []struct {
		name      string
		id        int64
		version   int64
		wantErr   error
		wantFound bool
	}{
		{name: "success increments version", id: 1, version: 2, wantErr: nil, wantFound: true},
		{name: "stale version rejected", id: 1, version: 1, wantErr: ErrUpdateAccountBusy, wantFound: true},
		{name: "missing account not found", id: 999, version: 1, wantErr: gorm.ErrRecordNotFound, wantFound: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupAccountTestDB(t)
			createTestUser(t, db, 1, "one", 2)
			service := NewAccountService(db, nil)

			_, err := service.UpdateAccount(context.Background(), &iam_v1.UpdateAccountRequest{
				Id:       test.id,
				Nickname: "updated-nickname",
				Version:  test.version,
			})
			if err != test.wantErr {
				t.Fatalf("UpdateAccount() error = %v, want %v", err, test.wantErr)
			}
			if !test.wantFound {
				return
			}

			var user model.User
			if err := db.First(&user, test.id).Error; err != nil {
				t.Fatalf("get user: %v", err)
			}
			if test.wantErr == nil {
				if user.Version != 3 {
					t.Fatalf("version = %d, want 3", user.Version)
				}
				// nickname 可改；username 为唯一登录名不可改
				if user.Nickname != "updated-nickname" {
					t.Fatalf("nickname = %q, want updated", user.Nickname)
				}
				if user.Username != "one" {
					t.Fatalf("username = %q, want unchanged one", user.Username)
				}
				return
			}
			if user.Version != 2 {
				t.Fatalf("version = %d, want unchanged 2", user.Version)
			}
		})
	}
}

// UpdateAccount email 唯一冲突转友好错误。
func TestUpdateAccountEmailDuplicated(t *testing.T) {
	db := setupAccountTestDB(t)
	email := "taken@example.com"
	one := createTestUser(t, db, 1, "one", 1)
	two := createTestUser(t, db, 2, "two", 1)
	one.Email = &email
	if err := db.Save(&one).Error; err != nil {
		t.Fatalf("save email: %v", err)
	}
	service := NewAccountService(db, nil)

	_, err := service.UpdateAccount(context.Background(), &iam_v1.UpdateAccountRequest{
		Id:      two.Id,
		Email:   email,
		Version: 1,
	})
	if err != ErrEmailDuplicated {
		t.Fatalf("UpdateAccount() error = %v, want %v", err, ErrEmailDuplicated)
	}

	// 自己沿用原 email 不冲突（排除自身）
	_, err = service.UpdateAccount(context.Background(), &iam_v1.UpdateAccountRequest{
		Id:      one.Id,
		Email:   email,
		Version: 1,
	})
	if err != nil {
		t.Fatalf("UpdateAccount() self email error = %v, want nil", err)
	}
}

// ResetAccountPassword：新密码 bcrypt 落库，version+1，审计注入。
func TestResetAccountPassword(t *testing.T) {
	ctx := context.WithValue(context.Background(), "accountId", "42")
	db := setupAccountTestDB(t)
	createTestUser(t, db, 1, "one", 1)
	service := NewAccountService(db, nil)

	if _, err := service.ResetAccountPassword(ctx, &iam_v1.ResetAccountPasswordRequest{
		Id:          1,
		NewPassword: "new-secret-password",
	}); err != nil {
		t.Fatalf("ResetAccountPassword() error = %v", err)
	}

	var user model.User
	if err := db.First(&user, 1).Error; err != nil {
		t.Fatalf("get user: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("new-secret-password")); err != nil {
		t.Fatalf("bcrypt verify new password: %v", err)
	}
	if user.Version != 2 {
		t.Fatalf("version = %d, want 2", user.Version)
	}
	if user.UpdatedBy != "42" {
		t.Fatalf("updated_by = %q, want 42", user.UpdatedBy)
	}

	// 用户不存在
	if _, err := service.ResetAccountPassword(ctx, &iam_v1.ResetAccountPasswordRequest{
		Id:          999,
		NewPassword: "new-secret-password",
	}); err != gorm.ErrRecordNotFound {
		t.Fatalf("ResetAccountPassword() missing user error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
}

func TestUpdateAccountRoles(t *testing.T) {
	db := setupAccountTestDB(t)
	user := createTestUser(t, db, 1, "one", 1)
	roles := []model.Role{
		{Id: 1, Name: "role one", Code: "role-one", Status: model.StatusEnabled},
		{Id: 2, Name: "role two", Code: "role-two", Status: model.StatusEnabled},
	}
	for _, role := range roles {
		if err := db.Create(&role).Error; err != nil {
			t.Fatalf("create role: %v", err)
		}
	}
	service := NewAccountService(db, nil)

	_, err := service.UpdateAccountRoles(context.Background(), &iam_v1.UpdateAccountRolesRequest{
		AccountId:         user.Id,
		AddedRoleCodeList: []string{"role-one", "role-missing"},
	})
	if err != ErrRoleNotFound {
		t.Fatalf("UpdateAccountRoles() error = %v, want %v", err, ErrRoleNotFound)
	}

	_, err = service.UpdateAccountRoles(context.Background(), &iam_v1.UpdateAccountRolesRequest{
		AccountId:         user.Id,
		AddedRoleCodeList: []string{"role-one", "role-two"},
	})
	if err != nil {
		t.Fatalf("UpdateAccountRoles() error = %v", err)
	}

	var got model.User
	if err := db.Preload("RoleList").First(&got, user.Id).Error; err != nil {
		t.Fatalf("get user: %v", err)
	}
	if len(got.RoleList) != 2 {
		t.Fatalf("role count = %d, want 2", len(got.RoleList))
	}

	_, err = service.UpdateAccountRoles(context.Background(), &iam_v1.UpdateAccountRolesRequest{
		AccountId:           user.Id,
		RemovedRoleCodeList: []string{"role-missing"},
	})
	if err != nil {
		t.Fatalf("remove missing role error = %v, want nil", err)
	}
}

func TestDeleteAccount(t *testing.T) {
	tests := []struct {
		name     string
		id       int64
		withRole bool
		wantErr  error
	}{
		{name: "unbound account deleted", id: 1},
		{name: "bound account rejected", id: 2, withRole: true, wantErr: ErrAccountReferenced},
		{name: "missing account rejected", id: 999, wantErr: gorm.ErrRecordNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupAccountTestDB(t)
			var user model.User
			if test.id != 999 {
				user = createTestUser(t, db, test.id, "one", 1)
			}
			if test.withRole {
				role := model.Role{Id: 10, Name: "role", Code: "role", Status: model.StatusEnabled}
				if err := db.Create(&role).Error; err != nil {
					t.Fatalf("create role: %v", err)
				}
				if err := db.Model(&user).Association("RoleList").Append(&role); err != nil {
					t.Fatalf("bind role: %v", err)
				}
			}

			service := NewAccountService(db, nil)
			_, err := service.DeleteAccount(context.Background(), &iam_v1.DeleteAccountRequest{Id: test.id})
			if err != test.wantErr {
				t.Fatalf("DeleteAccount() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}

			var count int64
			if err := db.Model(model.User{}).Where(model.UserColumnId, test.id).Count(&count).Error; err != nil {
				t.Fatalf("count user: %v", err)
			}
			if count != 0 {
				t.Fatalf("user count = %d, want 0", count)
			}
		})
	}
}

// specs/009-ak-sk-auth T025：删除用户前名下凭证数须为 0（引用完整性检查点）。
func TestDeleteAccountReferencedByCredential(t *testing.T) {
	db := setupAccountTestDB(t)
	user := createTestUser(t, db, 1, "cred", 1)
	credential := model.AccessCredential{
		Id:              900,
		AccessKey:       "FAKdeleteguard0000001",
		SecretKeyCipher: "cipher",
		UserId:          user.Id,
		Status:          constant.AccessCredentialStatusEnabled,
		Version:         1,
	}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	service := NewAccountService(db, nil)
	_, err := service.DeleteAccount(context.Background(), &iam_v1.DeleteAccountRequest{Id: user.Id})
	if err != ErrAccountReferencedByCredential {
		t.Fatalf("DeleteAccount() error = %v, want %v", err, ErrAccountReferencedByCredential)
	}

	var count int64
	if err := db.Model(model.User{}).Where(model.UserColumnId, user.Id).Count(&count).Error; err != nil {
		t.Fatalf("count user: %v", err)
	}
	if count != 1 {
		t.Fatalf("user count = %d, want 1 (deletion must be rejected)", count)
	}

	// 清空凭证后删除放行
	if err := db.Where(model.AccessCredentialColumnUserId, user.Id).Delete(&model.AccessCredential{}).Error; err != nil {
		t.Fatalf("delete credentials: %v", err)
	}
	if _, err := service.DeleteAccount(context.Background(), &iam_v1.DeleteAccountRequest{Id: user.Id}); err != nil {
		t.Fatalf("DeleteAccount() after credentials removed error = %v, want nil", err)
	}
}

// T044-① UpdateAccountRoles 成功后目标账号的权限快照键被删除（D6-①）。
func TestUpdateAccountRolesInvalidatesPermissionCache(t *testing.T) {
	ctx := context.Background()
	db := setupAccountTestDB(t)
	rdb, mr := setupTestRedis(t)
	user := createTestUser(t, db, 1, "one", 1)
	role := model.Role{Id: 1, Name: "role one", Code: "role-one", Status: model.StatusEnabled}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}

	key := constant.GetAccountPermissionKey(user.Id)
	if err := rdb.Set(ctx, key, "[]", time.Minute).Err(); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	service := NewAccountService(db, rdb)
	if _, err := service.UpdateAccountRoles(ctx, &iam_v1.UpdateAccountRolesRequest{
		AccountId:         user.Id,
		AddedRoleCodeList: []string{"role-one"},
	}); err != nil {
		t.Fatalf("UpdateAccountRoles() error = %v", err)
	}
	if mr.Exists(key) {
		t.Fatalf("permission snapshot key %q still exists, want deleted", key)
	}
}

// T044-④ 重复绑定幂等：关联表无重复行（FR-14、D7）。
func TestUpdateAccountRolesIdempotentNoDuplicateRows(t *testing.T) {
	db := setupAccountTestDB(t)
	user := createTestUser(t, db, 1, "one", 1)
	role := model.Role{Id: 1, Name: "role one", Code: "role-one", Status: model.StatusEnabled}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}

	service := NewAccountService(db, nil)
	request := &iam_v1.UpdateAccountRolesRequest{
		AccountId:         user.Id,
		AddedRoleCodeList: []string{"role-one"},
	}
	for i := 0; i < 2; i++ {
		if _, err := service.UpdateAccountRoles(context.Background(), request); err != nil {
			t.Fatalf("UpdateAccountRoles() #%d error = %v", i+1, err)
		}
	}

	var count int64
	if err := db.Model(model.RUserRole{}).
		Where(model.RUserRoleColumnUserId, user.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("count binding rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("binding rows = %d, want 1", count)
	}
}

// T044-③ 读穿回源：空快照下重建，快照按权限码并集写回且 TTL≈5min（D5/D6-②/D6-③）。
func TestLoadPermissionSnapshotReadThrough(t *testing.T) {
	ctx := context.Background()
	db := setupAccountTestDB(t)
	rdb, mr := setupTestRedis(t)
	user := createTestUser(t, db, 1, "one", 1)
	role := model.Role{Id: 1, Name: "role one", Code: "role-one", Status: model.StatusEnabled}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	permission := model.Permission{Id: 1, Name: "perm one", Code: "iam.user.list", Status: model.StatusEnabled}
	if err := db.Create(&permission).Error; err != nil {
		t.Fatalf("create permission: %v", err)
	}
	if err := db.Model(&role).Association("PermissionList").Append(&permission); err != nil {
		t.Fatalf("bind permission: %v", err)
	}
	if err := db.Model(&user).Association("RoleList").Append(&role); err != nil {
		t.Fatalf("bind role: %v", err)
	}

	key := constant.GetAccountPermissionKey(user.Id)
	if mr.Exists(key) {
		t.Fatalf("precondition: snapshot key %q should not exist", key)
	}

	codes, err := auth.LoadPermissionSnapshot(ctx, rdb, user.Id, NewPermissionSnapshotLoader(db))
	if err != nil {
		t.Fatalf("LoadPermissionSnapshot() error = %v", err)
	}
	want := "iam.user.list"
	found := false
	for _, code := range codes {
		if code == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("rebuilt codes = %v, want contains %q", codes, want)
	}

	str, err := rdb.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("snapshot not written back: %v", err)
	}
	var stored []string
	if err := json.Unmarshal([]byte(str), &stored); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if len(stored) != 1 || stored[0] != want {
		t.Fatalf("stored snapshot = %v, want [%s]", stored, want)
	}
	ttl := mr.TTL(key)
	if ttl <= 4*time.Minute || ttl > constant.AccountPermissionSnapshotTTL {
		t.Fatalf("snapshot ttl = %v, want ≈ %v", ttl, constant.AccountPermissionSnapshotTTL)
	}
}

// T060 审计字段注入（research.md D16）：Create 注入 created_by/updated_by（取
// 认证上下文操作者）；Update 仅更新 update_time/updated_by，created_by 保持不变；
// Get 回填审计字段。
func TestAccountAuditFields(t *testing.T) {
	createCtx := context.WithValue(context.Background(), "accountId", "42")
	updateCtx := context.WithValue(context.Background(), "accountId", "43")
	db := setupAccountTestDB(t)
	service := NewAccountService(db, nil)

	created, err := service.CreateAccount(createCtx, &iam_v1.CreateAccountRequest{
		Username: "audit-user",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if created.CreatedBy != "42" || created.UpdatedBy != "42" {
		t.Fatalf("created_by/updated_by = %q/%q, want 42/42", created.CreatedBy, created.UpdatedBy)
	}
	if created.CreateTime == nil || created.UpdateTime == nil {
		t.Fatalf("create_time/update_time = %v/%v, want both set", created.CreateTime, created.UpdateTime)
	}

	accountId := cast.ToInt64(created.Id)
	_, err = service.UpdateAccount(updateCtx, &iam_v1.UpdateAccountRequest{
		Id:       accountId,
		Nickname: "updated",
		Version:  1,
	})
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}

	var got model.User
	if err := db.First(&got, accountId).Error; err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.CreatedBy != "42" || got.UpdatedBy != "43" {
		t.Fatalf("created_by/updated_by = %q/%q, want 42/43", got.CreatedBy, got.UpdatedBy)
	}
	if got.CreateTime.Unix() != created.CreateTime.AsTime().Unix() {
		t.Fatalf("create_time = %v, want unchanged %v", got.CreateTime, created.CreateTime.AsTime())
	}
	if got.UpdateTime.IsZero() {
		t.Fatal("update_time is zero, want updated")
	}

	detail, err := service.GetAccount(context.Background(), &common_v1.GetRequest{Id: accountId})
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if detail.CreatedBy != "42" || detail.UpdatedBy != "43" {
		t.Fatalf("GetAccount created_by/updated_by = %q/%q, want 42/43", detail.CreatedBy, detail.UpdatedBy)
	}
	if detail.CreateTime == nil || detail.UpdateTime == nil {
		t.Fatalf("GetAccount create_time/update_time = %v/%v, want both set", detail.CreateTime, detail.UpdateTime)
	}
}

// T060 补充：无认证上下文时操作者注入为空串（内部调用/迁移场景）。
func TestAccountAuditFieldsWithoutAuthContext(t *testing.T) {
	db := setupAccountTestDB(t)
	service := NewAccountService(db, nil)

	created, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Username: "no-auth-user",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if created.CreatedBy != "" && created.CreatedBy != "0" {
		t.Fatalf("created_by = %q, want empty/0 without auth context", created.CreatedBy)
	}
}
