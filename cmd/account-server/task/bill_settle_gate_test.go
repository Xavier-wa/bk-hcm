/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const testLockDay = 8

// atShanghai 按上海时区构造时刻，用于覆盖锁定边界。
func atShanghai(year int, month time.Month, day, hour, minute, second int) time.Time {
	return time.Date(year, month, day, hour, minute, second, 0, settleLocation())
}

func TestLockTimeOf(t *testing.T) {
	// 2026 年 7 月账期的锁定时点是 2026-08-09 00:00:00。
	lockAt := lockTimeOf(billPeriod{Year: 2026, Month: 7}, testLockDay)
	assert.Equal(t, "2026-08-09 00:00:00", lockAt.Format("2006-01-02 15:04:05"))

	// 12 月账期跨年到次年 1 月。
	lockAt = lockTimeOf(billPeriod{Year: 2026, Month: 12}, testLockDay)
	assert.Equal(t, "2027-01-09 00:00:00", lockAt.Format("2006-01-02 15:04:05"))
}

func TestIsPeriodLockedBoundary(t *testing.T) {
	period := billPeriod{Year: 2026, Month: 7}

	tests := []struct {
		name   string
		now    time.Time
		locked bool
	}{
		{name: "8 日零点仍可覆盖", now: atShanghai(2026, time.August, 8, 0, 0, 0), locked: false},
		{name: "8 日整日仍可覆盖", now: atShanghai(2026, time.August, 8, 23, 59, 59), locked: false},
		{name: "9 日零点整锁定", now: atShanghai(2026, time.August, 9, 0, 0, 0), locked: true},
		{name: "9 日零点后锁定", now: atShanghai(2026, time.August, 9, 0, 0, 1), locked: true},
		{name: "当月内未锁定", now: atShanghai(2026, time.July, 31, 23, 59, 59), locked: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.locked, isPeriodLocked(period, testLockDay, tt.now))
		})
	}
}

func TestLockedPeriodsWindow(t *testing.T) {
	// 2026-08-09 00:00:00 时点，回溯 3 个月窗口覆盖 2026-05 ~ 2026-07，不含当月 08。
	periods := lockedPeriods(atShanghai(2026, time.August, 9, 0, 0, 0), testLockDay, 3)

	assert.Equal(t, []billPeriod{
		{Year: 2026, Month: 5},
		{Year: 2026, Month: 6},
		{Year: 2026, Month: 7},
	}, periods)
}

func TestLockedPeriodsBeforeLockTime(t *testing.T) {
	// 2026-08-08 当日，7 月账期尚未锁定，只应返回 5 月与 6 月。
	periods := lockedPeriods(atShanghai(2026, time.August, 8, 12, 0, 0), testLockDay, 3)

	assert.Equal(t, []billPeriod{
		{Year: 2026, Month: 5},
		{Year: 2026, Month: 6},
	}, periods)
}

func TestLockedPeriodsMonthEndNoOverflow(t *testing.T) {
	// 3 月 31 日做月度回溯时不得因为「2 月 31 日」而溢出到 3 月。
	periods := lockedPeriods(atShanghai(2026, time.March, 31, 10, 0, 0), testLockDay, 2)

	assert.Equal(t, []billPeriod{
		{Year: 2026, Month: 1},
		{Year: 2026, Month: 2},
	}, periods)
}

func TestLockedPeriodsOutOfWindowNotScanned(t *testing.T) {
	// 回溯窗口为 1 时，只看上月，更早的已锁定账期有意不再扫描。
	periods := lockedPeriods(atShanghai(2026, time.August, 20, 0, 0, 0), testLockDay, 1)

	assert.Equal(t, []billPeriod{{Year: 2026, Month: 7}}, periods)
}
