package model

import "time"

// MessageRecord 发送记录（t_message_record）：一次消息投递的留痕，承载接收对象、
// 所用模板、渠道与投递状态（GLOSSARY 消息中心）。按应用隔离（application_id 取
// sender 应用固定 id），运行时读 sender 自己的库。
// 删除策略: 业务型（软删除 + 审计字段 + 名称字段加删除时间戳后缀规避唯一键冲突）。
type MessageRecord struct {
	Id             int64     `gorm:"column:id;primaryKey"`
	ApplicationId  int64     `gorm:"column:application_id;index"`
	Recipient      string    `gorm:"column:recipient;size:255"`
	TemplateCode   string    `gorm:"column:template_code;size:64"`
	Content        string    `gorm:"column:content;size:4096"`
	Channel        string    `gorm:"column:channel;size:32"`
	SendStatus     string    `gorm:"column:send_status;size:16;default:pending"`
	Version        int64     `gorm:"column:version"`
	IsDeleted      bool      `gorm:"column:is_deleted;default:false"`
	CreatedBy      string    `gorm:"column:created_by;size:64"`
	UpdatedBy      string    `gorm:"column:updated_by;size:64"`
	CreateTime     time.Time `gorm:"column:create_time"`
	UpdateTime     time.Time `gorm:"column:update_time"`
}

func (m *MessageRecord) TableName() string {
	return "t_message_record"
}

// MessageRecordFields 是显式查询列清单（数据库红线：禁止 SELECT *）。
var MessageRecordFields = []string{
	MessageRecordColumnId, MessageRecordColumnApplicationId, MessageRecordColumnRecipient,
	MessageRecordColumnTemplateCode, MessageRecordColumnContent, MessageRecordColumnChannel,
	MessageRecordColumnSendStatus, MessageRecordColumnVersion, MessageRecordColumnIsDeleted,
	MessageRecordColumnCreatedBy, MessageRecordColumnUpdatedBy,
	MessageRecordColumnCreateTime, MessageRecordColumnUpdateTime,
}
