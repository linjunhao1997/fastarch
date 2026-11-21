package service

// methodPermissions：gRPC full method → 所需权限码（fail-closed，未登记即拒绝）。
// 与 IAM 迁移 000017 里插入的 22 个 flow.* 码逐条对应。
var methodPermissions = map[string][]string{
	"/flow.v1.MemberService/GetMe":       {"flow.member.get"},
	"/flow.v1.MemberService/ListMyRoles": {"flow.member.get"},
	"/flow.v1.MemberService/SwitchRole":  {"flow.member.switch"},
	// 成员与角色管理：网关只校验登录态与 flow.member.get；
	// "必须当前角色是 boss" 由 MemberService.requireBoss 在业务层强制（3001018）。
	"/flow.v1.MemberService/ListMembers":       {"flow.member.get"},
	"/flow.v1.MemberService/AssignMemberRoles": {"flow.member.get"},

	"/flow.v1.TaskService/ListMyTasks":       {"flow.task.list"},
	"/flow.v1.TaskService/GetTask":           {"flow.task.get"},
	"/flow.v1.TaskService/CreateTask":        {"flow.task.create"},
	"/flow.v1.TaskService/StartTask":         {"flow.task.create"},
	"/flow.v1.TaskService/SubmitTask":        {"flow.task.create"},
	"/flow.v1.TaskService/AcceptTask":        {"flow.task.close"},
	"/flow.v1.TaskService/FinishTask":        {"flow.task.close"},
	"/flow.v1.TaskService/GetWorkPackage":    {"flow.task.get"},
	"/flow.v1.TaskService/ListTaskArtifacts": {"flow.task.get"},
	"/flow.v1.TaskService/CloseTask":         {"flow.task.close"},
	"/flow.v1.TaskService/DiscardTask":       {"flow.task.discard"},

	"/flow.v1.QuestionService/ListMyQuestions":  {"flow.question.list"},
	"/flow.v1.QuestionService/RaiseQuestion":    {"flow.question.raise"},
	"/flow.v1.QuestionService/EscalateQuestion": {"flow.question.escalate"},
	"/flow.v1.QuestionService/AnswerQuestion":   {"flow.question.answer"},
	"/flow.v1.QuestionService/ReopenQuestion":   {"flow.question.answer"},

	"/flow.v1.LogService/ListEvents": {"flow.event.list"},

	"/flow.v1.MenuService/CreateMenu":   {"flow.menu.create"},
	"/flow.v1.MenuService/GetMenu":      {"flow.menu.get"},
	"/flow.v1.MenuService/ListMenuTree": {"flow.menu.list"},
	"/flow.v1.MenuService/UpdateMenu":   {"flow.menu.update"},
	"/flow.v1.MenuService/DeleteMenu":   {"flow.menu.delete"},

	"/flow.v1.DictService/CreateDictType": {"flow.dict.create"},
	"/flow.v1.DictService/UpdateDictType": {"flow.dict.update"},
	"/flow.v1.DictService/DeleteDictType": {"flow.dict.delete"},
	"/flow.v1.DictService/ListDictTypes":  {"flow.dict.list"},
	"/flow.v1.DictService/CreateDictItem": {"flow.dict.create"},
	"/flow.v1.DictService/UpdateDictItem": {"flow.dict.update"},
	"/flow.v1.DictService/DeleteDictItem": {"flow.dict.delete"},
	"/flow.v1.DictService/ListDictItems":  {"flow.dict.list"},
}

// authOnlyMethods 仅需登录态、不判权（服务端返回全量启用菜单，前端按码过滤）。
var authOnlyMethods = map[string]struct{}{
	"/flow.v1.MenuService/ListMyMenuTree": {},
}

// MethodPermsFor 供 authz 拦截器注入。
func MethodPermsFor(fullMethod string) []string { return methodPermissions[fullMethod] }

// AuthOnlyMethods 返回副本，避免外部修改内部 map。
func AuthOnlyMethods() map[string]struct{} {
	out := make(map[string]struct{}, len(authOnlyMethods))
	for k, v := range authOnlyMethods {
		out[k] = v
	}
	return out
}
