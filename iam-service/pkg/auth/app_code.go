package auth

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ErrAppNotAdmitted 表示请求选择的 appCode 不在令牌接入集合内（或解析失败）。
var ErrAppNotAdmitted = status.Error(codes.PermissionDenied, "application not admitted")

// AppResolver 把数据归属应用 code 解析为应用 id；仅启用应用有效，
// 停用/不存在返回 error。由宿主注入（DB 查询），拦截器不直接依赖存储。
type AppResolver func(ctx context.Context, code string) (appId int64, err error)

// AppCodeFromMetadata 提取数据归属应用选择头 x-fa-app-code；缺省返回空串。
// 空串由调用方决定语义（IAM 管理控制台可回落到控制台应用，业务服务 fail-closed）。
func AppCodeFromMetadata(md metadata.MD) string {
	values := md.Get(MetadataAppCode)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// resolveAppCode 把请求头 appCode 解析为数据归属应用 id，并校验 ∈ 接入集合。
// resolver 为空时无法解析 → 拒绝（fail-closed）；code 不存在/停用 → 拒绝；
// 校验不通过 → ErrAppNotAdmitted。授权完全来自签名 claim，header 仅是选择信号。
func resolveAppCode(code string, claims *CustomClaims, resolver AppResolver) (int64, error) {
	if resolver == nil || claims == nil {
		return 0, ErrAppNotAdmitted
	}
	appId, err := resolver(context.Background(), code)
	if err != nil || appId <= 0 {
		return 0, ErrAppNotAdmitted
	}
	if !IsAppAdmitted(claims, appId) {
		return 0, ErrAppNotAdmitted
	}
	return appId, nil
}

// IsAppAdmitted 判断 appId 是否在令牌快照的接入集合内。
// 语义：admin 旁路（可切换任意应用）；其余用户的授权完全来自签名的 claim，
// header 里的 appCode 只是选择信号，不在集合内一律拒绝（fail-closed）。
func IsAppAdmitted(claims *CustomClaims, appId int64) bool {
	if claims == nil || appId <= 0 {
		return false
	}
	if claims.IsAdmin {
		return true
	}
	for _, admitted := range claims.AdmittedAppIds {
		if admitted == appId {
			return true
		}
	}
	return false
}
