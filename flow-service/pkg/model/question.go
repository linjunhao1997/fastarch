package model

import "time"

// Question 是阻塞问题：任务被一条 open 的问题阻塞（派生，不落任务表）。
// options 是 AI 预填的候选选项（JSON 数组：[{"id":"A","label":"...","rec":true}]）。
type Question struct {
	Id               int64      `gorm:"column:id;primaryKey"`
	ApplicationId    int64      `gorm:"column:application_id;index"`
	TaskId           int64      `gorm:"column:task_id;index"`
	FromRoleId       int64      `gorm:"column:from_role_id"`
	ToRoleId         int64      `gorm:"column:to_role_id;index"`
	AskedBy          int64      `gorm:"column:asked_by"`
	Body             string     `gorm:"column:body;type:text"`
	Options          string     `gorm:"column:options;type:json"`
	Answer           string     `gorm:"column:answer;type:text"`
	Picked           string     `gorm:"column:picked;size:64"`
	AnswerKind       string     `gorm:"column:answer_kind;size:16;default:decision"`
	Evidence         string     `gorm:"column:evidence;size:512"`
	Status           string     `gorm:"column:status;size:16;default:open"`
	ParentQuestionId int64      `gorm:"column:parent_question_id;index"`
	ReopenCount      int        `gorm:"column:reopen_count"`
	SelfAnswer       bool       `gorm:"column:self_answer"`
	AnsweredBy       int64      `gorm:"column:answered_by"`
	AnsweredRoleId   int64      `gorm:"column:answered_role_id"`
	AnsweredAt       *time.Time `gorm:"column:answered_at"`
	Version          int64      `gorm:"column:version"`
	IsDeleted        bool       `gorm:"column:is_deleted;default:false"`
	CreatedBy        string     `gorm:"column:created_by;size:64"`
	UpdatedBy        string     `gorm:"column:updated_by;size:64"`
	CreateTime       time.Time  `gorm:"column:create_time"`
	UpdateTime       time.Time  `gorm:"column:update_time"`
}

func (Question) TableName() string { return "t_question" }

const (
	QuestionColumnId               = "id"
	QuestionColumnApplicationId    = "application_id"
	QuestionColumnTaskId           = "task_id"
	QuestionColumnFromRoleId       = "from_role_id"
	QuestionColumnToRoleId         = "to_role_id"
	QuestionColumnAskedBy          = "asked_by"
	QuestionColumnBody             = "body"
	QuestionColumnOptions          = "options"
	QuestionColumnAnswer           = "answer"
	QuestionColumnPicked           = "picked"
	QuestionColumnAnswerKind       = "answer_kind"
	QuestionColumnEvidence         = "evidence"
	QuestionColumnStatus           = "status"
	QuestionColumnParentQuestionId = "parent_question_id"
	QuestionColumnReopenCount      = "reopen_count"
	QuestionColumnSelfAnswer       = "self_answer"
	QuestionColumnAnsweredBy       = "answered_by"
	QuestionColumnAnsweredRoleId   = "answered_role_id"
	QuestionColumnAnsweredAt       = "answered_at"
	QuestionColumnVersion          = "version"
	QuestionColumnIsDeleted        = "is_deleted"
	QuestionColumnCreatedBy        = "created_by"
	QuestionColumnUpdatedBy        = "updated_by"
	QuestionColumnCreateTime       = "create_time"
	QuestionColumnUpdateTime       = "update_time"
)

var QuestionFields = []string{
	QuestionColumnId, QuestionColumnApplicationId, QuestionColumnTaskId,
	QuestionColumnFromRoleId, QuestionColumnToRoleId, QuestionColumnAskedBy,
	QuestionColumnBody, QuestionColumnOptions, QuestionColumnAnswer,
	QuestionColumnPicked, QuestionColumnAnswerKind, QuestionColumnEvidence,
	QuestionColumnStatus, QuestionColumnParentQuestionId, QuestionColumnReopenCount,
	QuestionColumnSelfAnswer, QuestionColumnAnsweredBy, QuestionColumnAnsweredRoleId, QuestionColumnAnsweredAt,
	QuestionColumnVersion, QuestionColumnIsDeleted,
	QuestionColumnCreatedBy, QuestionColumnUpdatedBy,
	QuestionColumnCreateTime, QuestionColumnUpdateTime,
}

// 问题状态
const (
	QuestionStatusOpen      = "open"
	QuestionStatusAnswered  = "answered"
	QuestionStatusEscalated = "escalated"
	QuestionStatusDiscarded = "discarded"
)

// 闭环方式：decision 选/写即可；evidence 必须给证据（commit/工单/部署单/文档）。
const (
	AnswerKindDecision = "decision"
	AnswerKindEvidence = "evidence"
)

// TaskQuestionLink 是任务↔问题关系表（t_r_task_question），用于"某任务全部关联问题"的追溯视图。
type TaskQuestionLink struct {
	TaskId     int64     `gorm:"column:task_id;primaryKey"`
	QuestionId int64     `gorm:"column:question_id;primaryKey"`
	CreateTime time.Time `gorm:"column:create_time"`
}

func (TaskQuestionLink) TableName() string { return "t_r_task_question" }
