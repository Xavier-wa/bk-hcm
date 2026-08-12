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

package returnplan

import (
	"time"

	mtypes "hcm/cmd/woa-server/types/meta"
	rptypes "hcm/cmd/woa-server/types/return-plan"
	"hcm/pkg/api/core"
	zoneproto "hcm/pkg/api/data-service/cloud/zone"
	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/cvmapi"
	cvt "hcm/pkg/tools/converter"
)

// OverwriteAppendReturnPlanTicket 覆盖追加退回计划主流程：
// 1. GetBizOrgRel 得到 部门/规划产品/运营产品，作为主单组织字段；
// 2. overwrite=true 时按 业务(→运营产品)+项目类型+技术分类+plan_time_range 查 CRP 命中退回计划，构造 cancel 条目；
// 3. return_details 构造 add 条目（默认退回原因大类、默认自研池）；
// 4. 合并 details → 创建 init 主单（Applicant 来自请求），后续由 dispatcher 自动拾取拆单提单。
// 返回主单 ID。
func (c *Controller) OverwriteAppendReturnPlanTicket(kt *kit.Kit, bkBizID int64,
	req *rptypes.OverwriteAppendReturnPlanTicketReq) (string, error) {

	// 1. 业务 → 组织维度转换。
	bizOrgRel, err := c.bizLogics.GetBizOrgRel(kt, bkBizID)
	if err != nil {
		logs.Errorf("failed to get biz org rel, err: %v, biz_id: %d, rid: %s", err, bkBizID, kt.Rid)
		return "", err
	}

	details := make(tablert.ReturnPlanDetails, 0)

	// 2. 覆盖筛选命中 CRP 退回计划，构造 cancel 条目（仅 Original）。
	if req.Overwrite {
		cancelDetails, err := c.buildOverwriteCancelDetails(kt, bizOrgRel, req)
		if err != nil {
			logs.Errorf("failed to build overwrite cancel details, err: %v, biz_id: %d, rid: %s",
				err, bkBizID, kt.Rid)
			return "", err
		}
		details = append(details, cancelDetails...)
	}

	// 3. 追加明细构造 add 条目（仅 Updated）。
	if len(req.ReturnDetails) > 0 {
		addDetails, err := c.buildAppendDetails(kt, req.ReturnDetails)
		if err != nil {
			logs.Errorf("failed to build append details, err: %v, biz_id: %d, rid: %s", err, bkBizID, kt.Rid)
			return "", err
		}
		details = append(details, addDetails...)
	}

	if len(details) == 0 {
		logs.Errorf("no return plan matched for overwrite and no details to append, biz_id: %d, rid: %s",
			bkBizID, kt.Rid)
		return "", errf.New(errf.InvalidParameter,
			"no return plan matched for overwrite and no details to append")
	}

	// 4. 创建 init 主单（Applicant 来自请求），后续由 dispatcher 自动拾取拆单提单。
	ticketID, err := c.createReturnPlanTicket(kt, bizOrgRel, req, details)
	if err != nil {
		logs.Errorf("failed to create return plan ticket, err: %v, biz_id: %d, rid: %s", err, bkBizID, kt.Rid)
		return "", err
	}

	logs.Infof("overwrite-append return plan ticket success, ticket_id: %s, biz_id: %d, detail_count: %d, rid: %s",
		ticketID, bkBizID, len(details), kt.Rid)
	return ticketID, nil
}

// buildOverwriteCancelDetails 分页查询 CRP 命中退回计划，构造 cancel 条目（仅 Original）。
func (c *Controller) buildOverwriteCancelDetails(kt *kit.Kit, bizOrgRel *mtypes.BizOrgRel,
	req *rptypes.OverwriteAppendReturnPlanTicketReq) (tablert.ReturnPlanDetails, error) {

	overwriteFilter := req.OverwriteFilter
	queryParam := &cvmapi.QueryReturnPlanParam{
		UserName:        req.Applicant,
		StartDate:       overwriteFilter.PlanTimeRange.Start,
		EndDate:         overwriteFilter.PlanTimeRange.End,
		DeptName:        []string{bizOrgRel.VirtualDeptName},
		PlanProductName: []string{bizOrgRel.PlanProductName},
		ProductName:     []string{bizOrgRel.OpProductName},
		ProjectName:     overwriteFilter.ObsProjects,
		TechnicalClass:  overwriteFilter.TechnicalClasses,
		Page: &cvmapi.Page{
			Start: 0,
			Size:  int(core.DefaultMaxPageLimit),
		},
	}

	details := make(tablert.ReturnPlanDetails, 0)
	for start := 0; ; start += int(core.DefaultMaxPageLimit) {
		queryParam.Page.Start = start
		resp, err := c.crpCli.QueryReturnPlan(kt.Ctx, kt.Header(), cvmapi.NewQueryReturnPlanReq(queryParam))
		if err != nil {
			logs.Errorf("failed to query return plan from crp, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		if resp.Result == nil {
			logs.Errorf("crp query return plan returned nil result, trace_id: %s, rid: %s", resp.TraceId, kt.Rid)
			return nil, errf.New(errf.Aborted, "crp query return plan returned nil result")
		}

		for _, item := range resp.Result.Data {
			if item == nil {
				continue
			}
			details = append(details, tablert.ReturnPlanDetail{Original: convCrpReturnPlanItem(item)})
		}

		if len(resp.Result.Data) < int(core.DefaultMaxPageLimit) {
			break
		}
	}

	return details, nil
}

// convCrpReturnPlanItem 将 CRP 退回计划项转为主单 Original 明细（用于覆盖删除定位）。
func convCrpReturnPlanItem(item *cvmapi.ReturnPlanItem) *tablert.ReturnPlanItem {
	return &tablert.ReturnPlanItem{
		CrpPlanID:     item.ID,
		ObsProject:    item.ProjectName,
		PlanTime:      item.PlanTime,
		City:          item.CityName,
		Zone:          item.ZoneName,
		InstanceModel: item.InstanceModel,
		CvmAmount:     int64(item.CvmAmount),
		InstanceType:  item.InstanceType,
		CoreTypeName:  item.CoreTypeName,
		CoreAmount:    item.CoreAmount.IntPart(),
	}
}

// buildAppendDetails 将追加明细转为主单 Updated 明细（add 条目）。
// region_id 转城市中文名（CRP 提单需中文城市名）；退回原因大类/资源池为空时填默认值。
func (c *Controller) buildAppendDetails(kt *kit.Kit, reqDetails []rptypes.AppendReturnPlanDetail) (
	tablert.ReturnPlanDetails, error) {

	regionMapPtr, err := c.client.DataService().Global.Meta.GetRegionAreaMap(kt)
	if err != nil {
		logs.Errorf("failed to get region area map, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	regionMap := cvt.PtrToVal(regionMapPtr)

	zoneNameMap, err := c.getZoneNameMap(kt)
	if err != nil {
		return nil, err
	}

	details := make(tablert.ReturnPlanDetails, 0, len(reqDetails))
	for i := range reqDetails {
		d := reqDetails[i]
		regionInfo, ok := regionMap[d.RegionID]
		if !ok {
			logs.Errorf("region not found, region_id: %s, rid: %s", d.RegionID, kt.Rid)
			return nil, errf.Newf(errf.InvalidParameter, "region not found, region_id: %s", d.RegionID)
		}

		// zone_id 转中文可用区名（CRP 提单需中文名）；zone_id 为空则留空，非空未命中则报错。
		zoneName := ""
		if d.ZoneID != "" {
			zoneName, ok = zoneNameMap[d.ZoneID]
			if !ok {
				logs.Errorf("zone not found, zone_id: %s, rid: %s", d.ZoneID, kt.Rid)
				return nil, errf.Newf(errf.InvalidParameter, "zone not found, zone_id: %s", d.ZoneID)
			}
		}

		resPoolName := d.ResourcePoolName
		if resPoolName == "" {
			resPoolName = constant.ReturnPlanResourcePoolSelfBuilt
		}
		returnReasonClass := d.ReturnReasonClass
		if returnReasonClass == "" {
			returnReasonClass = constant.DefaultReturnReasonClass
		}

		details = append(details, tablert.ReturnPlanDetail{
			Updated: &tablert.ReturnPlanItem{
				ObsProject:        d.ObsProject,
				PlanTime:          d.PlanTime,
				ResourcePoolName:  resPoolName,
				City:              regionInfo.RegionName,
				Zone:              zoneName,
				InstanceModel:     d.InstanceModel,
				CvmAmount:         d.CvmAmount,
				InstanceType:      d.InstanceType,
				CoreTypeName:      d.CoreTypeName,
				CoreAmount:        d.CoreAmount,
				ReturnReasonClass: returnReasonClass,
				Desc:              d.Desc,
			},
		})
	}

	return details, nil
}

// getZoneNameMap 获取 zone_id → 中文可用区名映射（tcloud_ziyan），追加明细 CRP 提单需中文可用区名。
func (c *Controller) getZoneNameMap(kt *kit.Kit) (map[string]string, error) {
	req := &zoneproto.ZoneListReq{
		Filter: tools.EqualExpression("vendor", enumor.TCloudZiyan),
		Page:   core.NewDefaultBasePage(),
	}

	zoneNameMap := make(map[string]string)
	for {
		zones, err := c.client.DataService().TCloudZiyan.Zone.ListZoneExt(kt, req)
		if err != nil {
			logs.Errorf("failed to list zone, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		for _, zone := range zones.Details {
			zoneNameMap[zone.Name] = zone.NameCn
		}
		if uint(len(zones.Details)) < req.Page.Limit {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	return zoneNameMap, nil
}

// createReturnPlanTicket 构造并创建 init 主单（type 由 details 推导，Applicant 来自请求）。
func (c *Controller) createReturnPlanTicket(kt *kit.Kit, bizOrgRel *mtypes.BizOrgRel,
	req *rptypes.OverwriteAppendReturnPlanTicketReq, details tablert.ReturnPlanDetails) (string, error) {

	ticketType, err := details.DeriveTicketType()
	if err != nil {
		logs.Errorf("failed to derive return plan ticket type, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	detailsField, err := tabletypes.NewJsonField(details)
	if err != nil {
		logs.Errorf("failed to marshal return plan details, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	// 以真实提单人身份创建主单，贯穿后续子单 Creator 与 CRP userName。
	kt.User = req.Applicant
	createReq := &rpproto.ReturnPlanTicketCreateReq{
		Type:            ticketType,
		Details:         detailsField,
		Applicant:       req.Applicant,
		BkBizID:         bizOrgRel.BkBizID,
		BkBizName:       bizOrgRel.BkBizName,
		OpProductID:     bizOrgRel.OpProductID,
		OpProductName:   bizOrgRel.OpProductName,
		PlanProductID:   bizOrgRel.PlanProductID,
		PlanProductName: bizOrgRel.PlanProductName,
		VirtualDeptID:   bizOrgRel.VirtualDeptID,
		VirtualDeptName: bizOrgRel.VirtualDeptName,
		Status:          enumor.ReturnPlanTicketStatusInit,
		Remark:          req.Remark,
		SubmittedAt:     time.Now().Format(constant.DateTimeLayout),
	}
	rst, err := c.client.DataService().Global.ReturnPlan.CreateReturnPlanTicket(kt, createReq)
	if err != nil {
		logs.Errorf("failed to create return plan ticket, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	return rst.ID, nil
}

