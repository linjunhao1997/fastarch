package dict

// 通用字典能力已抽至本包（contrib/pkg/dict，ADR 0001）：每服务嵌入自持，
// 宿主差异经 Config 钩子注入。业务错误经 bizerr 透传（业务码不变）。
// 分页/模糊过滤等纯逻辑随包自带，不依赖宿主 helper。
