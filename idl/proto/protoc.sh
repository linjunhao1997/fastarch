#!/usr/bin/env sh
# 生成 gRPC / grpc-gateway / openapiv2 代码。
# 依赖（go install 一次即可）：
#   go install github.com/bufbuild/buf/cmd/buf@v1.54.0
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#   go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
#   go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
#
# 用法：在 idl/ 或 idl/proto/ 下执行 `sh ./proto/protoc.sh` 均可。
cd "$(dirname "$0")" || exit 1

buf dep update
buf generate
