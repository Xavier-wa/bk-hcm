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
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/table/types"
)

// ExportResPlanTicketsReq is the request for batch-exporting res plan tickets.
type ExportResPlanTicketsReq struct {
	IDs []string `json:"ids"`
}

// Validate validates ExportResPlanTicketsReq.
func (r *ExportResPlanTicketsReq) Validate() error {
	if len(r.IDs) == 0 {
		return errf.New(errf.InvalidParameter, "ids cannot be empty")
	}
	return nil
}

// TicketExportRow is one data row in the exported Excel.
// Field order determines column order; the excel tag is the column header.
type TicketExportRow struct {
	TicketID        string        `excel:"单据ID"`
	Dept            string        `excel:"部门"`
	PlanProductID   int64         `excel:"规划产品ID"`
	PlanProductName string        `excel:"规划产品名称"`
	OpProductID     int64         `excel:"运营产品ID"`
	OpProductName   string        `excel:"运营产品名称"`
	BkBizName       string        `excel:"业务名称"`
	TicketType      string        `excel:"单据类型"`
	DemandClass     string        `excel:"预测用途"`
	TicketStatus    string        `excel:"单据状态"`
	Applicant       string        `excel:"提单人"`
	ObsProject      string        `excel:"项目类型"`
	RegionName      string        `excel:"城市"`
	DeviceType      string        `excel:"机型"`
	ExpectTime      string        `excel:"期望交付时间"`
	CoreType        string        `excel:"核心类型"`
	OSCount         types.Decimal `excel:"业务需求OS数"`
	CPUCorePerOS    int64         `excel:"单OS核心数"`
	CPUCore         int64         `excel:"CPU总核数"`
	DiskSize        int64         `excel:"数据盘总容量（G）"`
	DiskType        string        `excel:"数据盘类型"`
	DemandRemark    string        `excel:"预测说明"`
	Remark          string        `excel:"备注"`
}
