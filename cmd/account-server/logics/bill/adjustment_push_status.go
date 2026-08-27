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

package bill

import (
	billcore "hcm/pkg/api/core/bill"
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
)

// markAdjustmentPushing 把本账期已确认的调账整体置为推送中。
// 只置位 state=confirmed 的条目：未确认的调账本轮不会被推送，置位会让它被写入域的推送中闸门误拦。
func (sc *SyncController) markAdjustmentPushing(kt *kit.Kit, record *billcore.SyncRecord) error {
	req := &dsbill.BillAdjustmentItemStateUpdateReq{
		Filter: buildAdjustmentPeriodFilter(record,
			tools.RuleEqual("state", enumor.BillAdjustmentStateConfirmed)),
		PushStatus:     enumor.BillAdjustmentPushStatusPushing,
		PushFailReason: cvt.ValToPtr(""),
	}
	if err := sc.Client.DataService().Global.Bill.BatchUpdateBillAdjustmentItemState(kt, req); err != nil {
		logs.Errorf("mark adjustment pushing failed, err: %v, vendor: %s, period: %d-%02d, rid: %s",
			err, record.Vendor, record.BillYear, record.BillMonth, kt.Rid)
		return err
	}

	logs.Infof("mark adjustment pushing success, vendor: %s, period: %d-%02d, rid: %s",
		record.Vendor, record.BillYear, record.BillMonth, kt.Rid)

	return nil
}

// markAdjustmentPushFailed 把本账期仍处于推送中的调账落为失败态并写入失败原因。
// 只影响 pushing 条目：已 pushed 的说明本轮之前已成功写入，不应被回退。
func (sc *SyncController) markAdjustmentPushFailed(kt *kit.Kit, record *billcore.SyncRecord, reason string) error {
	req := &dsbill.BillAdjustmentItemStateUpdateReq{
		Filter: buildAdjustmentPeriodFilter(record,
			tools.RuleEqual("push_status", enumor.BillAdjustmentPushStatusPushing)),
		PushStatus:     enumor.BillAdjustmentPushStatusFailed,
		PushFailReason: cvt.ValToPtr(truncatePushFailReason(reason)),
	}
	if err := sc.Client.DataService().Global.Bill.BatchUpdateBillAdjustmentItemState(kt, req); err != nil {
		logs.Errorf("mark adjustment push failed status failed, err: %v, vendor: %s, period: %d-%02d, rid: %s",
			err, record.Vendor, record.BillYear, record.BillMonth, kt.Rid)
		return err
	}

	logs.Warnf("mark adjustment push failed, vendor: %s, period: %d-%02d, reason: %s, rid: %s",
		record.Vendor, record.BillYear, record.BillMonth, reason, kt.Rid)

	return nil
}

// buildAdjustmentPeriodFilter 按 (vendor, 账期) 加附加规则组装调账过滤条件。
func buildAdjustmentPeriodFilter(record *billcore.SyncRecord, extra ...*filter.AtomRule) *filter.Expression {
	rules := []*filter.AtomRule{
		tools.RuleEqual("vendor", record.Vendor),
		tools.RuleEqual("bill_year", record.BillYear),
		tools.RuleEqual("bill_month", record.BillMonth),
	}
	rules = append(rules, extra...)

	return tools.ExpressionAnd(rules...)
}

// pushFailReasonMaxLen push_fail_reason 列长度上限，超长截断避免写入失败。
const pushFailReasonMaxLen = 255

// truncatePushFailReason 截断超长的推送失败原因。
func truncatePushFailReason(reason string) string {
	if len(reason) <= pushFailReasonMaxLen {
		return reason
	}

	return reason[:pushFailReasonMaxLen]
}
