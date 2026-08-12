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

package splitter

import (
	"errors"
	"sort"
	"time"

	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/slice"
)

// splitGroupKey 拆单分组键：类型 + 项目类型 + 资源池。
// 部门/规划产品来自主单头（单主单内恒定），无需纳入分组键。
type splitGroupKey struct {
	subType     enumor.ReturnPlanTicketType
	obsProject  enumor.ObsProject
	resPoolName string
}

// splitGroup 拆单分组，一个分组对应一个子单（进而对应一个 CRP 单据）。
type splitGroup struct {
	key     splitGroupKey
	details tablert.ReturnPlanDetails
}

// groupDetails 按 (类型 + 项目类型 + 资源池) 对明细分组，add/cancel/adjust 天然分属不同分组。
// 类型由 original/updated 有无推导，分组维度取自明细的 GroupItem(优先 Original，以调整前数据分组)。
// 所有类型(含新增)均纳入资源池维度：一个分组=一个子单=一个 CRP 单据，避免单个子单在 CRP 侧产生多个 order。
// 结果按分组键排序，保证同一主单多次拆单的稳定性。
func groupDetails(kt *kit.Kit, details tablert.ReturnPlanDetails) ([]splitGroup, error) {
	grouped := make(map[splitGroupKey]tablert.ReturnPlanDetails)
	keys := make([]splitGroupKey, 0)
	for _, d := range details {
		subType, err := d.Type()
		if err != nil {
			logs.Errorf("group return plan details failed, get detail type err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		item := d.GroupItem()
		if item == nil {
			logs.Errorf("group return plan details failed, group item is nil, sub_type: %s, rid: %s",
				subType, kt.Rid)
			return nil, errors.New("return plan detail group item is nil")
		}

		key := splitGroupKey{
			subType:     subType,
			obsProject:  item.ObsProject,
			resPoolName: item.ResourcePoolName,
		}
		if _, ok := grouped[key]; !ok {
			keys = append(keys, key)
		}
		grouped[key] = append(grouped[key], d)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].subType != keys[j].subType {
			return keys[i].subType < keys[j].subType
		}
		if keys[i].obsProject != keys[j].obsProject {
			return keys[i].obsProject < keys[j].obsProject
		}
		return keys[i].resPoolName < keys[j].resPoolName
	})

	groups := make([]splitGroup, 0, len(keys))
	for _, key := range keys {
		groups = append(groups, splitGroup{key: key, details: grouped[key]})
	}

	logs.Infof("group return plan details success, detail_count: %d, group_count: %d, rid: %s",
		len(details), len(groups), kt.Rid)
	return groups, nil
}

// createSubTickets 将分组结果落 return_plan_sub_ticket（经 data-service client）。
func (s *SubTicketSplitter) createSubTickets(kt *kit.Kit, ticket *tablert.ReturnPlanTicketTable,
	groups []splitGroup) error {

	subTickets := make([]rpproto.ReturnPlanSubTicketCreateReq, 0, len(groups))
	for _, g := range groups {
		subDetails, err := tabletypes.NewJsonField(g.details)
		if err != nil {
			logs.Errorf("failed to create sub_details json field, err: %v, ticket_id: %s, rid: %s",
				err, ticket.ID, kt.Rid)
			return err
		}

		subTickets = append(subTickets, rpproto.ReturnPlanSubTicketCreateReq{
			TicketID:        ticket.ID,
			SubType:         g.key.subType,
			SubDetails:      subDetails,
			BkBizID:         ticket.BkBizID,
			BkBizName:       ticket.BkBizName,
			OpProductID:     ticket.OpProductID,
			OpProductName:   ticket.OpProductName,
			PlanProductID:   ticket.PlanProductID,
			PlanProductName: ticket.PlanProductName,
			VirtualDeptID:   ticket.VirtualDeptID,
			VirtualDeptName: ticket.VirtualDeptName,
			ObsProject:      g.key.obsProject,
			ResPoolName:     g.key.resPoolName,
			Status:          enumor.ReturnPlanSubTicketStatusInit,
			SubmittedAt:     time.Now().Format(constant.DateTimeLayout),
		})
	}

	// 后台任务，以提单人身份创建子单；按批量操作上限分批落库。
	kt.User = ticket.Applicant
	for _, batch := range slice.Split(subTickets, constant.BatchOperationMaxLimit) {
		createReq := &rpproto.ReturnPlanSubTicketBatchCreateReq{SubTickets: batch}
		if _, err := s.client.DataService().Global.ReturnPlan.BatchCreateReturnPlanSubTicket(kt, createReq); err != nil {
			logs.Errorf("failed to batch create return plan sub tickets, err: %v, ticket_id: %s, "+
				"batch_size: %d, rid: %s", err, ticket.ID, len(batch), kt.Rid)
			return err
		}
	}

	logs.Infof("create return plan sub tickets success, ticket_id: %s, sub_ticket_count: %d, rid: %s",
		ticket.ID, len(subTickets), kt.Rid)
	return nil
}
