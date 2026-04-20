/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2025 THL A29 Limited,
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

package ziyan

import (
	"strconv"

	ds "hcm/pkg/api/data-service"
	hc "hcm/pkg/api/hc-service"
	dataservice "hcm/pkg/client/data-service"
	hcservice "hcm/pkg/client/hc-service"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"

	"github.com/tidwall/gjson"
)

const (
	// BPaasApprovalStatusPass 1 审批通过
	BPaasApprovalStatusPass = "1"
	// BPaasApprovalStatusReject 2 拒绝
	BPaasApprovalStatusReject = "2"
	// BPaasApprovalStatusPending 0 待审批
	BPaasApprovalStatusPending = "0"
)

// CheckAndUpdateBPaasStatus 检查给定BPaas单是否被审批，若被审批则更新状态。理论上审批到终态后，其内内容不会再被修改，因此该函数是可重入的。
func CheckAndUpdateBPaasStatus(kt *kit.Kit, dsCli *dataservice.Client, hcCli *hcservice.Client,
	app *ds.ApplicationResp) error {

	if app.Status != enumor.Pending {
		// 非待审批状态，直接返回
		return nil
	}

	sn, err := strconv.ParseUint(app.SN, 10, 64)
	if err != nil {
		logs.Errorf("parse application sn failed, err: %v, application: %+v, rid: %s", err, app, kt.Rid)
		return err
	}
	accountID := gjson.Get(app.Content, "account_id").String()
	req := &hc.GetBPaasApplicationReq{
		BPaasSN:   sn,
		AccountID: accountID,
	}
	detail, err := hcCli.TCloudZiyan.Application.QueryBPaasApplicationDetail(kt, req)
	if err != nil {
		logs.Errorf("query bpaas application detail failed, err: %v, sn: %d, application id: %s, rid: %s",
			err, sn, app.ID, kt.Rid)
		return err
	}
	detailStr := string(cvt.PtrToVal(detail))
	statusStr := gjson.Get(detailStr, "Status").String()
	switch statusStr {
	case BPaasApprovalStatusReject:
		// 2 拒绝
		updateReq := &ds.ApplicationUpdateReq{
			Status:         enumor.Rejected,
			DeliveryDetail: cvt.ValToPtr(detailStr),
		}
		_, err := dsCli.Global.Application.UpdateApplication(kt, app.ID, updateReq)
		if err != nil {
			logs.Errorf("fail to update bpaas application to failed, err: %v, application id: %s, rid: %s",
				err, app.ID, kt.Rid)
			return err
		}
		logs.Infof("update bpaas application to rejected, application id: %s, bpaas sn: %d, rid: %s",
			app.ID, sn, kt.Rid)
	case BPaasApprovalStatusPass:
		// 1 审批通过;
		deliverReq := &hc.DeliverBPaasApplicationReq{
			Content: app.Content,
		}
		if err := hcCli.TCloudZiyan.Application.DeliverBPaasApplication(kt, deliverReq); err != nil {
			logs.Errorf("fail to deliver bpaas application after approval, err: %v, application id: %s, rid: %s",
				err, app.ID, kt.Rid)
			return err
		}
		logs.Infof("deliver bpaas application success, application id: %s, bpaas sn: %d, rid: %s",
			app.ID, sn, kt.Rid)

		updateReq := &ds.ApplicationUpdateReq{
			Status:         enumor.Pass,
			DeliveryDetail: cvt.ValToPtr(detailStr),
		}
		_, err := dsCli.Global.Application.UpdateApplication(kt, app.ID, updateReq)
		if err != nil {
			logs.Errorf("fail to update bpaas application to pass, err: %v, application id: %s, rid: %s",
				err, app.ID, kt.Rid)
			return err
		}
		logs.Infof("update bpaas application to pass, application id: %s, bpaas sn: %d, rid: %s",
			app.ID, sn, kt.Rid)
	default:
		// 其他状态，不处理
	}
	return nil
}
