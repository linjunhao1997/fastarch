package service

// DefaultHomepageJSON 是首次访问或数据缺失时回落的默认首页。
// 与迁移 000001 的种子、前端 default-homepage.ts 保持同构；字段契约见 docs/home-service-feature-design.md。
const DefaultHomepageJSON = `{
  "brandName": "星河科技",
  "logoText": "XH",
  "nav": ["首页", "产品", "解决方案", "关于我们"],
  "theme": {"primaryColor": "#2563eb", "background": "light", "layout": "classic"},
  "hero": {
    "eyebrow": "为企业增长而生",
    "title": "让复杂的业务，变得简单可增长",
    "subtitle": "从数据到决策，从流程到协同，一套平台连接团队、客户与未来。",
    "primaryCta": "预约演示",
    "secondaryCta": "了解产品"
  },
  "stats": [
    {"value": "12 年", "label": "行业积累"},
    {"value": "3000+", "label": "服务企业"},
    {"value": "99.99%", "label": "服务可用性"}
  ],
  "sections": [
    {
      "id": "sec-1",
      "title": "一套平台，覆盖全链路",
      "body": "从线索到交付，把分散的系统连接成一条可观测的业务链路。",
      "layout": "cards",
      "visible": true,
      "items": [
        {"title": "统一数据", "description": "跨系统数据标准化，实时同步。"},
        {"title": "流程协同", "description": "任务、审批、消息一体化。"},
        {"title": "智能决策", "description": "指标看板与预警，异常先发现。"}
      ]
    },
    {
      "id": "sec-2",
      "title": "为增长而设计的体验",
      "body": "让每个团队都能在自己的节奏里协作，让每一次决策都有数据支撑。",
      "layout": "center",
      "visible": true,
      "items": []
    },
    {
      "id": "sec-3",
      "title": "从今天开始，连接你的团队",
      "body": "预约一次演示，看看星河如何把复杂业务变简单。",
      "layout": "left",
      "visible": true,
      "items": []
    }
  ],
  "footer": "© 2026 星河科技. 保留所有权利。"
}`
