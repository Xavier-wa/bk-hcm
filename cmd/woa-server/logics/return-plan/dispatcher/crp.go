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

package dispatcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	tablerst "hcm/pkg/dal/table/return-plan/return-plan-sub-ticket"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/cvmapi"
)

// submitCrpOrder 按子单类型向 CRP 提单，返回 CRP 单号与详情链接。
// 拆单已按资源池维度拆分，一个子单恒对应一个 CRP 单据。
// add → submitAppendOrder；cancel/adjust → submitAdjustOrderForApi(删除模式 update 为空数组)。
func (d *Dispatcher) submitCrpOrder(kt *kit.Kit, subTicket *tablerst.ReturnPlanSubTicketTable) (
	string, string, error) {

	details, err := parseSubDetails(subTicket)
	if err != nil {
		logs.Errorf("failed to parse return plan sub details, err: %v, sub_ticket_id: %s, rid: %s",
			err, subTicket.ID, kt.Rid)
		return "", "", err
	}
	if len(details) == 0 {
		return "", "", errors.New("return plan sub ticket has no detail")
	}

	switch subTicket.SubType {
	case enumor.ReturnPlanTicketTypeAdd:
		return d.submitAppendOrder(kt, subTicket, details)
	case enumor.ReturnPlanTicketTypeCancel, enumor.ReturnPlanTicketTypeAdjust:
		return d.submitAdjustOrder(kt, subTicket, details)
	default:
		return "", "", fmt.Errorf("unsupported return plan sub ticket type: %s", subTicket.SubType)
	}
}

// submitAppendOrder 提交退回计划新增(追加)单。拆单已保证单子单单资源池，CRP 应只返回一个单号。
func (d *Dispatcher) submitAppendOrder(kt *kit.Kit, subTicket *tablerst.ReturnPlanSubTicketTable,
	details tablert.ReturnPlanDetails) (string, string, error) {

	orderDetails := make([]*cvmapi.SubmitReturnOrderDetail, 0, len(details))
	for _, det := range details {
		item := det.Updated
		if item == nil {
			return "", "", errors.New("add detail missing updated item")
		}
		orderDetails = append(orderDetails, &cvmapi.SubmitReturnOrderDetail{
			ProductName:      subTicket.OpProductName,
			PlanTime:         item.PlanTime,
			ResourcePoolName: item.ResourcePoolName,
			CityName:         item.City,
			ZoneName:         item.Zone,
			InstanceModel:    item.InstanceModel,
			CvmAmount:        item.CvmAmount,
			CoreTypeName:     item.CoreTypeName,
			InstanceType:     item.InstanceType,
			CoreAmount:       item.CoreAmount,
			Desc:             item.Desc,
		})
	}

	param := &cvmapi.SubmitAppendReturnOrderParam{
		UserName:          subTicket.Creator,
		DeptName:          subTicket.VirtualDeptName,
		PlanProductName:   subTicket.PlanProductName,
		ProjectName:       subTicket.ObsProject,
		ReturnReasonClass: reasonClass(details),
		Details:           orderDetails,
	}
	req := cvmapi.NewSubmitAppendReturnOrderReq(param)
	resp, err := d.crpCli.SubmitAppendReturnOrder(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("failed to submit append return order, err: %v, sub_ticket_id: %s, rid: %s",
			err, subTicket.ID, kt.Rid)
		return "", "", err
	}

	orderIDs := make([]string, 0, len(resp.Result))
	for _, item := range resp.Result {
		if item == nil {
			continue
		}
		if item.OrderId != "" {
			orderIDs = append(orderIDs, item.OrderId)
		}
	}
	if len(orderIDs) == 0 {
		logs.Errorf("crp submit append order returned no order id, sub_ticket_id: %s, trace_id: %s, rid: %s",
			subTicket.ID, resp.TraceId, kt.Rid)
		return "", "", errors.New("crp submit append order returned no order id")
	}
	// 拆单已按资源池维度保证单子单单资源池，CRP 预期只返回一个单号；多于一个视为异常
	if len(orderIDs) > 1 {
		logs.Errorf("crp submit append order returned multiple order ids: %v, sub_ticket_id: %s, "+
			"trace_id: %s, rid: %s", orderIDs, subTicket.ID, resp.TraceId, kt.Rid)
		return "", "", fmt.Errorf("crp submit append order returned multiple order ids: %v", orderIDs)
	}

	return orderIDs[0], buildCrpURL(orderIDs[0]), nil
}

// submitAdjustOrder 提交退回计划调整&删除单：src 取 original.crp_plan_id；删除模式 update 为空数组、调整模式一一对应。
func (d *Dispatcher) submitAdjustOrder(kt *kit.Kit, subTicket *tablerst.ReturnPlanSubTicketTable,
	details tablert.ReturnPlanDetails) (string, string, error) {

	src := make([]*cvmapi.AdjustReturnOrderSrc, 0, len(details))
	update := make([]*cvmapi.AdjustReturnOrderUpdate, 0, len(details))
	for _, det := range details {
		if det.Original == nil {
			return "", "", errors.New("cancel/adjust detail missing original item")
		}
		src = append(src, &cvmapi.AdjustReturnOrderSrc{ID: det.Original.CrpPlanID})

		// 调整模式：src 与 update 的 id 一一对应；删除模式(无 updated)则不追加 update
		if det.Updated != nil {
			update = append(update, &cvmapi.AdjustReturnOrderUpdate{
				ID:               det.Original.CrpPlanID,
				ProductName:      subTicket.OpProductName,
				PlanTime:         det.Updated.PlanTime,
				CityName:         det.Updated.City,
				ZoneName:         det.Updated.Zone,
				ResourcePoolName: det.Updated.ResourcePoolName,
				InstanceModel:    det.Updated.InstanceModel,
				CvmAmount:        det.Updated.CvmAmount,
				CoreTypeName:     det.Updated.CoreTypeName,
				CoreAmount:       det.Updated.CoreAmount,
				InstanceType:     det.Updated.InstanceType,
				Desc:             det.Updated.Desc,
			})
		}
	}

	param := &cvmapi.SubmitAdjustReturnOrderParam{
		UserName:          subTicket.Creator,
		ReturnReasonClass: reasonClass(details),
		Src:               src,
		Update:            update,
	}
	req := cvmapi.NewSubmitAdjustReturnOrderReq(param)
	resp, err := d.crpCli.SubmitAdjustReturnOrderForApi(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("failed to submit adjust return order, err: %v, sub_ticket_id: %s, rid: %s",
			err, subTicket.ID, kt.Rid)
		return "", "", err
	}
	if resp.Result == nil || resp.Result.OrderId == "" {
		logs.Errorf("crp submit adjust order returned no order id, sub_ticket_id: %s, trace_id: %s, rid: %s",
			subTicket.ID, resp.TraceId, kt.Rid)
		return "", "", errors.New("crp submit adjust order returned no order id")
	}

	return resp.Result.OrderId, buildCrpURL(resp.Result.OrderId), nil
}

// pollCrpOrder 轮询子单对应的 CRP 单据状态；返回子单终态与失败原因，
// 若尚未达终态(仍在审批中)则返回空状态、空原因、nil error，由上层继续轮询。
// 一个子单恒对应一个 CRP 单号。
func (d *Dispatcher) pollCrpOrder(kt *kit.Kit, subTicket *tablerst.ReturnPlanSubTicketTable) (
	enumor.ReturnPlanSubTicketStatus, string, error) {

	orderID := strings.TrimSpace(subTicket.CrpSN)
	if orderID == "" {
		return "", "", errors.New("return plan sub ticket has empty crp sn")
	}

	param := &cvmapi.QueryReturnOrderDetailParam{UserName: subTicket.Creator, OrderId: orderID}
	req := cvmapi.NewQueryReturnOrderDetailReq(param)
	resp, err := d.crpCli.QueryReturnOrderDetail(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("failed to query return order detail, err: %v, order_id: %s, rid: %s",
			err, orderID, kt.Rid)
		return "", "", err
	}
	if resp.Result == nil {
		logs.Errorf("crp query return order detail returned empty result, order_id: %s, trace_id: %s, rid: %s",
			orderID, resp.TraceId, kt.Rid)
		return "", "", fmt.Errorf("crp query order detail returned empty result, order_id: %s", orderID)
	}

	st := mapReturnOrderStatus(resp.Result.Status)
	if st == "" {
		// 仍在审批中，继续轮询
		return "", "", nil
	}
	msg := ""
	if st != enumor.ReturnPlanSubTicketStatusDone {
		msg = resp.Result.StatusMsg
	}
	return st, msg, nil
}

// mapReturnOrderStatus 将 CRP 退回计划订单状态码映射为子单状态；非终态返回空串表示继续轮询。
func mapReturnOrderStatus(status enumor.ReturnPlanOrderStatus) enumor.ReturnPlanSubTicketStatus {
	switch status {
	case enumor.ReturnPlanOrderStatusFinished:
		return enumor.ReturnPlanSubTicketStatusDone
	case enumor.ReturnPlanOrderStatusRejected:
		return enumor.ReturnPlanSubTicketStatusRejected
	default:
		// 0 待提交 / 1 资源团队审批 / 2 部门管理员审批 → 继续轮询
		return ""
	}
}

// parseSubDetails 解析子单 sub_details JSON。
func parseSubDetails(subTicket *tablerst.ReturnPlanSubTicketTable) (tablert.ReturnPlanDetails, error) {
	if subTicket.SubDetails.IsEmpty() {
		return nil, nil
	}
	var details tablert.ReturnPlanDetails
	if err := json.Unmarshal([]byte(subTicket.SubDetails), &details); err != nil {
		return nil, err
	}
	return details, nil
}

// reasonClass 取明细中的退回原因大类，缺省用默认原因大类常量。
func reasonClass(details tablert.ReturnPlanDetails) string {
	for _, det := range details {
		if det.Updated != nil && det.Updated.ReturnReasonClass != "" {
			return det.Updated.ReturnReasonClass
		}
		if det.Original != nil && det.Original.ReturnReasonClass != "" {
			return det.Original.ReturnReasonClass
		}
	}
	return constant.DefaultReturnReasonClass
}

// buildCrpURL 构造 CRP 单据详情链接。
// TODO: 退回计划订单详情链接前缀待真实环境确认，暂用退回单据链接前缀。
func buildCrpURL(orderID string) string {
	if orderID == "" {
		return ""
	}
	return cvmapi.CvmReturnPlanLinkPrefix + orderID
}
