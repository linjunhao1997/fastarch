package service

// methodPermissions 是 gRPC full method → 所需权限码的映射（fail-closed：未登记方法拒绝）。
// 权限码是唯一授权词汇（GLOSSARY），home 服务段隔离；种子见 IAM 迁移 000021。
var methodPermissions = map[string][]string{
	"/home.v1.HomepageService/GetHomepage":     {"home.page.get"},
	"/home.v1.HomepageService/SaveDraft":       {"home.page.update"},
	"/home.v1.HomepageService/PublishHomepage": {"home.page.publish"},
}

// authOnlyMethods 是公开读或仅需登录态、不判权的方法：
//   - 菜单由前端按码过滤，服务端返回全量启用菜单（ADR 0004）；
//   - GetPublishedHomepage 是企业官网公开首页的数据源，无需登录即可读取。
//
// 这些方法跳过 IAM 判权，不得据此做写操作归责。
var authOnlyMethods = map[string]struct{}{
	"/home.v1.MenuService/ListMyMenuTree":           {},
	"/home.v1.HomepageService/GetPublishedHomepage": {},
}

// MethodPermsFor 供 authz 拦截器注入：full method → 所需权限码，未登记返回 nil（fail-closed）。
func MethodPermsFor(fullMethod string) []string {
	return methodPermissions[fullMethod]
}

// AuthOnlyMethods 供 authz 拦截器注入：仅需登录态、不判权的方法集合。
func AuthOnlyMethods() map[string]struct{} {
	out := make(map[string]struct{}, len(authOnlyMethods))
	for k, v := range authOnlyMethods {
		out[k] = v
	}
	return out
}
