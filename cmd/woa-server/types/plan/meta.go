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

// Package plan ...
package plan

import (
	"time"

	"hcm/cmd/woa-server/logics/plan/demand-time"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/tools/times"
)

// RPTicketStatusItem defines resource plan ticket status item.
type RPTicketStatusItem struct {
	Status     enumor.RPTicketStatus `json:"status"`
	StatusName string                `json:"status_name"`
}

// DemandAvailTimeReq is get resource plan demand available time request.
type DemandAvailTimeReq struct {
	ExpectTime string `json:"expect_time" validate:"required"`
}

// Validate whether DemandAvailTimeReq is valid.
func (r *DemandAvailTimeReq) Validate() (time.Time, error) {
	if err := validator.Validate.Struct(r); err != nil {
		return time.Time{}, err
	}

	return times.ParseDay(r.ExpectTime)
}

// DemandAvailTimeResp is resource plan demand available time response.
type DemandAvailTimeResp struct {
	YearMonthWeek demandtime.DemandYearMonthWeek `json:"year_month_week"`
	DRInWeek      times.DateRange                `json:"date_range_in_week"`
	DRInMonth     times.DateRange                `json:"date_range_in_month"`
}
