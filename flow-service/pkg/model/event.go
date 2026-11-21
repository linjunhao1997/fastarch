package model

import "time"

// Event 是留痕：谁（actor_account_id）在什么对象上做了什么（action），detail 存 JSON。
// 只追加，不更新不删除。
type Event struct {
	Id             int64      `gorm:"column:id;primaryKey"`
	ApplicationId  int64      `gorm:"column:application_id;index"`
	ActorAccountId int64      `gorm:"column:actor_account_id;index"`
	Action         string     `gorm:"column:action;size:64"`
	Entity         string     `gorm:"column:entity;size:32"`
	EntityId       int64      `gorm:"column:entity_id"`
	Detail         string     `gorm:"column:detail;type:text"`
	CreateTime     *time.Time `gorm:"column:create_time"`
}

func (Event) TableName() string { return "t_event" }

const (
	EventColumnId             = "id"
	EventColumnApplicationId  = "application_id"
	EventColumnActorAccountId = "actor_account_id"
	EventColumnAction         = "action"
	EventColumnEntity         = "entity"
	EventColumnEntityId       = "entity_id"
	EventColumnDetail         = "detail"
	EventColumnCreateTime     = "create_time"
)

var EventFields = []string{
	EventColumnId, EventColumnApplicationId, EventColumnActorAccountId,
	EventColumnAction, EventColumnEntity, EventColumnEntityId,
	EventColumnDetail, EventColumnCreateTime,
}

// 留痕动作
const (
	EventActionRoleSwitch       = "role.switch"
	EventActionTaskCreate       = "task.create"
	EventActionTaskClose        = "task.close"
	EventActionTaskDiscard      = "task.discard"
	EventActionQuestionRaise    = "question.raise"
	EventActionQuestionEscalate = "question.escalate"
	EventActionQuestionAnswer   = "question.answer"
	// self_answer = 同一账号且同一角色（自己问自己）；self_cross_role = 同一账号换了角色闭环。
	EventActionQuestionSelfAnswer    = "question.self_answer"
	EventActionQuestionSelfCrossRole = "question.self_cross_role"
	EventActionQuestionAutoClose     = "question.auto_close"
	// v1.1：答错被打回；父问题由子问题自动回填后被连带打回。
	EventActionQuestionReopen        = "question.reopen"
	EventActionQuestionReopenCascade = "question.reopen_cascade"
	EventActionMemberBootstrap       = "member.bootstrap"
	EventActionMemberRolesAssign     = "member.roles.assign"
)

// 实体名
const (
	EntityTask     = "task"
	EntityQuestion = "question"
	EntityMember   = "member"
)
