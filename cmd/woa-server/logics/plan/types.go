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

package plan

import (
	"errors"
	"fmt"
	"time"

	mtypes "hcm/cmd/woa-server/types/meta"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/tools/times"
)

// CreateResPlanTicketReq is create resource plan ticket request.
type CreateResPlanTicketReq struct {
	TicketType  enumor.RPTicketType `json:"ticket_type" validate:"required"`
	DemandClass enumor.DemandClass  `json:"demand_class" validate:"required"`
	BizOrgRel   mtypes.BizOrgRel    `json:"biz_org_rel" validate:"required"`
	// Demands is pre-built table demands for adjust/delete/transfer tickets.
	// TODO could be removed after all tickets are migrated to create_demands.
	Demands rpt.ResPlanDemands `json:"demands" validate:"omitempty"`
	// CreateDemands is API create demand requests for add ticket; built into Demands in CreateResPlanTicket.
	CreateDemands []ptypes.CreateResPlanDemandReq `json:"create_demands" validate:"omitempty"`
	Remark        string                          `json:"remark" validate:"omitempty"`
	// Applicant 提单人。为空时回退 kt.User。仅覆盖追加接口会设置该字段，
	// 用于以真实提单人（而非调用账号，可能为 admin）身份贯穿主单及后续 CRP 提单。
	Applicant string `json:"applicant" validate:"omitempty"`
}

// Validate whether CreateResPlanTicketReq is valid.
func (r *CreateResPlanTicketReq) Validate() error {
	// budget_declare 仅允许通过 overwrite_append 创建，普通创建入口显式拒绝。
	if r.TicketType == enumor.RPTicketTypeBudgetDeclare {
		return fmt.Errorf("unsupported resource plan ticket type: %s", r.TicketType)
	}

	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if err := r.TicketType.Validate(); err != nil {
		return err
	}

	if err := r.DemandClass.Validate(); err != nil {
		return err
	}

	switch r.TicketType {
	case enumor.RPTicketTypeAdd:
		return r.validateAddTicket()
	case enumor.RPTicketTypeAdjust:
		if err := r.validateAdjustTicketDemands(); err != nil {
			return err
		}
	case enumor.RPTicketTypeDelete, enumor.RPTicketTypeAutomaticTransfer:
		if err := r.validateDeleteTicketDemands(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported resource plan ticket type: %s", r.TicketType)
	}

	return r.validateTableDemands()
}

func (r *CreateResPlanTicketReq) validateAddTicket() error {
	if len(r.CreateDemands) == 0 {
		return errors.New("create_demands is required for add ticket")
	}
	for _, demand := range r.CreateDemands {
		if err := demand.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (r *CreateResPlanTicketReq) validateAdjustTicketDemands() error {
	for _, demand := range r.Demands {
		if demand.Updated == nil {
			return errors.New("updated demand of adjust ticket can not be empty")
		}
	}
	return nil
}

func (r *CreateResPlanTicketReq) validateDeleteTicketDemands() error {
	for _, demand := range r.Demands {
		if demand.Original == nil {
			return fmt.Errorf("original demand of %s ticket can not be empty", r.TicketType)
		}
		if demand.Updated != nil {
			return fmt.Errorf("updated demand of %s ticket should be empty", r.TicketType)
		}
	}
	return nil
}

func (r *CreateResPlanTicketReq) validateTableDemands() error {
	if len(r.Demands) == 0 {
		return errors.New("demands is required")
	}
	for _, demand := range r.Demands {
		if err := demand.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// QueryIEGDemandsReq query IEG demands request.
type QueryIEGDemandsReq struct {
	ExpectTimeRange  *times.DateRange `json:"expect_time_range" validate:"omitempty"`
	CrpDemandIDs     []int64          `json:"crp_demand_ids" validate:"omitempty"`
	CrpSns           []string         `json:"crp_sns" validate:"omitempty"`
	DeviceClasses    []string         `json:"device_classes" validate:"omitempty"`
	PlanProdNames    []string         `json:"plan_prod_names" validate:"omitempty"`
	OpProdNames      []string         `json:"op_prod_names" validate:"omitempty"`
	ObsProjects      []string         `json:"obs_projects" validate:"omitempty"`
	RegionNames      []string         `json:"region_names" validate:"omitempty"`
	ZoneNames        []string         `json:"zone_names" validate:"omitempty"`
	TechnicalClasses []string         `json:"technical_classes" validate:"omitempty"`
}

// Validate whether QueryIEGDemandsReq is valid.
func (r *QueryIEGDemandsReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	for _, crpDemandID := range r.CrpDemandIDs {
		if crpDemandID <= 0 {
			return errors.New("crp demand id should be > 0")
		}
	}

	return nil
}

// VerifyResPlanElem verify resource plan element.
type VerifyResPlanElem struct {
	// if IsPrePaid is true, Verify function will examine:
	// 1. InPlan + OutPlan >= applied.
	// 2. InPlan * 120% - consumed >= applied.
	// otherwise, it will only examine InPlan + OutPlan >= applied.
	IsPrePaid     bool
	AvailableTime ptypes.AvailableMonth
	DeviceType    string
	ObsProject    enumor.ObsProject
	RegionName    string
	ZoneName      string
	CpuCore       int64
}

// VerifyResPlanElemV2 verify resource plan element v2.
type VerifyResPlanElemV2 struct {
	// if IsPrePaid is true, Verify function will examine:
	// 1. InPlan + OutPlan >= applied.
	// 2. InPlan * 100% - consumed >= applied.(120% to be implemented)
	// otherwise, it will only examine InPlan + OutPlan >= applied.
	IsPrePaid     bool
	AvailableTime ptypes.AvailableMonth
	DeviceType    string
	ObsProject    enumor.ObsProject
	BkBizID       int64
	DemandClass   enumor.DemandClass
	RegionID      string
	ZoneID        string
	CpuCore       int64
}

// VerifyResPlanResElem verify resource plan result element.
type VerifyResPlanResElem struct {
	VerifyResult   enumor.VerifyResPlanRst `json:"verify_result"`
	Reason         string                  `json:"reason"`
	NeedCPUCore    int64                   `json:"need_cpu_core"`
	ResPlanCore    int64                   `json:"res_plan_core"`
	MatchDemandIDs []string                `json:"match_demand_ids"`
}

// ResPlanElem resource plan element.
type ResPlanElem struct {
	PlanType      enumor.PlanType
	AvailableTime ptypes.AvailableMonth
	DeviceType    string
	ObsProject    enumor.ObsProject
	RegionName    string
	ZoneName      string
	CpuCore       float64
}

// ResPlanPoolKey resource plan pool key.
type ResPlanPoolKey struct {
	PlanType      enumor.PlanType
	AvailableTime ptypes.AvailableMonth
	DeviceType    string
	ObsProject    enumor.ObsProject
	RegionName    string
	ZoneName      string
}

// ResPlanPool resource plan pool.
type ResPlanPool map[ResPlanPoolKey]int64

// ResPlanPoolKeyV2 resource plan demand key v2.
type ResPlanPoolKeyV2 struct {
	PlanType      enumor.PlanTypeCode
	AvailableTime ptypes.AvailableMonth
	DeviceType    string
	ObsProject    enumor.ObsProject
	BkBizID       int64
	DemandClass   enumor.DemandClass
	RegionID      string
	// ZoneID        string  // 预测匹配不要求zoneID相同
	// DiskType enumor.DiskType // 预测的云盘类型，不再作为预测校验的强匹配条件 TODO 云盘的预测单独管理，单独进行余量校验
}

// ResPlanConsumePool resource plan consume pool.
type ResPlanConsumePool map[ResPlanPoolKeyV2]int64

// ResPlanPoolMatch resource plan demand match.
type ResPlanPoolMatch map[ResPlanPoolKeyV2]map[string]int64

// budgetDemand 预算提报人同步使用的需求项
type budgetDemand struct {
	ID          string
	Creator     string
	Reviser     string
	ExpectTime  time.Time
	OpProductID int64
}

// demandGroup 按运营产品+年份分组的需求集合
type demandGroup struct {
	OpProductID int64
	Year        int
	Items       []*budgetDemand
}

// StrUnionFind string union find struct.
type StrUnionFind struct {
	idx    []string
	parent map[string]string
}

// NewStrUnionFind news a string union find.
func NewStrUnionFind() *StrUnionFind {
	return &StrUnionFind{parent: make(map[string]string)}
}

// Add adds a new element x.
func (uf *StrUnionFind) Add(x string) {
	if _, ok := uf.parent[x]; ok {
		return
	}
	uf.parent[x] = x
	uf.idx = append(uf.idx, x)
}

// Elements return all elements in StrUnionFind.
func (uf *StrUnionFind) Elements() []string {
	var res []string
	for _, e := range uf.idx {
		res = append(res, e)
	}

	return res
}

// Find finds the root parent of x.
func (uf *StrUnionFind) Find(x string) string {
	if uf.parent[x] != x {
		uf.parent[x] = uf.Find(uf.parent[x])
	}

	return uf.parent[x]
}

// Union unions the unions where x and y are.
func (uf *StrUnionFind) Union(x, y string) {
	parentX := uf.Find(x)
	parentY := uf.Find(y)
	if parentX != parentY {
		uf.parent[parentY] = parentX
	}
}

// Connected judges whether x and y are connected.
func (uf *StrUnionFind) Connected(x, y string) bool {
	return uf.Find(x) == uf.Find(y)
}

// GetPlanTypeByChargeType 根据计费模式，获取映射的预测内或预测外.
func (c *Controller) GetPlanTypeByChargeType(chargeType cvmapi.ChargeType) (enumor.PlanTypeCode, error) {
	return chargeType.ToPlanType(), nil
}
