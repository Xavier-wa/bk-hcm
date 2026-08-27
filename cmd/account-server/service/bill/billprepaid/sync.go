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

package billprepaid

import (
	asbill "hcm/pkg/api/account-server/bill"
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// SyncBillPrepaidItem 接收推送的预付费账单，逐单落库为主单 + N+1 条调账。
// 逐单独立事务、互不影响：某单失败只在该单结果里记原因并继续处理后续单据，接口整体仍返回成功。
func (b *billPrepaidSvc) SyncBillPrepaidItem(cts *rest.Contexts) (interface{}, error) {
	req := new(asbill.PrepaidItemSyncReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	resp := &asbill.PrepaidItemSyncResp{Results: make([]asbill.PrepaidItemSyncResult, 0, len(req.Items))}
	succeedNum := 0
	for idx := range req.Items {
		result := b.syncOneItem(cts.Kit, &req.Items[idx])
		if result.Success {
			succeedNum++
		}
		resp.Results = append(resp.Results, result)
	}

	logs.Infof("sync prepaid item finished, total: %d, succeed: %d, rid: %s", len(req.Items), succeedNum, cts.Kit.Rid)

	return resp, nil
}

// syncOneItem 处理单个预付费订单并把错误收敛为该单的结果项，不向上抛错以免中断整批。
func (b *billPrepaidSvc) syncOneItem(kt *kit.Kit, item *asbill.PrepaidItemSyncItem) asbill.PrepaidItemSyncResult {
	result := asbill.PrepaidItemSyncResult{
		UUID:       item.UUID,
		OrderYear:  item.OrderYear,
		OrderMonth: item.OrderMonth,
	}

	id, err := b.doSyncOneItem(kt, item)
	if err != nil {
		logs.Errorf("sync prepaid item failed, err: %v, unique_key: %s, rid: %s", err, item.UniqueKey(), kt.Rid)
		result.Message = errf.Error(err).Message
		return result
	}

	result.ID, result.Success = id, true

	return result
}

// doSyncOneItem 单单处理链，顺序固定：云账号映射 → 实例级鉴权 → 业务校验 → 唯一键与双闸门 → 单事务落库。
// 映射必须早于鉴权：鉴权实例是映射出来的二级账号 ID，顺序调换会导致无实例可鉴权。
func (b *billPrepaidSvc) doSyncOneItem(kt *kit.Kit, item *asbill.PrepaidItemSyncItem) (string, error) {
	mainAccount, err := b.mapMainAccount(kt, item)
	if err != nil {
		return "", err
	}

	authRes := meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.AccountBillPrepaid, Action: meta.Create, ResourceID: mainAccount.ID},
	}
	if err = b.authorizer.AuthorizeWithPerm(kt, authRes); err != nil {
		return "", err
	}

	err = asbill.ValidatePrepaidRootAccountCloudID(item.RootAccountCloudID, mainAccount.RootAccountCloudID)
	if err != nil {
		return "", err
	}

	summaryRoot, err := b.getSummaryRoot(kt, mainAccount.RootAccountID)
	if err != nil {
		return "", err
	}
	if err = asbill.ValidatePrepaidCurrency(item.Currency, summaryRoot.Currency); err != nil {
		return "", err
	}

	exist, err := b.getPrepaidItemByUniqueKey(kt, item)
	if err != nil {
		return "", err
	}
	if err = b.checkSyncGate(kt, exist); err != nil {
		return "", err
	}

	return b.doSync(kt, item, mainAccount)
}

// doSync 组装 data-service 单事务请求并落库，返回落库后的预付费账单 ID。
func (b *billPrepaidSvc) doSync(kt *kit.Kit, item *asbill.PrepaidItemSyncItem,
	mainAccount *mappedMainAccount) (string, error) {

	dsReq := &dsbill.PrepaidItemSyncReq{
		Item:            buildPrepaidItemCreateReq(item, mainAccount),
		AdjustmentItems: buildAdjustmentItems(kt, item, mainAccount),
	}

	result, err := b.client.DataService().Global.Bill.SyncBillPrepaidItem(kt, dsReq)
	if err != nil {
		return "", err
	}

	logs.Infof("sync prepaid item success, id: %s, created: %v, adjustment_num: %d, deleted_num: %d, rid: %s",
		result.ID, result.Created, len(result.AdjustmentIDs), len(result.DeletedAdjustmentIDs), kt.Rid)

	return result.ID, nil
}
