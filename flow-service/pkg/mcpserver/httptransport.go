package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/linjunhao1997/contrib/pkg/authz"
	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"flow-service/conf"
)

const (
	// PermissionConnect 是"允许该账号接入 flow MCP"的唯一授权词汇。
	// 它与各工具自身的权限码（flow.member.get / flow.task.list / ...）正交：
	// 连得上 ≠ 能操作数据，具体工具调用仍由 IAM 按各自方法判权。
	PermissionConnect = "flow.mcp.connect"

	// 个人凭证头（MCP 客户端静态配置即可携带，无需逐请求签名）。
	headerAccessKey = "x-fa-access-key"
	headerSecretKey = "x-fa-secret-key"

	// maxMCPBodyBytes 限制单次 JSON-RPC 请求体，避免异常请求撑爆内存。
	maxMCPBodyBytes = 4 << 20
	// httpTransportTimeout 是单次工具调用转发到 flow 网关的时长上限。
	httpTransportTimeout = 20 * time.Second
	// defaultAppCode 是 flow 控制台在 IAM 的接入应用 code（判权时用于数据归属校验）。
	defaultAppCode = "flow"
	// tokenCacheTTL 是 AK/SK 换来的 token 在本进程内的缓存上限（秒）。
	// 令牌本身 30 分钟有效，缓存 5 分钟：既省掉每次工具调用的换发开销，
	// 又能在凭证被停用/重置后较快失效（服务端 CheckPermission 仍逐请求判权）。
	tokenCacheTTL = 5 * time.Minute
)

// HTTPHandler 返回挂在 flow-service HTTP 网关上的 MCP 接入点（POST /flow/mcp）。
//
// 身份来自每个请求自己的个人凭证：请求头带 x-fa-access-key / x-fa-secret-key，
// 服务端向 IAM 换取短期 token（CredentialAuth），不保存任何人的 SK 明文。
// 换取后仍先判 flow.mcp.connect，再按该 token 透传调用网关——工具权限继续由
// IAM CheckPermission 逐方法裁决，不存在第二套鉴权。
func HTTPHandler(cfg *conf.Config) (http.Handler, error) {
	conn, err := grpc.NewClient(cfg.Flow.IamAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
	)
	if err != nil {
		return nil, fmt.Errorf("init mcp iam client: %w", err)
	}
	h := &mcpHTTPHandler{
		base:  "http://" + cfg.Http.AddrWithPort(),
		authz: authz.NewWithConn(conn),
		iam:   iam_v1.NewAuthenticationServiceClient(conn),
		cred:  newCredentialCache(),
		http:  &http.Client{Timeout: httpTransportTimeout},
	}
	return h, nil
}

type mcpHTTPHandler struct {
	base  string
	authz *authz.Client
	iam   iam_v1.AuthenticationServiceClient
	cred  *credentialCache
	http  *http.Client
}

func (h *mcpHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		// 唯一支持的入口。
	case http.MethodGet:
		// MCP 的 GET 是可选的服务端推送 SSE 通道，当前实现不提供。
		w.Header().Set("Allow", "POST")
		writeJSONRPCError(w, http.StatusMethodNotAllowed, nil, -32601, "GET 推送通道未启用，请使用 POST")
		return
	case http.MethodDelete:
		// 无状态实现没有会话可关，直接 204。
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		w.Header().Set("Allow", "POST")
		writeJSONRPCError(w, http.StatusMethodNotAllowed, nil, -32601, "method not allowed: "+r.Method)
		return
	}

	token, ge := h.resolveToken(r)
	if ge != nil {
		if ge.status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Bearer realm="flow-mcp"`)
		}
		writeJSONRPCError(w, ge.status, nil, ge.code, ge.msg)
		return
	}

	if err := h.checkConnect(r.Context(), r, token); err != nil {
		var ge *gateError
		if errors.As(err, &ge) {
			writeJSONRPCError(w, ge.status, nil, ge.code, ge.msg)
			return
		}
		writeJSONRPCError(w, http.StatusServiceUnavailable, nil, -32004, err.Error())
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxMCPBodyBytes+1))
	if err != nil {
		writeJSONRPCError(w, http.StatusBadRequest, nil, -32700, "读取请求体失败")
		return
	}
	if len(body) > maxMCPBodyBytes {
		writeJSONRPCError(w, http.StatusRequestEntityTooLarge, nil, -32600, "请求体过大")
		return
	}

	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRPCError(w, http.StatusBadRequest, nil, -32700, "parse error")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), httpTransportTimeout)
	defer cancel()

	// 每次请求一个独立副本：token 只属于这次调用，不落到共享状态里。
	s := &server{base: h.base, token: token, http: h.http}
	resp := s.handle(ctx, &req)
	if resp == nil {
		// 通知类消息（无 id）不需要回复。
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// resolveToken 取本次调用的身份：
// 1) 个人凭证头 x-fa-access-key + x-fa-secret-key → 向 IAM 换取短期 token（带缓存）；
// 2) 兜底的 Authorization: Bearer <token>（本地调试用登录 JWT）。
func (h *mcpHTTPHandler) resolveToken(r *http.Request) (string, *gateError) {
	accessKey := strings.TrimSpace(r.Header.Get(headerAccessKey))
	if accessKey != "" {
		secretKey := strings.TrimSpace(r.Header.Get(headerSecretKey))
		if secretKey == "" {
			return "", errMissingCredential
		}
		token, ge := h.cred.exchange(r.Context(), h.iam, accessKey, secretKey)
		if ge != nil {
			return "", ge
		}
		return token, nil
	}
	if token := bearerFromRequest(r); token != "" {
		return token, nil
	}
	return "", errMissingCredential
}

// credentialCache 缓存 AK/SK 换来的 token，key 用 SK 摘要，进程内存中不存 SK 明文。
type credentialCache struct {
	mu sync.RWMutex
	m  map[string]cachedToken
}

type cachedToken struct {
	token   string
	expires time.Time
}

func newCredentialCache() *credentialCache {
	return &credentialCache{m: make(map[string]cachedToken)}
}

func cacheKey(accessKey, secretKey string) string {
	sum := sha256.Sum256([]byte(secretKey))
	return accessKey + "|" + hex.EncodeToString(sum[:])
}

func (c *credentialCache) exchange(ctx context.Context, cli iam_v1.AuthenticationServiceClient, accessKey, secretKey string) (string, *gateError) {
	key := cacheKey(accessKey, secretKey)
	c.mu.RLock()
	if item, ok := c.m[key]; ok && time.Now().Before(item.expires) {
		c.mu.RUnlock()
		return item.token, nil
	}
	c.mu.RUnlock()

	resp, err := cli.CredentialAuth(ctx, &iam_v1.CredentialAuthRequest{AccessKey: accessKey, SecretKey: secretKey})
	if err != nil {
		switch status.Code(err) {
		case codes.Unauthenticated, codes.PermissionDenied:
			return "", errBadCredential
		}
		return "", &gateError{status: http.StatusServiceUnavailable, code: -32004,
			msg: "凭证认证服务不可用，请稍后重试：" + status.Convert(err).Message()}
	}
	if resp.GetAccessToken() == "" {
		return "", &gateError{status: http.StatusInternalServerError, code: -32005, msg: "凭证认证未返回 token"}
	}

	// 缓存时长：取令牌有效期的 1/6，最多 5 分钟，最少 30 秒。
	ttl := tokenCacheTTL
	if exp := time.Duration(resp.GetExpiresIn()) * time.Second; exp > 0 {
		if sixth := exp / 6; sixth < ttl {
			ttl = sixth
		}
	}
	if ttl < 30*time.Second {
		ttl = 30 * time.Second
	}
	c.mu.Lock()
	c.m[key] = cachedToken{token: resp.GetAccessToken(), expires: time.Now().Add(ttl)}
	c.mu.Unlock()
	return resp.GetAccessToken(), nil
}

var (
	errNoConnectPermission = &gateError{status: http.StatusForbidden, code: -32003,
		msg: "无 flow.mcp.connect 权限，当前账号不能接入 MCP；请联系管理员授予 MCP 接入权限"}
	errMissingCredential = &gateError{status: http.StatusUnauthorized, code: -32001,
		msg: "缺少个人凭证：请在自己的 agent 配置里带上 x-fa-access-key 与 x-fa-secret-key（个人凭证在 IAM「访问凭证」里创建）"}
	errBadCredential = &gateError{status: http.StatusUnauthorized, code: -32002,
		msg: "个人凭证无效：AK 不存在，或凭证已停用/过期，或 SK 不匹配；请核对后在 IAM「访问凭证」里重置"}
	errBadToken = &gateError{status: http.StatusUnauthorized, code: -32006,
		msg: "携带的 token 无效或已过期：请改用 AK/SK 个人凭证，或重新登录换取 token"}
)

// gateError 是接入判权失败时回给 agent 的确定性错误。
type gateError struct {
	status int
	code   int
	msg    string
}

func (e *gateError) Error() string { return e.msg }

// checkConnect 用本次调用换来的 token 向 IAM 判定 flow.mcp.connect。
// IAM 不可达同样拒绝（fail-closed），不把失败当成放行。
func (h *mcpHTTPHandler) checkConnect(ctx context.Context, r *http.Request, token string) error {
	appCode := strings.TrimSpace(r.Header.Get(authz.MetadataAppCode))
	if appCode == "" {
		appCode = defaultAppCode
	}
	md := metadata.Pairs(
		"authorization", "Bearer "+token,
		authz.MetadataAppCode, appCode,
	)
	ctx = metadata.NewIncomingContext(ctx, md)
	ok, _, err := h.authz.CheckPermission(ctx, PermissionConnect)
	if err != nil {
		switch status.Code(err) {
		case codes.Unauthenticated:
			return errBadToken
		case codes.PermissionDenied:
			return errNoConnectPermission
		}
		return fmt.Errorf("IAM 判权不可用，请稍后重试：%w", err)
	}
	if !ok {
		return errNoConnectPermission
	}
	return nil
}

// bearerFromRequest 取 Authorization: Bearer <token>；裸 token 也接受，与 authz 拦截器口径一致。
func bearerFromRequest(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return ""
	}
	if len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}

func writeJSONRPCError(w http.ResponseWriter, status int, id json.RawMessage, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(rpcResponse{Jsonrpc: "2.0", Id: id, Error: &rpcError{Code: code, Message: msg}})
}
