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

package tcloud

import (
	"fmt"
	"strconv"

	typeaccount "hcm/pkg/adaptor/types/account"
	"hcm/pkg/api/core"
	corecloud "hcm/pkg/api/core/cloud"
	protocloud "hcm/pkg/api/data-service/cloud"
	dssubaccount "hcm/pkg/api/data-service/cloud/sub-account"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/assert"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
)

// SyncSubAccountPermissionTmplOption defines options for syncing subaccount permission templates.
type SyncSubAccountPermissionTmplOption struct {
	AccountID string `json:"account_id" validate:"required"`
}

// Validate SyncSubAccountPermissionTmplOption.
func (opt SyncSubAccountPermissionTmplOption) Validate() error {
	return validator.Validate.Struct(opt)
}

// SubAccountPermissionTemplate 同步指定账号下所有子账号绑定的权限模板信息。
func (cli *client) SubAccountPermissionTemplate(kt *kit.Kit, opt *SyncSubAccountPermissionTmplOption) (
	*SyncResult, error) {

	if err := opt.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	subAccounts, err := cli.listSubAccountFromDB(kt, &SyncSubAccountOption{AccountID: opt.AccountID})
	if err != nil {
		return nil, err
	}

	if len(subAccounts) == 0 {
		logs.Infof("[%s] sync sub account permission template: no sub accounts found, accountID: %s, rid: %s",
			enumor.TCloud, opt.AccountID, kt.Rid)
		return new(SyncResult), nil
	}

	cloudIDToLocalID, err := cli.buildPermissionTmplCloudIDMap(kt, opt.AccountID)
	if err != nil {
		return nil, err
	}

	updateItems := make([]dssubaccount.UpdateField, 0, len(subAccounts))
	for _, subAccount := range subAccounts {
		if subAccount.AccountType == string(enumor.MainAccount) {
			continue
		}

		if subAccount.Extension == nil || subAccount.Extension.Uin == nil {
			logs.Errorf("[%s] sync sub account(%s) failed, extension has no uin, err: %v, rid: %s",
				enumor.TCloud, subAccount.ID, err, kt.Rid)
			return nil, errf.NewFromErr(errf.InvalidParameter,
				fmt.Errorf("sub account %s has no uin", subAccount.ID))
		}

		templateIDs, updatedCloudIDToLocalID, err := cli.listSubAccountPermissionTemplateIDs(
			kt, converter.PtrToVal(subAccount.Extension.Uin), opt.AccountID, cloudIDToLocalID)
		if err != nil {
			logs.Errorf("[%s] list sub account(%s) permission template ids failed, err: %v, rid: %s",
				enumor.TCloud, subAccount.ID, err, kt.Rid)
			return nil, err
		}
		cloudIDToLocalID = updatedCloudIDToLocalID

		if assert.IsStringSliceEqual(subAccount.PermissionTemplateIDs, templateIDs) {
			continue
		}

		updateItems = append(updateItems, dssubaccount.UpdateField{
			ID:                    subAccount.ID,
			PermissionTemplateIDs: templateIDs,
		})
	}

	if len(updateItems) > 0 {
		updateReq := &dssubaccount.UpdateReq{Items: updateItems}
		if err = cli.dbCli.Global.SubAccount.BatchUpdate(kt, updateReq); err != nil {
			logs.Errorf("[%s] batch update sub account permission template ids failed, err: %v, rid: %s",
				enumor.TCloud, err, kt.Rid)
			return nil, err
		}
	}

	logs.Infof("[%s] sync sub account permission template done, accountID: %s, total: %d, rid: %s",
		enumor.TCloud, opt.AccountID, len(subAccounts), kt.Rid)

	return new(SyncResult), nil
}

// listSubAccountPermissionTemplateIDs 获取单个子账号绑定的所有本地权限模板 ID 列表。
// 若某条云上策略在本地不存在，则自动调用 GetPolicyDetail 拉取详情并批量补录到 DB。
func (cli *client) listSubAccountPermissionTemplateIDs(kt *kit.Kit, uin uint64, accountID string,
	cloudIDToLocalID map[string]string) ([]string, map[string]string, error) {

	const pageSize = uint64(100)
	templateIDs := make([]string, 0)
	// 记录是否云上绑定了三级账号但是未同步下来的模版
	missingPolicies := make([]typeaccount.TCloudAttachedPolicy, 0)

	// page是1开始的
	for page := uint64(1); ; page++ {
		result, err := cli.cloudCli.ListAttachedUserAllPolicies(kt,
			&typeaccount.TCloudListAttachedUserAllPoliciesOption{
				TargetUin: uin,
				Page:      page,
				Rp:        pageSize,
				// 直接关联和组关联都返回，所以 attach_type 传 0
				AttachType: converter.ValToPtr(uint64(0)),
			})
		if err != nil {
			return nil, nil, err
		}

		for _, policy := range result.PolicyList {
			localID, ok := cloudIDToLocalID[policy.PolicyID]
			if !ok {
				logs.Warnf("[%s] policy cloud_id %s of sub-account(uin: %d) not found locally, "+
					"will create it, rid: %s", enumor.TCloud, policy.PolicyID, uin, kt.Rid)
				missingPolicies = append(missingPolicies, policy)
				continue
			}
			templateIDs = append(templateIDs, localID)
		}

		if uint64(len(result.PolicyList)) < pageSize {
			break
		}
	}

	if len(missingPolicies) > 0 {
		newTmplIDs, updatedCloudIDToLocalID, err := cli.createMissingPermissionTemplates(kt, accountID,
			missingPolicies, cloudIDToLocalID)
		if err != nil {
			logs.Errorf("[%s] create missing permission templates failed, account: %s, err: %v, rid: %s",
				enumor.TCloud, accountID, err, kt.Rid)
			return nil, nil, err
		}

		templateIDs = append(templateIDs, newTmplIDs...)
		cloudIDToLocalID = updatedCloudIDToLocalID
	}

	return slice.Unique(templateIDs), cloudIDToLocalID, nil
}

// createMissingPermissionTemplates fetches full policy details for missing policies and batch-creates them in DB.
// After creation, it re-queries the DB to get accurate cloud_id → local_id mappings and updates cloudIDToLocalID
// in place, so subsequent sub-accounts sharing the same policies find them in the map and avoid duplicate inserts.
// Returns the cloud_id → local_id mappings of the newly created templates.
func (cli *client) createMissingPermissionTemplates(kt *kit.Kit, accountID string,
	missing []typeaccount.TCloudAttachedPolicy, cloudIDToLocalID map[string]string) (
	[]string, map[string]string, error) {

	items := make([]protocloud.PermissionTemplateCreate[corecloud.TCloudPermissionTemplateExtension], 0, len(missing))
	for _, policy := range missing {
		policyIDUint, err := strconv.ParseUint(policy.PolicyID, 10, 64)
		if err != nil {
			logs.Errorf("[%s] parse policy_id %s to uint64 failed, err: %v, rid: %s",
				enumor.TCloud, policy.PolicyID, err, kt.Rid)
			return nil, nil, fmt.Errorf("parse policy_id %s failed, err: %v", policy.PolicyID, err)
		}

		detail, err := cli.cloudCli.GetPolicyDetail(kt, &typeaccount.TCloudGetPolicyDetailOption{
			PolicyID: policyIDUint,
		})
		if err != nil {
			logs.Errorf("[%s] get policy detail for cloud_id %s failed, err: %v, rid: %s",
				enumor.TCloud, policy.PolicyID, err, kt.Rid)
			return nil, nil, err
		}

		memo := detail.Description
		items = append(items, protocloud.PermissionTemplateCreate[corecloud.TCloudPermissionTemplateExtension]{
			CloudID:        detail.GetCloudID(),
			Name:           detail.PolicyName,
			AccountID:      accountID,
			PolicyDocument: detail.PolicyDocument,
			Memo:           converter.ValToPtr(memo),
			Extension: &corecloud.TCloudPermissionTemplateExtension{
				CloudType: detail.PolicyType,
			},
		})
	}

	batches := slice.Split(items, constant.CloudResourceSyncMaxLimit)
	for _, batch := range batches {
		createReq := &protocloud.PermissionTemplateBatchCreateReq[corecloud.TCloudPermissionTemplateExtension]{
			PermissionTemplates: batch,
		}
		if _, err := cli.dbCli.TCloud.PermissionTemplate.BatchCreate(kt, createReq); err != nil {
			logs.Errorf("[%s] batch create missing permission templates failed, account: %s, err: %v, rid: %s",
				enumor.TCloud, accountID, err, kt.Rid)
			return nil, nil, err
		}
	}

	// Re-query DB with the newly created cloud IDs to get accurate cloud_id → local_id mappings.
	missingCloudIDs := make([]string, 0, len(missing))
	for _, p := range missing {
		missingCloudIDs = append(missingCloudIDs, p.PolicyID)
	}

	// 返回新增的本地模版ID列表和更新后的 cloud_id → local_id 映射表
	newTmplIDs, updatedCloudIDToLocalID, err := cli.queryAndUpdateCloudIDMap(kt, accountID,
		missingCloudIDs, cloudIDToLocalID)
	if err != nil {
		logs.Errorf("[%s] query and update cloud_id to local_id failed, account: %s, err: %v, rid: %s",
			enumor.TCloud, accountID, err, kt.Rid)
		return nil, nil, err
	}

	logs.Infof("[%s] created missing permission templates success, account: %s, now total count: %d, rid: %s",
		enumor.TCloud, accountID, len(updatedCloudIDToLocalID), kt.Rid)

	return newTmplIDs, updatedCloudIDToLocalID, nil
}

// queryAndUpdateCloudIDMap queries local DB for the given cloud IDs, updates cloudIDToLocalID in place,
// and returns the local IDs corresponding to the given cloud IDs.
func (cli *client) queryAndUpdateCloudIDMap(kt *kit.Kit, accountID string, cloudIDs []string,
	cloudIDToLocalID map[string]string) ([]string, map[string]string, error) {

	// Batch the IN clause to respect the query limit.
	// 查询出来补入的模版本地数据，并更新 cloudIDToLocalID
	idBatches := slice.Split(cloudIDs, int(core.DefaultMaxPageLimit))
	newTmplIDs := make([]string, 0, len(cloudIDs))
	for _, batch := range idBatches {
		req := &protocloud.PermissionTemplateExtListReq{
			Filter: tools.ExpressionAnd(
				tools.RuleEqual("vendor", enumor.TCloud),
				tools.RuleEqual("account_id", accountID),
				tools.RuleIn("cloud_id", batch)),
			Page: core.NewDefaultBasePage(),
		}
		resp, err := cli.dbCli.TCloud.PermissionTemplate.ListPermissionTemplateExt(kt, req)
		if err != nil {
			logs.Errorf("[%s] re-query permission templates from db failed, account: %s, err: %v, rid: %s",
				enumor.TCloud, accountID, err, kt.Rid)
			return nil, nil, err
		}

		for _, tmpl := range resp.Details {
			cloudIDToLocalID[tmpl.CloudID] = tmpl.ID
			newTmplIDs = append(newTmplIDs, tmpl.ID)
		}
	}

	return newTmplIDs, cloudIDToLocalID, nil
}

// buildPermissionTmplCloudIDMap 构建 cloud_id → local_id 的映射表。
func (cli *client) buildPermissionTmplCloudIDMap(kt *kit.Kit, accountID string) (map[string]string, error) {
	req := &protocloud.PermissionTemplateExtListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("vendor", enumor.TCloud),
			tools.RuleEqual("account_id", accountID)),
		Page: core.NewDefaultBasePage(),
	}

	cloudIDToLocalID := make(map[string]string)
	start := uint32(0)
	for {
		req.Page.Start = start
		resp, err := cli.dbCli.TCloud.PermissionTemplate.ListPermissionTemplateExt(kt, req)
		if err != nil {
			logs.Errorf("[%s] list permission template from db failed, accountID: %s, err: %v, rid: %s",
				enumor.TCloud, accountID, err, kt.Rid)
			return nil, err
		}

		for _, tmpl := range resp.Details {
			cloudIDToLocalID[tmpl.CloudID] = tmpl.ID
		}

		if len(resp.Details) < int(core.DefaultMaxPageLimit) {
			break
		}
		start += uint32(core.DefaultMaxPageLimit)
	}

	return cloudIDToLocalID, nil
}
