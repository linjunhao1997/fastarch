# Research: AK/SK 签名鉴权凭证管理

**Feature**: specs/009-ak-sk-auth | **Date**: 2026-09-12

## R1. 签名方案与算法

**Decision**: HMAC-SHA256，请求头携带四要素（AK、时间戳、nonce、签名），规范化串覆盖 方法 + 路径 + 查询串 + AK + 时间戳 + nonce + 请求体 SHA-256 摘要。完整规范见 [contracts/ak-sk-signature-spec.md](./contracts/ak-sk-signature-spec.md)。

**Rationale**: 与规格 FR-012 一致（业界通用 HMAC 类方案）；Go 标准库 `crypto/hmac + crypto/sha256` 直接支持，零第三方依赖（宪法原则 VIII）；类 AWS SigV4 / 阿里云 RPC 签名的心智模型，调用方易实现；请求体摘要入签防止传输篡改。

**Alternatives considered**:
- AWS SigV4 完整实现：依赖 AWS SDK 生态与凭证作用域概念，对本项目过重，心知负担高。
- JWT/mTLS：JWT 面向登录会话不适合纯 M2M 静态密钥场景；mTLS 需证书分发体系，运维成本高。
- 自研非对称签名（Ed25519）：验签快且 SK 不可逆，但"AK/SK"语义与用户预期（对称密钥对）不符，SDK 门槛更高。

## R2. SK 存储方式（规格 FR-003 偏差裁决）

**Decision**: SK 以 **AES-256-GCM 加密密文**落库，密钥经环境变量 `IAM_CREDENTIAL_ENCRYPT_KEY` 注入（32 字节，hex/base64 编码）。创建/重置时明文 SK 仅在响应中返回一次；验签时解密还原后计算 HMAC。

**Rationale**: HMAC 是对称算法，服务端验签必须持有可还原的 SK——规格 FR-003 的"不可逆保护"在密码学上与 HMAC 验签互斥（不可逆即无法重算签名）。本方案达成 FR-003 的本质目标：非明文落库；攻击者仅拖库拿到密文无法还原 SK（还需应用侧密钥）。这与云厂商（阿里云/腾讯云 AK/SK）实践一致：SK 均可还原、以加密或受控方式存储。

**Alternatives considered**:
- 仅存 SHA-256(SK) 哈希：验签需改为"客户端持 SK、服务端持哈希"的自定义方案（如 PBKDF2 风格逐请求派生），偏离通用 HMAC 规范，调用方无法用标准库直接实现。
- 明文落库：违反 FR-003 与安全基线，否决。

## R3. AK / SK 生成规则

**Decision**: AK = `"FAK"` 前缀 + 16 字节随机数的 Base62 编码（总长约 24 字符，全局唯一约束兜底，冲突时重试生成）；SK = 32 字节 CSPRNG 随机数的 Base64 编码（约 44 字符，≈256 位强度）。均使用 sponge `krand`（项目既有随机源，内部为 crypto/rand）。

**Rationale**: 前缀便于识别与日志脱敏检索；32 字节满足密码学安全强度（FR-001）；唯一索引兜底 + 重试满足边界场景（AK 冲突）。

## R4. 防重放（nonce 去重）与时间戳窗口

**Decision**: Redis `SET NX EX` 键 `aksk:nonce:<ak>:<nonce>`，TTL 与时间戳窗口一致（默认 300s）；窗口可配置（默认 ±300s，conf 配置段）。SETNX 失败即判定重放拒绝。

**Rationale**: 单条 Redis 命令原子去重，多实例一致（宪法原则 X）；TTL 自动清理无需扫描；miniredis 可测。重启丢失 nonce 键的理论误放窗口 ≤ 时效窗口且签名本身仍需正确，风险可接受（与规格假设一致）。

**Alternatives considered**: DB 唯一表存 nonce——每请求一次写库，写放大严重，否决；单机内存去重——多实例失效，违反分布式裁决，否决。

## R5. 鉴权拦截器双轨扩展

**Decision**: 扩展 `pkg/auth/auth_interceptor.go`：请求元数据含 `x-fa-access-key` 时走签名验签分支（`pkg/auth/signature.go` + `signature_store.go`），验签通过后 `WithAccountId(绑定用户)` 注入上下文，**复用既有权限快照判定逻辑（http:/rpc: 资源键）**；无该头时维持 JWT 分支不变。两轨失败均 fail-closed。

**Rationale**: FR-010a 双轨并存、同一套权限判定；改动收敛在拦截器一处，service 层零感知身份来源（原则 VI）；`GetAccountId` 语义不变。

**注意**: grpc-gateway 网关需透传 `x-fa-*` 头为元数据——在 `gwlib`（idl/lib）的 header matcher/with_metadata 中追加，属契约基础设施小改动。

## R6. LastUsedTime 更新策略

**Decision**: 验签成功后节流更新 `last_used_time`——Redis 键 `aksk:lastused:<ak>` SETNX EX 60 控制每凭证每分钟最多一次写库（异步 best-effort，失败仅记日志不影响请求）。

**Rationale**: 避免每次调用写库的写放大与锁竞争（宪法 XIV 分布式细则）；列表展示精度到分钟对管理场景足够。

## R7. 凭证启停/删除的实时性

**Decision**: 凭证状态以 MySQL 为权威；验签路径**每请求查库**校验 AK 状态与过期时间（单行主键/唯一索引点查，毫秒级），不引入凭证状态缓存——保证禁用/删除后立即生效（SC-003 ≤5s 实际为即时）。权限快照仍走既有 Redis 快照（用户权限变更生效语义与登录态一致）。

**Rationale**: 凭证量级小（管理配置数据），点查成本低；缓存失效一致性问题（5s SLA）不值得引入。删除凭证即行消失，天然即时。

## R8. 引用完整性与审计

**Decision**: 凭证为**基础架构型模型**（宪法 XIV）：允许物理删除；删除前应用层校验（凭证本身为叶子资源，无下游引用；反向地，删除用户时凭证计入引用方——在账户删除校验中追加"名下凭证须为空或一并删除"的检查点说明）；全部写操作（创建/启停/删除/重置）记录结构化审计日志（操作人、凭证 AK、动作、traceId）。

**Alternatives considered**: 软删除——宪法 XIV 明确豁免基础架构型模型，徒增唯一键（AK）负担，否决。

## R9. 契约与代码生成

**Decision**: 新增 `idl/proto/iam/credential.proto`（package `iam.v1`），沿用 `google.api.http` 注解 + `buf.validate` 校验 + `common.v1.Page` 分页；按项目既有流程重新生成 pb/gw/openapiv2（检查入库）。注意项目已知问题：contrib gorm 生成器会丢失 uniqueIndex 列常量，`go generate` 后需手工补回（记忆库既有教训）。

**Rationale**: 与 account/authorization/authentication/application 四个既有域完全同构，无平行体系。

## R10. 前端形态

**Decision**: 认证管理菜单下新增"访问凭证"列表页（复用 ListPageShell + useTableQuery 四态模式）；创建用分步弹窗（选择用户 → 填写备注/过期 → 成功步骤一次性展示 SK 并复制）；编辑用弹窗（仅备注/过期时间——契约可改字段全集）；启用/禁用/重置 SK/删除均二次确认弹窗。路由 key 纳入 MENU_KEYS 与后端 menu 资源种子，权限控制与其他页面同构。

**Rationale**: 契约仅 remark/expireTime 可编辑，页面级路由对两字段编辑过重；弹窗在现有工程中已有 ConfirmDelete 等同构先例；一次性 SK 展示必须强提示 + 复制（FR-018）。
