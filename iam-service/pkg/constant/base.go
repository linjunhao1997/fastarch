package constant

// 权限快照资源键前缀（specs/005-typed-permission-model research.md D5）：
// 快照值为带前缀的资源键数组，前缀标识执行域。
const (
	SnapshotKeyPrefixMenu = "menu:"
	SnapshotKeyPrefixHTTP = "http:"
	SnapshotKeyPrefixRPC  = "rpc:"
)

// 网关来源标记（idl/lib/gwlib 注入，拦截器据此区分 HTTP 域与服务间 gRPC 域，research.md D3）。
const (
	MetadataGatewayOrigin = "x-gateway-origin"
	GatewayOriginHTTP     = "http"
)

const (
	ApplicationCode           = "iam-service"
	ApplicationStatusEnabled  = "enabled"
	ApplicationStatusDisabled = "disabled"
)

// 账号状态（创建可指定、编辑可切换；禁用账号拒绝登录）
const (
	AccountStatusEnabled  = "enabled"
	AccountStatusDisabled = "disabled"
)
