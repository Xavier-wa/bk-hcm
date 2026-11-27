/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
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
    1. 新增 IaC 仓库主表 iac_repository
    2. 新增 IaC Spec 版本记录表 iac_spec
    3. 新增 IaC Spec 目录与文件节点表 iac_spec_node
    4. 新增 IaC 受管实例状态表 iac_state
*/

START TRANSACTION;

CREATE TABLE IF NOT EXISTS `iac_repository`
(
    `id`                varchar(64)  NOT NULL COMMENT '主键ID，即全局唯一的 repository_id',
    `bk_biz_id`         bigint       NOT NULL COMMENT '业务ID',
    `name`              varchar(128) NOT NULL COMMENT 'IaC仓库名称',
    `latest_version`    varchar(64)  NOT NULL DEFAULT '' COMMENT '最新已上传的Spec版本号（未上传过版本时为空字符串）',
    `last_exec_version` varchar(64)  NOT NULL DEFAULT '' COMMENT '最近一次触发执行的Spec版本号（未执行过时为空字符串）',
    `memo`              varchar(255)          DEFAULT '' COMMENT '仓库备注说明',
    `tenant_id`         varchar(64)  NOT NULL DEFAULT 'default' COMMENT '租户ID',
    `creator`           varchar(64)  NOT NULL COMMENT '创建者',
    `reviser`           varchar(64)  NOT NULL COMMENT '更新者',
    `created_at`        timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`        timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_uk_tenant_biz_name` (`tenant_id`, `bk_biz_id`, `name`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin COMMENT ='IaC仓库主表';

INSERT INTO id_generator(`resource`, `max_id`)
VALUES ('iac_repository', '0');

CREATE TABLE IF NOT EXISTS `iac_spec`
(
    `id`            varchar(64)  NOT NULL COMMENT '主键ID（spec_id）',
    `bk_biz_id`     bigint       NOT NULL COMMENT '业务ID（与所属 iac_repository.bk_biz_id 一致）',
    `repository_id` varchar(64)  NOT NULL COMMENT '所属IaC仓库ID，关联 iac_repository.id',
    `version`       varchar(64)  NOT NULL COMMENT '版本号（由用户上传时指定，同一仓库下唯一）',
    `status`        varchar(32)  NOT NULL COMMENT '版本状态：uploading（上传写入中）/ uploaded（已上传生效）/ upload_failed（上传失败）',
    `last_run_id`   varchar(64)  NOT NULL DEFAULT '' COMMENT '最近一次执行的编排记录ID，关联 iac_run.id',
    `memo`          varchar(255)          DEFAULT '' COMMENT '版本备注',
    `tenant_id`     varchar(64)  NOT NULL DEFAULT 'default' COMMENT '租户ID',
    `creator`       varchar(64)  NOT NULL COMMENT '创建者',
    `reviser`       varchar(64)  NOT NULL COMMENT '更新者',
    `created_at`    timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`    timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_uk_repository_version` (`repository_id`, `version`),
    KEY `idx_tenant_biz_repository` (`tenant_id`, `bk_biz_id`, `repository_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin COMMENT ='IaC Spec版本记录表';

INSERT INTO id_generator(`resource`, `max_id`)
VALUES ('iac_spec', '0');

CREATE TABLE IF NOT EXISTS `iac_spec_node`
(
    `id`                varchar(64)  NOT NULL COMMENT '主键ID，即全局唯一的节点ID（node_id）',
    `repository_id`     varchar(64)  NOT NULL COMMENT '所属IaC仓库ID，关联 iac_repository.id',
    `spec_id`           varchar(64)  NOT NULL COMMENT '所属Spec版本ID，关联 iac_spec.id',
    `parent_id`         varchar(64)  NOT NULL DEFAULT '' COMMENT '父目录节点ID（根目录 level=0 时为空字符串）',
    `level`             int          NOT NULL COMMENT '节点所在目录层级（根目录为 0，每深入一层 +1）',
    `node_type`         varchar(16)  NOT NULL COMMENT '节点类型：dir（目录）/ file（.tf 文件）',
    `node_name`         varchar(256) NOT NULL COMMENT '当前层的目录名或 .tf 文件名（根目录为 "."）',
    `children_node_ids` json                  DEFAULT NULL COMMENT '当前目录下的直接子节点 ID 列表（仅 dir 有值；file 为 NULL）',
    `file_size`         bigint       NOT NULL DEFAULT 0 COMMENT '文件字节数（dir 为 0）',
    `content`           mediumtext            DEFAULT NULL COMMENT '文件原始文本（仅 file 有值；dir 为 NULL）',
    `tenant_id`         varchar(64)  NOT NULL DEFAULT 'default' COMMENT '租户ID',
    `creator`           varchar(64)  NOT NULL COMMENT '创建者',
    `reviser`           varchar(64)  NOT NULL COMMENT '更新者',
    `created_at`        timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`        timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_spec_level` (`spec_id`, `level`),
    KEY `idx_repository_id` (`repository_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin COMMENT ='IaC Spec版本目录与文件节点表';

INSERT INTO id_generator(`resource`, `max_id`)
VALUES ('iac_spec_node', '0');

CREATE TABLE IF NOT EXISTS `iac_state`
(
    `id`                  varchar(64)  NOT NULL COMMENT '主键ID（由 id_generator 分配，即 state_id）',
    `repository_id`       varchar(64)  NOT NULL COMMENT '所属IaC仓库ID，关联 iac_repository.id',
    `block_id`            varchar(512) NOT NULL COMMENT 'Terraform标准格式的全局实例地址（如 tencentcloud_subnet.app[0]）',
    `kind`                varchar(32)  NOT NULL COMMENT '实例类别，固定为 resource 或 data',
    `current`             json                  DEFAULT NULL COMMENT '当前对外生效的 StateObject JSON，主实例已删但 deposed 仍有残留时为 NULL',
    `deposed`             json         NOT NULL COMMENT '先建后删过程中暂存的待销毁旧实例字典（map[DeposedKey]StateObject，平时为空对象 {}）',
    `last_flow_id`        varchar(64)  NOT NULL DEFAULT '' COMMENT '最近一次成功写入该状态的异步任务流ID，关联 async_flow.id',
    `import_from_spec_id` varchar(64)  NOT NULL DEFAULT '' COMMENT '初始导入暂存标记：空串表示已正式生效；非空为对应 uploading/upload_failed 的 spec_id',
    `tenant_id`           varchar(64)  NOT NULL DEFAULT 'default' COMMENT '租户ID',
    `creator`             varchar(64)  NOT NULL COMMENT '创建者',
    `reviser`             varchar(64)  NOT NULL COMMENT '更新者',
    `created_at`          timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`          timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_uk_repository_block` (`repository_id`, `block_id`),
    KEY `idx_tenant_repository` (`tenant_id`, `repository_id`),
    KEY `idx_repository_import_spec` (`repository_id`, `import_from_spec_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin COMMENT ='IaC受管实例状态表';

INSERT INTO id_generator(`resource`, `max_id`)
VALUES ('iac_state', '0');

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v9.9.9' as `hcm_ver`, '9999' as `sql_ver`;

COMMIT;
