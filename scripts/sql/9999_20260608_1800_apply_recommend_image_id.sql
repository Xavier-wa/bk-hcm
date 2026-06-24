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
    1. ziyan_cvm_device_info 新增 image_id 字段
    2. ziyan_cvm_apply_user_recommend 新增 image_id 字段，唯一键追加 image_id
    3. ziyan_cvm_apply_biz_recommend 新增 image_id 字段，唯一键追加 image_id
*/

START TRANSACTION;

ALTER TABLE `ziyan_cvm_device_info`
    ADD COLUMN `image_id` varchar(64) NOT NULL DEFAULT '' COMMENT '镜像ID' AFTER `device_type`;

ALTER TABLE `ziyan_cvm_apply_user_recommend`
    ADD COLUMN `image_id` varchar(64) NOT NULL DEFAULT '' COMMENT '镜像ID' AFTER `device_type`,
    DROP INDEX `idx_uk_bk_username_bk_biz_id_require_type_region_device_type`,
    ADD UNIQUE KEY `idx_uk_username_biz_id_require_type_region_device_type_image_id` (`bk_username`, `bk_biz_id`,
                                                                                      `require_type`, `region`,
                                                                                      `device_type`, `image_id`);

ALTER TABLE `ziyan_cvm_apply_biz_recommend`
    ADD COLUMN `image_id` varchar(64) NOT NULL DEFAULT '' COMMENT '镜像ID' AFTER `device_type`,
    DROP INDEX `idx_uk_bk_biz_id_require_type_region_device_type`,
    ADD UNIQUE KEY `idx_uk_biz_id_require_type_region_device_type_image_id` (`bk_biz_id`, `require_type`, `region`, `device_type`, `image_id`);

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v9.9.9' as `hcm_ver`, '9999' as `sql_ver`;

COMMIT;
