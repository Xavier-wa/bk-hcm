/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

/*
    SQLVER=9999,HCMVER=v9.9.9

    Notes:
    1. 新增用户维度申领推荐表 ziyan_cvm_apply_user_recommend，存储按 (bk_biz_id, bk_username) 分组的 Top-K 推荐三元组
    2. 新增业务维度申领推荐表 ziyan_cvm_apply_biz_recommend，存储按 bk_biz_id 分组的 Top-K 推荐三元组
    3. 注册两个表的 id_generator 资源
*/

START TRANSACTION;

CREATE TABLE IF NOT EXISTS `ziyan_cvm_apply_user_recommend`
(
    `id`           varchar(64)  NOT NULL COMMENT '主键ID',
    `bk_biz_id`    bigint       NOT NULL COMMENT '业务ID',
    `bk_username`  varchar(64)  NOT NULL COMMENT '申请人',
    `require_type` int          NOT NULL DEFAULT 0 COMMENT '需求类型',
    `region`       varchar(128) NOT NULL DEFAULT '' COMMENT '地域',
    `device_type`  varchar(64)  NOT NULL DEFAULT '' COMMENT '机型',
    `count`        int          NOT NULL DEFAULT 0 COMMENT '历史申领次数',
    `creator`      varchar(64)  NOT NULL DEFAULT '' COMMENT '创建人',
    `reviser`      varchar(64)  NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`   datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_uk_bk_username_bk_biz_id_require_type_region_device_type` (`bk_username`, `bk_biz_id`, `require_type`, `region`, `device_type`),
    KEY `idx_updated_at` (`updated_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='用户维度申领机型推荐表';

CREATE TABLE IF NOT EXISTS `ziyan_cvm_apply_biz_recommend`
(
    `id`           varchar(64)  NOT NULL COMMENT '主键ID',
    `bk_biz_id`    bigint       NOT NULL COMMENT '业务ID',
    `require_type` int          NOT NULL DEFAULT 0 COMMENT '需求类型',
    `region`       varchar(128) NOT NULL DEFAULT '' COMMENT '地域',
    `device_type`  varchar(64)  NOT NULL DEFAULT '' COMMENT '机型',
    `count`        int          NOT NULL DEFAULT 0 COMMENT '历史申领次数',
    `creator`      varchar(64)  NOT NULL DEFAULT '' COMMENT '创建人',
    `reviser`      varchar(64)  NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`   datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_uk_bk_biz_id_require_type_region_device_type` (`bk_biz_id`, `require_type`, `region`, `device_type`),
    KEY `idx_updated_at` (`updated_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='业务维度申领机型推荐表';

INSERT INTO id_generator(`resource`, `max_id`)
VALUES ('ziyan_cvm_apply_user_recommend', '0'),
       ('ziyan_cvm_apply_biz_recommend', '0');

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v9.9.9' as `hcm_ver`, '9999' as `sql_ver`;

COMMIT;
