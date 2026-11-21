// Package model 定义 home 自持库的持久化模型（无跨服务物理外键，宪法 XIV）。
package model

import "time"

const (
	HomepageTableName = "t_homepage"

	HomepageColumnId            = "id"
	HomepageColumnApplicationId = "application_id"
	HomepageColumnDraftJson     = "draft_json"
	HomepageColumnPublishedJson = "published_json"
	HomepageColumnPublishedAt   = "published_at"
	HomepageColumnVersion       = "version"
	HomepageColumnCreatedBy     = "created_by"
	HomepageColumnUpdatedBy     = "updated_by"
	HomepageColumnCreateTime    = "create_time"
	HomepageColumnUpdateTime    = "update_time"

	HomepageFields = "id,application_id,draft_json,published_json,published_at,version,created_by,updated_by,create_time,update_time"
)

// Homepage 企业官网首页配置（单应用单行）。Draft/Published 均为首页内容 JSON。
type Homepage struct {
	Id            int64      `gorm:"column:id;primaryKey"`
	ApplicationId int64      `gorm:"column:application_id"`
	DraftJson     string     `gorm:"column:draft_json"`
	PublishedJson string     `gorm:"column:published_json"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
	Version       int64      `gorm:"column:version"`
	CreatedBy     string     `gorm:"column:created_by"`
	UpdatedBy     string     `gorm:"column:updated_by"`
	CreateTime    time.Time  `gorm:"column:create_time"`
	UpdateTime    time.Time  `gorm:"column:update_time"`
}

// TableName 返回表名。
func (m *Homepage) TableName() string {
	return HomepageTableName
}
