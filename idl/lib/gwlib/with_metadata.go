package gwlib

import (
	"context"
	"google.golang.org/grpc/metadata"
	"net/http"
)

const (
	HeaderCookieKeyAuthorization = "Authorization"
)

func WithMetadata(ctx context.Context, req *http.Request) metadata.MD {
	md := metadata.New(map[string]string{})
	pair, _ := req.Cookie(HeaderCookieKeyAuthorization)
	if pair != nil {
		md.Append(HeaderCookieKeyAuthorization, pair.Value)
	}
	return md
}
