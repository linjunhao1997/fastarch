package service

import (
	"context"

	"iam-service/pkg/model"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// loadApplicationSummaries 批量把应用 id 解析为展示摘要（IN 一次查询，禁 N+1）。
// id<=0 被 loadApplicationsByIds 跳过，对应 map 无条目，调用方按 nil 处理即可。
func loadApplicationSummaries(ctx context.Context, db *gorm.DB, ids []int64) (map[int64]*common_v1.ApplicationSummary, error) {
	apps, err := loadApplicationsByIds(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*common_v1.ApplicationSummary, len(apps))
	for id, app := range apps {
		result[id] = &common_v1.ApplicationSummary{
			Id:     cast.ToString(app.Id),
			Code:   app.Code,
			Name:   app.Name,
			Status: app.Status,
		}
	}
	return result, nil
}

// loadApplicationSummary 单个应用摘要（详情/创建场景，复用批量加载器）。
func loadApplicationSummary(ctx context.Context, db *gorm.DB, id int64) (*common_v1.ApplicationSummary, error) {
	summaries, err := loadApplicationSummaries(ctx, db, []int64{id})
	if err != nil {
		return nil, err
	}
	return summaries[id], nil
}

// loadHomeApplicationSummariesByUserIds 按用户 id 批量取"所属应用"摘要，按 user id 索引。
// 两次 IN 查询（用户 home id → 应用摘要），禁 N+1。凭证域用此加载凭证所属用户的 home 应用（ADR 0006）。
func loadHomeApplicationSummariesByUserIds(ctx context.Context, db *gorm.DB, userIds []int64) (map[int64]*common_v1.ApplicationSummary, error) {
	result := make(map[int64]*common_v1.ApplicationSummary, len(userIds))
	if len(userIds) == 0 {
		return result, nil
	}
	var users []model.User
	if err := db.WithContext(ctx).
		Select(model.UserColumnId, model.UserColumnHomeApplicationId).
		Where(model.UserColumnId+" IN ?", userIds).
		Find(&users).Error; err != nil {
		return nil, err
	}
	homeIdByUser := make(map[int64]int64, len(users))
	homeIds := make([]int64, 0, len(users))
	for _, u := range users {
		if u.HomeApplicationId <= 0 {
			continue
		}
		homeIdByUser[u.Id] = u.HomeApplicationId
		homeIds = append(homeIds, u.HomeApplicationId)
	}
	summaries, err := loadApplicationSummaries(ctx, db, homeIds)
	if err != nil {
		return nil, err
	}
	for userId, homeId := range homeIdByUser {
		result[userId] = summaries[homeId]
	}
	return result, nil
}

// homeApplicationSummaryByUserId 单个用户所属应用摘要（详情/创建/重置场景）。
func homeApplicationSummaryByUserId(ctx context.Context, db *gorm.DB, userId int64) (*common_v1.ApplicationSummary, error) {
	byUser, err := loadHomeApplicationSummariesByUserIds(ctx, db, []int64{userId})
	if err != nil {
		return nil, err
	}
	return byUser[userId], nil
}

// loadAdmittedApplicationSummaries 返回用户可切换应用摘要（home + 显式准入绑定）。
// admin 返回全部启用应用（应用选择页需要列出所有注册应用）；非 admin 只列接入集合。
func loadAdmittedApplicationSummaries(ctx context.Context, db *gorm.DB, userId, homeAppId int64, isAdmin bool) ([]*common_v1.ApplicationSummary, error) {
	if isAdmin {
		var apps []model.Application
		if err := db.WithContext(ctx).
			Select(model.ApplicationFields).
			Where(model.ApplicationColumnStatus, model.StatusEnabled).
			Order(model.ApplicationColumnId + " ASC").
			Find(&apps).Error; err != nil {
			return nil, err
		}
		out := make([]*common_v1.ApplicationSummary, 0, len(apps))
		for i := range apps {
			out = append(out, applicationSummaryFromModel(&apps[i]))
		}
		return out, nil
	}
	ids, err := loadAdmittedAppIds(ctx, db, userId, homeAppId)
	if err != nil {
		return nil, err
	}
	byId, err := loadApplicationSummaries(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	// 按 ids 顺序输出，保证 home 在前、结果稳定。
	out := make([]*common_v1.ApplicationSummary, 0, len(ids))
	for _, id := range ids {
		if summary := byId[id]; summary != nil {
			out = append(out, summary)
		}
	}
	return out, nil
}

func applicationSummaryFromModel(app *model.Application) *common_v1.ApplicationSummary {
	return &common_v1.ApplicationSummary{
		Id:     cast.ToString(app.Id),
		Code:   app.Code,
		Name:   app.Name,
		Status: app.Status,
	}
}

// loadCredentialApplicationSummaries 按凭证集合的归属应用 id 批量取摘要，
// 返回 map[凭证 id]*摘要；两次 IN 查询（凭证 app id → 应用行），禁 N+1。
func loadCredentialApplicationSummaries(ctx context.Context, db *gorm.DB, credentials []model.AccessCredential) (map[int64]*common_v1.ApplicationSummary, error) {
	appIds := make([]int64, 0, len(credentials))
	seen := make(map[int64]struct{}, len(credentials))
	for _, c := range credentials {
		if c.ApplicationId <= 0 {
			continue
		}
		if _, exists := seen[c.ApplicationId]; exists {
			continue
		}
		seen[c.ApplicationId] = struct{}{}
		appIds = append(appIds, c.ApplicationId)
	}
	byApp, err := loadApplicationSummaries(ctx, db, appIds)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*common_v1.ApplicationSummary, len(credentials))
	for _, c := range credentials {
		result[c.Id] = byApp[c.ApplicationId]
	}
	return result, nil
}
