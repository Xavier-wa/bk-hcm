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

package tziyan

import (
	"hcm/cmd/cloud-server/service/sync/detail"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// SyncPublicResourceOption ...
type SyncPublicResourceOption struct {
	AccountID string `json:"account_id" validate:"required"`
}

// Validate SyncPublicResourceOption
func (opt *SyncPublicResourceOption) Validate() error {
	return validator.Validate.Struct(opt)
}

// SyncPublicResource ...
func SyncPublicResource(kt *kit.Kit, cliSet *client.ClientSet, opt *SyncPublicResourceOption,
	sd *detail.SyncDetail) (failedRes enumor.CloudResourceType, err error) {

	if err := opt.Validate(); err != nil {
		return "", err
	}

	if err := syncPublicResWithStatus(kt, sd, enumor.RegionCloudResType, func() error {
		return SyncRegion(kt, cliSet.HCService(), opt.AccountID)
	}); err != nil {
		return enumor.RegionCloudResType, err
	}

	regions, err := ListRegion(kt, cliSet.DataService())
	if err != nil {
		return "", err
	}

	if err := syncPublicResWithStatus(kt, sd, enumor.ZoneCloudResType, func() error {
		return SyncZone(kt, cliSet.HCService(), opt.AccountID, regions)
	}); err != nil {
		return enumor.ZoneCloudResType, err
	}

	if err := syncPublicResWithStatus(kt, sd, enumor.ImageCloudResType, func() error {
		return SyncImage(kt, cliSet.HCService(), opt.AccountID, regions)
	}); err != nil {
		return enumor.ImageCloudResType, err
	}

	return "", nil
}

// syncPublicResWithStatus 包装公共资源同步流程，统一记录同步中/成功/失败状态。
// 同步失败时先写入失败状态，再返回原始错误，保持上层对失败资源类型和错误的识别语义不变。
func syncPublicResWithStatus(kt *kit.Kit, sd *detail.SyncDetail, resType enumor.CloudResourceType,
	syncFunc func() error) error {

	if err := sd.ResSyncStatusSyncing(resType); err != nil {
		logs.Errorf("set res sync status syncing failed, res: %s, err: %v, accountID: %s, rid: %s",
			resType, err, sd.AccountID, kt.Rid)
		return err
	}

	if syncErr := syncFunc(); syncErr != nil {
		if err := sd.ResSyncStatusFailed(resType, syncErr); err != nil {
			logs.Errorf("set res sync status failed failed, res: %s, err: %v, accountID: %s, rid: %s",
				resType, err, sd.AccountID, kt.Rid)
		}
		return syncErr
	}

	if err := sd.ResSyncStatusSuccess(resType); err != nil {
		logs.Errorf("set res sync status success failed, res: %s, err: %v, accountID: %s, rid: %s",
			resType, err, sd.AccountID, kt.Rid)
		return err
	}

	return nil
}
