package dict

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-dev-frame/sponge/pkg/krand"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	common_dict_v1 "github.com/linjunhao1997/idl/pb/common/dict"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"github.com/linjunhao1997/contrib/model"
	"github.com/linjunhao1997/contrib/pkg/bizerr"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// 通用字典能力（ADR 0001）：每服务嵌入自持，契约共享（iam.v1.DictService）。
// 宿主差异经 Config 钩子注入：应用上下文、账号身份。宿主的分页/审计等
// 拦截器机制与本实现解耦，业务错误经 bizerr 透传。

// 业务错误（业务码与既有客户端约定保持不变）。
var (
	ErrDictTypeCodeDuplicated = bizerr.NewErrorWithCode("1001053", codes.AlreadyExists, errors.New("字典编码已被占用"))
	ErrDictTypeReferenced     = bizerr.NewErrorWithCode("1001054", codes.FailedPrecondition, errors.New("字典类型名下存在字典项，请先删除字典项"))
	ErrUpdateDictTypeBusy     = bizerr.NewErrorWithCode("1001055", codes.FailedPrecondition, errors.New("字典类型已被他人修改，请刷新后重试"))
	ErrUpdateDictItemBusy     = bizerr.NewErrorWithCode("1001056", codes.FailedPrecondition, errors.New("字典项已被他人修改，请刷新后重试"))
)

// Config 是字典能力的宿主接入配置。必填钩子缺省时 NewDictService 报错（fail-closed）。
type Config struct {
	DB *gorm.DB

	// ApplicationIdFromContext 返回当前请求的应用上下文 id；未选应用时返回错误。
	ApplicationIdFromContext func(ctx context.Context) (int64, error)
	// AccountIdFromContext 返回当前登录账号 id（无登录态返回 0）。
	AccountIdFromContext func(ctx context.Context) int64
}

type DictService struct {
	cfg Config
}

var _ iam_v1.DictServiceServer = (*DictService)(nil)

// NewDictService 构建通用字典服务；必填钩子缺失时返回错误。
func NewDictService(cfg Config) (*DictService, error) {
	if cfg.DB == nil {
		return nil, errors.New("dict: DB is required")
	}
	if cfg.ApplicationIdFromContext == nil || cfg.AccountIdFromContext == nil {
		return nil, errors.New("dict: hooks ApplicationIdFromContext/AccountIdFromContext are required")
	}
	return &DictService{cfg: cfg}, nil
}

func dictTypeToPb(d model.DictType) *common_dict_v1.DictType {
	return &common_dict_v1.DictType{
		Id:         cast.ToString(d.Id),
		Code:       d.Code,
		Name:       d.Name,
		Remark:     d.Remark,
		Status:     d.Status,
		Version:    d.Version,
		CreatedBy:  d.CreatedBy,
		UpdatedBy:  d.UpdatedBy,
		CreateTime: timestamppb.New(d.CreateTime),
		UpdateTime: timestamppb.New(d.UpdateTime),
	}
}

func dictItemToPb(d model.DictItem) *common_dict_v1.DictItem {
	return &common_dict_v1.DictItem{
		Id:         cast.ToString(d.Id),
		TypeId:     cast.ToString(d.TypeId),
		Label:      d.Label,
		Value:      d.Value,
		Sort:       d.Sort,
		Status:     d.Status,
		Version:    d.Version,
		CreatedBy:  d.CreatedBy,
		UpdatedBy:  d.UpdatedBy,
		CreateTime: timestamppb.New(d.CreateTime),
		UpdateTime: timestamppb.New(d.UpdateTime),
	}
}

// getLimitAndOffset 分页参数换算（与宿主 helper.GetLimitAndOffset 同语义）。
func getLimitAndOffset(currentPage, pageSize int64) (limit, offset int) {
	if currentPage <= 0 {
		currentPage = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 500 {
		pageSize = 500
	}
	return int(pageSize), int((currentPage - 1) * pageSize)
}

// getPage 组装分页响应（与宿主 helper.GetPage 同语义）。
func getPage(currentPage, pageSize, total int64) *common_v1.Page {
	if currentPage <= 0 {
		currentPage = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}
	return &common_v1.Page{
		CurrentPage: currentPage,
		PageSize:    pageSize,
		Total:       total,
		TotalPages:  totalPages,
	}
}

// applyKeywordFilter keyword 模糊过滤（条件组之间 OR）。
func applyKeywordFilter(query *gorm.DB, keywordConditions []string, keyword string) *gorm.DB {
	trimmed := trimSpace(keyword)
	if trimmed == "" || len(keywordConditions) == 0 {
		return query
	}
	like := "%" + trimmed + "%"
	args := make([]interface{}, 0, len(keywordConditions))
	for range keywordConditions {
		args = append(args, like)
	}
	return query.Where("("+joinOr(keywordConditions)+")", args...)
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func joinOr(conditions []string) string {
	out := ""
	for i, c := range conditions {
		if i > 0 {
			out += " OR "
		}
		out += c
	}
	return out
}

// requireDictTypeInApp 校验字典类型存在且归属当前应用上下文；
// 字典项操作经此保证不跨应用读写。
func (s *DictService) requireDictTypeInApp(ctx context.Context, typeId int64, appId int64) error {
	var count int64
	err := s.cfg.DB.WithContext(ctx).Model(model.DictType{}).
		Where(model.DictTypeColumnId, typeId).
		Where(model.DictTypeColumnApplicationId, appId).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CreateDictType 创建字典类型：code 应用内唯一。
func (s *DictService) CreateDictType(ctx context.Context, request *common_dict_v1.CreateDictTypeRequest) (*common_dict_v1.DictType, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	pkId := krand.NewID()
	now := time.Now()
	operator := cast.ToString(s.cfg.AccountIdFromContext(ctx))
	status := request.Status
	if status == "" {
		status = model.StatusEnabled
	}
	dictType := model.DictType{
		Id:            pkId,
		Code:          request.Code,
		ApplicationId: appId,
		Name:          request.Name,
		Remark:        request.Remark,
		Status:        status,
		Version:       1,
		CreatedBy:     operator,
		UpdatedBy:     operator,
		CreateTime:    now,
		UpdateTime:    now,
	}
	err = s.cfg.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(model.DictType{}).
			Where(model.DictTypeColumnApplicationId, appId).
			Where(model.DictTypeColumnCode, request.Code).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrDictTypeCodeDuplicated
		}
		return tx.Select(model.DictTypeFields).Create(&dictType).Error
	})
	if err != nil {
		slog.Warn("create dict type failed", "code", request.Code, "error", err)
		return nil, err
	}
	return dictTypeToPb(dictType), nil
}

// UpdateDictType 修改字典类型：name/remark/status 可选更新（空串 = 不修改）。
// 限当前应用上下文，跨应用按不存在处理。
func (s *DictService) UpdateDictType(ctx context.Context, request *common_dict_v1.UpdateDictTypeRequest) (*emptypb.Empty, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var dictType model.DictType
	err = s.cfg.DB.WithContext(ctx).
		Select(model.DictTypeFields).
		Where(model.DictTypeColumnId, request.Id).
		Where(model.DictTypeColumnApplicationId, appId).
		First(&dictType).Error
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if len(request.Name) > 0 {
		updates[model.DictTypeColumnName] = request.Name
	}
	if len(request.Remark) > 0 {
		updates[model.DictTypeColumnRemark] = request.Remark
	}
	if len(request.Status) > 0 {
		updates[model.DictTypeColumnStatus] = request.Status
	}
	if len(updates) == 0 {
		return &emptypb.Empty{}, nil
	}
	updates[model.DictTypeColumnUpdateTime] = time.Now()
	updates[model.DictTypeColumnUpdatedBy] = cast.ToString(s.cfg.AccountIdFromContext(ctx))
	updates[model.DictTypeColumnVersion] = dictType.Version + 1

	result := s.cfg.DB.WithContext(ctx).Model(model.DictType{}).
		Where(model.DictTypeColumnId, request.Id).
		Where(model.DictTypeColumnVersion, request.Version).
		Updates(updates)
	if result.Error != nil {
		slog.Warn("update dict type failed", "dict_type_id", request.Id, "version", request.Version, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("update dict type rejected: version conflict", "dict_type_id", request.Id, "version", request.Version)
		return nil, ErrUpdateDictTypeBusy
	}
	return &emptypb.Empty{}, nil
}

// DeleteDictType 删除字典类型：限当前应用上下文；名下字典项引用检查，物理删除。
func (s *DictService) DeleteDictType(ctx context.Context, request *common_dict_v1.DeleteDictTypeRequest) (*emptypb.Empty, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireDictTypeInApp(ctx, request.Id, appId); err != nil {
		return nil, err
	}
	var itemCount int64
	err = s.cfg.DB.WithContext(ctx).Model(model.DictItem{}).
		Where(model.DictItemColumnTypeId, request.Id).
		Count(&itemCount).Error
	if err != nil {
		slog.Warn("check dict type items failed", "dict_type_id", request.Id, "error", err)
		return nil, err
	}
	if itemCount > 0 {
		slog.Warn("delete dict type rejected: has items", "dict_type_id", request.Id, "item_count", itemCount)
		return nil, ErrDictTypeReferenced
	}
	result := s.cfg.DB.WithContext(ctx).
		Where(model.DictTypeColumnId, request.Id).
		Where(model.DictTypeColumnApplicationId, appId).
		Delete(model.DictType{})
	if result.Error != nil {
		slog.Warn("delete dict type failed", "dict_type_id", request.Id, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("delete dict type rejected: not found", "dict_type_id", request.Id)
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}

// ListDictTypes 字典类型分页列表：限当前应用上下文；keyword 模糊匹配 code/name。
func (s *DictService) ListDictTypes(ctx context.Context, request *common_dict_v1.ListDictTypesRequest) (*common_dict_v1.ListDictTypesResponse, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := getLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		model.DictTypeColumnCode + " LIKE ?",
		model.DictTypeColumnName + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		return applyKeywordFilter(
			s.cfg.DB.WithContext(ctx).Model(model.DictType{}).Where(model.DictTypeColumnApplicationId, appId),
			keywordConditions, request.Keyword)
	}
	var count int64
	err = buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var types []model.DictType
	err = buildQuery().
		Order(model.DictTypeColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Find(&types).Error
	if err != nil {
		return nil, err
	}
	list := make([]*common_dict_v1.DictType, 0, len(types))
	for _, d := range types {
		list = append(list, dictTypeToPb(d))
	}
	return &common_dict_v1.ListDictTypesResponse{
		DictTypes: list,
		Page:      getPage(request.CurrentPage, request.PageSize, count),
	}, nil
}

// CreateDictItem 创建字典项：type_id 必须指向存在的字典类型。
func (s *DictService) CreateDictItem(ctx context.Context, request *common_dict_v1.CreateDictItemRequest) (*common_dict_v1.DictItem, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireDictTypeInApp(ctx, request.TypeId, appId); err != nil {
		slog.Warn("create dict item rejected: dict type not found in app", "type_id", request.TypeId, "error", err)
		return nil, err
	}
	pkId := krand.NewID()
	now := time.Now()
	operator := cast.ToString(s.cfg.AccountIdFromContext(ctx))
	status := request.Status
	if status == "" {
		status = model.StatusEnabled
	}
	dictItem := model.DictItem{
		Id:         pkId,
		TypeId:     request.TypeId,
		Label:      request.Label,
		Value:      request.Value,
		Sort:       request.Sort,
		Status:     status,
		Version:    1,
		CreatedBy:  operator,
		UpdatedBy:  operator,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := s.cfg.DB.WithContext(ctx).Select(model.DictItemFields).Create(&dictItem).Error; err != nil {
		slog.Warn("create dict item failed", "type_id", request.TypeId, "error", err)
		return nil, err
	}
	return dictItemToPb(dictItem), nil
}

// UpdateDictItem 修改字典项：label/value/sort/status 可选更新。
// 字典项归属经其类型的应用上下文校验。
func (s *DictService) UpdateDictItem(ctx context.Context, request *common_dict_v1.UpdateDictItemRequest) (*emptypb.Empty, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var dictItem model.DictItem
	err = s.cfg.DB.WithContext(ctx).
		Select(model.DictItemFields).
		Where(model.DictItemColumnId, request.Id).
		First(&dictItem).Error
	if err != nil {
		return nil, err
	}
	if err := s.requireDictTypeInApp(ctx, dictItem.TypeId, appId); err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if len(request.Label) > 0 {
		updates[model.DictItemColumnLabel] = request.Label
	}
	if len(request.Value) > 0 {
		updates[model.DictItemColumnValue] = request.Value
	}
	if request.Sort != 0 {
		updates[model.DictItemColumnSort] = request.Sort
	}
	if len(request.Status) > 0 {
		updates[model.DictItemColumnStatus] = request.Status
	}
	if len(updates) == 0 {
		return &emptypb.Empty{}, nil
	}
	updates[model.DictItemColumnUpdateTime] = time.Now()
	updates[model.DictItemColumnUpdatedBy] = cast.ToString(s.cfg.AccountIdFromContext(ctx))
	updates[model.DictItemColumnVersion] = dictItem.Version + 1

	result := s.cfg.DB.WithContext(ctx).Model(model.DictItem{}).
		Where(model.DictItemColumnId, request.Id).
		Where(model.DictItemColumnVersion, request.Version).
		Updates(updates)
	if result.Error != nil {
		slog.Warn("update dict item failed", "dict_item_id", request.Id, "version", request.Version, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("update dict item rejected: version conflict", "dict_item_id", request.Id, "version", request.Version)
		return nil, ErrUpdateDictItemBusy
	}
	return &emptypb.Empty{}, nil
}

// DeleteDictItem 删除字典项：归属经其类型的应用上下文校验，物理删除。
func (s *DictService) DeleteDictItem(ctx context.Context, request *common_dict_v1.DeleteDictItemRequest) (*emptypb.Empty, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var dictItem model.DictItem
	err = s.cfg.DB.WithContext(ctx).
		Select(model.DictItemFields).
		Where(model.DictItemColumnId, request.Id).
		First(&dictItem).Error
	if err != nil {
		return nil, err
	}
	if err := s.requireDictTypeInApp(ctx, dictItem.TypeId, appId); err != nil {
		return nil, err
	}
	result := s.cfg.DB.WithContext(ctx).
		Where(model.DictItemColumnId, request.Id).
		Delete(model.DictItem{})
	if result.Error != nil {
		slog.Warn("delete dict item failed", "dict_item_id", request.Id, "error", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		slog.Warn("delete dict item rejected: not found", "dict_item_id", request.Id)
		return nil, gorm.ErrRecordNotFound
	}
	return &emptypb.Empty{}, nil
}

// ListDictItems 字典项列表：按 type_id 筛选（类型须归属当前应用），keyword 模糊匹配 label/value。
func (s *DictService) ListDictItems(ctx context.Context, request *common_dict_v1.ListDictItemsRequest) (*common_dict_v1.ListDictItemsResponse, error) {
	appId, err := s.cfg.ApplicationIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireDictTypeInApp(ctx, request.TypeId, appId); err != nil {
		return nil, err
	}
	limit, offset := getLimitAndOffset(request.CurrentPage, request.PageSize)
	keywordConditions := []string{
		model.DictItemColumnLabel + " LIKE ?",
		model.DictItemColumnValue + " LIKE ?",
	}
	buildQuery := func() *gorm.DB {
		query := s.cfg.DB.WithContext(ctx).Model(model.DictItem{}).
			Where(model.DictItemColumnTypeId, request.TypeId)
		return applyKeywordFilter(query, keywordConditions, request.Keyword)
	}
	var count int64
	err = buildQuery().Count(&count).Error
	if err != nil {
		return nil, err
	}
	var items []model.DictItem
	err = buildQuery().
		Order(model.DictItemColumnSort + " ASC").
		Order(model.DictItemColumnId + " ASC").
		Limit(limit).
		Offset(offset).
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	list := make([]*common_dict_v1.DictItem, 0, len(items))
	for _, d := range items {
		list = append(list, dictItemToPb(d))
	}
	return &common_dict_v1.ListDictItemsResponse{
		DictItems: list,
		Page:      getPage(request.CurrentPage, request.PageSize, count),
	}, nil
}
