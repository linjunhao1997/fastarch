package model

import "time"

// MessageTemplate 消息模板（t_message_template）：可复用的消息内容定义，code 应用内唯一。
// 按应用隔离（application_id），运行时读 sender 自己的库。
// 删除策略: 业务型（软删除 + 审计字段 + code 加删除时间戳后缀规避唯一键冲突）。
type MessageTemplate struct {
	Id            int64     `gorm:"column:id;primaryKey"`
	ApplicationId int64     `gorm:"column:application_id;uniqueIndex:idx_template_app_code"`
	Code          string    `gorm:"column:code;size:64;uniqueIndex:idx_template_app_code"`
	Name          string    `gorm:"column:name;size:64"`
	Content       string    `gorm:"column:content;size:4096"`
	Remark        string    `gorm:"column:remark;size:255"`
	Version       int64     `gorm:"column:version"`
	IsDeleted     bool      `gorm:"column:is_deleted;default:false"`
	CreatedBy     string    `gorm:"column:created_by;size:64"`
	UpdatedBy     string    `gorm:"column:updated_by;size:64"`
	CreateTime    time.Time `gorm:"column:create_time"`
	UpdateTime    time.Time `gorm:"column:update_time"`
}

func (m *MessageTemplate) TableName() string {
	return "t_message_template"
}

// MessageTemplateFields 是显式查询列清单（禁止 SELECT *）。
var MessageTemplateFields = []string{
	MessageTemplateColumnId, MessageTemplateColumnApplicationId, MessageTemplateColumnCode,
	MessageTemplateColumnName, MessageTemplateColumnContent, MessageTemplateColumnRemark,
	MessageTemplateColumnVersion, MessageTemplateColumnIsDeleted,
	MessageTemplateColumnCreatedBy, MessageTemplateColumnUpdatedBy,
	MessageTemplateColumnCreateTime, MessageTemplateColumnUpdateTime,
}
