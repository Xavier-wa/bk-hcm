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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"time"

	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	rpdaotypes "hcm/pkg/dal/dao/types/resource-plan"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/excel"
	"hcm/pkg/tools/slice"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

const ticketExcelSheetName = "单据数据"

// ExportResPlanTicket exports res plan tickets to an Excel file.
// Input: ticket_ids array. Each demand in a ticket produces one row.
func (s *service) ExportResPlanTicket(cts *rest.Contexts) (interface{}, error) {
	req := new(ptypes.ExportResPlanTicketsReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("decode export res plan tickets request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("validate export res plan tickets request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Application, Action: meta.Find}}
	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		logs.Errorf("authorize with perm failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	tickets, err := s.fetchTicketsWithStatus(cts.Kit, req.IDs)
	if err != nil {
		logs.Errorf("fetch res plan tickets with status failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	tmpPath, err := generateTicketExcel(cts.Kit, tickets)
	if err != nil {
		logs.Errorf("generate ticket excel failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &rest.FileResp{
		ContentTypeStr:        "application/octet-stream",
		ContentDispositionStr: fmt.Sprintf(`attachment; filename="res_plan_tickets_%d.xlsx"`, time.Now().Unix()),
		FilePath:              tmpPath,
	}, nil
}

// toExcelRow converts a ptypes.TicketExportRow to an ordered slice of cell values.
// types.Decimal is converted to float64 so excelize writes a numeric cell, not text.
func toExcelRow(row ptypes.TicketExportRow) []interface{} {
	v := reflect.ValueOf(row)
	t := reflect.TypeOf(row)
	values := make([]interface{}, 0, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		if t.Field(i).Tag.Get("excel") == "" {
			continue
		}
		val := v.Field(i).Interface()
		if d, ok := val.(types.Decimal); ok {
			values = append(values, d.InexactFloat64())
		} else {
			values = append(values, val)
		}
	}
	return values
}

// fetchTicketsWithStatus fetches tickets with their status in batches of 500.
func (s *service) fetchTicketsWithStatus(kt *kit.Kit, ticketIDs []string) ([]rpdaotypes.RPTicketWithStatus, error) {
	all := make([]rpdaotypes.RPTicketWithStatus, 0, len(ticketIDs))
	for _, batch := range slice.Split(ticketIDs, int(core.DefaultMaxPageLimit)) {
		rst, err := s.planController.ListResPlanTicketWithRes(kt, &core.ListReq{
			Filter: tools.ContainersExpression("id", batch),
			Page:   core.NewDefaultBasePage(),
		})
		if err != nil {
			logs.Errorf("list res plan tickets with status failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		for _, ticket := range rst.Details {
			all = append(all, ticket.RPTicketWithStatus)
		}
	}

	if len(all) != len(ticketIDs) {
		logs.Errorf("some res plan ticket ids not found, ids: %v, rid: %s", ticketIDs, kt.Rid)
		return nil, errf.Newf(errf.InvalidParameter, "some res plan ticket ids not found")
	}

	return all, nil
}

// buildExportRows expands each ticket's demands into individual export rows.
// Returns an error if any demand's Updated field is nil.
func buildExportRows(kt *kit.Kit, tickets []rpdaotypes.RPTicketWithStatus) ([]ptypes.TicketExportRow, error) {
	rows := make([]ptypes.TicketExportRow, 0, len(tickets))
	for _, ticket := range tickets {
		var demands rpt.ResPlanDemands
		if err := json.Unmarshal([]byte(ticket.Demands), &demands); err != nil {
			logs.Errorf("unmarshal demands for ticket %s failed, err: %v, rid: %s",
				ticket.ID, err, kt.Rid)
			return nil, errf.Newf(errf.Aborted, "unmarshal demands for ticket %s failed: %v", ticket.ID, err)
		}

		for idx, demand := range demands {
			// 目前对于没有Updated的单据直接返回报错。TODO：后续根据需求优化
			if demand.Updated == nil {
				logs.Errorf("ticket %s demand[%d] has nil updated item, rid: %s", ticket.ID, idx, kt.Rid)
				return nil, errf.Newf(errf.Aborted, "ticket %s demand[%d] has nil updated item", ticket.ID, idx)
			}
			updated := demand.Updated
			totalCores := types.Decimal{Decimal: updated.Cvm.Os.Mul(decimal.NewFromInt(updated.Cvm.CpuCore))}

			rows = append(rows, ptypes.TicketExportRow{
				TicketID:        ticket.ID,
				Dept:            ticket.VirtualDeptName,
				PlanProductID:   ticket.PlanProductID,
				PlanProductName: ticket.PlanProductName,
				BkBizName:       ticket.BkBizName,
				OpProductID:     ticket.OpProductID,
				OpProductName:   ticket.OpProductName,
				TicketType:      ticket.Type.Name(),
				TicketStatus:    ticket.Status.Name(),
				Applicant:       ticket.Applicant,
				ObsProject:      string(updated.ObsProject),
				RegionName:      updated.RegionName,
				DeviceType:      updated.Cvm.DeviceType,
				DiskType:        string(updated.Cbs.DiskType),
				CoreType:        updated.Cvm.CoreType,
				DemandClass:     string(ticket.DemandClass),
				DiskSize:        updated.Cbs.DiskSize,
				ExpectTime:      updated.ExpectTime,
				OSCount:         updated.Cvm.Os,
				DemandRemark:    updated.Remark,
				Remark:          ticket.Remark,
				CpuCore:         updated.Cvm.CpuCore,
				TotalCores:      totalCores,
			})
		}
	}
	return rows, nil
}

// generateTicketExcel builds export rows from tickets, writes to a temp file, and returns its path.
func generateTicketExcel(kt *kit.Kit, tickets []rpdaotypes.RPTicketWithStatus) (string, error) {
	rows, err := buildExportRows(kt, tickets)
	if err != nil {
		logs.Errorf("build export rows failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	tmpFile, err := os.CreateTemp("", "res_plan_tickets_*.xlsx")
	if err != nil {
		logs.Errorf("create temp file failed, err: %v, rid: %s", err, kt.Rid)
		return "", errf.Newf(errf.Aborted, "create temp file failed: %v", err)
	}
	defer tmpFile.Close()

	tmpPath := tmpFile.Name()
	if err = writeTicketExcel(kt, rows, tmpFile); err != nil {
		if rErr := os.Remove(tmpPath); rErr != nil {
			logs.Errorf("remove temp file failed, err: %v, rid: %s", rErr, kt.Rid)
		}
		return "", err
	}

	return tmpPath, nil
}

// writeTicketExcel writes a header row and data rows into w as an Excel file.
func writeTicketExcel(kt *kit.Kit, rows []ptypes.TicketExportRow, w io.Writer) error {
	f := excelize.NewFile()
	defer f.Close()

	// 设置sheet名称
	if err := f.SetSheetName("Sheet1", ticketExcelSheetName); err != nil {
		logs.Errorf("set sheet name failed, err: %v, rid: %s", err, kt.Rid)
		return errf.Newf(errf.Aborted, "set sheet name failed: %v", err)
	}

	headers, err := excel.TicketExportRowHeaders(ptypes.TicketExportRow{})
	if err != nil {
		return errf.Newf(errf.Aborted, "get export headers failed: %v", err)
	}
	if err := f.SetSheetRow(ticketExcelSheetName, "A1", &headers); err != nil {
		return errf.Newf(errf.Aborted, "set header row failed: %v", err)
	}
	if err := excel.SetHeaderStyle(f, ticketExcelSheetName, len(headers)); err != nil {
		logs.Warnf("apply header style failed, err: %v, rid: %s", err, kt.Rid)
	}

	widthTracker := excel.NewColWidthTracker(len(headers))
	for i, h := range headers {
		widthTracker.UpdateHeader(i, fmt.Sprint(h))
	}

	for i, row := range rows {
		rowData := toExcelRow(row)
		for j, v := range rowData {
			widthTracker.Update(j, fmt.Sprint(v))
		}
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			logs.Errorf("compute cell name for row %d failed, err: %v, rid: %s", i+2, err, kt.Rid)
			return errf.Newf(errf.Aborted, "compute cell name for row %d failed: %v", i+2, err)
		}
		if err = f.SetSheetRow(ticketExcelSheetName, cell, &rowData); err != nil {
			logs.Errorf("set data row %d failed, err: %v, rid: %s", i+2, err, kt.Rid)
			return errf.Newf(errf.Aborted, "set data row %d failed: %v", i+2, err)
		}
	}

	if err := widthTracker.Apply(f, ticketExcelSheetName); err != nil {
		logs.Warnf("apply column widths failed, err: %v, rid: %s", err, kt.Rid)
	}

	if err := f.Write(w); err != nil {
		logs.Errorf("write excel failed, err: %v, rid: %s", err, kt.Rid)
		return errf.Newf(errf.Aborted, "write excel failed: %v", err)
	}

	return nil
}
