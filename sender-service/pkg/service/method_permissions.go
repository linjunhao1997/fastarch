package service

// methodPermissions 是 gRPC full method → 所需权限码的映射（fail-closed：未登记方法拒绝）。
// 权限码是唯一授权词汇（GLOSSARY），sender 服务段隔离；种子见 IAM 迁移 000010。
var methodPermissions = map[string][]string{
	"/sender.v1.MessageService/SendMessage":   {"sender.record.send"},
	"/sender.v1.MessageService/GetMessage":    {"sender.record.get"},
	"/sender.v1.MessageService/ListMessages":  {"sender.record.list"},
	"/sender.v1.MessageService/DeleteMessage": {"sender.record.delete"},

	"/sender.v1.TemplateService/CreateTemplate": {"sender.template.create"},
	"/sender.v1.TemplateService/GetTemplate":    {"sender.template.get"},
	"/sender.v1.TemplateService/ListTemplates":  {"sender.template.list"},
	"/sender.v1.TemplateService/UpdateTemplate": {"sender.template.update"},
	"/sender.v1.TemplateService/DeleteTemplate": {"sender.template.delete"},

	"/sender.v1.MenuService/CreateMenu":   {"sender.menu.create"},
	"/sender.v1.MenuService/GetMenu":      {"sender.menu.get"},
	"/sender.v1.MenuService/ListMenuTree": {"sender.menu.list"},
	"/sender.v1.MenuService/UpdateMenu":   {"sender.menu.update"},
	"/sender.v1.MenuService/DeleteMenu":   {"sender.menu.delete"},

	"/sender.v1.DictService/CreateDictType": {"sender.dict.create"},
	"/sender.v1.DictService/UpdateDictType": {"sender.dict.update"},
	"/sender.v1.DictService/DeleteDictType": {"sender.dict.delete"},
	"/sender.v1.DictService/ListDictTypes":  {"sender.dict.list"},
	"/sender.v1.DictService/CreateDictItem": {"sender.dict.create"},
	"/sender.v1.DictService/UpdateDictItem": {"sender.dict.update"},
	"/sender.v1.DictService/DeleteDictItem": {"sender.dict.delete"},
	"/sender.v1.DictService/ListDictItems":  {"sender.dict.list"},
}

// authOnlyMethods 仅需登录态、不判权的方法（纯码门槛：菜单由前端按码过滤，
// 服务端返回全量启用菜单，ADR 0004）。这些方法跳过 IAM 判权，身份无 IAM 背书，
// 不得据此做写操作归责。
var authOnlyMethods = map[string]struct{}{
	"/sender.v1.MenuService/ListMyMenuTree": {},
	// 登录接口：凭据校验委托 IAM（ADR 0007），无 JWT 场景，必须免判权。
	"/sender.v1.AuthenticationService/PasswordAuth": {},
}

// MethodPermsFor 供 authz 拦截器注入：full method → 所需权限码，未登记返回 nil（fail-closed）。
func MethodPermsFor(fullMethod string) []string {
	return methodPermissions[fullMethod]
}

// AuthOnlyMethods 供 authz 拦截器注入：仅需登录态、不判权的方法集合。
// 返回副本，避免外部修改内部 map。
func AuthOnlyMethods() map[string]struct{} {
	out := make(map[string]struct{}, len(authOnlyMethods))
	for k, v := range authOnlyMethods {
		out[k] = v
	}
	return out
}
