package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"iam-service/pkg/auth"
	"iam-service/pkg/cipher"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"
)

// AccessCredentialService 访问凭证服务（specs/009-ak-sk-auth）。
// 凭证为认证管理域基础架构型模型：物理删除 + 引用校验 + 审计日志（宪法 XIV）。
type AccessCredentialService struct {
	db            *gorm.DB
	rdb           *redis.Client
	aes           *cipher.AESGCM
	maxPerAccount int
}

var _ iam_v1.AccessCredentialServiceServer = (*AccessCredentialService)(nil)

const (
	accessKeyPrefix         = "FAK"
	accessKeyRandomLength   = 21 // AK 总长 24：前缀 3 + 随机 21（Base62）
	secretKeySize           = 32 // SK 32 字节（256 位强度）
	accessKeyMaxGenerateTry = 5  // AK 唯一冲突重试次数
)

func NewAccessCredentialService(db *gorm.DB, rdb *redis.Client, aes *cipher.AESGCM, maxPerAccount int) *AccessCredentialService {
	return &AccessCredentialService{db: db, rdb: rdb, aes: aes, maxPerAccount: maxPerAccount}
}

// accessCredentialToPb 模型转 pb；secret_key_cipher 永不出服务层。
// app 为凭证归属应用摘要（由 c.ApplicationId 批量加载后传入），决定机机调用数据归属。
func accessCredentialToPb(c model.AccessCredential, accountName string, app *common_v1.ApplicationSummary) *iam_v1.AccessCredential {
	return &iam_v1.AccessCredential{
		Id:          cast.ToString(c.Id),
		AccessKey:   c.AccessKey,
		UserId:      cast.ToString(c.UserId),
		UserName:    accountName,
		Remark:      c.Remark,
		Status:      c.Status,
		ExpireTime:  timeToTsPtr(c.ExpireTime),
		LastUsedTime: timeToTsPtr(c.LastUsedTime),
		Version:     c.Version,
		CreatedBy:   cast.ToString(c.CreatedBy),
		UpdatedBy:   cast.ToString(c.UpdatedBy),
		CreateTime:  timestamppb.New(c.CreateTime),
		UpdateTime:  timestamppb.New(c.UpdateTime),
		Application: app,
	}
}

func timeToTsPtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func tsToTimePtr(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

// generateAccessKey 生成 FAK 前缀的公开访问键（标识用途，非机密，krand 足够；
// 全局唯一由唯一索引兜底 + 冲突重试）。
func generateAccessKey() string {
	return accessKeyPrefix + krand.String(krand.R_All, accessKeyRandomLength)
}

// generateSecretKey 生成 32 字节 CSPRNG 随机 SK，Base64 编码（FR-001 密码学安全强度）。
func generateSecretKey() (string, error) {
	raw := make([]byte, secretKeySize)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func (service *AccessCredentialService) CreateAccessCredential(ctx context.Context, request *iam_v1.CreateAccessCredentialRequest) (*iam_v1.CreateAccessCredentialResponse, error) {
	operator := auth.GetAccountId(ctx)
	// 审计字段服务端注入（宪法 XII）；created_by/updated_by 服务端赋值
	now := time.Now()
	var created model.AccessCredential
	var plainSecret string
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 绑定账号必须存在且启用（FR-004）；行锁防止并发删除/停用窗口内的校验竞态
		var account model.User
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select(model.UserFields).
			Where(model.UserColumnId, request.UserId).
			First(&account).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAccessCredentialAccountInvalid
		}
		if err != nil {
			return err
		}
		if account.Status != constant.AccountStatusEnabled {
			return ErrAccessCredentialAccountInvalid
		}
		// 数据边界：凭证随所属用户 home 隔离（账号已行锁加载，不复查）
		homeId, scoped, err := operatorHomeScope(ctx, tx)
		if err != nil {
			return err
		}
		if scoped && (homeId <= 0 || homeId != account.HomeApplicationId) {
			return ErrAccountOutOfScope
		}
		// 凭证归属应用：显式指定则校验 ∈ 绑定用户接入集合；缺省回落到绑定用户 home
		// （与迁移回填、"home 降级为默认落地应用"一致；API 契约仍要求必填，
		// 此处仅对内部调用/历史客户端做防御性兼容）。
		appId := request.ApplicationId
		if appId <= 0 {
			appId = account.HomeApplicationId
		}
		if err := requireCredentialAppAdmitted(ctx, tx, account, appId); err != nil {
			return err
		}
		// 单账号凭证上限（FR-010）；行锁下计数，规避并发创建的 check-then-act 放大
		var count int64
		err = tx.Model(model.AccessCredential{}).
			Where(model.AccessCredentialColumnUserId, request.UserId).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count >= int64(service.maxPerAccount) {
			slog.Warn("create access credential rejected: limit exceeded",
				"operator", operator, "user_id", request.UserId, "count", count)
			return ErrAccessCredentialLimitExceeded
		}
		plainSecret, err = generateSecretKey()
		if err != nil {
			return err
		}
		cipherText, err := service.aes.Encrypt([]byte(plainSecret))
		if err != nil {
			return err
		}
		// AK 冲突重试（唯一索引兜底，见 data-model.md 边界场景）
		for attempt := 0; attempt < accessKeyMaxGenerateTry; attempt++ {
			candidate := model.AccessCredential{
				Id:              krand.NewID(),
				AccessKey:       generateAccessKey(),
				SecretKeyCipher: cipherText,
				UserId:          request.UserId,
				ApplicationId:   appId,

				Remark:     request.Remark,
				Status:     constant.AccessCredentialStatusEnabled,
				ExpireTime: tsToTimePtr(request.ExpireTime),
				CreateTime: now,
				UpdateTime: now,
				CreatedBy:  operator,
				UpdatedBy:  operator,
				Version:    1,
			}
			var exists int64
			err = tx.Model(model.AccessCredential{}).
				Where(model.AccessCredentialColumnAccessKey, candidate.AccessKey).
				Count(&exists).Error
			if err != nil {
				return err
			}
			if exists > 0 {
				continue
			}
			if err := tx.Create(&candidate).Error; err != nil {
				// 并发窗口撞唯一索引：重试
				if strings.Contains(err.Error(), "1062") || strings.Contains(strings.ToLower(err.Error()), "unique") {
					continue
				}
				return err
			}
			created = candidate
			return nil
		}
		return errors.New("generate unique access key failed")
	})
	if err != nil {
		return nil, err
	}
	// 操作审计（不记录 SK，FR-014a / 宪法 XIV 七）
	slog.Info("access credential created",
		"operator", operator,
		"credential_id", created.Id,
		"access_key", created.AccessKey,
		"account_id", created.UserId)
	appSummary, err := loadApplicationSummary(ctx, service.db, created.ApplicationId)
	if err != nil {
		return nil, err
	}
	return &iam_v1.CreateAccessCredentialResponse{
		Credential: accessCredentialToPb(created, accountNameById(ctx, service.db, created.UserId), appSummary),
		SecretKey:  plainSecret, // 一次性明文 SK（FR-002）
	}, nil
}

// requireCredentialAppAdmitted 校验凭证归属应用 ∈ 绑定用户的接入集合（home + 显式绑定）。
// admin/系统调用（accountId=0）旁路；归属应用须存在且启用，否则拒绝（fail-closed）。
func requireCredentialAppAdmitted(ctx context.Context, db *gorm.DB, account model.User, appId int64) error {
	if auth.IsAdminFromContext(ctx) || auth.GetAccountId(ctx) == 0 {
		// 旁路仍需确认应用存在且启用，避免写入悬空归属。
		_, err := loadApplicationSummary(ctx, db, appId)
		if err != nil {
			return err
		}
		return nil
	}
	if appId <= 0 {
		return ErrCredentialAppNotAdmitted
	}
	admitted, err := loadAdmittedAppIds(ctx, db, account.Id, account.HomeApplicationId)
	if err != nil {
		return err
	}
	for _, id := range admitted {
		if id == appId {
			return nil
		}
	}
	return ErrCredentialAppNotAdmitted
}

// firstCredential 点查凭证，未命中统一映射 ErrAccessCredentialNotFound（1001040）。
func firstCredential(ctx context.Context, db *gorm.DB, id int64) (model.AccessCredential, error) {
	var credential model.AccessCredential
	err := db.WithContext(ctx).
		Select(model.AccessCredentialFields).
		Where(model.AccessCredentialColumnId, id).
		First(&credential).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return credential, ErrAccessCredentialNotFound
	}
	return credential, err
}

// requireCredentialScope 凭证数据边界（ADR 0006 补充）：凭证随所属用户 home 隔离。
// admin/系统调用旁路；非 admin home 未设置、所属用户不存在或与 home 不一致一律拒绝。
func (service *AccessCredentialService) requireCredentialScope(ctx context.Context, userId int64) error {
	homeId, scoped, err := operatorHomeScope(ctx, service.db)
	if err != nil || !scoped {
		return err
	}
	var owner model.User
	err = service.db.WithContext(ctx).
		Select(model.UserColumnId, model.UserColumnHomeApplicationId).
		Where(model.UserColumnId, userId).
		First(&owner).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrAccountOutOfScope
	}
	if err != nil {
		return err
	}
	if homeId <= 0 || homeId != owner.HomeApplicationId {
		return ErrAccountOutOfScope
	}
	return nil
}

func (service *AccessCredentialService) GetAccessCredential(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.AccessCredential, error) {
	credential, err := firstCredential(ctx, service.db, request.Id)
	if err != nil {
		return nil, err
	}
	if err := service.requireCredentialScope(ctx, credential.UserId); err != nil {
		return nil, err
	}
	appSummary, err := loadApplicationSummary(ctx, service.db, credential.ApplicationId)
	if err != nil {
		return nil, err
	}
	return accessCredentialToPb(credential, accountNameById(ctx, service.db, credential.UserId), appSummary), nil
}

func (service *AccessCredentialService) ListAccessCredentials(ctx context.Context, request *iam_v1.ListAccessCredentialsRequest) (*iam_v1.ListAccessCredentialsResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	// 数据边界（ADR 0006 补充）：凭证随所属用户 home 隔离；home 未设置 fail-closed 空列表
	homeId, scoped, err := operatorHomeScope(ctx, service.db)
	if err != nil {
		return nil, err
	}
	if scoped && homeId <= 0 {
		return &iam_v1.ListAccessCredentialsResponse{
			AccessCredentials: []*iam_v1.AccessCredential{},
			Page:              helper.GetPage(request.CurrentPage, request.PageSize, 0),
		}, nil
	}
	var count int64
	var res []model.AccessCredential
	credentialTable := (&model.AccessCredential{}).TableName()
	buildQuery := func() *gorm.DB {
		query := service.db.WithContext(ctx).Model(model.AccessCredential{})
		if scoped {
			// 按凭证自身归属应用过滤（000015 起凭证有 application_id 列），
			// 与账号/角色/权限共用同一数据边界（appCode 选中应用，回落 home）。
			query = query.Where(credentialTable+"."+model.AccessCredentialColumnApplicationId+" = ?", homeId)
		}
		if request.UserId > 0 {
			query = query.Where(credentialTable+"."+model.AccessCredentialColumnUserId, request.UserId)
		}
		if len(request.AccessKey) > 0 {
			query = query.Where(credentialTable+"."+model.AccessCredentialColumnAccessKey+" LIKE ?", request.AccessKey+"%")
		}
		if len(request.Status) > 0 {
			query = query.Where(credentialTable+"."+model.AccessCredentialColumnStatus, request.Status)
		}
		return query
	}
	if err := buildQuery().Count(&count).Error; err != nil {
		return nil, err
	}
	if err := buildQuery().
		Limit(limit).
		Offset(offset).
		Order(credentialTable + "." + model.AccessCredentialColumnId + " DESC").
		Find(&res).Error; err != nil {
		return nil, err
	}
	// 账号名批量组装（IN 批量查询，禁 N+1）
	accountIds := make([]int64, 0, len(res))
	seen := make(map[int64]struct{}, len(res))
	for _, c := range res {
		if _, ok := seen[c.UserId]; !ok {
			seen[c.UserId] = struct{}{}
			accountIds = append(accountIds, c.UserId)
		}
	}
	nameById := accountNamesByIds(ctx, service.db, accountIds)
	appByCredential, err := loadCredentialApplicationSummaries(ctx, service.db, res)
	if err != nil {
		return nil, err
	}
	list := make([]*iam_v1.AccessCredential, 0, len(res))
	for _, c := range res {
		list = append(list, accessCredentialToPb(c, nameById[c.UserId], appByCredential[c.Id]))
	}
	return &iam_v1.ListAccessCredentialsResponse{
		Page:              helper.GetPage(request.CurrentPage, request.PageSize, count),
		AccessCredentials: list,
	}, nil
}

func (service *AccessCredentialService) UpdateAccessCredential(ctx context.Context, request *iam_v1.UpdateAccessCredentialRequest) (*emptypb.Empty, error) {
	operator := auth.GetAccountId(ctx)
	credential, err := firstCredential(ctx, service.db, request.Id)
	if err != nil {
		return nil, err
	}
	if err := service.requireCredentialScope(ctx, credential.UserId); err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if len(request.Remark) > 0 && request.Remark != credential.Remark {
		updates[model.AccessCredentialColumnRemark] = request.Remark
	}
	// expire_time presence 语义：nil = 不修改（与既有 Update 局部更新语义一致）
	if request.ExpireTime != nil {
		updates[model.AccessCredentialColumnExpireTime] = tsToTimePtr(request.ExpireTime)
	}
	if len(updates) == 0 {
		return &emptypb.Empty{}, nil
	}
	updates[model.AccessCredentialColumnUpdateTime] = time.Now()
	updates[model.AccessCredentialColumnUpdatedBy] = operator
	updates[model.AccessCredentialColumnVersion] = credential.Version + 1
	result := service.db.WithContext(ctx).Model(model.AccessCredential{}).
		Where(model.AccessCredentialColumnId, request.Id).
		Where(model.AccessCredentialColumnVersion, request.Version).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrAccessCredentialUpdateBusy
	}
	slog.Info("access credential updated",
		"operator", operator, "credential_id", request.Id, "access_key", credential.AccessKey, "fields", updates)
	return &emptypb.Empty{}, nil
}

// ChangeAccessCredentialStatus 启用/禁用（幂等：目标状态与当前一致时直接成功）。
func (service *AccessCredentialService) ChangeAccessCredentialStatus(ctx context.Context, request *iam_v1.ChangeAccessCredentialStatusRequest) (*emptypb.Empty, error) {
	if request.Status != constant.AccessCredentialStatusEnabled && request.Status != constant.AccessCredentialStatusDisabled {
		return nil, errors.New("invalid credential status")
	}
	operator := auth.GetAccountId(ctx)
	credential, err := firstCredential(ctx, service.db, request.Id)
	if err != nil {
		return nil, err
	}
	if err := service.requireCredentialScope(ctx, credential.UserId); err != nil {
		return nil, err
	}
	if credential.Status == request.Status {
		return &emptypb.Empty{}, nil // 幂等
	}
	now := time.Now()
	result := service.db.WithContext(ctx).Model(model.AccessCredential{}).
		Where(model.AccessCredentialColumnId, request.Id).
		Where(model.AccessCredentialColumnVersion, credential.Version).
		Updates(map[string]interface{}{
			model.AccessCredentialColumnStatus:     request.Status,
			model.AccessCredentialColumnUpdateTime: now,
			model.AccessCredentialColumnUpdatedBy:  operator,
			model.AccessCredentialColumnVersion:    credential.Version + 1,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrAccessCredentialUpdateBusy
	}
	slog.Info("access credential status changed",
		"operator", operator, "credential_id", request.Id, "access_key", credential.AccessKey,
		"from", credential.Status, "to", request.Status)
	return &emptypb.Empty{}, nil
}

// ResetAccessCredentialSecret 重置 SK：AK 不变、旧 SK 立即失效、明文一次性返回（FR-008）。
func (service *AccessCredentialService) ResetAccessCredentialSecret(ctx context.Context, request *iam_v1.ResetAccessCredentialSecretRequest) (*iam_v1.ResetAccessCredentialSecretResponse, error) {
	operator := auth.GetAccountId(ctx)
	credential, err := firstCredential(ctx, service.db, request.Id)
	if err != nil {
		return nil, err
	}
	if err := service.requireCredentialScope(ctx, credential.UserId); err != nil {
		return nil, err
	}
	plainSecret, err := generateSecretKey()
	if err != nil {
		return nil, err
	}
	cipherText, err := service.aes.Encrypt([]byte(plainSecret))
	if err != nil {
		return nil, err
	}
	result := service.db.WithContext(ctx).Model(model.AccessCredential{}).
		Where(model.AccessCredentialColumnId, request.Id).
		Where(model.AccessCredentialColumnVersion, credential.Version).
		Updates(map[string]interface{}{
			model.AccessCredentialColumnSecretKeyCipher: cipherText,
			model.AccessCredentialColumnUpdateTime:      time.Now(),
			model.AccessCredentialColumnUpdatedBy:       operator,
			model.AccessCredentialColumnVersion:         credential.Version + 1,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrAccessCredentialUpdateBusy
	}
	slog.Info("access credential secret reset",
		"operator", operator, "credential_id", request.Id, "access_key", credential.AccessKey)
	appSummary, err := loadApplicationSummary(ctx, service.db, credential.ApplicationId)
	if err != nil {
		return nil, err
	}
	return &iam_v1.ResetAccessCredentialSecretResponse{
		Credential: accessCredentialToPb(credential, accountNameById(ctx, service.db, credential.UserId), appSummary),
		SecretKey:  plainSecret,
	}, nil
}

// DeleteAccessCredential 物理删除（基础架构型模型，宪法 XIV）：凭证为叶子资源无下游引用，
// 删除前应用层校验（记录存在性）；操作审计记录操作人与 AK（FR-007）。
func (service *AccessCredentialService) DeleteAccessCredential(ctx context.Context, request *iam_v1.DeleteAccessCredentialRequest) (*emptypb.Empty, error) {
	operator := auth.GetAccountId(ctx)
	credential, err := firstCredential(ctx, service.db, request.Id)
	if err != nil {
		return nil, err
	}
	if err := service.requireCredentialScope(ctx, credential.UserId); err != nil {
		return nil, err
	}
	result := service.db.WithContext(ctx).
		Where(model.AccessCredentialColumnId, request.Id).
		Delete(model.AccessCredential{})
	if result.Error != nil {
		return nil, result.Error
	}
	slog.Info("access credential deleted",
		"operator", operator, "credential_id", request.Id, "access_key", credential.AccessKey,
		"account_id", credential.UserId)
	return &emptypb.Empty{}, nil
}

// accountNameById 单账号名查询（详情场景一次性调用）。
func accountNameById(ctx context.Context, db *gorm.DB, accountId int64) string {
	names := accountNamesByIds(ctx, db, []int64{accountId})
	return names[accountId]
}

// accountNamesByIds 批量查询账号名（IN 一次查询，禁 N+1）。011 模型：账号名即 username。
func accountNamesByIds(ctx context.Context, db *gorm.DB, accountIds []int64) map[int64]string {
	nameById := make(map[int64]string, len(accountIds))
	if len(accountIds) == 0 {
		return nameById
	}
	var rows []model.User
	err := db.WithContext(ctx).
		Select(model.UserColumnId, model.UserColumnUsername).
		Where(model.UserColumnId+" IN ?", accountIds).
		Find(&rows).Error
	if err != nil {
		slog.Warn("load account names failed", "account_ids", accountIds, "error", err)
		return nameById
	}
	for _, row := range rows {
		nameById[row.Id] = row.Username
	}
	return nameById
}
