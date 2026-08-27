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

package times

import (
	"time"
)

const (
	// thirdPartyBillFirstPullHour is the local-timezone hour on the 1st of the current UTC
	// month when last-month daily pull stubs may start being created.
	thirdPartyBillFirstPullHour = 16
)

// ShouldAutoPullBillPeriod reports whether automatic daily pull task stubs may be created
// for the given UTC bill year-month at now.
//
// The loc parameter is the local timezone used to evaluate the 16:00 threshold on the 1st.
// The current UTC month is always false. The last UTC month is true only at or after
// 16:00 in loc on the 1st of the current UTC month. Older months are true.
func ShouldAutoPullBillPeriod(now time.Time, loc *time.Location, billYear, billMonth int) bool {
	// 账期按 UTC 自然月，与 GetCurrentMonthUTC 一致。
	nowUTC := now.UTC()
	curYear, curMonth := nowUTC.Year(), int(nowUTC.Month())
	if billYear == curYear && billMonth == curMonth {
		return false
	}

	lastYear, lastMonth := getRelativeMonth(curYear, curMonth, -1)
	if billYear == lastYear && billMonth == lastMonth {
		threshold := time.Date(curYear, time.Month(curMonth), 1, thirdPartyBillFirstPullHour, 0, 0, 0, loc)
		return !now.Before(threshold)
	}

	return true
}
