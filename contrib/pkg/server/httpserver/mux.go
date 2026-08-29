// Package httpserver 提供 grpc-gateway 反向代理 mux 的组装和 HTTP 服务的优雅启停。
package httpserver

import (
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/linjunhao1997/idl/lib/gwlib"
)

// NewMux 创建 grpc-gateway mux，默认带上 gwlib 的 header matcher 和 metadata 规则；
// 业务可通过 opts 追加自定义 ServeMuxOption。
func NewMux(opts ...runtime.ServeMuxOption) *runtime.ServeMux {
	defaults := []runtime.ServeMuxOption{
		runtime.WithIncomingHeaderMatcher(gwlib.IncomingHeaderMatcher),
		runtime.WithOutgoingHeaderMatcher(gwlib.OutgoingHeaderMatcher),
		runtime.WithMetadata(gwlib.WithMetadata),
	}
	return runtime.NewServeMux(append(defaults, opts...)...)
}
