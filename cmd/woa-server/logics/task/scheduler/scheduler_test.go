/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package scheduler

import (
	"testing"
	"time"

	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/thirdparty/cvmapi"
)

func TestTicketToUnifyOrder_EmptySuborders(t *testing.T) {
	s := &scheduler{}
	now := time.Now()

	tickets := []*types.ApplyTicket{
		{
			OrderId:     1001,
			BkBizId:     100,
			User:        "testuser",
			RequireType: enumor.RequireTypeRegular,
			ExpectTime:  "2026-06-01",
			Remark:      "test remark",
			Stage:       types.TicketStageTerminate,
			Suborders:   nil, // 空 Suborders
			CreateAt:    now,
			UpdateAt:    now,
		},
	}

	result := s.ticketToUnifyOrder(tickets)

	if len(result) != 1 {
		t.Fatalf("expected 1 UnifyOrder, got %d", len(result))
	}

	order := result[0]
	if order.OrderId != 1001 {
		t.Errorf("expected OrderId 1001, got %d", order.OrderId)
	}
	if order.BkBizId != 100 {
		t.Errorf("expected BkBizId 100, got %d", order.BkBizId)
	}
	if order.User != "testuser" {
		t.Errorf("expected User 'testuser', got '%s'", order.User)
	}
	if order.TotalNum != 0 {
		t.Errorf("expected TotalNum 0, got %d", order.TotalNum)
	}
	if order.Stage != types.TicketStageTerminate {
		t.Errorf("expected Stage TERMINATE, got %s", order.Stage)
	}
}

func TestTicketToUnifyOrder_MultipleSuborders(t *testing.T) {
	s := &scheduler{}
	now := time.Now()

	tickets := []*types.ApplyTicket{
		{
			OrderId:     1002,
			BkBizId:     200,
			User:        "testuser2",
			RequireType: enumor.RequireTypeRegular,
			ExpectTime:  "2026-06-15",
			Remark:      "main order remark",
			Stage:       types.TicketStageTerminate,
			Suborders: []*types.Suborder{
				{
					ResourceType:      types.ResourceTypeCvm,
					Replicas:          5,
					AntiAffinityLevel: "high",
					EnableDiskCheck:   true,
					Remark:            "suborder1 remark",
					Source:            enumor.ApplyTicketSrcBusiness,
					Spec: &types.ResourceSpec{
						Region:     "ap-guangzhou",
						Zones:      []string{"ap-guangzhou-3"},
						DeviceType: "S5.MEDIUM4",
						ChargeType: cvmapi.ChargeTypePrePaid,
					},
				},
				{
					ResourceType:      types.ResourceTypeCvm,
					Replicas:          3,
					AntiAffinityLevel: "low",
					EnableDiskCheck:   false,
					Remark:            "suborder2 remark",
					Source:            enumor.ApplyTicketSrcBusiness,
					Spec: &types.ResourceSpec{
						Region:     "ap-shanghai",
						Zones:      []string{"ap-shanghai-2"},
						DeviceType: "S5.LARGE8",
						ChargeType: cvmapi.ChargeTypePostPaidByHour,
					},
				},
			},
			CreateAt: now,
			UpdateAt: now,
		},
	}

	result := s.ticketToUnifyOrder(tickets)

	if len(result) != 2 {
		t.Fatalf("expected 2 UnifyOrders, got %d", len(result))
	}

	// 验证第一个子单转换
	order1 := result[0]
	if order1.OrderId != 1002 {
		t.Errorf("order1: expected OrderId 1002, got %d", order1.OrderId)
	}
	if order1.ResourceType != types.ResourceTypeCvm {
		t.Errorf("order1: expected ResourceType CVM, got %s", order1.ResourceType)
	}
	if order1.OriginNum != 5 {
		t.Errorf("order1: expected OriginNum 5, got %d", order1.OriginNum)
	}
	if order1.TotalNum != 5 {
		t.Errorf("order1: expected TotalNum 5, got %d", order1.TotalNum)
	}
	if order1.AntiAffinityLevel != "high" {
		t.Errorf("order1: expected AntiAffinityLevel 'high', got '%s'", order1.AntiAffinityLevel)
	}
	if order1.EnableDiskCheck != true {
		t.Errorf("order1: expected EnableDiskCheck true, got %v", order1.EnableDiskCheck)
	}
	if order1.Remark != "suborder1 remark" {
		t.Errorf("order1: expected Remark 'suborder1 remark', got '%s'", order1.Remark)
	}
	if order1.Spec == nil || order1.Spec.Region != "ap-guangzhou" {
		t.Errorf("order1: expected Spec.Region 'ap-guangzhou'")
	}

	// 验证第二个子单转换
	order2 := result[1]
	if order2.OriginNum != 3 {
		t.Errorf("order2: expected OriginNum 3, got %d", order2.OriginNum)
	}
	if order2.TotalNum != 3 {
		t.Errorf("order2: expected TotalNum 3, got %d", order2.TotalNum)
	}
	if order2.Spec == nil || order2.Spec.Region != "ap-shanghai" {
		t.Errorf("order2: expected Spec.Region 'ap-shanghai'")
	}
}

func TestTicketToUnifyOrder_ZoneToZonesConversion(t *testing.T) {
	s := &scheduler{}
	now := time.Now()

	tickets := []*types.ApplyTicket{
		{
			OrderId:     1003,
			BkBizId:     300,
			User:        "testuser3",
			RequireType: enumor.RequireTypeRegular,
			ExpectTime:  "2026-06-20",
			Remark:      "zone conversion test",
			Stage:       types.TicketStageTerminate,
			Suborders: []*types.Suborder{
				{
					ResourceType: types.ResourceTypeCvm,
					Replicas:     2,
					Spec: &types.ResourceSpec{
						Region:     "ap-beijing",
						Zone:       "ap-beijing-5", // 旧版格式：单个 Zone
						Zones:      nil,            // Zones 为空
						DeviceType: "S5.MEDIUM4",
					},
				},
			},
			CreateAt: now,
			UpdateAt: now,
		},
	}

	result := s.ticketToUnifyOrder(tickets)

	if len(result) != 1 {
		t.Fatalf("expected 1 UnifyOrder, got %d", len(result))
	}

	order := result[0]
	if order.Spec == nil {
		t.Fatal("expected Spec not nil")
	}
	if len(order.Spec.Zones) != 1 {
		t.Fatalf("expected Zones length 1, got %d", len(order.Spec.Zones))
	}
	if order.Spec.Zones[0] != "ap-beijing-5" {
		t.Errorf("expected Zones[0] 'ap-beijing-5', got '%s'", order.Spec.Zones[0])
	}
}

func TestTicketToUnifyOrder_SeparateCampusConversion(t *testing.T) {
	s := &scheduler{}
	now := time.Now()

	tickets := []*types.ApplyTicket{
		{
			OrderId:     1004,
			BkBizId:     400,
			User:        "testuser4",
			RequireType: enumor.RequireTypeRegular,
			ExpectTime:  "2026-06-25",
			Remark:      "campus test",
			Stage:       types.TicketStageTerminate,
			Suborders: []*types.Suborder{
				{
					ResourceType: types.ResourceTypeCvm,
					Replicas:     10,
					Spec: &types.ResourceSpec{
						Region:     "ap-guangzhou",
						Zone:       cvmapi.CvmSeparateCampus, // 分 Campus 场景
						Zones:      nil,
						DeviceType: "S5.LARGE16",
					},
				},
			},
			CreateAt: now,
			UpdateAt: now,
		},
	}

	result := s.ticketToUnifyOrder(tickets)

	if len(result) != 1 {
		t.Fatalf("expected 1 UnifyOrder, got %d", len(result))
	}

	order := result[0]
	if order.Spec == nil {
		t.Fatal("expected Spec not nil")
	}

	// 验证 Zones 转换为 [CvmZoneAll]
	if len(order.Spec.Zones) != 1 {
		t.Fatalf("expected Zones length 1, got %d", len(order.Spec.Zones))
	}
	if order.Spec.Zones[0] != cvmapi.CvmZoneAll {
		t.Errorf("expected Zones[0] '%s', got '%s'", cvmapi.CvmZoneAll, order.Spec.Zones[0])
	}

	// 验证 ResAssign 设置为 CampusResAssign
	if order.Spec.ResAssign != enumor.CampusResAssign {
		t.Errorf("expected ResAssign %d, got %d", enumor.CampusResAssign, order.Spec.ResAssign)
	}
}

func TestTicketToUnifyOrder_MultipleTickets(t *testing.T) {
	s := &scheduler{}
	now := time.Now()

	tickets := []*types.ApplyTicket{
		{
			OrderId:     2001,
			BkBizId:     100,
			User:        "user1",
			RequireType: enumor.RequireTypeRegular,
			Stage:       types.TicketStageTerminate,
			Suborders:   nil, // 空 Suborders
			CreateAt:    now,
			UpdateAt:    now,
		},
		{
			OrderId:     2002,
			BkBizId:     200,
			User:        "user2",
			RequireType: enumor.RequireTypeRegular,
			Stage:       types.TicketStageTerminate,
			Suborders: []*types.Suborder{
				{
					ResourceType: types.ResourceTypeCvm,
					Replicas:     3,
					Spec: &types.ResourceSpec{
						Region:     "ap-shanghai",
						Zones:      []string{"ap-shanghai-1"},
						DeviceType: "S5.MEDIUM4",
					},
				},
			},
			CreateAt: now,
			UpdateAt: now,
		},
	}

	result := s.ticketToUnifyOrder(tickets)

	// 第一个 ticket 生成 1 条汇总记录，第二个 ticket 生成 1 条子单记录
	if len(result) != 2 {
		t.Fatalf("expected 2 UnifyOrders, got %d", len(result))
	}

	// 验证第一个是汇总记录
	if result[0].OrderId != 2001 {
		t.Errorf("expected first OrderId 2001, got %d", result[0].OrderId)
	}
	if result[0].TotalNum != 0 {
		t.Errorf("expected first TotalNum 0, got %d", result[0].TotalNum)
	}

	// 验证第二个是子单记录
	if result[1].OrderId != 2002 {
		t.Errorf("expected second OrderId 2002, got %d", result[1].OrderId)
	}
	if result[1].TotalNum != 3 {
		t.Errorf("expected second TotalNum 3, got %d", result[1].TotalNum)
	}
}
