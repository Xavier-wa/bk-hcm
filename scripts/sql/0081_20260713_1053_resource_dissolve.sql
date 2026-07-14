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
    SQLVER=0081,HCMVER=v1.9.2.2

    Notes:
    1. 裁撤主机表新增裁撤截止时间字段

*/

START TRANSACTION;

-- 1. 裁撤主机表新增裁撤截止时间字段
alter table recycle_host_info
    add column `expect_abolish_time` varchar(32) not null default '' comment '裁撤截止时间,格式yyyy-MM-dd';

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v1.9.2.2' as `hcm_ver`, '0081' as `sql_ver`;

COMMIT
