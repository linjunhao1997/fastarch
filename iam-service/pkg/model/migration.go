package model

var Models []interface{}

func migrate(model ...interface{}) {
	Models = append(Models, model...)
}
