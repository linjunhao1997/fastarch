package model

// MENU_KEYS 是菜单资源常量清单（specs/005-typed-permission-model T028）。
// 与 app/src/routers/router.tsx 的 handle.key 一一镜像：包含全部层级键与带
// label 的叶子键。前端按 menu:<key> 匹配路由键决定菜单显隐——新增前端菜单
// 必须同步本清单，否则无法授权。
var MENU_KEYS = []string{
	// 层级键
	"iam",
	"account_management",
	"application_management",
	"authentication_management",
	"authorization_management",
	// 叶子键
	"account_list",
	"application_list",
	"authentication_policy_list",
	"access_credential_list",
	"permission_list",
	"role_management",
	"permission_subscription",
	"resource_management",
}

// MENU_KEY_NAMES 是菜单资源的中文名（与前端路由 label 镜像，用于展示）。
var MENU_KEY_NAMES = map[string]string{
	"iam":                        "身份访问管理",
	"account_management":         "账号管理",
	"application_management":     "应用管理",
	"authentication_management":  "认证管理",
	"authorization_management":   "权限管理",
	"account_list":               "用户列表",
	"application_list":           "应用列表",
	"authentication_policy_list": "认证策略",
	"access_credential_list":     "访问凭证",
	"permission_list":            "权限列表",
	"role_management":            "角色管理",
	"permission_subscription":    "权限订阅",
	"resource_management":        "资源管理",
}
