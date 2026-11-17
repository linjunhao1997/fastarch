// Do not Edit Generated File.

package model

const (
	MessageRecordColumnId            = "id"
	MessageRecordColumnApplicationId = "application_id"
	MessageRecordColumnVersion       = "version"
	MessageRecordColumnIsDeleted     = "is_deleted"
	MessageRecordColumnCreateTime    = "create_time"
	MessageRecordColumnUpdateTime    = "update_time"
	MessageRecordColumnCreatedBy     = "created_by"
	MessageRecordColumnUpdatedBy     = "updated_by"
)

const (
	MessageRecordColumnRecipient    = "recipient"
	MessageRecordColumnTemplateCode = "template_code"
	MessageRecordColumnContent      = "content"
	MessageRecordColumnChannel      = "channel"
	MessageRecordColumnSendStatus   = "send_status"
)

// 发送状态取值（字典 send_status 的代码值）。
const (
	SendStatusPending = "pending"
	SendStatusSent    = "sent"
	SendStatusFailed  = "failed"
)
