# AK/SK 请求签名规范（Fastarch IAM）

**版本**: v1.0 | **日期**: 2026-09-12 | **状态**: 随 specs/009-ak-sk-auth 设计冻结

本规范面向调用方（人或程序），描述如何用 AK/SK 对请求签名以调用 fastarch 平台 HTTP 接口。规范公开、可复现：任何具备标准 HMAC-SHA256 的语言/平台均可实现，无需专用 SDK。

## 1. 凭证要素

| 要素 | 说明 |
| --- | --- |
| AK（Access Key） | 公开标识，`FAK` 前缀，与用户绑定；随请求明文传输 |
| SK（Secret Key） | 私密密钥，仅在凭证创建/重置时一次性展示；仅用于本地计算签名，**不得**随请求传输 |

一对 AK/SK 等价于其绑定用户的身份；接口可访问范围 = 绑定用户的 RBAC 授权。

## 2. 请求头

| 头 | 必填 | 说明 |
| --- | --- | --- |
| `X-Fa-Access-Key` | 是 | AK 明文 |
| `X-Fa-Timestamp` | 是 | Unix 秒级时间戳；与服务端偏差须在 ±300s 内（可配置） |
| `X-Fa-Nonce` | 是 | 每次请求唯一的随机串（建议 32 位十六进制或 UUID）；同一 AK + nonce 在时间窗口内不可重复 |
| `X-Fa-Signature` | 是 | 按第 3 节计算的签名（小写十六进制） |
| `X-Fa-Signature-Method` | 否 | 固定 `HMAC-SHA256`，缺省即此值 |

## 3. 签名计算

### 3.1 规范化请求串（StringToSign）

按以下顺序用换行符 `\n` 连接（均为 UTF-8）：

```text
StringToSign =
    HTTPMethod + "\n" +
    CanonicalURI + "\n" +
    CanonicalQueryString + "\n" +
    accessKey + "\n" +
    timestamp + "\n" +
    nonce + "\n" +
    HexLower(SHA256(RequestBody))
```

- **HTTPMethod**：大写，如 `POST`。
- **CanonicalURI**：路径部分，以 `/` 开头；不做 URL 解码重复编码，保持与实际请求路径逐字节一致（推荐始终发送编码后路径）。
- **CanonicalQueryString**：查询参数按 key 升序排序，`key=value` 以 `&` 连接；key 与 value 均做 URL 编码（RFC 3986，空格 → `%20`）；空查询串为空字符串。仅编码一次（编码后的值不再重复编码）。
- **HexLower(SHA256(RequestBody))**：请求体原始字节的 SHA-256 小写十六进制；GET/DELETE 等无 body 请求取空字节的摘要（`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`）。

### 3.2 计算签名

```text
Signature = HexLower(HMAC-SHA256(StringToSign, SK))
```

密钥为 SK 的原始字符串字节，不做任何预处理。

### 3.3 示例（伪代码）

```text
body  = '{"remark":"ci-deploy"}'
sts   = "POST"                          + "\n"
      + "/access-credential/access-credentials" + "\n"
      + ""                              + "\n"
      + "FAKxxxxxxxxxxxxxxxx"           + "\n"
      + "1760000000"                    + "\n"
      + "d3f1c2a4-...-nonce"            + "\n"
      + sha256_hex(body)
sig   = hmac_sha256_hex(sts, sk)
```

## 4. 服务端校验顺序与错误语义

服务端按顺序校验，失败即拒绝（fail-closed），返回 gRPC `Unauthenticated` + ErrorInfo reason：

| 顺序 | 校验 | 失败 reason |
| --- | --- | --- |
| 1 | 四要素头齐全 | `SIGNATURE_MISSING` |
| 2 | AK 存在 | `KEY_UNKNOWN` |
| 3 | 凭证启用 | `KEY_DISABLED` |
| 4 | 未过 expire_time | `KEY_EXPIRED` |
| 5 | 时间戳在 ±300s 窗口内 | `TIMESTAMP_EXPIRED` |
| 6 | AK + nonce 未出现过（Redis SETNX） | `NONCE_REPLAY` |
| 7 | HMAC 签名一致（常数时间比较） | `SIGNATURE_MISMATCH` |

错误响应不泄露 SK 或其任何派生信息。连续验签失败不会自动封禁凭证（记录结构化日志并触发监控告警，由管理员人工处置）。

## 5. 验签通过后

请求以 AK 绑定用户身份进入平台统一权限判定：目标接口资源（`http:<full_method>`）必须在该用户的权限快照内，否则 `PERMISSION_DENIED`（与其他登录态调用一致）。签名鉴权不授予任何额外权限。

## 6. 调用方注意事项

- SK 视同密码：仅存于服务端配置/密钥管理，不入前端代码、不入版本库。
- 时钟偏差是常见故障源：调用方建议启用 NTP；收到 `TIMESTAMP_EXPIRED` 先校对本机时间。
- nonce 生成必须保证时间窗口内唯一（UUID v4 足够）；重试同一请求必须换新 nonce 与时间戳重新签名。
- 网关透传：经反向代理调用时确保 `X-Fa-*` 头不被剥离。
