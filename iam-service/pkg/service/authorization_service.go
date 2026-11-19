package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"iam-service/pkg/auth"
	"iam-service/pkg/constant"
	"iam-service/pkg/helper"
	"iam-service/pkg/model"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

func applyKeywordFilter(db *gorm.DB, conditions []string, keyword string) *gorm.DB {
	if len(keyword) == 0 {
		return db
	}
	escapedKeyword := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(keyword)
	likeKeyword := "%" + escapedKeyword + "%"
	// 每个 LIKE ? 条件对应一个参数，按条件数动态填充
	args := make([]any, 0, len(conditions))
	for range conditions {
		args = append(args, likeKeyword)
	}
	return db.Where(strings.Join(conditions, " OR "), args...)
}

type AuthorizationService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewAuthorizationService(db *gorm.DB, rdb *redis.Client) *AuthorizationService {
	return &AuthorizationService{db: db, rdb: rdb}
}

// loadAccountPermissionSnapshot 读取账号权限快照：缓存命中直接返回，未命中或
// 格式不符走读穿回源重建（与 auth 拦截器共用回源逻辑）。快照内容为纯权限码集合
// （011 权限码模型）。Redis 故障 fail-closed，底层错误仅入日志。
func loadAccountPermissionSnapshot(ctx context.Context, rdb *redis.Client, db *gorm.DB, accountId int64) ([]string, error) {
	str, err := rdb.Get(ctx, constant.GetAccountPermissionKey(accountId)).Result()
	var permKeys []string
	if err == nil {
		err = json.Unmarshal([]byte(str), &permKeys)
		if err == nil {
			return permKeys, nil
		}
		// 格式不符（旧格式残留）视为未命中，走回源重建
		slog.Warn("unmarshal permission snapshot failed, rebuilding from db", "account_id", accountId, "error", err)
	} else if !errors.Is(err, redis.Nil) {
		slog.Error("get permission snapshot from redis failed", "account_id", accountId, "error", err)
		return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
	}
	permKeys, err = auth.LoadPermissionSnapshot(ctx, rdb, accountId, NewPermissionSnapshotLoader(db))
	if err != nil {
		slog.Error("rebuild permission snapshot from db failed", "account_id", accountId, "error", err)
		return nil, status.Error(codes.FailedPrecondition, "权限校验服务暂时不可用，请稍后重试")
	}
	return permKeys, nil
}

func (a *AuthorizationService) CheckPermission(ctx context.Context, request *iam_v1.CheckPermissionRequest) (*iam_v1.CheckPermissionResponse, error) {
	// 数据归属应用：拦截器已校验 appCode ∈ 接入集合并注入 ctx；回传给下游服务
	// （如 sender）作为数据隔离依据。未带 appCode 时为 0，下游按其默认应用回落。
	dataAppId := auth.GetDataAppId(ctx)
	// 权限码是唯一授权词汇（011 裁决）：直接按码匹配快照
	if auth.IsAdminFromContext(ctx) {
		return &iam_v1.CheckPermissionResponse{Ok: true, DataAppId: dataAppId}, nil
	}
	accountId := auth.GetAccountId(ctx)
	permKeys, err := loadAccountPermissionSnapshot(ctx, a.rdb, a.db, accountId)
	if err != nil {
		return nil, err
	}
	for _, key := range permKeys {
		if key == request.PermissionCode {
			return &iam_v1.CheckPermissionResponse{
				Ok: true, DataAppId: dataAppId,
			}, nil
		}
	}
	return &iam_v1.CheckPermissionResponse{
		Ok: false, DataAppId: dataAppId,
	}, nil
}

func permissionToPb(p model.Permission, app *common_v1.ApplicationSummary) *iam_v1.Permission {
	return &iam_v1.Permission{
		Id:             cast.ToString(p.Id),
		Name:           p.Name,
		PermissionCode: p.Code,
		EnabledStatus:  p.Status,
		Description:    p.Description,
		Version:        p.Version,
		CreatedBy:      p.CreatedBy,
		UpdatedBy:      p.UpdatedBy,
		CreateTime:     timestamppb.New(p.CreateTime),
		UpdateTime:     timestamppb.New(p.UpdateTime),
		Application:    app,
	}
}

func roleToPb(r model.Role, app *common_v1.ApplicationSummary) *iam_v1.Role {
	return &iam_v1.Role{
		Id:            cast.ToString(r.Id),
		Name:          r.Name,
		RoleCode:      r.Code,
		EnabledStatus: r.Status,
		ApplicationId: r.ApplicationId,
		Application:   app,
		Version:       r.Version,
		CreatedBy:     r.CreatedBy,
		UpdatedBy:     r.UpdatedBy,
		CreateTime:    timestamppb.New(r.CreateTime),
		UpdateTime:    timestamppb.New(r.UpdateTime),
		PermissionCodeList: func() []string {
			codes := make([]string, 0, len(r.PermissionList))
			for _, permission := range r.PermissionList {
				codes = append(codes, permission.Code)
			}
			return codes
		}(),
		PermissionList: func() []*iam_v1.PermissionSummary {
			summaries := make([]*iam_v1.PermissionSummary, 0, len(r.PermissionList))
			for _, permission := range r.PermissionList {
				summaries = append(summaries, &iam_v1.PermissionSummary{
					Id:             cast.ToString(permission.Id),
					Name:           permission.Name,
					PermissionCode: permission.Code,
					EnabledStatus:  permission.Status,
				})
			}
			return summaries
		}(),
	}
}

// GetMyPermissions 返回当前认证账号持有的全部权限码（011 模型：纯权限码快照）。
func (a *AuthorizationService) GetMyPermissions(ctx context.Context, request *emptypb.Empty) (*iam_v1.GetMyPermissionsResponse, error) {
	// admin 旁路：返回全量启用权限码（与 CheckPermission/拦截器 admin 旁路一致）。
	// 前端菜单/按钮码门槛对 admin 全放行，跨应用码（sender.* 等）亦覆盖。
	if auth.IsAdminFromContext(ctx) {
		var codes []string
		if err := a.db.WithContext(ctx).Model(model.Permission{}).
			Where(model.PermissionColumnStatus, model.StatusEnabled).
			Pluck(model.PermissionColumnCode, &codes).Error; err != nil {
			return nil, err
		}
		return &iam_v1.GetMyPermissionsResponse{PermissionCodeList: codes}, nil
	}
	permKeys, err := loadAccountPermissionSnapshot(ctx, a.rdb, a.db, auth.GetAccountId(ctx))
	if err != nil {
		return nil, err
	}
	return &iam_v1.GetMyPermissionsResponse{
		PermissionCodeList: permKeys,
	}, nil
}

func (a *AuthorizationService) CreatePermission(ctx context.Context, request *iam_v1.CreatePermissionRequest) (*iam_v1.Permission, error) {
	// 权限码格式校验（011 裁决：服务.模块.操作，如 iam.user.delete）
	if !model.IsValidPermissionCode(request.Code) {
		return nil, ErrInvalidPermissionCode
	}
	// 数据边界（ADR 0006）：非 admin 的码归属其 home，服务段必须等于 home 应用 code；
	// admin/系统调用沿用当前应用上下文。
	appId, scoped, err := dataScopeApplicationId(ctx, a.db)
	if err != nil {
		return nil, err
	}
	if scoped && appId <= 0 {
		return nil, ErrDataScopeRequired
	}
	if err := requireCodeSegmentMatchesApp(ctx, a.db, appId, request.Code); err != nil {
		return nil, err
	}
	pkId := krand.NewID()
	now := time.Now()
	// 审计字段服务端注入：created_by/updated_by 取认证上下文操作者
	operator := cast.ToString(auth.GetAccountId(ctx))
	permission := model.Permission{
		Id:            pkId,
		Name:          request.Name,
		Code:          request.Code,
		ApplicationId: appId,
		Type:          "api",
		Description:   request.Description,
		Status:        model.StatusEnabled,
		Version:       1,
		CreatedBy:     operator,
		UpdatedBy:     operator,
		CreateTime:    now,
		UpdateTime:    now,
	}
	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(model.Permission{}).
			Where(model.PermissionColumnCode, request.Code).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrPermissionCodeDuplicated
		}
		return tx.Select(model.PermissionFields).Create(&permission).Error
	})
	if err != nil {
		slog.Warn("create permission failed", "permission_code", request.Code, "error", err)
		return nil, err
	}
	var created model.Permission
	if err := a.db.WithContext(ctx).
		Select(model.PermissionFields).
		Where(model.PermissionColumnId, pkId).First(&created).Error; err != nil {
		return nil, err
	}
	app, err := loadApplicationSummary(ctx, a.db, created.ApplicationId)
	if err != nil {
		return nil, err
	}
	return permissionToPb(created, app), nil
}

func (a *AuthorizationService) GetPermission(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Permission, error) {
	var permission model.Permission
	err := a.db.WithContext(ctx).
		Select(model.PermissionFields).
		Where(model.PermissionColumnId, request.Id).
		First(&permission).Error
	if err != nil {
		return nil, err
	}
	if err := requirePermissionScope(ctx, a.db, permission.ApplicationId); err != nil {
		return nil, err
	}
	app, err := loadApplicationSummary(ctx, a.db, permission.ApplicationId)
	if err != nil {
		return nil, err
	}
	return permissionToPb(permission, app), nil
}

func (a *AuthorizationService) ListPermissions(ctx context.Context, request *iam_v1.ListPermissionsRequest) (*iam_v1.ListPermissionsResponse, error) {
	appId, scoped, err := dataScopeApplicationId(ctx, a.db)
	if err != nil {
		return nil, err
	}
	// home 未设置的非 admin fail-closed：返回空列表，不暴露其他应用的权限码。
	if scoped && appId <= 0 {
		return &iam_v1.ListPermissionsResponse{Permissions: []*iam_v1.Permission{}}, nil
	}
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		model.PermissionColumnName + " LIKE ?",
		model.PermissionColumnCode + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		return applyKeywordFilter(
			a.db.WithContext(ctx).Model(model.Permission{}).Where(model.PermissionColumnApplicationId, appId),
			keywordConditions, request.Keyword)
	}
	var count int64
	err = buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var permissions []model.Permission
	err = buildQuery().
		Order(model.PermissionColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Find(&permissions).Error
	if err != nil {
		return nil, err
	}
	appIds := make([]int64, 0, len(permissions))
	for _, p := range permissions {
		appIds = append(appIds, p.ApplicationId)
	}
	apps, err := loadApplicationSummaries(ctx, a.db, appIds)
	if err != nil {
		return nil, err
	}
	permList := make([]*iam_v1.Permission, 0, len(permissions))
	for _, p := range permissions {
		permList = append(permList, permissionToPb(p, apps[p.ApplicationId]))
	}
	return &iam_v1.ListPermissionsResponse{
		Permissions: permList,
		Page:        helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (a *AuthorizationService) UpdatePermission(ctx context.Context, request *iam_v1.UpdatePermissionRequest) (*emptypb.Empty, error) {
	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.PermissionColumnName] = request.Name
	}
	if len(request.EnabledStatus) > 0 {
		updates[model.PermissionColumnStatus] = request.EnabledStatus
	}
	updates[model.PermissionColumnUpdateTime] = time.Now()
	// 审计字段服务端注入：仅更新修改人与修改时间
	updates[model.PermissionColumnUpdatedBy] = cast.ToString(auth.GetAccountId(ctx))
	updates[model.PermissionColumnVersion] = gorm.Expr(model.PermissionColumnVersion + " + 1")

	changedStatus := len(request.EnabledStatus) > 0
	var permission model.Permission
	if err := a.db.WithContext(ctx).
		Select(model.PermissionColumnId, model.PermissionColumnApplicationId).
		Where(model.PermissionColumnId, request.Id).
		First(&permission).Error; err != nil {
		return nil, err
	}
	if err := requirePermissionScope(ctx, a.db, permission.ApplicationId); err != nil {
		return nil, err
	}
	appId := permission.ApplicationId
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(model.Permission{}).
			Where(model.PermissionColumnId, request.Id).
			Where(model.PermissionColumnApplicationId, appId).
			Where(model.PermissionColumnVersion, request.Version).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrUpdatePermissionBusy
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrUpdatePermissionBusy) {
			slog.Warn("update permission rejected: version conflict", "permission_id", request.Id, "version", request.Version)
			return nil, err
		}
		slog.Warn("update permission failed", "permission_id", request.Id, "version", request.Version, "error", err)
		return nil, err
	}
	// 禁用状态变更立即生效：失效引用该权限账号的权限快照
	if changedStatus {
		a.invalidatePermissionAccountsSnapshot(ctx, request.Id)
	}
	return &emptypb.Empty{}, nil
}

func (a *AuthorizationService) CreateRole(ctx context.Context, request *iam_v1.CreateRoleRequest) (*iam_v1.Role, error) {
	pkId := krand.NewID()
	now := time.Now()
	// 审计字段服务端注入：created_by/updated_by 取认证上下文操作者
	operator := cast.ToString(auth.GetAccountId(ctx))
	homeId, scoped, err := operatorDataScope(ctx, a.db)
	if err != nil {
		return nil, err
	}
	applicationId := request.ApplicationId
	if scoped {
		// 非 admin 不能自选归属：强制锁定自身 home；home 未设置则 fail-closed。
		if homeId <= 0 {
			return nil, ErrDataScopeRequired
		}
		applicationId = homeId
	}
	role := model.Role{
		Id:            pkId,
		Name:          request.Name,
		Code:          request.RoleCode,
		Status:        model.StatusEnabled,
		ApplicationId: applicationId,
		Version:       1,
		CreatedBy:     operator,
		UpdatedBy:     operator,
		CreateTime:    now,
		UpdateTime:    now,
	}
	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 角色码应用内唯一（000014 起）：查重带归属应用，跨应用同名码互不冲突
		var count int64
		if err := tx.Model(model.Role{}).
			Where(model.RoleColumnCode, request.RoleCode).
			Where(model.RoleColumnApplicationId, applicationId).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrRoleCodeDuplicated
		}
		return tx.Select(model.RoleFields).Create(&role).Error
	})
	if err != nil {
		slog.Warn("create role failed", "role_code", request.RoleCode, "error", err)
		return nil, err
	}
	var created model.Role
	if err := a.db.WithContext(ctx).
		Select(model.RoleFields).
		Where(model.RoleColumnId, pkId).First(&created).Error; err != nil {
		return nil, err
	}
	app, err := loadApplicationSummary(ctx, a.db, created.ApplicationId)
	if err != nil {
		return nil, err
	}
	return roleToPb(created, app), nil
}
