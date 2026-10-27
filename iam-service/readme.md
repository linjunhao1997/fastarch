## 启动
grpc --addr 127.0.0.1 --port 9090

http --addr 127.0.0.1 --port 8080 --endpoint 127.0.0.1:9090

## AK/SK 签名鉴权（specs/009-ak-sk-auth）

访问凭证（AK/SK）签名调用规范见 [specs/009-ak-sk-auth/contracts/ak-sk-signature-spec.md](../specs/009-ak-sk-auth/contracts/ak-sk-signature-spec.md)。

启动前置：必须注入 SK 静态加密密钥环境变量 `IAM_CREDENTIAL_ENCRYPT_KEY`（32 字节，hex/base64 编码），缺失时服务启动报错（fail-fast，禁止明文/无加密运行）。
