package model

import "time"

// DictType 字典类型（t_dict_type）：code 唯一业务码，字典项经 type_id 挂载。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
//
type DictType struct {
	Id         int64     `gorm:"column:id;primaryKey"`
	Code       string    `gorm:"column:code;size:64;uniqueIndex"`
	Name       string    `gorm:"column:name;size:64"`
	Remark     string    `gorm:"column:remark;size:255"`
	Status     string    `gorm:"column:status;size:16;default:enabled"`
	Version    int64     `gorm:"column:version"`
	CreatedBy  string    `gorm:"column:created_by;size:64"`
	UpdatedBy  string    `gorm:"column:updated_by;size:64"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateTime time.Time `gorm:"column:update_time"`
}

func (d *DictType) TableName() string {
	return "t_dict_type"
}

// DictItem 字典项（t_dict_item）：type_id 挂载到字典类型，label/value 二元组。
// 删除策略: 基础架构型（物理删除 + 引用检查 + 审计日志）
type DictItem struct {
	Id         int64     `gorm:"column:id;primaryKey"`
	TypeId     int64     `gorm:"column:type_id;index"`
	Label      string    `gorm:"column:label;size:64"`
	Value      string    `gorm:"column:value;size:64"`
	Sort       int32     `gorm:"column:sort;default:0"`
	Status     string    `gorm:"column:status;size:16;default:enabled"`
	Version    int64     `gorm:"column:version"`
	CreatedBy  string    `gorm:"column:created_by;size:64"`
	UpdatedBy  string    `gorm:"column:updated_by;size:64"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateTime time.Time `gorm:"column:update_time"`
}

func (d *DictItem) TableName() string {
	return "t_dict_item"
}

var DictTypeFields = []string{
	DictTypeColumnId, DictTypeColumnCode, DictTypeColumnName, DictTypeColumnRemark,
	DictTypeColumnStatus, DictTypeColumnVersion, DictTypeColumnCreatedBy,
	DictTypeColumnUpdatedBy, DictTypeColumnCreateTime, DictTypeColumnUpdateTime,
}

var DictItemFields = []string{
	DictItemColumnId, DictItemColumnTypeId, DictItemColumnLabel, DictItemColumnValue,
	DictItemColumnSort, DictItemColumnStatus, DictItemColumnVersion,
	DictItemColumnCreatedBy, DictItemColumnUpdatedBy, DictItemColumnCreateTime,
	DictItemColumnUpdateTime,
}
