package gwlib

import (
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

const (
	HeaderTraceID       = "trace-id"
	HeaderSetCookie     = "set-cookie"
	HeaderAuthorization = "Authorization"
)

var OriginalIncomingHeaderDict = map[string]struct{}{
	HeaderTraceID: {},
	//HeaderAuthorization: {},
}

func IncomingHeaderMatcher(key string) (string, bool) {
	if _, ok := OriginalIncomingHeaderDict[key]; ok {
		return key, ok
	}
	return "", false
}

var OriginalOutgoingHeaderDict = map[string]struct{}{
	HeaderSetCookie: {},
}

func OutgoingHeaderMatcher(key string) (string, bool) {
	if _, ok := OriginalOutgoingHeaderDict[key]; ok {
		return key, ok
	}
	return fmt.Sprintf("%s%s", runtime.MetadataHeaderPrefix, key), true
}
