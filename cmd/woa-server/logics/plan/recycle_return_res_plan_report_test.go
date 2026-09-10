/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
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
	"strings"
	"testing"
	"time"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/thirdparty/cvmapi"
)

const testBkHcmURL = "https://hcm.example.com"

func TestCalcRecycleReturnResPlanLastWeekRange(t *testing.T) {
	tests := []struct {
		name  string
		now   time.Time
		start time.Time
		end   time.Time
	}{
		{
			name:  "monday trigger",
			now:   time.Date(2026, 7, 6, 8, 0, 0, 0, time.Local),
			start: time.Date(2026, 6, 29, 0, 0, 0, 0, time.Local),
			end:   time.Date(2026, 7, 5, 23, 59, 59, 0, time.Local),
		},
		{
			name:  "wednesday manual trigger",
			now:   time.Date(2026, 7, 8, 15, 30, 0, 0, time.Local),
			start: time.Date(2026, 6, 29, 0, 0, 0, 0, time.Local),
			end:   time.Date(2026, 7, 5, 23, 59, 59, 0, time.Local),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := calcRecycleReturnResPlanLastWeekRange(tt.now)
			if !start.Equal(tt.start) {
				t.Errorf("unexpected start: %s", start.Format("2006-01-02 15:04:05"))
			}
			if !end.Equal(tt.end) {
				t.Errorf("unexpected end: %s", end.Format("2006-01-02 15:04:05"))
			}
			if start.Weekday() != time.Monday {
				t.Errorf("start should be Monday, got %v", start.Weekday())
			}
			if end.Weekday() != time.Sunday {
				t.Errorf("end should be Sunday, got %v", end.Weekday())
			}
		})
	}
}

func TestToRecycleReturnResPlanOrder(t *testing.T) {
	qo := &cvmapi.QueryOrderInfo{
		OrderID:           "TH20260101001",
		PlanProductName:   "规划产品A",
		ToPlanProductName: "运营产品B",
		OrderTypeName:     "项目类型C",
		CreateTime:        "2026-01-01 10:00:00",
		AllCoreAmount:     128,
		AllDiskAmount:     2048,
		CvmData: []cvmapi.CvmData{
			{Name: "SA5", Value: 256, Unit: "GB", CityName: "南京"},
			{Name: "M5", Value: 512, Unit: "GB", CityName: "南京"},
			{Name: "S5", Value: 8, Unit: "核", CityName: "上海"},
		},
	}

	// 无 UseTime，应回退到 CreateTime
	o := toRecycleReturnResPlanOrder(qo)
	if o.useTime != "2026-01-01 10:00:00" {
		t.Errorf("unexpected useTime: %s", o.useTime)
	}
	if o.planProduct != "规划产品A" || o.opProduct != "运营产品B" || o.orderType != "项目类型C" {
		t.Errorf("unexpected mapping: %+v", o)
	}
	// 内存 = GB 项求和：256 + 512 = 768
	if o.memoryGB != 768 {
		t.Errorf("unexpected memoryGB: %v", o.memoryGB)
	}
	if o.cpuCore != 128 {
		t.Errorf("unexpected cpuCore: %d", o.cpuCore)
	}
	if o.diskGB != 2048 {
		t.Errorf("unexpected diskGB: %d", o.diskGB)
	}
	// 机型族 = 全部 CvmData Name 拼接
	if o.deviceFamily != "SA5、M5、S5" {
		t.Errorf("unexpected deviceFamily: %s", o.deviceFamily)
	}
	// 城市取自 CvmData.CityName，去重拼接
	if o.city != "南京、上海" {
		t.Errorf("unexpected city: %s", o.city)
	}
	// 单据链接
	expectedURL := cvmapi.CvmPlanLinkPrefix + "TH20260101001"
	if o.orderURL != expectedURL {
		t.Errorf("unexpected orderURL: %s", o.orderURL)
	}
}

func TestToRecycleReturnResPlanOrderUseTimePrefer(t *testing.T) {
	qo := &cvmapi.QueryOrderInfo{
		UseTime: "2026-02-02 12:00:00",
		OrderID: "TH1",
	}
	o := toRecycleReturnResPlanOrder(qo)
	if o.useTime != "2026-02-02 12:00:00" {
		t.Errorf("should prefer UseTime, got %s", o.useTime)
	}
}

func TestConvertValidRecycleReturnResPlanOrders(t *testing.T) {
	queryOrders := []*cvmapi.QueryOrderInfo{
		{OrderID: "TH1", Status: enumor.QueryOrderInfoStatusSuccess},
		{OrderID: "TH2", Status: 1},
		nil,
	}

	orders := convertValidRecycleReturnResPlanOrders(queryOrders)
	if len(orders) != 1 {
		t.Fatalf("expected 1 valid order, got %d", len(orders))
	}
	if !strings.HasSuffix(orders[0].orderURL, "TH1") {
		t.Errorf("unexpected order url: %s", orders[0].orderURL)
	}
}

func TestBuildRecycleReturnResPlanReportEmpty(t *testing.T) {
	sendDate := time.Date(2026, 7, 6, 8, 0, 0, 0, time.Local)
	start := time.Date(2026, 6, 29, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 7, 5, 23, 59, 59, 0, time.Local)

	title, content := buildRecycleReturnResPlanReport(sendDate, start, end, testBkHcmURL, nil)
	if title != "IEG-销毁返还预测统计周报-2026-07-06" {
		t.Errorf("unexpected title: %s", title)
	}
	if !strings.Contains(content, "本统计周期内，无销毁返还预测单") {
		t.Errorf("empty report should contain new empty message, got snippet: %s",
			content[min(len(content), 200):])
	}
	// 命中 0 条不渲染空表
	if strings.Contains(content, "<tbody>") && strings.Contains(content, "</tbody>") &&
		strings.Contains(strings.Split(content, "<tbody>")[1], "</tbody>") &&
		len(strings.Split(strings.Split(content, "<tbody>")[1], "</tbody>")[0]) > 10 {
		t.Errorf("empty report should not render table rows")
	}
}

func TestBuildRecycleReturnResPlanReportWithRows(t *testing.T) {
	sendDate := time.Date(2026, 7, 6, 8, 0, 0, 0, time.Local)
	start := time.Date(2026, 6, 29, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 7, 5, 23, 59, 59, 0, time.Local)

	orders := []recycleReturnResPlanOrder{
		{planProduct: "P1", opProduct: "O1", orderType: "T1", useTime: "2026-06-30",
			deviceFamily: "SA5", city: "-", cpuCore: 100, memoryGB: 200, diskGB: 500,
			orderURL: cvmapi.CvmPlanLinkPrefix + "TH1"},
		{planProduct: "P2", opProduct: "O2", orderType: "T2", useTime: "2026-07-01",
			deviceFamily: "M5", city: "-", cpuCore: 50, memoryGB: 100, diskGB: 300,
			orderURL: cvmapi.CvmPlanLinkPrefix + "TH2"},
	}

	title, content := buildRecycleReturnResPlanReport(sendDate, start, end, testBkHcmURL, orders)
	if title != "IEG-销毁返还预测统计周报-2026-07-06" {
		t.Errorf("unexpected title: %s", title)
	}
	if !strings.Contains(content, "销毁返还预测详情") {
		t.Errorf("report should contain detail section")
	}
	// 汇总：CPU = 150，内存 = 300
	if !strings.Contains(content, "150") {
		t.Errorf("report should contain total CPU 150")
	}
	if !strings.Contains(content, "300") {
		t.Errorf("report should contain total memory 300")
	}
	// 两行均渲染
	if !strings.Contains(content, "TH1") || !strings.Contains(content, "TH2") {
		t.Errorf("report should contain both order links")
	}
}

func TestExtractRecycleReturnResPlanCity(t *testing.T) {
	city := extractRecycleReturnResPlanCity(nil)
	if city != "-" {
		t.Errorf("empty cvm data should fallback to '-', got %s", city)
	}

	city = extractRecycleReturnResPlanCity([]cvmapi.CvmData{
		{CityName: "南京"},
		{CityName: "南京"},
		{CityName: "上海"},
	})
	if city != "南京、上海" {
		t.Errorf("unexpected city: %s", city)
	}
}

func TestBuildTableRowsWithMergeColumnCount(t *testing.T) {
	orders := []recycleReturnResPlanOrder{
		{planProduct: "P1", opProduct: "O1", orderType: "T1", useTime: "2026-06-30",
			deviceFamily: "SA5", city: "-", cpuCore: 100, memoryGB: 200, diskGB: 500,
			orderURL: cvmapi.CvmPlanLinkPrefix + "TH1"},
	}
	rowsHTML := buildTableRowsWithMerge(orders)
	tdCount := strings.Count(rowsHTML, "<td")
	if tdCount != 10 {
		t.Errorf("table row should contain 10 columns, got %d", tdCount)
	}
}
