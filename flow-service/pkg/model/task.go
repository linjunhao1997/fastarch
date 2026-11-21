package model

import "time"

// Task 是任务树中的一个节点：归创建它的「人 + 角色」所有，
// 下游角色只能基于父任务新建自己的子任务，不能改派或替别人执行。
// status 只存 open/done/discarded 三个终态；能否开工（ready/blocked）由问题表现算，
// 绝不落库（宪法 V 派生而非复制）。
type Task struct {
	Id                    int64      `gorm:"column:id;primaryKey"`
	ApplicationId         int64      `gorm:"column:application_id;index"`
	RoleId                int64      `gorm:"column:role_id;index"`
	OwnerId               int64      `gorm:"column:owner_id;index"`
	ParentTaskId          int64      `gorm:"column:parent_task_id;index"`
	Title                 string     `gorm:"column:title;size:255"`
	Brief                 string     `gorm:"column:brief;type:text"`
	RequiredArtifactTypes string     `gorm:"column:required_artifact_types;size:512"`
	Status                string     `gorm:"column:status;size:16;default:open"`
	StartedAt             *time.Time `gorm:"column:started_at"`
	SubmittedAt           *time.Time `gorm:"column:submitted_at"`
	ContextHash           string     `gorm:"column:context_hash;size:64"`
	ContextJson           string     `gorm:"column:context_json;type:longtext"`
	Version               int64      `gorm:"column:version"`
	IsDeleted             bool       `gorm:"column:is_deleted;default:false"`
	CreatedBy             string     `gorm:"column:created_by;size:64"`
	CreatedRoleId         int64      `gorm:"column:created_role_id"`
	UpdatedBy             string     `gorm:"column:updated_by;size:64"`
	CreateTime            time.Time  `gorm:"column:create_time"`
	UpdateTime            time.Time  `gorm:"column:update_time"`
}

func (Task) TableName() string { return "t_task" }

const (
	TaskColumnId                    = "id"
	TaskColumnApplicationId         = "application_id"
	TaskColumnRoleId                = "role_id"
	TaskColumnOwnerId               = "owner_id"
	TaskColumnParentTaskId          = "parent_task_id"
	TaskColumnTitle                 = "title"
	TaskColumnBrief                 = "brief"
	TaskColumnRequiredArtifactTypes = "required_artifact_types"
	TaskColumnStatus                = "status"
	TaskColumnStartedAt             = "started_at"
	TaskColumnSubmittedAt           = "submitted_at"
	TaskColumnContextHash           = "context_hash"
	TaskColumnContextJson           = "context_json"
	TaskColumnVersion               = "version"
	TaskColumnIsDeleted             = "is_deleted"
	TaskColumnCreatedBy             = "created_by"
	TaskColumnCreatedRoleId         = "created_role_id"
	TaskColumnUpdatedBy             = "updated_by"
	TaskColumnCreateTime            = "create_time"
	TaskColumnUpdateTime            = "update_time"
)

var TaskFields = []string{
	TaskColumnId, TaskColumnApplicationId, TaskColumnRoleId, TaskColumnOwnerId,
	TaskColumnParentTaskId, TaskColumnTitle, TaskColumnBrief, TaskColumnRequiredArtifactTypes,
	TaskColumnStatus, TaskColumnStartedAt, TaskColumnSubmittedAt, TaskColumnContextHash, TaskColumnContextJson,
	TaskColumnVersion, TaskColumnIsDeleted,
	TaskColumnCreatedBy, TaskColumnCreatedRoleId, TaskColumnUpdatedBy, TaskColumnCreateTime, TaskColumnUpdateTime,
}

// 任务终态（只有这三个；blocked 是派生状态，不落库）
const (
	TaskStatusOpen      = "open"
	TaskStatusDone      = "done"
	TaskStatusDiscarded = "discarded"
)

// 派生状态（只出现在响应里）
const (
	TaskStateReady      = "ready"
	TaskStateBlocked    = "blocked"
	TaskStateInProgress = "in_progress"
	TaskStateSubmitted  = "submitted"
	TaskStateDone       = "done"
	TaskStateDiscarded  = "discarded"
)
