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
	"testing"

	"hcm/pkg/criteria/enumor"
	tablerst "hcm/pkg/dal/table/return-plan/return-plan-sub-ticket"

	"github.com/stretchr/testify/assert"
)

// subTicketWithStatus 构造仅含状态的子单，用于状态聚合测试。
func subTicketWithStatus(status enumor.ReturnPlanSubTicketStatus) tablerst.ReturnPlanSubTicketTable {
	return tablerst.ReturnPlanSubTicketTable{Status: status}
}

func TestDetermineTicketStatus(t *testing.T) {
	testCases := []struct {
		name       string
		subTickets []tablerst.ReturnPlanSubTicketTable
		expected   enumor.ReturnPlanTicketStatus
	}{
		{
			name:       "no terminal sub tickets -> auditing",
			subTickets: nil,
			expected:   enumor.ReturnPlanTicketStatusAuditing,
		},
		{
			name: "all done -> done",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusDone),
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusDone),
			},
			expected: enumor.ReturnPlanTicketStatusDone,
		},
		{
			name: "all failed -> failed",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusFailed),
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusTerminated),
			},
			expected: enumor.ReturnPlanTicketStatusFailed,
		},
		{
			name: "all rejected -> rejected",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusRejected),
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusRevoked),
			},
			expected: enumor.ReturnPlanTicketStatusRejected,
		},
		{
			name: "done and failed -> partial failed",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusDone),
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusFailed),
			},
			expected: enumor.ReturnPlanTicketStatusPartialFailed,
		},
		{
			name: "done and rejected -> partial rejected",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusDone),
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusRejected),
			},
			expected: enumor.ReturnPlanTicketStatusPartialRejected,
		},
		{
			name: "failed takes priority over rejected",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusDone),
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusFailed),
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusRejected),
			},
			expected: enumor.ReturnPlanTicketStatusPartialFailed,
		},
		{
			name: "invalid sub tickets are skipped, remaining all done -> done",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusInvalid),
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusDone),
			},
			expected: enumor.ReturnPlanTicketStatusDone,
		},
		{
			name: "only invalid sub tickets -> auditing",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				subTicketWithStatus(enumor.ReturnPlanSubTicketStatusInvalid),
			},
			expected: enumor.ReturnPlanTicketStatusAuditing,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, determineTicketStatus(tc.subTickets))
		})
	}
}

func TestAggregateFailMessage(t *testing.T) {
	testCases := []struct {
		name       string
		subTickets []tablerst.ReturnPlanSubTicketTable
		expected   string
	}{
		{
			name:       "empty sub tickets -> empty message",
			subTickets: nil,
			expected:   "",
		},
		{
			name: "done and empty message sub tickets are skipped",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				{ID: "001", ResPoolName: "自研池", Status: enumor.ReturnPlanSubTicketStatusDone, Message: "ok"},
				{ID: "002", ResPoolName: "公有池", Status: enumor.ReturnPlanSubTicketStatusFailed, Message: ""},
			},
			expected: "",
		},
		{
			name: "invalid sub ticket message is skipped",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				{ID: "001", ResPoolName: "自研池", Status: enumor.ReturnPlanSubTicketStatusInvalid, Message: "old fail"},
				{ID: "002", ResPoolName: "公有池", Status: enumor.ReturnPlanSubTicketStatusFailed, Message: "crp error"},
			},
			expected: "[002/公有池] crp error",
		},
		{
			name: "multiple failed sub tickets joined by newline",
			subTickets: []tablerst.ReturnPlanSubTicketTable{
				{ID: "001", ResPoolName: "自研池", Status: enumor.ReturnPlanSubTicketStatusFailed, Message: "err a"},
				{ID: "002", ResPoolName: "公有池", Status: enumor.ReturnPlanSubTicketStatusRejected, Message: "err b"},
			},
			expected: "[001/自研池] err a\n[002/公有池] err b",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, aggregateFailMessage(tc.subTickets))
		})
	}
}

func TestTruncateMessage(t *testing.T) {
	testCases := []struct {
		name     string
		msg      string
		max      int
		expected string
	}{
		{name: "shorter than max returned as is", msg: "hello", max: 10, expected: "hello"},
		{name: "equal to max returned as is", msg: "hello", max: 5, expected: "hello"},
		{name: "truncated by rune", msg: "abcdef", max: 3, expected: "abc"},
		{name: "multibyte not broken", msg: "退回计划失败", max: 2, expected: "退回"},
		{name: "non positive max -> empty", msg: "hello", max: 0, expected: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, truncateMessage(tc.msg, tc.max))
		})
	}
}
