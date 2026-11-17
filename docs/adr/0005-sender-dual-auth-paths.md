# sender 双轨鉴权：入站透传 JWT 薄判权，出站保留 AK/SK 服务账号

sender 作为第二个宿主接入 IAM 时，存在两个方向相反的调用：用户请求经 sender 访问受保护的 `sender.*` 接口（入站），以及 sender 主动调用 IAM（出站，如后续取用户联系方式）。本裁决明确：入站不验签、透传调用方 JWT 给 IAM `CheckPermission`，以 IAM 的判定为唯一授权来源；出站沿用 ADR-0004 的 AK/SK 服务账号签名验签。

这不是推翻 ADR-0004——那份裁决的第二部分只覆盖出站方向。本 ADR 补全此前未明确的入站方向，两者正交。

## Considered Options

入站方向曾考虑按 ADR-0004 的 AK/SK 服务账号模式：sender 以服务账号身份调 IAM。否决，因为入站调用方是终端用户，其授权关系（哪个账号持哪些码）的唯一权威是 IAM；sender 若以服务账号身份判权，等于把 IAM 的裁决复制成第二数据源，违反宪法原则 V（单一数据源），且服务账号身份无法表达"当前用户是否有权"。

## Consequences

- 入站：`application_id` 固定 sender app（id=2）；解出的 accountId/jti 仅作审计，不参与授权；未登记码、IAM 调用失败、拒绝一律 fail-closed；admin 无本地旁路（防伪造 `isAdmin`）。
- 出站：AK/SK 服务账号与现有 IAM 接口不变，留待真正需要的跨服务调用（#15 及以后）。
- 入站与出站可独立演进：更换入站判权机制不影响出站服务账号，反之亦然。
