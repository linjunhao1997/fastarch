package model

import "time"

// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志），specs/009-ak-sk-auth。
// AccessCredential 是 AK/SK 签名鉴权凭证（specs/009-ak-sk-auth data-model）：
// 绑定账号创建，凭证自身不携带权限；SK 以 AES-256-GCM 密文落库（secret_key_cipher），
// 明文仅在创建/重置响应中一次性返回。
type AccessCredential struct {
	Id              int64      `gorm:"column:id;primaryKey"`
	AccessKey       string     `gorm:"column:access_key;size:64;uniqueIndex"`
	SecretKeyCipher string     `gorm:"column:secret_key_cipher;size:256"`
	UserId          int64      `gorm:"column:user_id;index"`
	Remark          string     `gorm:"column:remark;size:255"`
	Status          string     `gorm:"column:status;size:16"`
	ExpireTime      *time.Time `gorm:"column:expire_time"`
	LastUsedTime    *time.Time `gorm:"column:last_used_time"`
	CreateTime      time.Time  `gorm:"column:create_time"`
	UpdateTime      time.Time  `gorm:"column:update_time"`
	CreatedBy       int64      `gorm:"column:created_by"`
	UpdatedBy       int64      `gorm:"column:updated_by"`
	Version         int64      `gorm:"column:version"`
}

func (a *AccessCredential) TableName() string {
	return "t_access_credential"
}

var AccessCredentialFields = []string{
	AccessCredentialColumnId, AccessCredentialColumnAccessKey, AccessCredentialColumnSecretKeyCipher,
	AccessCredentialColumnUserId, AccessCredentialColumnRemark, AccessCredentialColumnStatus,
	AccessCredentialColumnExpireTime, AccessCredentialColumnLastUsedTime,
	AccessCredentialColumnCreateTime, AccessCredentialColumnUpdateTime,
	AccessCredentialColumnCreatedBy, AccessCredentialColumnUpdatedBy, AccessCredentialColumnVersion,
}
