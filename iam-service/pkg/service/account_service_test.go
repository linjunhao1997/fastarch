package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
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
	err = db.AutoMigrate(&model.Application{}, &model.Role{}, &model.Permission{}, &model.Account{}, &model.RAccountRole{}, &model.RAccountApplication{}, &model.Resource{}, &model.PermissionResource{})
	if err != nil {
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

func createTestAccount(t *testing.T, db *gorm.DB, id int64, email string, version int64) model.Account {
	t.Helper()
	now := time.Now()
	account := model.Account{
		Id:         id,
		Code:       "acc",
		Name:       "account",
		Email:      email,
		Status:     "enabled",
		Kind:       model.AccountKindIdUser,
		CreateTime: now,
		UpdateTime: now,
		Version:    version,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	return account
}

func TestAccountEmailUnique(t *testing.T) {
	db := setupAccountTestDB(t)
	createTestAccount(t, db, 1, "one@example.com", 0)
	service := NewAccountService(db, nil)

	_, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Name:  "account",
		Email: "one@example.com",
		Kind:  model.AccountKindIdUser,
	})
	if err != ErrEmailDuplicated {
		t.Fatalf("CreateAccount() error = %v, want %v", err, ErrEmailDuplicated)
	}
}

func TestListAccountsKeyword(t *testing.T) {
	db := setupAccountTestDB(t)
	createTestAccount(t, db, 1, "alpha@example.com", 0)
	createTestAccount(t, db, 2, "beta@example.com", 0)
	service := NewAccountService(db, nil)

	response, err := service.ListAccounts(context.Background(), &iam_v1.ListAccountsRequest{Keyword: "alpha"})
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}
	if len(response.Accounts) != 1 || response.Accounts[0].Email != "alpha@example.com" {
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
			createTestAccount(t, db, 1, "one@example.com", 2)
			service := NewAccountService(db, nil)

			_, err := service.UpdateAccount(context.Background(), &iam_v1.UpdateAccountRequest{
				Id:      test.id,
				Name:    "updated",
				Email:   "updated@example.com",
				Version: test.version,
			})
			if err != test.wantErr {
				t.Fatalf("UpdateAccount() error = %v, want %v", err, test.wantErr)
			}
			if !test.wantFound {
				return
			}

			var account model.Account
			if err := db.First(&account, test.id).Error; err != nil {
				t.Fatalf("get account: %v", err)
			}
			if test.wantErr == nil {
				if account.Version != 3 {
					t.Fatalf("version = %d, want 3", account.Version)
				}
				if account.Email != "updated@example.com" {
					t.Fatalf("email = %q, want updated", account.Email)
				}
				return
			}
			if account.Version != 2 {
				t.Fatalf("version = %d, want unchanged 2", account.Version)
			}
		})
	}
}

func TestUpdateAccountRoles(t *testing.T) {
	db := setupAccountTestDB(t)
	account := createTestAccount(t, db, 1, "one@example.com", 1)
	roles := []model.Role{
		{Id: 1, Name: "role one", Code: "role-one"},
		{Id: 2, Name: "role two", Code: "role-two"},
	}
	for _, role := range roles {
		if err := db.Create(&role).Error; err != nil {
			t.Fatalf("create role: %v", err)
		}
	}
	service := NewAccountService(db, nil)

	_, err := service.UpdateAccountRoles(context.Background(), &iam_v1.UpdateAccountRolesRequest{
		AccountId:         account.Id,
		AddedRoleCodeList: []string{"role-one", "role-missing"},
	})
	if err != ErrRoleNotFound {
		t.Fatalf("UpdateAccountRoles() error = %v, want %v", err, ErrRoleNotFound)
	}

	_, err = service.UpdateAccountRoles(context.Background(), &iam_v1.UpdateAccountRolesRequest{
		AccountId:         account.Id,
		AddedRoleCodeList: []string{"role-one", "role-two"},
	})
	if err != nil {
		t.Fatalf("UpdateAccountRoles() error = %v", err)
	}

	var got model.Account
	if err := db.Preload("RoleList").First(&got, account.Id).Error; err != nil {
		t.Fatalf("get account: %v", err)
	}
	if len(got.RoleList) != 2 {
		t.Fatalf("role count = %d, want 2", len(got.RoleList))
	}

	_, err = service.UpdateAccountRoles(context.Background(), &iam_v1.UpdateAccountRolesRequest{
		AccountId:           account.Id,
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
			var account model.Account
			if test.id != 999 {
				account = createTestAccount(t, db, test.id, "one@example.com", 1)
			}
			if test.withRole {
				role := model.Role{Id: 10, Name: "role", Code: "role"}
				if err := db.Create(&role).Error; err != nil {
					t.Fatalf("create role: %v", err)
				}
				if err := db.Model(&account).Association("RoleList").Append(&role); err != nil {
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
			if err := db.Model(model.Account{}).Where(model.AccountColumnId, test.id).Count(&count).Error; err != nil {
				t.Fatalf("count account: %v", err)
			}
			if count != 0 {
				t.Fatalf("account count = %d, want 0", count)
			}
		})
	}
}

// T044-① UpdateAccountRoles 成功后目标账号的权限快照键被删除（D6-①）。
func TestUpdateAccountRolesInvalidatesPermissionCache(t *testing.T) {
	ctx := context.Background()
	db := setupAccountTestDB(t)
	rdb, mr := setupTestRedis(t)
	account := createTestAccount(t, db, 1, "one@example.com", 1)
	role := model.Role{Id: 1, Name: "role one", Code: "role-one"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}

	key := constant.GetAccountPermissionKey(account.Id)
	if err := rdb.Set(ctx, key, "[]", time.Minute).Err(); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	service := NewAccountService(db, rdb)
	if _, err := service.UpdateAccountRoles(ctx, &iam_v1.UpdateAccountRolesRequest{
		AccountId:         account.Id,
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
	account := createTestAccount(t, db, 1, "one@example.com", 1)
	role := model.Role{Id: 1, Name: "role one", Code: "role-one"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}

	service := NewAccountService(db, nil)
	request := &iam_v1.UpdateAccountRolesRequest{
		AccountId:         account.Id,
		AddedRoleCodeList: []string{"role-one"},
	}
	for i := 0; i < 2; i++ {
		if _, err := service.UpdateAccountRoles(context.Background(), request); err != nil {
			t.Fatalf("UpdateAccountRoles() #%d error = %v", i+1, err)
		}
	}

	var count int64
	if err := db.Model(model.RAccountRole{}).
		Where(model.RAccountRoleColumnAccountId, account.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("count binding rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("binding rows = %d, want 1", count)
	}
}

// T044-③ 读穿回源：空快照下重建，快照按带前缀资源键并集写回且 TTL≈5min（D5/D6-②/D6-③）。
func TestLoadPermissionSnapshotReadThrough(t *testing.T) {
	ctx := context.Background()
	db := setupAccountTestDB(t)
	rdb, mr := setupTestRedis(t)
	account := createTestAccount(t, db, 1, "one@example.com", 1)
	application := model.Application{Id: 1, Name: "fastarch", Code: "fastarch"}
	if err := db.Create(&application).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	role := model.Role{Id: 1, Name: "role one", Code: "role-one", ApplicationId: application.Id}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	menuResource := model.Resource{Kind: model.PermissionKindMenu, Key: "iam.account_management.account_list", Name: "用户列表"}
	if err := db.Create(&menuResource).Error; err != nil {
		t.Fatalf("create resource: %v", err)
	}
	permission := model.Permission{Id: 1, Name: "perm one", Code: "perm-one", Kind: model.PermissionKindMenu}
	if err := db.Create(&permission).Error; err != nil {
		t.Fatalf("create permission: %v", err)
	}
	if err := db.Model(&permission).Association("ResourceList").Append(&menuResource); err != nil {
		t.Fatalf("bind resource: %v", err)
	}
	if err := db.Model(&role).Association("PermissionList").Append(&permission); err != nil {
		t.Fatalf("bind permission: %v", err)
	}
	if err := db.Model(&account).Association("RoleList").Append(&role); err != nil {
		t.Fatalf("bind role: %v", err)
	}

	key := constant.GetAccountPermissionKey(account.Id)
	if mr.Exists(key) {
		t.Fatalf("precondition: snapshot key %q should not exist", key)
	}

	codes, err := auth.LoadPermissionSnapshot(ctx, rdb, account.Id, NewPermissionSnapshotLoader(db))
	if err != nil {
		t.Fatalf("LoadPermissionSnapshot() error = %v", err)
	}
	want := constant.SnapshotKeyPrefixMenu + menuResource.Key
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

// T060 审计字段注入（research.md D16）：Create 注入四件套（created_by/updated_by 取
// 认证上下文操作者 id，无认证上下文时 0）；Update 仅更新 update_time/updated_by，
// created_by/create_time 保持不变；Get 回填审计字段。
func TestAccountAuditFields(t *testing.T) {
	createCtx := context.WithValue(context.Background(), "accountId", "42")
	updateCtx := context.WithValue(context.Background(), "accountId", "43")
	db := setupAccountTestDB(t)
	service := NewAccountService(db, nil)

	created, err := service.CreateAccount(createCtx, &iam_v1.CreateAccountRequest{
		Name:  "account",
		Email: "audit@example.com",
		Kind:  model.AccountKindIdUser,
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
		Id:      accountId,
		Name:    "updated",
		Version: 1,
	})
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}

	var got model.Account
	if err := db.First(&got, accountId).Error; err != nil {
		t.Fatalf("get account: %v", err)
	}
	if got.CreatedBy != 42 || got.UpdatedBy != 43 {
		t.Fatalf("created_by/updated_by = %d/%d, want 42/43", got.CreatedBy, got.UpdatedBy)
	}
	if got.CreateTime.Unix() != created.CreateTime.AsTime().Unix() {
		t.Fatalf("create_time = %v, want unchanged %v", got.CreateTime, created.CreateTime.AsTime())
	}
	if got.UpdateTime.IsZero() {
		t.Fatal("update_time is zero, want updated")
	}

	detail, err := service.GetAccount(context.Background(), &common_v1.GetRequest{Id: cast.ToInt64(accountId)})
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

// T060 补充：无认证上下文时操作者注入为 0（内部调用/迁移场景）。
func TestAccountAuditFieldsWithoutAuthContext(t *testing.T) {
	db := setupAccountTestDB(t)
	service := NewAccountService(db, nil)

	created, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Name:  "account",
		Email: "no-auth@example.com",
		Kind:  model.AccountKindIdUser,
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if created.CreatedBy != "" && created.CreatedBy != "0" {
		t.Fatalf("created_by = %q, want empty/0 without auth context", created.CreatedBy)
	}
}

// ---- specs/008-account-app-context：账号-应用归属（T008）----

func createTestApplicationForAccount(t *testing.T, db *gorm.DB, id int64, name string) model.Application {
	t.Helper()
	now := time.Now()
	app := model.Application{
		Id:         id,
		Code:       fmt.Sprintf("app-%d", id),
		Name:       name,
		Disabled:   0,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	return app
}

func loadAccountApplicationIds(t *testing.T, db *gorm.DB, accountId int64) []int64 {
	t.Helper()
	var account model.Account
	if err := db.Preload("ApplicationList").First(&account, accountId).Error; err != nil {
		t.Fatalf("load account: %v", err)
	}
	ids := make([]int64, 0, len(account.ApplicationList))
	for _, app := range account.ApplicationList {
		ids = append(ids, app.Id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func TestCreateAccountWithApplications(t *testing.T) {
	db := setupAccountTestDB(t)
	createTestApplicationForAccount(t, db, 101, "应用A")
	createTestApplicationForAccount(t, db, 102, "应用B")
	service := NewAccountService(db, nil)

	created, err := service.CreateAccount(auth.WithAccountId(context.Background(), 9), &iam_v1.CreateAccountRequest{
		Name:              "alice",
		Email:             "alice@example.com",
		Kind:              model.AccountKindIdUser,
		ApplicationIdList: []int64{101, 102},
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	got := loadAccountApplicationIds(t, db, cast.ToInt64(created.Id))
	if len(got) != 2 || got[0] != 101 || got[1] != 102 {
		t.Fatalf("application ids = %v, want [101 102]", got)
	}
}

func TestCreateAccountWithoutApplications(t *testing.T) {
	db := setupAccountTestDB(t)
	service := NewAccountService(db, nil)

	created, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Name:  "bob",
		Email: "bob@example.com",
		Kind:  model.AccountKindIdUser,
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if got := loadAccountApplicationIds(t, db, cast.ToInt64(created.Id)); len(got) != 0 {
		t.Fatalf("application ids = %v, want empty", got)
	}
}

func TestUpdateAccountApplicationsFullReplace(t *testing.T) {
	db := setupAccountTestDB(t)
	createTestAccount(t, db, 1, "one@example.com", 1)
	createTestApplicationForAccount(t, db, 101, "应用A")
	createTestApplicationForAccount(t, db, 102, "应用B")
	createTestApplicationForAccount(t, db, 103, "应用C")
	service := NewAccountService(db, nil)
	ctx := auth.WithAccountId(context.Background(), 9)

	if _, err := service.UpdateAccount(ctx, &iam_v1.UpdateAccountRequest{
		Id:                1,
		Name:              "account",
		Version:           1,
		ApplicationIdList: []int64{101, 102},
	}); err != nil {
		t.Fatalf("UpdateAccount(add) error = %v", err)
	}
	if got := loadAccountApplicationIds(t, db, 1); len(got) != 2 || got[0] != 101 || got[1] != 102 {
		t.Fatalf("after add: application ids = %v, want [101 102]", got)
	}

	// 全量替换：移除 101，新增 103
	if _, err := service.UpdateAccount(ctx, &iam_v1.UpdateAccountRequest{
		Id:                1,
		Version:           2,
		ApplicationIdList: []int64{102, 103},
	}); err != nil {
		t.Fatalf("UpdateAccount(replace) error = %v", err)
	}
	if got := loadAccountApplicationIds(t, db, 1); len(got) != 2 || got[0] != 102 || got[1] != 103 {
		t.Fatalf("after replace: application ids = %v, want [102 103]", got)
	}

	// 显式空集 = 清空归属（合法：无归属账号）
	if _, err := service.UpdateAccount(ctx, &iam_v1.UpdateAccountRequest{
		Id:                1,
		Version:           3,
		ApplicationIdList: []int64{},
	}); err != nil {
		t.Fatalf("UpdateAccount(clear) error = %v", err)
	}
	if got := loadAccountApplicationIds(t, db, 1); len(got) != 0 {
		t.Fatalf("after clear: application ids = %v, want empty", got)
	}
}

func TestUpdateAccountApplicationsIdempotentNoDuplicates(t *testing.T) {
	db := setupAccountTestDB(t)
	createTestAccount(t, db, 1, "one@example.com", 1)
	createTestApplicationForAccount(t, db, 101, "应用A")
	service := NewAccountService(db, nil)
	ctx := auth.WithAccountId(context.Background(), 9)

	for i := 0; i < 3; i++ {
		if _, err := service.UpdateAccount(ctx, &iam_v1.UpdateAccountRequest{
			Id:                1,
			Version:           int64(i) + 1,
			ApplicationIdList: []int64{101, 101}, // 重复 id 幂等
		}); err != nil {
			t.Fatalf("UpdateAccount() #%d error = %v", i, err)
		}
	}
	var count int64
	if err := db.Model(&model.RAccountApplication{}).Where(model.RAccountApplicationColumnAccountId, 1).Count(&count).Error; err != nil {
		t.Fatalf("count associations: %v", err)
	}
	if count != 1 {
		t.Fatalf("association rows = %d, want 1", count)
	}
}

func TestUpdateAccountApplicationsNotProvidedKeepsAssociations(t *testing.T) {
	db := setupAccountTestDB(t)
	createTestAccount(t, db, 1, "one@example.com", 1)
	createTestApplicationForAccount(t, db, 101, "应用A")
	service := NewAccountService(db, nil)
	ctx := auth.WithAccountId(context.Background(), 9)

	if _, err := service.UpdateAccount(ctx, &iam_v1.UpdateAccountRequest{
		Id:                1,
		Version:           1,
		ApplicationIdList: []int64{101},
	}); err != nil {
		t.Fatalf("UpdateAccount(bind) error = %v", err)
	}
	// 不携带 application_id_list（nil）：不触碰归属（密码修改等局部更新不受影响）
	// 仅归属变更不修改账号行，version 不递增，故仍传 1
	if _, err := service.UpdateAccount(ctx, &iam_v1.UpdateAccountRequest{
		Id:      1,
		Version: 1,
		Name:    "renamed",
	}); err != nil {
		t.Fatalf("UpdateAccount(without list) error = %v", err)
	}
	if got := loadAccountApplicationIds(t, db, 1); len(got) != 1 || got[0] != 101 {
		t.Fatalf("application ids = %v, want [101]", got)
	}
}

func TestUpdateAccountApplicationsInvalidApp(t *testing.T) {
	db := setupAccountTestDB(t)
	createTestAccount(t, db, 1, "one@example.com", 1)
	service := NewAccountService(db, nil)

	_, err := service.UpdateAccount(auth.WithAccountId(context.Background(), 9), &iam_v1.UpdateAccountRequest{
		Id:                1,
		Version:           1,
		ApplicationIdList: []int64{999},
	})
	if err != ErrApplicationNotFound {
		t.Fatalf("UpdateAccount() error = %v, want ErrApplicationNotFound", err)
	}
}

func TestListAccountsCarriesApplications(t *testing.T) {
	db := setupAccountTestDB(t)
	createTestApplicationForAccount(t, db, 101, "应用A")
	service := NewAccountService(db, nil)

	if _, err := service.CreateAccount(context.Background(), &iam_v1.CreateAccountRequest{
		Name:              "alice",
		Email:             "alice@example.com",
		Kind:              model.AccountKindIdUser,
		ApplicationIdList: []int64{101},
	}); err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	resp, err := service.ListAccounts(context.Background(), &iam_v1.ListAccountsRequest{CurrentPage: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}
	if len(resp.Accounts) != 1 || len(resp.Accounts[0].ApplicationIdList) != 1 || resp.Accounts[0].ApplicationIdList[0] != "101" {
		t.Fatalf("ListAccounts() accounts = %+v, want applicationIdList [101]", resp.Accounts)
	}
}
