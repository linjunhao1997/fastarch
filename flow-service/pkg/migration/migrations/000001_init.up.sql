-- 000001 flow-service 基线 Schema。
-- 落 flow 自己的库（flow_dev）；application_id 固定 3（IAM 迁移 000017 注册的 flow 应用）。
-- 宪法 XIV：无物理外键；审计字段服务端注入；业务型模型软删除 + version。
-- 宪法 V：任务的可开工状态（ready/blocked）绝不落库，由 t_question 的 open 数派生。

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

CREATE TABLE IF NOT EXISTS `t_dict_type` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `code` varchar(64) DEFAULT NULL,
  `application_id` bigint DEFAULT NULL,
  `name` varchar(64) DEFAULT NULL,
  `remark` varchar(255) DEFAULT NULL,
  `status` varchar(16) DEFAULT 'enabled',
  `version` bigint DEFAULT NULL,
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_dict_app_code` (`application_id`, `code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `t_dict_item` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `type_id` bigint DEFAULT NULL,
  `label` varchar(64) DEFAULT NULL,
  `value` varchar(64) DEFAULT NULL,
  `sort` int DEFAULT '0',
  `status` varchar(16) DEFAULT 'enabled',
  `version` bigint DEFAULT NULL,
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_t_dict_item_type_id` (`type_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 工作角色：环节职责（≠ IAM 授权角色）。stage 是它在流程里的阶段。
CREATE TABLE IF NOT EXISTS `t_work_role` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `application_id` bigint DEFAULT NULL,
  `code` varchar(64) DEFAULT NULL,
  `name` varchar(64) DEFAULT NULL,
  `stage` varchar(64) DEFAULT NULL,
  `sort` int DEFAULT '0',
  `status` varchar(16) DEFAULT 'enabled',
  `version` bigint DEFAULT NULL,
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_work_role_code` (`application_id`, `code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 一人多角色：关系表，只表达"能担任"，不产生任何 IAM 权限语义。
CREATE TABLE IF NOT EXISTS `t_r_member_role` (
  `member_id` bigint NOT NULL,
  `role_id` bigint NOT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`member_id`, `role_id`),
  KEY `idx_member_role_role` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 员工档案：account_id 弱引用 IAM 账号（唯一），current_role_id 是"当前正在用的角色"。
CREATE TABLE IF NOT EXISTS `t_member` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `account_id` bigint DEFAULT NULL,
  `name` varchar(64) DEFAULT NULL,
  `title` varchar(64) DEFAULT NULL,
  `current_role_id` bigint DEFAULT NULL,
  `version` bigint DEFAULT NULL,
  `is_deleted` tinyint(1) DEFAULT '0',
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_member_account` (`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 任务：只归属角色，不归属人。status 只有 open/done/discarded。
CREATE TABLE IF NOT EXISTS `t_task` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `application_id` bigint DEFAULT NULL,
  `role_id` bigint DEFAULT NULL,
  `title` varchar(255) DEFAULT NULL,
  `brief` text,
  `status` varchar(16) DEFAULT 'open',
  `version` bigint DEFAULT NULL,
  `is_deleted` tinyint(1) DEFAULT '0',
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_task_role` (`role_id`, `status`),
  KEY `idx_task_is_deleted` (`is_deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 阻塞问题：一条 open 的问题 = 任务被阻塞；options 是 AI 预填的选项 JSON。
CREATE TABLE IF NOT EXISTS `t_question` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `application_id` bigint DEFAULT NULL,
  `task_id` bigint DEFAULT NULL,
  `from_role_id` bigint DEFAULT NULL,
  `to_role_id` bigint DEFAULT NULL,
  `asked_by` bigint DEFAULT NULL,
  `body` text,
  `options` json DEFAULT NULL,
  `answer` text,
  `picked` varchar(64) DEFAULT NULL,
  `answer_kind` varchar(16) DEFAULT 'decision',
  `evidence` varchar(512) DEFAULT NULL,
  `status` varchar(16) DEFAULT 'open',
  `parent_question_id` bigint DEFAULT '0',
  `reopen_count` int DEFAULT '0',
  `self_answer` tinyint(1) DEFAULT '0',
  `answered_by` bigint DEFAULT NULL,
  `answered_at` datetime(3) DEFAULT NULL,
  `version` bigint DEFAULT NULL,
  `is_deleted` tinyint(1) DEFAULT '0',
  `created_by` varchar(64) DEFAULT NULL,
  `updated_by` varchar(64) DEFAULT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_question_task_status` (`task_id`, `status`),
  KEY `idx_question_to_role` (`to_role_id`, `status`),
  KEY `idx_question_parent` (`parent_question_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 任务↔问题追溯关系（便于"这个任务历史问过什么"整表扫描）。
CREATE TABLE IF NOT EXISTS `t_r_task_question` (
  `task_id` bigint NOT NULL,
  `question_id` bigint NOT NULL,
  `create_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`task_id`, `question_id`),
  KEY `idx_task_question_question` (`question_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 留痕：只追加。actor_account_id 是审计身份，detail 存 JSON。
CREATE TABLE IF NOT EXISTS `t_event` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `application_id` bigint DEFAULT NULL,
  `actor_account_id` bigint DEFAULT NULL,
  `action` varchar(64) DEFAULT NULL,
  `entity` varchar(32) DEFAULT NULL,
  `entity_id` bigint DEFAULT NULL,
  `detail` text,
  `create_time` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_event_entity` (`entity`, `entity_id`),
  KEY `idx_event_actor` (`actor_account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- ============ 种子：工作角色（8 个环节） ============
INSERT INTO `t_work_role` (`id`,`application_id`,`code`,`name`,`stage`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 1,3,'boss','老板','需求提出',1,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_work_role` WHERE `id`=1);
INSERT INTO `t_work_role` (`id`,`application_id`,`code`,`name`,`stage`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 2,3,'analyst','需求分析师','需求分析',2,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_work_role` WHERE `id`=2);
INSERT INTO `t_work_role` (`id`,`application_id`,`code`,`name`,`stage`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 3,3,'pm','产品经理','产品方案',3,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_work_role` WHERE `id`=3);
INSERT INTO `t_work_role` (`id`,`application_id`,`code`,`name`,`stage`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 4,3,'arch','架构师','技术方案',4,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_work_role` WHERE `id`=4);
INSERT INTO `t_work_role` (`id`,`application_id`,`code`,`name`,`stage`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 5,3,'pjm','项目经理','任务拆分与汇总',5,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_work_role` WHERE `id`=5);
INSERT INTO `t_work_role` (`id`,`application_id`,`code`,`name`,`stage`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 6,3,'qa','测试','验证',7,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_work_role` WHERE `id`=6);
INSERT INTO `t_work_role` (`id`,`application_id`,`code`,`name`,`stage`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 7,3,'dev','开发','开发实现',6,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_work_role` WHERE `id`=7);
INSERT INTO `t_work_role` (`id`,`application_id`,`code`,`name`,`stage`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 8,3,'ops','运维','部署上线',8,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_work_role` WHERE `id`=8);

-- ============ 种子：菜单（路径与前端路由一一对应） ============
INSERT INTO `t_menu` (`id`,`parent_id`,`application_id`,`name`,`path`,`icon`,`sort`,`permission_code`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 3000000000000000001,0,3,'任务工作台','/flow/tasks','UnorderedListOutlined',1,'flow.task.list','enabled',0,'system',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_menu` WHERE `id`=3000000000000000001);
INSERT INTO `t_menu` (`id`,`parent_id`,`application_id`,`name`,`path`,`icon`,`sort`,`permission_code`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 3000000000000000002,0,3,'问题中心','/flow/questions','QuestionCircleOutlined',2,'flow.question.list','enabled',0,'system',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_menu` WHERE `id`=3000000000000000002);

-- ============ 种子：字典 ============
INSERT INTO `t_dict_type` (`id`,`code`,`application_id`,`name`,`remark`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 3000000000000000011,'task_state',3,'任务状态','派生状态，不落库','enabled',0,'system',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_dict_type` WHERE `id`=3000000000000000011);
INSERT INTO `t_dict_type` (`id`,`code`,`application_id`,`name`,`remark`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 3000000000000000012,'answer_kind',3,'闭环方式','decision=选择即可 / evidence=必须给证据','enabled',0,'system',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_dict_type` WHERE `id`=3000000000000000012);

INSERT INTO `t_dict_item` (`id`,`type_id`,`label`,`value`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 3000000000000000021,3000000000000000011,'可开工','ready',1,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_dict_item` WHERE `id`=3000000000000000021);
INSERT INTO `t_dict_item` (`id`,`type_id`,`label`,`value`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 3000000000000000022,3000000000000000011,'被阻塞','blocked',2,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_dict_item` WHERE `id`=3000000000000000022);
INSERT INTO `t_dict_item` (`id`,`type_id`,`label`,`value`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 3000000000000000023,3000000000000000012,'选择/文本','decision',1,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_dict_item` WHERE `id`=3000000000000000023);
INSERT INTO `t_dict_item` (`id`,`type_id`,`label`,`value`,`sort`,`status`,`version`,`created_by`,`create_time`,`update_time`)
SELECT 3000000000000000024,3000000000000000012,'需提交证据','evidence',2,'enabled',0,'system',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_dict_item` WHERE `id`=3000000000000000024);

-- ============ 种子：本人需求作为一条任务（一人多角色的真实验证对象） ============
-- T1 是我（单人验证者）在 dev 角色下的任务：实现工作台 + MCP，并走通闭环。
INSERT INTO `t_task` (`id`,`application_id`,`role_id`,`title`,`brief`,`status`,`version`,`is_deleted`,`created_by`,`create_time`,`update_time`)
SELECT 1,3,7,'实现任务工作台前后端并接入 MCP 验证',
'需求（复盘者：本人一人兼全部角色）：把 AI 研发工作流做成任务工作台——每个环节角色认领任务，卡住就抛阻塞问题（AI 预填选项），上游闭环后任务自动解冻；员工在 agent 里问「我这个任务能开工了吗」，全程留痕可追溯；再提供一个 MCP，能确认我是谁、当前什么角色，并通过输入输出把问题闭环。验收：①同一账号可切换 boss/analyst/pm/arch/pjm/dev/qa/ops 八个角色，任务列表按当前角色隔离；②阻塞问题闭环后任务由 blocked 自动变 ready，无任何同步代码；③evidence 类问题不填证据不能闭环；④在 MCP 客户端走完 whoami → raise → answer → check_ready 一整圈。',
'open',0,0,'seed',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_task` WHERE `id`=1);

INSERT INTO `t_task` (`id`,`application_id`,`role_id`,`title`,`brief`,`status`,`version`,`is_deleted`,`created_by`,`create_time`,`update_time`)
SELECT 2,3,7,'补齐 flow-service 接口联调记录','把 6 个服务 22 个方法的联调结果记成一条留痕，方便回溯。','open',0,0,'seed',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_task` WHERE `id`=2);

INSERT INTO `t_task` (`id`,`application_id`,`role_id`,`title`,`brief`,`status`,`version`,`is_deleted`,`created_by`,`create_time`,`update_time`)
SELECT 3,3,6,'回归任务工作台主流程','覆盖：切换角色、抛问题、答题、证据校验、任务解冻、作废任务。','open',0,0,'seed',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_task` WHERE `id`=3);

INSERT INTO `t_task` (`id`,`application_id`,`role_id`,`title`,`brief`,`status`,`version`,`is_deleted`,`created_by`,`create_time`,`update_time`)
SELECT 4,3,5,'汇总 v2.4 需求与技术方案','把产品方案和技术方案合成一份可下发的任务包。','open',0,0,'seed',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_task` WHERE `id`=4);

-- ============ 种子：AI 预填好的阻塞问题（我登录后就能直接拿它们验证 MCP） ============
INSERT INTO `t_question` (`id`,`application_id`,`task_id`,`from_role_id`,`to_role_id`,`asked_by`,`body`,`options`,`answer`,`picked`,`answer_kind`,`evidence`,`status`,`parent_question_id`,`reopen_count`,`self_answer`,`answered_by`,`answered_at`,`version`,`is_deleted`,`created_by`,`create_time`,`update_time`)
SELECT 1,3,1,7,3,0,'任务工作台首屏默认展示什么？只看当前角色，还是全部角色？',
'[{"id":"A","label":"只看当前角色（切换角色才换列表）","rec":true},{"id":"B","label":"全部角色的任务都展示，按角色分组"},{"id":"C","label":"默认当前角色，可一键切换成全局视图"}]',
'',NULL,'decision','','open',0,0,0,NULL,NULL,0,0,'seed',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_question` WHERE `id`=1);

INSERT INTO `t_question` (`id`,`application_id`,`task_id`,`from_role_id`,`to_role_id`,`asked_by`,`body`,`options`,`answer`,`picked`,`answer_kind`,`evidence`,`status`,`parent_question_id`,`reopen_count`,`self_answer`,`answered_by`,`answered_at`,`version`,`is_deleted`,`created_by`,`create_time`,`update_time`)
SELECT 2,3,1,7,4,0,'MCP 的身份方案用哪种？',
'[{"id":"A","label":"方案A：登录取 JWT，MCP 带 Bearer 调网关（复用现有判权）","rec":true},{"id":"B","label":"方案B：给每个员工发个人 AK/SK，再补入站验签"}]',
'',NULL,'decision','','open',0,0,0,NULL,NULL,0,0,'seed',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_question` WHERE `id`=2);

INSERT INTO `t_question` (`id`,`application_id`,`task_id`,`from_role_id`,`to_role_id`,`asked_by`,`body`,`options`,`answer`,`picked`,`answer_kind`,`evidence`,`status`,`parent_question_id`,`reopen_count`,`self_answer`,`answered_by`,`answered_at`,`version`,`is_deleted`,`created_by`,`create_time`,`update_time`)
SELECT 3,3,3,6,7,0,'联调环境是否已具备 iam / flow / sender 三个服务？',
'[{"id":"A","label":"三服务齐备，可直接开跑","rec":true},{"id":"B","label":"只有 flow，其余用 mock"}]',
'',NULL,'evidence','','open',0,0,0,NULL,NULL,0,0,'seed',NOW(3),NOW(3) FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `t_question` WHERE `id`=3);

INSERT INTO `t_r_task_question` (`task_id`,`question_id`,`create_time`)
SELECT 1,1,NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_r_task_question` WHERE `task_id`=1 AND `question_id`=1);
INSERT INTO `t_r_task_question` (`task_id`,`question_id`,`create_time`)
SELECT 1,2,NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_r_task_question` WHERE `task_id`=1 AND `question_id`=2);
INSERT INTO `t_r_task_question` (`task_id`,`question_id`,`create_time`)
SELECT 3,3,NOW(3) FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `t_r_task_question` WHERE `task_id`=3 AND `question_id`=3);
