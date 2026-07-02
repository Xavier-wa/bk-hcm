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
 SQLVER=0005,HCMVER=v1.9.2.0

 Notes:
 obs_aws_bills、obs_huawei_bills、obs_gcp_bills 新增 GpuCardCategory 和 APIBrandName 字段
*/
START TRANSACTION;

ALTER TABLE `obs_aws_bills`
    ADD COLUMN `GpuCardCategory` varchar(64) NOT NULL DEFAULT '',
    ADD COLUMN `APIBrandName`    varchar(64) NOT NULL DEFAULT '';

ALTER TABLE `obs_huawei_bills`
    ADD COLUMN `GpuCardCategory` varchar(64) NOT NULL DEFAULT '',
    ADD COLUMN `APIBrandName`    varchar(64) NOT NULL DEFAULT '';

ALTER TABLE `obs_gcp_bills`
    ADD COLUMN `GpuCardCategory` varchar(64) NOT NULL DEFAULT '',
    ADD COLUMN `APIBrandName`    varchar(64) NOT NULL DEFAULT '';

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v1.9.2.0' as `hcm_ver`,
       '0005'   as `sql_ver`;

COMMIT;
