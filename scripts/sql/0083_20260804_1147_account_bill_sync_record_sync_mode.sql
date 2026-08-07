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
    SQLVER=0083,HCMVER=v1.9.2.8

    Notes:
    1. account_bill_sync_record 表新增 sync_mode 字段，标识对外同步模式（full/adjustment_only），默认 full 兼容存量数据
*/

START TRANSACTION;

ALTER TABLE `account_bill_sync_record`
    ADD COLUMN `sync_mode` varchar(32) NOT NULL DEFAULT 'full' COMMENT '同步模式：full-全量同步，adjustment_only-只同步调账' AFTER `adjustment_flow_id`;

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v1.9.2.8' as `hcm_ver`, '0083' as `sql_ver`;

COMMIT;
