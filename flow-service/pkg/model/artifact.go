package model

import "time"

// Artifact 是任务产出的 Markdown 物料。规划类提交后待下游确认，实施类完成时直接归档为 accepted。
type Artifact struct {
	Id            int64      `gorm:"column:id;primaryKey"`
	ApplicationId int64      `gorm:"column:application_id;index"`
	TaskId        int64      `gorm:"column:task_id;index"`
	Type          string     `gorm:"column:type;size:64;index"`
	Title         string     `gorm:"column:title;size:255"`
	ContentMd     string     `gorm:"column:content_md;type:longtext"`
	Version       int        `gorm:"column:version"`
	Status        string     `gorm:"column:status;size:16;default:submitted"`
	Hash          string     `gorm:"column:hash;size:64"`
	CreatedBy     int64      `gorm:"column:created_by"`
	CreatedRoleId int64      `gorm:"column:created_role_id"`
	AcceptedBy    int64      `gorm:"column:accepted_by"`
	AcceptedAt    *time.Time `gorm:"column:accepted_at"`
	IsDeleted     bool       `gorm:"column:is_deleted;default:false"`
	CreateTime    time.Time  `gorm:"column:create_time"`
	UpdateTime    time.Time  `gorm:"column:update_time"`
}

func (Artifact) TableName() string { return "t_artifact" }

const (
	ArtifactColumnId            = "id"
	ArtifactColumnApplicationId = "application_id"
	ArtifactColumnTaskId        = "task_id"
	ArtifactColumnType          = "type"
	ArtifactColumnTitle         = "title"
	ArtifactColumnContentMd     = "content_md"
	ArtifactColumnVersion       = "version"
	ArtifactColumnStatus        = "status"
	ArtifactColumnHash          = "hash"
	ArtifactColumnCreatedBy     = "created_by"
	ArtifactColumnCreatedRoleId = "created_role_id"
	ArtifactColumnAcceptedBy    = "accepted_by"
	ArtifactColumnAcceptedAt    = "accepted_at"
	ArtifactColumnIsDeleted     = "is_deleted"
	ArtifactColumnCreateTime    = "create_time"
	ArtifactColumnUpdateTime    = "update_time"
)

var ArtifactFields = []string{
	ArtifactColumnId, ArtifactColumnApplicationId, ArtifactColumnTaskId, ArtifactColumnType,
	ArtifactColumnTitle, ArtifactColumnContentMd, ArtifactColumnVersion, ArtifactColumnStatus,
	ArtifactColumnHash, ArtifactColumnCreatedBy, ArtifactColumnCreatedRoleId,
	ArtifactColumnAcceptedBy, ArtifactColumnAcceptedAt, ArtifactColumnIsDeleted,
	ArtifactColumnCreateTime, ArtifactColumnUpdateTime,
}

const (
	ArtifactStatusSubmitted  = "submitted"
	ArtifactStatusAccepted   = "accepted"
	ArtifactStatusSuperseded = "superseded"
	ArtifactStatusDisputed   = "disputed"
)
