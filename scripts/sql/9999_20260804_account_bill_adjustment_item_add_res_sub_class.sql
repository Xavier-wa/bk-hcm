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
 1. 账单调整明细表新增资源子类字段，单列承载卡型或模型厂商，语义由同一行的 res_class 决定
 2. 存量 res_class='gpu' 的记录刷为 gpu_card，res_sub_class 留空；该语句幂等，重复执行影响 0 行
*/

START TRANSACTION;

ALTER TABLE `account_bill_adjustment_item`
    ADD COLUMN `res_sub_class` varchar(64) NULL DEFAULT NULL COMMENT '调账资源子类，gpu_card 下为卡型、gpu_api 下为模型厂商' AFTER `res_class`;

UPDATE `account_bill_adjustment_item`
SET `res_class` = 'gpu_card'
WHERE `res_class` = 'gpu';

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v9.9.9' as `hcm_ver`,
       '9999'   as `sql_ver`;

COMMIT;
