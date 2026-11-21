-- 000001 home-service 基线 Schema：自持菜单表 + 首页配置单行表 + 默认首页/菜单种子。
-- application_id 固定为 IAM 注册的 home 应用（id=4）。宪法 XIV：无物理外键；审计字段服务端注入。
CREATE TABLE IF NOT EXISTS `t_menu` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `parent_id` bigint DEFAULT '0',
  `application_id` bigint DEFAULT NULL,
  `name` varchar(64) DEFAULT NULL,
  `path` varchar(128) DEFAULT NULL,
  `icon` varchar(64) DEFAULT NULL,
  `sort` int DEFAULT '0',
  `permission_code` varchar(64) DEFAULT NULL,
  `status` varchar(16) DEFAULT 'enabled',
  `version` bigint DEFAULT NULL,
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_t_menu_parent_id` (`parent_id`),
  KEY `idx_t_menu_application_id` (`application_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_homepage` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `application_id` bigint NOT NULL,
  `draft_json` longtext,
  `published_json` longtext,
  `published_at` datetime(3) DEFAULT NULL,
  `version` bigint DEFAULT '0',
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_t_homepage_app` (`application_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 默认首页（草稿=已发布，首次进入即可见、可编辑）
INSERT INTO `t_homepage` (`id`,`application_id`,`draft_json`,`published_json`,`published_at`,`version`,`created_by`,`updated_by`,`create_time`,`update_time`)
SELECT 1,4,'{"brandName":"星河科技","logoText":"XH","nav":["首页","产品","解决方案","关于我们"],"theme":{"primaryColor":"#2563eb","background":"light","layout":"classic"},"hero":{"eyebrow":"为企业增长而生","title":"让复杂的业务，变得简单可增长","subtitle":"从数据到决策，从流程到协同，一套平台连接团队、客户与未来。","primaryCta":"预约演示","secondaryCta":"了解产品"},"stats":[{"value":"12 年","label":"行业积累"},{"value":"3000+","label":"服务企业"},{"value":"99.99%","label":"服务可用性"}],"sections":[{"id":"sec-1","title":"一套平台，覆盖全链路","body":"从线索到交付，把分散的系统连接成一条可观测的业务链路。","layout":"cards","visible":true,"items":[{"title":"统一数据","description":"跨系统数据标准化，实时同步。"},{"title":"流程协同","description":"任务、审批、消息一体化。"},{"title":"智能决策","description":"指标看板与预警，异常先发现。"}]},{"id":"sec-2","title":"为增长而设计的体验","body":"让每个团队都能在自己的节奏里协作，让每一次决策都有数据支撑。","layout":"center","visible":true,"items":[]},{"id":"sec-3","title":"从今天开始，连接你的团队","body":"预约一次演示，看看星河如何把复杂业务变简单。","layout":"left","visible":true,"items":[]}],"footer":"© 2026 星河科技. 保留所有权利。"}','{"brandName":"星河科技","logoText":"XH","nav":["首页","产品","解决方案","关于我们"],"theme":{"primaryColor":"#2563eb","background":"light","layout":"classic"},"hero":{"eyebrow":"为企业增长而生","title":"让复杂的业务，变得简单可增长","subtitle":"从数据到决策，从流程到协同，一套平台连接团队、客户与未来。","primaryCta":"预约演示","secondaryCta":"了解产品"},"stats":[{"value":"12 年","label":"行业积累"},{"value":"3000+","label":"服务企业"},{"value":"99.99%","label":"服务可用性"}],"sections":[{"id":"sec-1","title":"一套平台，覆盖全链路","body":"从线索到交付，把分散的系统连接成一条可观测的业务链路。","layout":"cards","visible":true,"items":[{"title":"统一数据","description":"跨系统数据标准化，实时同步。"},{"title":"流程协同","description":"任务、审批、消息一体化。"},{"title":"智能决策","description":"指标看板与预警，异常先发现。"}]},{"id":"sec-2","title":"为增长而设计的体验","body":"让每个团队都能在自己的节奏里协作，让每一次决策都有数据支撑。","layout":"center","visible":true,"items":[]},{"id":"sec-3","title":"从今天开始，连接你的团队","body":"预约一次演示，看看星河如何把复杂业务变简单。","layout":"left","visible":true,"items":[]}],"footer":"© 2026 星河科技. 保留所有权利。"}',NOW(3),1,'system','system',NOW(3),NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM `t_homepage` WHERE `application_id`=4);

-- 菜单种子：路径与前端路由一一对应
INSERT INTO `t_menu` (`id`,`parent_id`,`application_id`,`name`,`path`,`icon`,`sort`,`permission_code`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 4000000000000000001,0,4,'首页配置','/home/homepage','BookOutlined',1,'home.page.get','enabled',0,'system',NOW(3),NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM `t_menu` WHERE `id`=4000000000000000001);
