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

package sync

import (
	actcli "hcm/cmd/task-server/logics/action/cli"
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
)

// markAdjustmentPushed 在整月全部分页插入成功后统一回写推送状态。
// 分两步：本次处理的 ID 集合置 pushed；该账期内其余 pushing 残留兜底置回 unpushed，
// 保证一次整月推送结束后账期内不存在 pushing 残留。
func markAdjustmentPushed(kt *kit.Kit, opt *AdjustmentOption, pushedIDs []string) error {
	for _, batch := range slice.Split(pushedIDs, constant.BatchOperationMaxLimit) {
		req := &dsbill.BillAdjustmentItemStateUpdateReq{
			Filter:         tools.ContainersExpression("id", batch),
			PushStatus:     enumor.BillAdjustmentPushStatusPushed,
			PushFailReason: cvt.ValToPtr(""),
		}
		if err := actcli.GetDataService().Global.Bill.BatchUpdateBillAdjustmentItemState(kt, req); err != nil {
			logs.Errorf("mark adjustment pushed failed, err: %v, vendor: %s, period: %d-%02d, rid: %s",
				err, opt.Vendor, opt.BillYear, opt.BillMonth, kt.Rid)
			return err
		}
	}

	resetReq := &dsbill.BillAdjustmentItemStateUpdateReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("vendor", opt.Vendor),
			tools.RuleEqual("bill_year", opt.BillYear),
			tools.RuleEqual("bill_month", opt.BillMonth),
			tools.RuleEqual("push_status", enumor.BillAdjustmentPushStatusPushing),
		),
		PushStatus:     enumor.BillAdjustmentPushStatusUnpushed,
		PushFailReason: cvt.ValToPtr(""),
	}
	if err := actcli.GetDataService().Global.Bill.BatchUpdateBillAdjustmentItemState(kt, resetReq); err != nil {
		logs.Errorf("reset residual pushing adjustment failed, err: %v, vendor: %s, period: %d-%02d, rid: %s",
			err, opt.Vendor, opt.BillYear, opt.BillMonth, kt.Rid)
		return err
	}

	logs.Infof("mark adjustment pushed success, vendor: %s, period: %d-%02d, pushed_num: %d, rid: %s",
		opt.Vendor, opt.BillYear, opt.BillMonth, len(pushedIDs), kt.Rid)

	return nil
}
