# 企业首页展示后台 · 功能设计（feature_design）

## 1. 信息架构

新增第 4 个控制台「企业官网」（console code = `home`，HTTP 前缀 `/home`，服务端口 8083 / gRPC 9093）。

```
企业官网控制台 /home
└── 首页配置 /home/homepage   （菜单种子，权限码 home.page.get）
    ├── 顶部：保存草稿 / 发布 / 全屏预览 / 草稿态与发布时间
    ├── 左栏：编辑区
    │   ├── 站点设置（品牌名、Logo 文字、导航项）
    │   ├── 首屏 Hero（眉标、主标题、副标题、主/次按钮）
    │   ├── 主题与布局（主题色、背景风格、整页布局）
    │   └── 内容区块（列表：新增、上移、下移、删除、显示/隐藏）
    └── 右栏：实时预览（与左栏同一份表单状态，无需先保存）
```

## 2. 页面结构

### 2.1 编辑区

- **站点设置**：品牌名（Input）、Logo 文字（Input）、导航菜单（标签输入，逗号分隔）。
- **首屏**：眉标（Input）、主标题（TextArea）、副标题（TextArea）、主按钮文案、次按钮文案。
- **主题**：主题色（ColorPicker，默认 `#2563eb`）、背景风格（浅色/深色/渐变）、整页布局（经典 / 居中 / 分栏）。
- **区块**：每个区块一张可折叠卡片，字段为标题、正文、单区块布局（居中/左对齐/卡片组）、是否显示；卡片头提供上移/下移/删除。底部「新增区块」。
- 所有控件受控；任何 change 直接更新 `HomepageContent` 状态，预览同步重渲染。

### 2.2 预览区

- 渲染完整企业首页：顶部导航 + Hero + 统计条 + 区块列表 + 页脚。
- 按主题色、背景风格、整页布局、区块布局与顺序真实反映配置。
- 右栏预览为「桌面宽度」缩略；全屏预览打开全屏 Modal，按 1200px 内容宽度渲染。
- 预览数据来自当前表单状态，**包括未保存修改**；这与「已发布版本预览」不同。

### 2.3 状态与操作

| 操作 | 行为 | 失败处理 |
| --- | --- | --- |
| 保存草稿 | `PUT /home/homepage/draft`，带当前 version | 版本冲突提示“内容已被更新，请刷新后重试”；其他错误统一通知 |
| 发布 | 先保存草稿，再 `POST /home/homepage/publish` | 保存失败则不发布；发布成功刷新已发布时间 |
| 全屏预览 | 打开 Modal，渲染当前表单状态 | 无 |
| 刷新 | `GET /home/homepage` 返回草稿 + 已发布 + 版本 | 读取失败显示重试 |

脏数据标记：表单与最近一次加载/保存的 JSON 不一致时，顶部显示「有未保存修改」；离开页面不做拦截（本期不做路由守卫）。

## 3. 数据契约（JSON）

`HomepageContent` 存于 `t_homepage.draft_json / published_json`，proto 以 `content_json` 字符串承载，服务端只校验为合法 JSON 对象与大小上限，字段演进由前后端共享 TS 类型约束。

```json
{
  "version": 1,
  "brandName": "星河科技",
  "logoText": "XH",
  "nav": ["首页", "产品", "解决方案", "关于我们"],
  "theme": {
    "primaryColor": "#2563eb",
    "background": "light",
    "layout": "classic"
  },
  "hero": {
    "eyebrow": "为企业增长而生",
    "title": "让复杂的业务，变得简单可增长",
    "subtitle": "从数据到决策，从流程到协同，一套平台连接团队、客户与未来。",
    "primaryCta": "预约演示",
    "secondaryCta": "了解产品"
  },
  "stats": [
    { "value": "12 年", "label": "行业积累" },
    { "value": "3000+", "label": "服务企业" },
    { "value": "99.99%", "label": "服务可用性" }
  ],
  "sections": [
    {
      "id": "sec-1",
      "title": "一套平台，覆盖全链路",
      "body": "从线索到交付，把分散的系统连接成一条可观测的业务链路。",
      "layout": "cards",
      "visible": true,
      "items": [
        { "title": "统一数据", "description": "跨系统数据标准化，实时同步。" },
        { "title": "流程协同", "description": "任务、审批、消息一体化。" },
        { "title": "智能决策", "description": "指标看板与预警，异常先发现。" }
      ]
    }
  ],
  "footer": "© 2026 星河科技. 保留所有权利。"
}
```

约束：

- `theme.background` ∈ `light | dark | gradient`；`theme.layout` ∈ `classic | centered | split`。
- `section.layout` ∈ `center | left | cards`；`visible` 缺省 true。
- `sections` 最多 12 个；`section.items` 最多 6 个；字符串字段长度由前端 `maxLength` 限制，服务端校验 JSON ≤ 200KB。
- 版本字段由服务端维护，前端提交的 JSON 中 `version` 可省略。

## 4. 交互细节

- 新增区块默认：标题「新内容区块」、正文一句话、布局 `center`、显示 true，并自动滚动到新卡片。
- 删除区块需二次确认（Popconfirm）。
- 上移/下移在数组层面交换，首/末项禁用对应按钮。
- 隐藏区块仍保留编辑数据，预览跳过渲染；再次显示即恢复。
- 发布前若存在未保存修改，自动先保存再发布；发布成功后提示“已发布 vX”。
- 主题色使用 AntD ColorPicker，输出 hex；预览中所有按钮/强调色/渐变以此色计算（用 `color-mix`/rgba 简化）。

## 5. 权限与菜单

| 动作 | 权限码 |
| --- | --- |
| 打开页面/读取配置 | `home.page.get` |
| 保存草稿 | `home.page.update` |
| 发布 | `home.page.publish` |
| 侧栏菜单「首页配置」 | `home.page.get` |

服务端方法映射 fail-closed；前端按钮按权限码显隐（无 update 不显示保存，无 publish 不显示发布）。

## 6. 兼容与降级

- 后端默认种子提供一份完整首页，前端读取为空或解析失败时回落到同款默认常量，保证页面可用。
- `GetPublishedHomepage` 供后续官网 SSR/静态站读取；本期控制台只用 `GetHomepage` 的草稿与已发布时间。
