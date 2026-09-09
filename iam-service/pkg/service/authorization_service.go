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
	return db.Where(strings.Join(conditions, " OR "), likeKeyword, likeKeyword)
}

type AuthorizationService struct {
	db  *gorm.DB
	rdb *redis.Client
}

// loadAccountPermissionSnapshot 璇诲彇璐﹀彿鏉冮檺蹇収锛氱紦瀛樺懡涓洿鎺ヨ繑鍥烇紝鏈懡涓垨
// 鏍煎紡涓嶇璧拌绌垮洖婧愰噸寤猴紙涓?auth 鎷︽埅鍣ㄥ叡鐢ㄥ洖婧愰€昏緫锛宺esearch.md D6-鈶?D8锛夈€?
// Redis 鏁呴殰 fail-closed锛屽簳灞傞敊璇粎鍏ユ棩蹇楋紙research.md D12/D13锛夈€?
func loadAccountPermissionSnapshot(ctx context.Context, rdb *redis.Client, db *gorm.DB, accountId int64) ([]string, error) {
	str, err := rdb.Get(ctx, constant.GetAccountPermissionKey(accountId)).Result()
	var permKeys []string
	if err == nil {
		err = json.Unmarshal([]byte(str), &permKeys)
		if err == nil {
			return permKeys, nil
		}
		// 鏍煎紡涓嶇锛堟棫鏍煎紡娈嬬暀锛夎涓烘湭鍛戒腑锛岃蛋鍥炴簮閲嶅缓锛坮esearch.md D8锛?
		slog.Warn("unmarshal permission snapshot failed, rebuilding from db", "account_id", accountId, "error", err)
	} else if !errors.Is(err, redis.Nil) {
		slog.Error("get permission snapshot from redis failed", "account_id", accountId, "error", err)
		return nil, status.Error(codes.FailedPrecondition, "鏉冮檺鏍￠獙鏈嶅姟鏆傛椂涓嶅彲鐢紝璇风◢鍚庨噸璇?)")
	}
	permKeys, err = auth.LoadPermissionSnapshot(ctx, rdb, accountId, NewPermissionSnapshotLoader(db))
	if err != nil {
		slog.Error("rebuild permission snapshot from db failed", "account_id", accountId, "error", err)
		return nil, status.Error(codes.FailedPrecondition, "鏉冮檺鏍￠獙鏈嶅姟鏆傛椂涓嶅彲鐢紝璇风◢鍚庨噸璇?)")
	}
	return permKeys, nil
}

func (a AuthorizationService) CheckPermission(ctx context.Context, request *iam_v1.CheckPermissionRequest) (*iam_v1.CheckPermissionResponse, error) {
	// 璇箟瑁佸喅锛坈ontracts 搂2.8锛夛細permission_code 鎸夎祫婧?key 瑙ｉ噴锛?
	// gRPC 鍩熸嫾 rpc: 鍓嶇紑鍦ㄥ揩鐓т腑鍖归厤锛涙棫鏍煎紡澶嶅悎鐮佺洿鎺ュ垽 false锛屼笉鍋氬吋瀹硅浆鎹€?
	requiredKey := constant.SnapshotKeyPrefixRPC + request.PermissionCode

	if auth.IsAdminFromContext(ctx) {
		return &iam_v1.CheckPermissionResponse{Ok: true}, nil
	}
	accountId := auth.GetAccountId(ctx)
	permKeys, err := loadAccountPermissionSnapshot(ctx, a.rdb, a.db, accountId)
	if err != nil {
		return nil, err
	}
	for _, key := range permKeys {
		if key == requiredKey {
			return &iam_v1.CheckPermissionResponse{
				Ok: true,
			}, nil
		}
	}
	return &iam_v1.CheckPermissionResponse{
		Ok: false,
	}, nil
}

func permissionToPb(p model.Permission) *iam_v1.Permission {
	return &iam_v1.Permission{
		Id:             cast.ToString(p.Id),
		Name:           p.Name,
		ApplicationId:  p.ApplicationId,
		PermissionCode: p.Code,
		EnabledStatus:  boolToStatus(p.Disabled),
		Version:        p.Version,
		CreatedBy:      cast.ToString(p.CreatedBy),
		UpdatedBy:      cast.ToString(p.UpdatedBy),
		CreateTime:     timestamppb.New(p.CreateTime),
		UpdateTime:     timestamppb.New(p.UpdateTime),
		Kind:           p.Kind,
		ResourceList: func() []*iam_v1.Resource {
			list := make([]*iam_v1.Resource, 0, len(p.ResourceList))
			for _, r := range p.ResourceList {
				list = append(list, resourceToPb(*r, nil))
			}
			return list
		}(),
	}
}

func resourceToPb(r model.Resource, application *iam_v1.ApplicationSummary) *iam_v1.Resource {
	pb := &iam_v1.Resource{
		Id:            cast.ToString(r.Id),
		Kind:          r.Kind,
		Key:           r.Key,
		Name:          r.Name,
		RpcFullMethod: r.RpcFullMethod,
		EnabledStatus: boolToStatus(r.Disabled),
		CreateTime:    timestamppb.New(r.CreateTime),
		UpdateTime:    timestamppb.New(r.UpdateTime),
		ApplicationId: r.ApplicationId,
		Version:       r.Version,
		DisplayOrder:  r.DisplayOrder,
		Application:   application,
	}
	if r.ParentId != nil {
		pb.ParentId = *r.ParentId
	}
	return pb
}

// resolveResources 鏍￠獙鏉冮檺绫诲瀷涓庤祫婧愮粦瀹氾紙data-model VR-1锛夛細kind 蹇呴』鍚堟硶銆?
// 缁戝畾闈炵┖銆佽祫婧愬瓨鍦ㄤ笖涓庢潈闄愬悓绫诲瀷锛涜繑鍥炲幓閲嶅悗鐨勮祫婧愬垪琛紙淇濇寔璇锋眰椤哄簭锛夈€?
func resolveResources(tx *gorm.DB, applicationId int64, kind string, resourceKeys []string, resourceIds []int64) ([]model.Resource, error) {
	if !model.IsValidPermissionKind(kind) {
		return nil, ErrInvalidPermissionKind
	}
	if len(resourceKeys) == 0 && len(resourceIds) == 0 {
		return nil, ErrPermissionResourceRequired
	}
	if len(resourceKeys) > 0 && len(resourceIds) > 0 {
		return nil, ErrPermissionResourceInputAmbiguous
	}
	var resources []model.Resource
	if len(resourceIds) > 0 {
		uniqueIds := make([]int64, 0, len(resourceIds))
		idSeen := make(map[int64]struct{}, len(resourceIds))
		for _, id := range resourceIds {
			if _, exists := idSeen[id]; exists {
				continue
			}
			idSeen[id] = struct{}{}
			uniqueIds = append(uniqueIds, id)
		}
		if err := tx.Where(model.ResourceColumnId+" IN ?", uniqueIds).
			Select(model.ResourceFields).Find(&resources).Error; err != nil {
			return nil, err
		}
		if len(resources) != len(uniqueIds) {
			return nil, ErrResourceNotFound
		}
	} else {
		seen := make(map[string]struct{}, len(resourceKeys))
		keys := make([]string, 0, len(resourceKeys))
		for _, key := range resourceKeys {
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
		if err := tx.Where("`key` IN ?", keys).
			Select(model.ResourceFields).Find(&resources).Error; err != nil {
			return nil, err
		}
		if len(resources) != len(keys) {
			return nil, ErrResourceNotFound
		}
	}
	ordered := make([]model.Resource, 0, len(resources))
	for _, resource := range resources {
		if resource.ApplicationId != applicationId {
			return nil, ErrResourceParentInvalid
		}
		ordered = append(ordered, resource)
	}
	return ordered, nil
}

func selectResourcesWithAncestors(tx *gorm.DB, applicationId int64, selected []model.Resource) ([]model.Resource, error) {
	var resources []model.Resource
	if err := tx.Where(model.ResourceColumnApplicationId, applicationId).
		Select(model.ResourceFields).Find(&resources).Error; err != nil {
		return nil, err
	}
	resourceById := make(map[int64]model.Resource, len(resources))
	for _, r := range resources {
		resourceById[r.Id] = r
	}
	selectedById := make(map[int64]struct{}, len(selected))
	for _, resource := range selected {
		selectedById[resource.Id] = struct{}{}
	}
	for _, resource := range selected {
		current := resource
		for current.ParentId != nil {
			parentId := *current.ParentId
			parent, exists := resourceById[parentId]
			if !exists {
				return nil, ErrResourceParentInvalid
			}
			if _, alreadySelected := selectedById[parentId]; !alreadySelected {
				selectedById[parentId] = struct{}{}
				selected = append(selected, parent)
			}
			current = parent
		}
	}
	return selected, nil
}

// replacePermissionResourceBindings 灏嗘潈闄愮殑璧勬簮缁戝畾鏇挎崲涓虹粰瀹氳祫婧愬垪琛紙FR-004锛夈€?
func replacePermissionResourceBindings(tx *gorm.DB, permissionId int64, resources []model.Resource) error {
	if err := tx.Where(model.PermissionResourceColumnPermissionId, permissionId).
		Delete(&model.PermissionResource{}).Error; err != nil {
		return err
	}
	bindings := make([]model.PermissionResource, 0, len(resources))
	for _, r := range resources {
		bindings = append(bindings, model.PermissionResource{PermissionId: permissionId, ResourceId: r.Id})
	}
	if len(bindings) > 0 {
		return tx.Create(&bindings).Error
	}
	return nil
}

func roleToPb(r model.Role, application *iam_v1.ApplicationSummary) *iam_v1.Role {
	response := &iam_v1.Role{
		Id:            cast.ToString(r.Id),
		Name:          r.Name,
		ApplicationId: r.ApplicationId,
		RoleCode:      r.Code,
		EnabledStatus: boolToStatus(r.Disabled),
		Version:       r.Version,
		CreatedBy:     cast.ToString(r.CreatedBy),
		UpdatedBy:     cast.ToString(r.UpdatedBy),
		CreateTime:    timestamppb.New(r.CreateTime),
		UpdateTime:    timestamppb.New(r.UpdateTime),
		PermissionCodeList: func() []string {
			codes := make([]string, 0, len(r.PermissionList))
			for _, permission := range r.PermissionList {
				codes = append(codes, permission.Code)
			}
			return codes
		}(),
	}
	response.Application = application
	response.PermissionList = func() []*iam_v1.PermissionSummary {
		summaries := make([]*iam_v1.PermissionSummary, 0, len(r.PermissionList))
		for _, permission := range r.PermissionList {
			summaries = append(summaries, &iam_v1.PermissionSummary{
				Id:             permission.Id,
				Name:           permission.Name,
				PermissionCode: permission.Code,
				Kind:           permission.Kind,
				EnabledStatus:  boolToStatus(permission.Disabled),
			})
		}
		return summaries
	}()
	return response
}

func (a AuthorizationService) CreatePermission(ctx context.Context, request *iam_v1.CreatePermissionRequest) (*iam_v1.Permission, error) {
	pkId := krand.NewID()
	now := time.Now()
	// 瀹¤瀛楁鏈嶅姟绔敞鍏ワ細created_by/updated_by 鍙栬璇佷笂涓嬫枃鎿嶄綔鑰咃紝鏃犺璇佷笂涓嬫枃鏃?0锛坮esearch.md D16锛?
	operator := auth.GetAccountId(ctx)
	permission := model.Permission{
		Id:            pkId,
		Name:          request.Name,
		Code:          request.Code,
		Kind:          request.Kind,
		Value:         request.Value,
		Description:   request.Description,
		ApplicationId: request.ApplicationId,
		Disabled:      false,
		CreateTime:    now,
		UpdateTime:    now,
		CreatedBy:     operator,
		UpdatedBy:     operator,
	}
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(model.Permission{}).
			Where(model.PermissionColumnCode, request.Code).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrPermissionCodeDuplicated
		}
		// 绫诲瀷涓庤祫婧愮粦瀹氭牎楠岋紙data-model VR-1锛?
		resources, err := resolveResources(tx, request.ApplicationId, request.Kind, request.ResourceKeyList, request.ResourceIdList)
		if err != nil {
			return err
		}
		resources, err = selectResourcesWithAncestors(tx, request.ApplicationId, resources)
		if err != nil {
			return err
		}
		if err := tx.Model(model.Permission{}).Create(&permission).Error; err != nil {
			return err
		}
		return replacePermissionResourceBindings(tx, pkId, resources)
	})
	if err != nil {
		slog.Warn("create permission failed", "permission_code", request.Code, "kind", request.Kind, "error", err)
		return nil, err
	}
	// 鍥炰紶鍚祫婧愮粦瀹氾紙瀹硶 XII 鍏ㄩ噺鍥炰紶锛?
	var created model.Permission
	if err := a.db.WithContext(ctx).
		Select(model.PermissionFields).
		Where(model.PermissionColumnId, pkId).First(&created).Error; err != nil {
		return nil, err
	}
	if err := loadPermissionResources(ctx, a.db, &created); err != nil {
		return nil, err
	}
	return permissionToPb(created), nil
}

func (a AuthorizationService) GetPermission(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Permission, error) {
	var permission model.Permission
	err := a.db.WithContext(ctx).
		Select(model.PermissionFields).
		Where(model.PermissionColumnId, request.Id).First(&permission).Error
	if err != nil {
		return nil, err
	}
	if err := loadPermissionResources(ctx, a.db, &permission); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return permissionToPb(permission), nil
}

func (a AuthorizationService) ListPermissions(ctx context.Context, request *iam_v1.ListPermissionsRequest) (*iam_v1.ListPermissionsResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		model.PermissionColumnName + " LIKE ?",
		model.PermissionColumnCode + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		query := applyKeywordFilter(a.db.WithContext(ctx).Model(model.Permission{}), keywordConditions, request.Keyword)
		// 鎸夌被鍨嬭繃婊わ紙濂戠害澧為噺 ListPermissionsRequest.kind锛?
		if len(request.Kind) > 0 {
			query = query.Where(model.PermissionColumnKind, request.Kind)
		}
		return query
	}
	var count int64
	err := buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var permissions []model.Permission
	err = buildQuery().
		Order(model.PermissionColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Preload("ResourceList").
		Find(&permissions).Error
	if err != nil {
		return nil, err
	}
	permList := make([]*iam_v1.Permission, 0, len(permissions))
	for _, p := range permissions {
		permList = append(permList, permissionToPb(p))
	}
	return &iam_v1.ListPermissionsResponse{
		Permissions: permList,
		Page:        helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (a AuthorizationService) UpdatePermission(ctx context.Context, request *iam_v1.UpdatePermissionRequest) (*emptypb.Empty, error) {
	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.PermissionColumnName] = request.Name
	}
	if len(request.EnabledStatus) > 0 {
		updates[model.PermissionColumnDisabled] = statusToBool(request.EnabledStatus)
	}
	updates[model.PermissionColumnUpdateTime] = time.Now()
	// 瀹¤瀛楁鏈嶅姟绔敞鍏ワ細浠呮洿鏂颁慨鏀逛汉涓庝慨鏀规椂闂达紙research.md D16锛屽娉?XII锛?
	updates[model.PermissionColumnUpdatedBy] = auth.GetAccountId(ctx)
	updates[model.PermissionColumnVersion] = gorm.Expr(model.PermissionColumnVersion + " + 1")

	changedBinding := len(request.ResourceKeyList) > 0 || len(request.ResourceIdList) > 0
	changedStatus := len(request.EnabledStatus) > 0
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var permission model.Permission
		if err := tx.Where(model.PermissionColumnId, request.Id).Select(model.PermissionFields).First(&permission).Error; err != nil {
			return err
		}
		result := tx.Model(model.Permission{}).
			Where(model.PermissionColumnId, request.Id).
			Where(model.PermissionColumnVersion, request.Version).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrUpdatePermissionBusy
		}
		// 璧勬簮缁戝畾鏇挎崲寮忔洿鏂帮細鎻愪緵鐨?key 娓呭崟鏁翠綋鏇挎崲鏃㈡湁缁戝畾锛團R-004锛?
		// kind 涓嶅彲鍙橈細浠ュ簱鍐?permission.Kind 鏍￠獙缁戝畾绫诲瀷涓€鑷存€?
		if changedBinding {
			resources, err := resolveResources(tx, permission.ApplicationId, permission.Kind, request.ResourceKeyList, request.ResourceIdList)
			if err != nil {
				return err
			}
			resources, err = selectResourcesWithAncestors(tx, permission.ApplicationId, resources)
			if err != nil {
				return err
			}
			if err := replacePermissionResourceBindings(tx, permission.Id, resources); err != nil {
				return err
			}
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
	// 缁戝畾澧炲噺鎴栫鐢ㄧ姸鎬佸彉鏇寸珛鍗崇敓鏁堬細澶辨晥寮曠敤璇ユ潈闄愯处鍙风殑鏉冮檺蹇収锛坉ata-model VR-3锛?
	if changedBinding || changedStatus {
		a.invalidatePermissionAccountsSnapshot(ctx, request.Id)
	}
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) CreateRole(ctx context.Context, request *iam_v1.CreateRoleRequest) (*iam_v1.Role, error) {
	pkId := krand.NewID()
	now := time.Now()
	if request.ApplicationId <= 0 {
		return nil, ErrApplicationRequired
	}
	var application model.Application
	if err := a.db.WithContext(ctx).
		Where(model.ApplicationColumnId, request.ApplicationId).
		Select(model.ApplicationFields).First(&application).Error; err != nil {
		return nil, ErrApplicationNotFound
	}
	// 瀹¤瀛楁鏈嶅姟绔敞鍏ワ細created_by/updated_by 鍙栬璇佷笂涓嬫枃鎿嶄綔鑰咃紝鏃犺璇佷笂涓嬫枃鏃?0锛坮esearch.md D16锛?
	operator := auth.GetAccountId(ctx)
	role := model.Role{
		Id:            pkId,
		ApplicationId: request.ApplicationId,
		Name:          request.Name,
		Code:          request.RoleCode,
		Disabled:      false,
		CreateTime:    now,
		UpdateTime:    now,
		CreatedBy:     operator,
		UpdatedBy:     operator,
	}
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(model.Role{}).
			Where(model.RoleColumnCode, request.RoleCode).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrRoleCodeDuplicated
		}
		return tx.Model(model.Role{}).Create(&role).Error
	})
	if err != nil {
		slog.Warn("create role failed", "role_code", request.RoleCode, "error", err)
		return nil, err
	}
	summaries, err := loadApplicationSummaries(ctx, a.db, []int64{role.ApplicationId})
	if err != nil {
		return nil, err
	}
	return roleToPb(role, summaries[role.ApplicationId]), nil
}

func (a AuthorizationService) GetRole(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Role, error) {
	var role model.Role
	err := a.db.WithContext(ctx).
		Select(model.RoleFields).
		Where(model.RoleColumnId, request.Id).First(&role).Error
	if err != nil {
		return nil, err
	}
	if err := loadRolePermissions(ctx, a.db, &role); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	summaries, err := loadApplicationSummaries(ctx, a.db, []int64{role.ApplicationId})
	if err != nil {
		return nil, err
	}
	return roleToPb(role, summaries[role.ApplicationId]), nil
}

func (a AuthorizationService) ListRoles(ctx context.Context, request *iam_v1.ListRolesRequest) (*iam_v1.ListRolesResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		model.RoleColumnName + " LIKE ?",
		model.RoleColumnCode + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		return applyKeywordFilter(a.db.WithContext(ctx).Model(model.Role{}), keywordConditions, request.Keyword)
	}
	var count int64
	err := buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var roles []model.Role
	err = buildQuery().
		Order(model.RoleColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Find(&roles).Error
	if err != nil {
		return nil, err
	}
	if err := loadRolePermissionsForRoles(ctx, a.db, roles); err != nil {
		return nil, err
	}
	applicationIds := make([]int64, 0, len(roles))
	for _, role := range roles {
		applicationIds = append(applicationIds, role.ApplicationId)
	}
	roleApplications, err := loadApplicationSummaries(ctx, a.db, applicationIds)
	if err != nil {
		return nil, err
	}
	roleList := make([]*iam_v1.Role, 0, len(roles))
	for _, r := range roles {
		roleList = append(roleList, roleToPb(r, roleApplications[r.ApplicationId]))
	}
	return &iam_v1.ListRolesResponse{
		Roles: roleList,
		Page:  helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (a AuthorizationService) UpdateRole(ctx context.Context, request *iam_v1.UpdateRoleRequest) (*emptypb.Empty, error) {
	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.RoleColumnName] = request.Name
	}
	if len(request.EnabledStatus) > 0 {
		updates[model.RoleColumnDisabled] = statusToBool(request.EnabledStatus)
	}
	updates[model.RoleColumnUpdateTime] = time.Now()
	// 瀹¤瀛楁鏈嶅姟绔敞鍏ワ細浠呮洿鏂颁慨鏀逛汉涓庝慨鏀规椂闂达紙research.md D16锛屽娉?XII锛?
	updates[model.RoleColumnUpdatedBy] = auth.GetAccountId(ctx)
	updates[model.RoleColumnVersion] = gorm.Expr(model.RoleColumnVersion + " + 1")
	resp, err := a.updateRoleInTransaction(ctx, request, updates)
	if err != nil {
		return nil, err
	}
	// 鏉冮檺闆嗗悎澧炲噺鎴栫鐢ㄧ姸鎬佸彉鏇寸珛鍗崇敓鏁堬細澶辨晥缁戝畾璐﹀彿鐨勬潈闄愬揩鐓э紙research.md D6-鈶狅級
	if len(request.AddedPermissionCodeList) > 0 || len(request.RemovedPermissionCodeList) > 0 || len(request.EnabledStatus) > 0 {
		a.invalidateRoleAccountsSnapshot(ctx, request.Id)
	}
	return resp, nil
}

// invalidateRoleAccountsSnapshot 鍒犻櫎缁戝畾璇ヨ鑹茬殑鍏ㄩ儴璐﹀彿鐨勬潈闄愬揩鐓ч敭锛坮esearch.md D6-鈶狅級銆?
// 蹇収涓烘淳鐢熸暟鎹細鍒犻櫎澶辫触浠呰鏃ュ織锛岀敱鐭?TTL 鍏滃簳锛屼笉褰卞搷宸叉彁浜ょ殑涓氬姟鍙樻洿銆?
func (a AuthorizationService) invalidateRoleAccountsSnapshot(ctx context.Context, roleId int64) {
	if a.rdb == nil {
		return
	}
	var accountIds []int64
	err := a.db.WithContext(ctx).Model(model.RAccountRole{}).
		Where(model.RAccountRoleColumnRoleId, roleId).
		Pluck(model.RAccountRoleColumnAccountId, &accountIds).Error
	if err != nil {
		slog.Warn("query role bound accounts failed", "role_id", roleId, "error", err)
		return
	}
	for _, accountId := range accountIds {
		a.deleteAccountPermissionSnapshot(ctx, accountId, roleId)
	}
}

// invalidatePermissionAccountsSnapshot 鍒犻櫎寮曠敤璇ユ潈闄愶紙缁忕敱浠绘剰瑙掕壊缁戝畾锛夌殑
// 鍏ㄩ儴璐﹀彿鐨勬潈闄愬揩鐓ч敭锛坰pecs/005-typed-permission-model VR-3锛夈€?
func (a AuthorizationService) invalidatePermissionAccountsSnapshot(ctx context.Context, permissionId int64) {
	if a.rdb == nil {
		return
	}
	var accountIds []int64
	err := a.db.WithContext(ctx).Raw(
		"SELECT DISTINCT ar.account_id FROM t_r_account_role ar "+
			"JOIN t_r_role_permission rp ON rp.role_id = ar.role_id "+
			"WHERE rp.permission_id = ?", permissionId).Scan(&accountIds).Error
	if err != nil {
		slog.Warn("query permission bound accounts failed", "permission_id", permissionId, "error", err)
		return
	}
	for _, accountId := range accountIds {
		a.deleteAccountPermissionSnapshot(ctx, accountId, permissionId)
	}
}

// deleteAccountPermissionSnapshot 鍒犻櫎鍗曚釜璐﹀彿鐨勬潈闄愬揩鐓ч敭锛屽け璐ヤ粎璁版棩蹇楋紙娲剧敓鏁版嵁鍏滃簳鍙噸寤猴級銆?
func (a AuthorizationService) deleteAccountPermissionSnapshot(ctx context.Context, accountId, sourceId int64) {
	if err := a.rdb.Del(ctx, constant.GetAccountPermissionKey(accountId)).Err(); err != nil {
		slog.Warn("invalidate account permission snapshot failed",
			"account_id", accountId, "source_id", sourceId, "error", err)
	}
}

func (a AuthorizationService) updateRoleInTransaction(ctx context.Context, request *iam_v1.UpdateRoleRequest, updates map[string]interface{}) (*emptypb.Empty, error) {
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.Where(model.RoleColumnId, request.Id).Select(model.RoleFields).First(&role).Error; err != nil {
			return err
		}
		result := tx.Model(model.Role{}).
			Where(model.RoleColumnId, request.Id).
			Where(model.RoleColumnVersion, request.Version).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrUpdateRoleBusy
		}

		if len(request.AddedPermissionCodeList) == 0 && len(request.RemovedPermissionCodeList) == 0 {
			return nil
		}
		permissionCodes := make([]string, 0, len(request.AddedPermissionCodeList)+len(request.RemovedPermissionCodeList))
		permissionCodes = append(permissionCodes, request.AddedPermissionCodeList...)
		permissionCodes = append(permissionCodes, request.RemovedPermissionCodeList...)
		var permissions []model.Permission
		if err := tx.Where(model.PermissionColumnCode+" IN ?", permissionCodes).Select(model.PermissionFields).Find(&permissions).Error; err != nil {
			return err
		}
		if len(permissions) != len(permissionCodes) {
			return ErrPermissionNotFound
		}

		permissionByCode := make(map[string]model.Permission, len(permissions))
		for _, permission := range permissions {
			permissionByCode[permission.Code] = permission
		}
		added := make([]model.Permission, 0, len(request.AddedPermissionCodeList))
		for _, code := range request.AddedPermissionCodeList {
			added = append(added, permissionByCode[code])
		}
		removed := make([]model.Permission, 0, len(request.RemovedPermissionCodeList))
		for _, code := range request.RemovedPermissionCodeList {
			removed = append(removed, permissionByCode[code])
		}
		association := tx.Model(&role).Association("PermissionList")
		if err := association.Append(added); err != nil {
			return err
		}
		if len(removed) > 0 {
			return association.Delete(removed)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrPermissionNotFound) {
			slog.Warn("update role rejected: permission code not found",
				"role_id", request.Id,
				"added_permission_codes", request.AddedPermissionCodeList,
				"removed_permission_codes", request.RemovedPermissionCodeList)
		} else {
			slog.Warn("commit role update failed",
				"role_id", request.Id, "version", request.Version, "error", err)
		}
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) DeletePermission(ctx context.Context, request *iam_v1.DeletePermissionRequest) (*emptypb.Empty, error) {
	var count int64
	err := a.db.WithContext(ctx).Table("t_r_role_permission").
		Where("permission_id = ?", request.Id).
		Count(&count).Error
	if err != nil {
		slog.Warn("check permission references failed", "permission_id", request.Id, "error", err)
		return nil, err
	}
	if count > 0 {
		slog.Warn("delete permission rejected: referenced by roles", "permission_id", request.Id, "reference_count", count)
		return nil, ErrPermissionReferenced
	}

	// 鏈瑙掕壊寮曠敤鐨勬潈闄愬垹闄ゆ椂绾ц仈娓呯悊璧勬簮缁戝畾锛屼笉浜х敓鎮┖缁戝畾琛岋紙FR-011锛?
	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(model.PermissionResourceColumnPermissionId, request.Id).
			Delete(&model.PermissionResource{}).Error; err != nil {
			return err
		}
		result := tx.Where(model.PermissionColumnId, request.Id).Delete(model.Permission{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Warn("delete permission rejected: permission not found", "permission_id", request.Id)
			return nil, gorm.ErrRecordNotFound
		}
		slog.Warn("delete permission failed", "permission_id", request.Id, "error", err)
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) DeleteRole(ctx context.Context, request *iam_v1.DeleteRoleRequest) (*emptypb.Empty, error) {
	// VR-4锛氱鐞嗗憳瑙掕壊涓虹郴缁熷唴缃紝绂佹鍒犻櫎锛坰pecs/005-typed-permission-model US3锛?
	var role model.Role
	if err := a.db.WithContext(ctx).Where(model.RoleColumnId, request.Id).Select(model.RoleFields).First(&role).Error; err != nil {
		slog.Warn("delete role rejected: role not found", "role_id", request.Id, "error", err)
		return nil, err
	}
	if role.Code == model.RoleAdmin {
		slog.Warn("delete role rejected: builtin admin role", "role_id", request.Id)
		return nil, ErrRoleAdminProtected
	}

	var count int64
	err := a.db.WithContext(ctx).Model(model.RAccountRole{}).
		Where(model.RAccountRoleColumnRoleId, request.Id).
		Count(&count).Error
	if err != nil {
		slog.Warn("check role bindings failed", "role_id", request.Id, "error", err)
		return nil, err
	}
	if count > 0 {
		slog.Warn("delete role rejected: bound by accounts", "role_id", request.Id, "binding_count", count)
		return nil, ErrRoleReferenced
	}

	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM t_r_role_permission WHERE role_id = ?", request.Id).Error; err != nil {
			return err
		}
		result := tx.Where(model.RoleColumnId, request.Id).Delete(model.Role{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		slog.Warn("delete role failed", "role_id", request.Id, "error", err)
		return nil, err
	}
	// 绾ц仈娓呴櫎鏉冮檺鍏宠仈绔嬪嵆鐢熸晥锛氬け鏁堬紙娈嬩綑锛夌粦瀹氳处鍙风殑鏉冮檺蹇収锛坮esearch.md D6-鈶狅級
	a.invalidateRoleAccountsSnapshot(ctx, request.Id)
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) ListResources(ctx context.Context, request *iam_v1.ListResourcesRequest) (*iam_v1.ListResourcesResponse, error) {
	limit, offset := helper.GetLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		// key 鏄?MySQL 淇濈暀瀛楋紝鎷煎叆鍘熷 SQL 蹇呴』鍙嶅紩鍙峰寘瑁?
		"`key` LIKE ?",
		model.ResourceColumnName + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		query := applyKeywordFilter(a.db.WithContext(ctx).Model(model.Resource{}), keywordConditions, request.Keyword)
		if len(request.Kind) > 0 {
			query = query.Where(model.ResourceColumnKind, request.Kind)
		}
		return query
	}
	var count int64
	err := buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var resources []model.Resource
	err = buildQuery().
		Select(model.ResourceFields).
		Order(model.ResourceColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Find(&resources).Error
	if err != nil {
		return nil, err
	}
	resourceList := make([]*iam_v1.Resource, 0, len(resources))
	for _, r := range resources {
		resourceList = append(resourceList, resourceToPb(r, nil))
	}
	return &iam_v1.ListResourcesResponse{
		Resources: resourceList,
		Page:      helper.GetPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

func (a AuthorizationService) CreateResource(ctx context.Context, request *iam_v1.CreateResourceRequest) (*iam_v1.Resource, error) {
	if request.ApplicationId <= 0 {
		return nil, ErrResourceApplicationRequired
	}
	now := time.Now()
	operator := auth.GetAccountId(ctx)
	resource := model.Resource{
		Id:            krand.NewID(),
		ApplicationId: request.ApplicationId,
		Kind:          request.Kind,
		Key:           request.Key,
		Name:          request.Name,
		RpcFullMethod: request.RpcFullMethod,
		Disabled:      request.Disabled,
		DisplayOrder:  request.DisplayOrder,
		CreateTime:    now,
		UpdateTime:    now,
		CreatedBy:     operator,
		UpdatedBy:     operator,
		Version:       1,
	}
	if request.ParentId > 0 {
		resource.ParentId = &request.ParentId
	}
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var app model.Application
		if err := tx.Where(model.ApplicationColumnId, request.ApplicationId).
			Select(model.ApplicationFields).First(&app).Error; err != nil {
			return ErrResourceApplicationInvalid
		}
		var count int64
		if err := tx.Model(model.Resource{}).
			Where(model.ResourceColumnApplicationId, request.ApplicationId).
			Where(model.ResourceColumnKey, request.Key).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrResourceKeyDuplicated
		}
		if request.ParentId > 0 {
			if err := validateParent(ctx, tx, request.ParentId, request.ApplicationId, 0); err != nil {
				return err
			}
		}
		return tx.Create(&resource).Error
	})
	if err != nil {
		slog.Warn("create resource failed", "key", request.Key, "error", err)
		return nil, err
	}
	summaries, err := loadApplicationSummaries(ctx, a.db, []int64{resource.ApplicationId})
	if err != nil {
		return nil, err
	}
	return resourceToPb(resource, summaries[resource.ApplicationId]), nil
}

func (a AuthorizationService) GetResource(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Resource, error) {
	var resource model.Resource
	if err := a.db.WithContext(ctx).
		Select(model.ResourceFields).
		Where(model.ResourceColumnId, request.Id).First(&resource).Error; err != nil {
		return nil, err
	}
	summaries, err := loadApplicationSummaries(ctx, a.db, []int64{resource.ApplicationId})
	if err != nil {
		return nil, err
	}
	return resourceToPb(resource, summaries[resource.ApplicationId]), nil
}

func (a AuthorizationService) UpdateResource(ctx context.Context, request *iam_v1.UpdateResourceRequest) (*iam_v1.Resource, error) {
	updates := map[string]interface{}{
		model.ResourceColumnUpdateTime: time.Now(),
		model.ResourceColumnUpdatedBy:  auth.GetAccountId(ctx),
		model.ResourceColumnVersion:    gorm.Expr(model.ResourceColumnVersion + " + 1"),
	}
	if len(request.Name) > 0 {
		updates[model.ResourceColumnName] = request.Name
	}
	if len(request.RpcFullMethod) > 0 {
		updates[model.ResourceColumnRpcFullMethod] = request.RpcFullMethod
	}
	updates[model.ResourceColumnDisabled] = request.Disabled
	if request.DisplayOrder != 0 {
		updates[model.ResourceColumnDisplayOrder] = request.DisplayOrder
	}

	var parentId *int64
	if request.ParentCleared {
		parentId = nil
	} else if request.ParentId > 0 {
		parentId = &request.ParentId
	}

	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var resource model.Resource
		if err := tx.Where(model.ResourceColumnId, request.Id).
			Select(model.ResourceFields).First(&resource).Error; err != nil {
			return err
		}
		if parentId != nil {
			if err := validateParent(ctx, tx, *parentId, resource.ApplicationId, resource.Id); err != nil {
				return err
			}
		}
		updates[model.ResourceColumnParentId] = parentId
		result := tx.Model(model.Resource{}).
			Where(model.ResourceColumnId, request.Id).
			Where(model.ResourceColumnVersion, request.Version).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrResourceUpdateBusy
		}
		return nil
	})
	if err != nil {
		slog.Warn("update resource failed", "id", request.Id, "error", err)
		return nil, err
	}
	return a.GetResource(ctx, &common_v1.GetRequest{Id: request.Id})
}

func (a AuthorizationService) DeleteResource(ctx context.Context, request *iam_v1.DeleteResourceRequest) (*emptypb.Empty, error) {
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var resource model.Resource
		if err := tx.Where(model.ResourceColumnId, request.Id).
			Select(model.ResourceFields).First(&resource).Error; err != nil {
			return err
		}
		var childCount int64
		if err := tx.Model(model.Resource{}).
			Where(model.ResourceColumnParentId, request.Id).
			Count(&childCount).Error; err != nil {
			return err
		}
		if childCount > 0 {
			return ErrResourceHasChildren
		}
		var bindingCount int64
		if err := tx.Model(model.PermissionResource{}).
			Where(model.PermissionResourceColumnResourceId, request.Id).
			Count(&bindingCount).Error; err != nil {
			return err
		}
		if bindingCount > 0 {
			return ErrResourceReferenced
		}
		result := tx.Where(model.ResourceColumnId, request.Id).Delete(model.Resource{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		slog.Warn("delete resource failed", "id", request.Id, "error", err)
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) ListResourceTree(ctx context.Context, request *iam_v1.ListResourceTreeRequest) (*iam_v1.ListResourceTreeResponse, error) {
	var resources []model.Resource
	if err := a.db.WithContext(ctx).
		Where(model.ResourceColumnApplicationId, request.ApplicationId).
		Select(model.ResourceFields).
		Order(model.ResourceColumnDisplayOrder + " ASC, " + model.ResourceColumnId + " ASC").
		Find(&resources).Error; err != nil {
		return nil, err
	}
	appSummaries, err := loadApplicationSummaries(ctx, a.db, []int64{request.ApplicationId})
	if err != nil {
		return nil, err
	}
	resourceById := make(map[int64]*model.Resource, len(resources))
	for i := range resources {
		resourceById[resources[i].Id] = &resources[i]
	}
	var roots []*iam_v1.ResourceTreeNode
	for _, r := range resources {
		if r.ParentId == nil {
			roots = append(roots, buildResourceTreeNode(r, resourceById, appSummaries[request.ApplicationId]))
		}
	}
	return &iam_v1.ListResourceTreeResponse{Roots: roots}, nil
}

// ListMyMenuTree 杩斿洖褰撳墠鐢ㄦ埛鍦ㄦ寚瀹氬簲鐢ㄤ笅鍙鐨勮彍鍗曟爲锛堜粎 kind=menu锛宻pecs/008 FR-008/009锛夈€?
// 鑿滃崟鍙鎬х敱鏈嶅姟绔鍐筹紙research.md D3锛夛細鏅€氱敤鎴锋寜鏉冮檺閿揩鐓э紙menu: 鍓嶇紑锛?
// 杩囨护涓斿懡涓妭鐐圭殑绁栧厛璺緞涓€骞惰繑鍥烇紱绠＄悊鍛樿繑鍥炶搴旂敤鍏ㄩ儴鑿滃崟鑺傜偣銆?
// 搴旂敤涓嶅瓨鍦ㄦ垨宸插仠鐢ㄨ繑鍥炰笟鍔￠敊璇紙research.md D4锛夛紱绌鸿彍鍗曡繑鍥炵┖ roots銆?
func (a AuthorizationService) ListMyMenuTree(ctx context.Context, request *iam_v1.ListMyMenuTreeRequest) (*iam_v1.ListResourceTreeResponse, error) {
	var application model.Application
	err := a.db.WithContext(ctx).
		Select(model.ApplicationFields).
		Where(model.ApplicationColumnId, request.ApplicationId).
		First(&application).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplicationNotFound
		}
		slog.Warn("get application for menu tree failed", "application_id", request.ApplicationId, "error", err)
		return nil, err
	}
	if application.Disabled == 1 {
		slog.Warn("list my menu tree rejected: application disabled", "application_id", request.ApplicationId)
		return nil, ErrApplicationDisabled
	}

	// 浠呭姞杞借搴旂敤鐨?menu 绫昏祫婧愶紙display_order 鎺掑簭锛?
	var resources []model.Resource
	if err := a.db.WithContext(ctx).
		Where(model.ResourceColumnApplicationId, request.ApplicationId).
		Where(model.ResourceColumnKind, model.PermissionKindMenu).
		Select(model.ResourceFields).
		Order(model.ResourceColumnDisplayOrder + " ASC, " + model.ResourceColumnId + " ASC").
		Find(&resources).Error; err != nil {
		slog.Warn("load menu resources failed", "application_id", request.ApplicationId, "error", err)
		return nil, err
	}

	visible := make(map[int64]struct{}, len(resources))
	// 管理员与普通用户同受权限约束（规格变更，原管理员全量可见分支已移除）：
	// 菜单可见性一律按账号权限键过滤，新增菜单需绑定到某权限后才对所有人可见。
	keys, err := a.loadAccountMenuKeys(ctx)
	if err != nil {
		return nil, err
	}
	byId := make(map[int64]model.Resource, len(resources))
	for _, r := range resources {
		byId[r.Id] = r
		if _, ok := keys[constant.SnapshotKeyPrefixMenu+r.Key]; ok {
			visible[r.Id] = struct{}{}
		}
	}
	// 鍛戒腑鑺傜偣鐨勭鍏堣矾寰勪竴骞惰繑鍥烇紝淇濊瘉鏍戝彲娓叉煋锛坰pecs/008 research.md D3锛?
	for _, r := range resources {
		if _, ok := visible[r.Id]; !ok {
			continue
		}
		for parent := r.ParentId; parent != nil; parent = byId[*parent].ParentId {
			if _, seen := visible[*parent]; seen {
				break
			}
			// 绁栧厛鍙兘涓嶅湪 menu 鏌ヨ缁撴灉涓紙鐞嗚涓?menu 鏍戠埗绾у悓涓?menu锛夛紝缂哄け鍗虫
			node, ok := byId[*parent]
			if !ok {
				break
			}
			visible[node.Id] = struct{}{}
		}
	}

	kept := make([]model.Resource, 0, len(visible))
	byParent := make(map[int64][]model.Resource, len(resources))
	var rootCandidates []model.Resource
	for _, r := range resources {
		if _, ok := visible[r.Id]; !ok {
			continue
		}
		kept = append(kept, r)
		if r.ParentId == nil {
			rootCandidates = append(rootCandidates, r)
		} else {
			byParent[*r.ParentId] = append(byParent[*r.ParentId], r)
		}
	}

	appSummaries, err := loadApplicationSummaries(ctx, a.db, []int64{request.ApplicationId})
	if err != nil {
		return nil, err
	}
	summary := appSummaries[request.ApplicationId]
	roots := make([]*iam_v1.ResourceTreeNode, 0, len(rootCandidates))
	for _, r := range rootCandidates {
		roots = append(roots, buildMyMenuNode(r, byParent, summary))
	}
	return &iam_v1.ListResourceTreeResponse{Roots: roots}, nil
}

// buildMyMenuNode 浠?kept 闆嗗悎鏋勫缓鑿滃崟鏍戣妭鐐癸紙children 浠呭惈杩囨护鍚庝粛淇濈暀鐨勮妭鐐癸級
func buildMyMenuNode(r model.Resource, byParent map[int64][]model.Resource, appSummary *iam_v1.ApplicationSummary) *iam_v1.ResourceTreeNode {
	node := &iam_v1.ResourceTreeNode{
		Resource: resourceToPb(r, appSummary),
		Children: []*iam_v1.ResourceTreeNode{},
	}
	for _, child := range byParent[r.Id] {
		node.Children = append(node.Children, buildMyMenuNode(child, byParent, appSummary))
	}
	return node
}

// loadAccountMenuKeys 璇诲彇璐﹀彿鏉冮檺閿揩鐓э紙甯?menu:/http:/rpc: 鍓嶇紑锛夈€?
// 蹇収缂哄け鏃剁粡璇荤┛鍥炴簮閲嶅缓锛坮esearch.md D6-鈶?鍚屼竴瀹炵幇璺緞锛夈€?
func (a AuthorizationService) loadAccountMenuKeys(ctx context.Context) (map[string]struct{}, error) {
	accountId := auth.GetAccountId(ctx)
	if a.rdb != nil {
		raw, err := a.rdb.Get(ctx, constant.GetAccountPermissionKey(accountId)).Result()
		if err == nil {
			var keys []string
			if err := json.Unmarshal([]byte(raw), &keys); err != nil {
				slog.Warn("unmarshal permission snapshot failed", "account_id", accountId, "error", err)
				return nil, err
			}
			keyDict := make(map[string]struct{}, len(keys))
			for _, key := range keys {
				keyDict[key] = struct{}{}
			}
			return keyDict, nil
		}
		if !errors.Is(err, redis.Nil) {
			slog.Warn("get permission snapshot failed", "account_id", accountId, "error", err)
			return nil, err
		}
	}
	// 蹇収缂哄け锛氳绌垮洖婧?
	keys, err := NewPermissionSnapshotLoader(a.db)(ctx, accountId)
	if err != nil {
		slog.Warn("load permission snapshot read-through failed", "account_id", accountId, "error", err)
		return nil, err
	}
	keyDict := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keyDict[key] = struct{}{}
	}
	return keyDict, nil
}

// validateParent checks that the parent exists, belongs to the same application, and
// that placing the child under it would not create a cycle or exceed max depth.
// It loads all resources for the application once and walks in memory.
func validateParent(ctx context.Context, tx *gorm.DB, parentId, applicationId, childId int64) error {
	var resources []model.Resource
	if err := tx.Where(model.ResourceColumnApplicationId, applicationId).
		Select(model.ResourceFields).Find(&resources).Error; err != nil {
		return err
	}
	byId := make(map[int64]model.Resource, len(resources))
	for _, r := range resources {
		byId[r.Id] = r
	}
	parent, ok := byId[parentId]
	if !ok {
		return ErrResourceParentInvalid
	}
	if parent.ApplicationId != applicationId {
		return ErrResourceParentInvalid
	}
	// Cycle detection: walk from parent to root; if we encounter childId, it's a cycle
	current := &parent
	visited := make(map[int64]bool)
	visited[parent.Id] = true
	for current.ParentId != nil {
		if *current.ParentId == childId && childId != 0 {
			return ErrResourceHierarchyCircular
		}
		grandParent, exists := byId[*current.ParentId]
		if !exists {
			break
		}
		if visited[grandParent.Id] {
			break
		}
		visited[grandParent.Id] = true
		current = &grandParent
	}
	// Depth check: compute parent's depth and ensure child would stay within 5 levels
	parentDepth := 1
	walker := &parent
	for walker.ParentId != nil {
		parentDepth++
		p, exists := byId[*walker.ParentId]
		if !exists {
			break
		}
		walker = &p
	}
	// childDepth = parentDepth + 1 (child goes under parent)
	if parentDepth >= 5 {
		return ErrResourceHierarchyDepthExceeded
	}
	return nil
}

func buildResourceTreeNode(r model.Resource, byId map[int64]*model.Resource, appSummary *iam_v1.ApplicationSummary) *iam_v1.ResourceTreeNode {
	node := &iam_v1.ResourceTreeNode{
		Resource: resourceToPb(r, appSummary),
		Children: []*iam_v1.ResourceTreeNode{},
	}
	for _, child := range byId {
		if child.ParentId != nil && *child.ParentId == r.Id {
			node.Children = append(node.Children, buildResourceTreeNode(*child, byId, appSummary))
		}
	}
	return node
}

func (a AuthorizationService) CreateSubscription(ctx context.Context, request *iam_v1.CreateSubscriptionRequest) (*iam_v1.Subscription, error) {
	return &iam_v1.Subscription{}, nil
}

func (a AuthorizationService) GetSubscription(ctx context.Context, request *common_v1.GetRequest) (*iam_v1.Subscription, error) {
	return &iam_v1.Subscription{}, nil
}

func (a AuthorizationService) UpdateSubscription(ctx context.Context, request *iam_v1.UpdateSubscriptionRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (a AuthorizationService) ListSubscriptions(ctx context.Context, request *iam_v1.ListSubscriptionsRequest) (*iam_v1.ListSubscriptionsResponse, error) {
	return &iam_v1.ListSubscriptionsResponse{
		Subscriptions: []*iam_v1.Subscription{},
		Page:          &common_v1.Page{},
	}, nil
}

func (a AuthorizationService) GetMyPermissions(ctx context.Context, _ *emptypb.Empty) (*iam_v1.GetMyPermissionsResponse, error) {
	// admin 涓庢櫘閫氳处鍙风粺涓€璧板揩鐓х湡瀹炴暟鎹紙research.md D9锛夛細鍓嶇娓叉煋鏁版嵁涓?DB 鎺堟潈涓€鑷?
	accountId := auth.GetAccountId(ctx)
	permKeys, err := loadAccountPermissionSnapshot(ctx, a.rdb, a.db, accountId)
	if err != nil {
		return nil, err
	}

	response := &iam_v1.GetMyPermissionsResponse{
		MenuKeyList:        []string{},
		HttpFullMethodList: []string{},
		RpcFullMethodList:  []string{},
	}
	for _, key := range permKeys {
		switch {
		case strings.HasPrefix(key, constant.SnapshotKeyPrefixMenu):
			response.MenuKeyList = append(response.MenuKeyList, strings.TrimPrefix(key, constant.SnapshotKeyPrefixMenu))
		case strings.HasPrefix(key, constant.SnapshotKeyPrefixHTTP):
			response.HttpFullMethodList = append(response.HttpFullMethodList, strings.TrimPrefix(key, constant.SnapshotKeyPrefixHTTP))
		case strings.HasPrefix(key, constant.SnapshotKeyPrefixRPC):
			response.RpcFullMethodList = append(response.RpcFullMethodList, strings.TrimPrefix(key, constant.SnapshotKeyPrefixRPC))
		}
	}
	return response, nil
}

func boolToStatus(disabled bool) string {
	if disabled {
		return "disabled"
	}
	return "enabled"
}

func statusToBool(status string) bool {
	return status == "disabled"
}

var _ iam_v1.AuthorizationServiceServer = (*AuthorizationService)(nil)

func NewAuthorizationService(db *gorm.DB, rdb *redis.Client) iam_v1.AuthorizationServiceServer {
	return &AuthorizationService{db: db, rdb: rdb}
}
