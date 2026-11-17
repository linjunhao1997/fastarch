package authz

import (
	"context"

	iam_v1 "github.com/linjunhao1997/idl/pb/iam"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// iamAuthClient 是 authz 依赖的 IAM 判权接口（便于测试替身与解耦具体 pb client）。
type iamAuthClient interface {
	CheckPermission(ctx context.Context, in *iam_v1.CheckPermissionRequest, opts ...grpc.CallOption) (*iam_v1.CheckPermissionResponse, error)
}

// Client 把"判定某权限码"委托给 IAM。自身无状态，JWT 从调用方 ctx 的入站
// metadata 透传，不在本包保存任何凭证或 secret。
type Client struct {
	iam  iamAuthClient
	conn *grpc.ClientConn // 由 New 创建时持有，供 Close；注入构造时为 nil
}

// New 连接到 iamAddr 构造 Client。dev 用明文 gRPC；生产加密由调用方通过
// NewWithConn 注入带 TLS 的连接。
func New(iamAddr string) (*Client, error) {
	conn, err := grpc.NewClient(iamAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
	)
	if err != nil {
		return nil, err
	}
	return NewWithConn(conn), nil
}

// NewWithConn 用已有连接构造 Client（测试注入 mock、或调用方自管连接/TLS 时使用）。
func NewWithConn(conn *grpc.ClientConn) *Client {
	return &Client{iam: iam_v1.NewAuthorizationServiceClient(conn), conn: conn}
}

// newWithIamClient 仅供测试注入自定义 iamAuthClient。
func newWithIamClient(c iamAuthClient) *Client {
	return &Client{iam: c}
}

// CheckPermission 透传入站 metadata（含用户 JWT）调 IAM，判定当前用户是否持有 code。
// 必须在持有入站 metadata 的 gRPC handler ctx 中调用；若 ctx 无入站 metadata，
// IAM 侧会因缺少 token 而拒绝（fail-closed），不会误放行。
//
// 返回的 bool 是 IAM 的判定结果；error 非 nil 表示 IAM 不可达或调用失败，
// 调用方应 fail-closed（拒绝），不得把 error 当作"放行"。
func (c *Client) CheckPermission(ctx context.Context, code string) (bool, error) {
	out := forwardIncomingMetadata(ctx)
	resp, err := c.iam.CheckPermission(out, &iam_v1.CheckPermissionRequest{PermissionCode: code})
	if err != nil {
		return false, err
	}
	return resp.GetOk(), nil
}

// forwardIncomingMetadata 把入站 metadata 原样转为出站，使 IAM 收到同一 JWT。
// 无入站 metadata 时返回原 ctx（IAM 将因无 token 拒绝）。
func forwardIncomingMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, md.Copy())
}

// Close 关闭由 New 创建的连接。NewWithConn 构造的 Client 连接由调用方自管，
// 此时 Close 不做事。
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
