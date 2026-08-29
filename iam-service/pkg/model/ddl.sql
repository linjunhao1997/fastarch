CREATE TABLE `t_account` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `code` longtext,
  `name` longtext,
  `email` longtext,
  `status` longtext,
  `salt` longtext,
  `password` longtext,
  `ak` longtext,
  `sk` longtext,
  `kind` longtext,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  `version` bigint DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1756141676752540712 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `t_application` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `name` longtext,
  `status` longtext,
  `kind` longtext,
  `create_time` datetime(3) DEFAULT NULL,
  `update_time` datetime(3) DEFAULT NULL,
  `code` longtext,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `t_permission` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `name` longtext,
  `code` longtext,
  `value` longtext,
  `description` longtext,
  `disabled` tinyint(1) DEFAULT NULL,
  `application_id` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_t_permission_application` (`application_id`),
  CONSTRAINT `fk_t_permission_application` FOREIGN KEY (`application_id`) REFERENCES `t_application` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `t_r_account_role` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `account_id` bigint DEFAULT NULL,
  `role_id` bigint DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `t_r_role_permission` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `role_id` bigint DEFAULT NULL,
  `permission_id` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_t_r_role_permission_role` (`role_id`),
  KEY `fk_t_r_role_permission_permission` (`permission_id`),
  CONSTRAINT `fk_t_r_role_permission_permission` FOREIGN KEY (`permission_id`) REFERENCES `t_permission` (`id`),
  CONSTRAINT `fk_t_r_role_permission_role` FOREIGN KEY (`role_id`) REFERENCES `t_role` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `t_role` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `name` longtext,
  `code` longtext,
  `description` longtext,
  `application_id` bigint DEFAULT NULL,
  `disabled` tinyint(1) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_t_role_application` (`application_id`),
  CONSTRAINT `fk_t_role_application` FOREIGN KEY (`application_id`) REFERENCES `t_application` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;