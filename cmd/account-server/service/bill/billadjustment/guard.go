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

package billadjustment

import (
	"strings"

	"hcm/pkg/api/core"
	billcore "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/slice"
)

// guardFields 守卫判定所需的字段集合，一次读取供来源守卫与状态守卫共用，不新增额外查询轮次。
var guardFields = []string{"id", "state", "vendor", "res_class", "res_sub_class",
	"source", "source_id", "push_status", "settle_state"}

// loadAdjustmentForGuard 读取待操作的调账记录，条数不匹配说明存在不存在的 ID，直接整批拒绝。
func (b *billAdjustmentSvc) loadAdjustmentForGuard(cts *rest.Contexts, ids []string) (
	[]*billcore.AdjustmentItem, error) {

	items := make([]*billcore.AdjustmentItem, 0, len(ids))
	for _, batch := range slice.Split(ids, int(filter.DefaultMaxInLimit)) {
		listReq := &core.ListReq{
			Filter: tools.ContainersExpression("id", batch),
			Page:   core.NewDefaultBasePage(),
			Fields: guardFields,
		}
		itemResp, err := b.client.DataService().Global.Bill.ListBillAdjustmentItem(cts.Kit, listReq)
		if err != nil {
			logs.Errorf("query bill adjustment for guard failed, err: %v, ids: %v, rid: %s",
				err, batch, cts.Kit.Rid)
			return nil, err
		}
		items = append(items, itemResp.Details...)
	}
	if len(items) != len(ids) {
		return nil, errf.New(errf.RecordNotFound, "item not found")
	}

	return items, nil
}

// guardSourceManual 来源守卫：预付费派生的调账禁止在 HCM 侧编辑或删除。
func guardSourceManual(items []*billcore.AdjustmentItem) error {
	rejected := collectRejected(items, func(item *billcore.AdjustmentItem) bool {
		return item.Source == enumor.BillAdjustmentSourcePrepaid
	})
	if len(rejected) == 0 {
		return nil
	}

	return errf.New(errf.InvalidParameter,
		"prepaid items can not be modified, ids: "+strings.Join(rejected, ","))
}

// guardNotPushingOrSettled 状态守卫：推送中的条目改动会与同步流程冲突，已定账的条目单向不可逆。
func guardNotPushingOrSettled(items []*billcore.AdjustmentItem) error {
	pushing := collectRejected(items, func(item *billcore.AdjustmentItem) bool {
		return item.PushStatus == enumor.BillAdjustmentPushStatusPushing
	})
	if len(pushing) > 0 {
		return errf.New(errf.InvalidParameter,
			"pushing items can not be modified, ids: "+strings.Join(pushing, ","))
	}

	settled := collectRejected(items, func(item *billcore.AdjustmentItem) bool {
		return item.SettleState == enumor.BillSettleStateSettled
	})
	if len(settled) > 0 {
		return errf.New(errf.InvalidParameter,
			"settled items can not be modified, ids: "+strings.Join(settled, ","))
	}

	return nil
}

// guardNotConfirmed 已确认守卫：仅批量确认路径使用，防止重复确认。
func guardNotConfirmed(items []*billcore.AdjustmentItem) error {
	confirmed := collectRejected(items, func(item *billcore.AdjustmentItem) bool {
		return item.State == enumor.BillAdjustmentStateConfirmed
	})
	if len(confirmed) == 0 {
		return nil
	}

	return errf.New(errf.InvalidParameter,
		"confirmed items can not be modified, ids: "+strings.Join(confirmed, ","))
}

// collectRejected 收集命中拒绝条件的条目 ID。
func collectRejected(items []*billcore.AdjustmentItem, reject func(*billcore.AdjustmentItem) bool) []string {
	rejected := make([]string, 0)
	for _, item := range items {
		if reject(item) {
			rejected = append(rejected, item.ID)
		}
	}

	return rejected
}

// checkAdjustmentEditable 编辑与删除路径的守卫组合：来源 + 状态。
func (b *billAdjustmentSvc) checkAdjustmentEditable(cts *rest.Contexts, ids []string) (
	[]*billcore.AdjustmentItem, error) {

	items, err := b.loadAdjustmentForGuard(cts, ids)
	if err != nil {
		return nil, err
	}

	if err = guardSourceManual(items); err != nil {
		return nil, err
	}
	if err = guardNotPushingOrSettled(items); err != nil {
		return nil, err
	}

	return items, nil
}

// checkAdjustmentConfirmable 批量确认路径的守卫组合：来源 + 已确认。
func (b *billAdjustmentSvc) checkAdjustmentConfirmable(cts *rest.Contexts, ids []string) (
	[]*billcore.AdjustmentItem, error) {

	items, err := b.loadAdjustmentForGuard(cts, ids)
	if err != nil {
		return nil, err
	}

	if err = guardSourceManual(items); err != nil {
		return nil, err
	}
	if err = guardNotConfirmed(items); err != nil {
		return nil, err
	}

	return items, nil
}
