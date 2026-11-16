package constant

// 网关来源标记（idl/lib/gwlib 注入，仅用于日志与审计标注来源，不参与权限判定：
// 权限码模型下 HTTP 网关与服务间 gRPC 共用同一 full method → 权限码映射）。
const (
	MetadataGatewayOrigin = "x-gateway-origin"
	GatewayOriginHTTP     = "http"
)

// 联邦内部契约的应用凭证元数据键（机器间调用，FederationService 服务内自鉴权）
const (
	MetadataAppId     = "x-fa-app-id"
	MetadataAppSecret = "x-fa-app-secret"
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

// 访问凭证状态（specs/009-ak-sk-auth：启用/禁用二元，无自动封禁，FR-014a）
const (
	AccessCredentialStatusEnabled  = "enabled"
	AccessCredentialStatusDisabled = "disabled"
)
