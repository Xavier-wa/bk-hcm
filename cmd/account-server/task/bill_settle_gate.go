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

// Package task 提供 account-server 的定时任务实现。
package task

import "time"

// settleTimeZone 定账闸门统一使用的时区，锁定时点定义在 Asia/Shanghai 上。
const settleTimeZone = "Asia/Shanghai"

// billPeriod 账期，年 + 月。
type billPeriod struct {
	// Year 账期年份
	Year int
	// Month 账期月份
	Month int
}

// settleLocation 返回定账闸门所用时区，加载失败时退回固定 +8 时区，避免容器缺失 tzdata 时任务不可用。
func settleLocation() *time.Location {
	loc, err := time.LoadLocation(settleTimeZone)
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}

	return loc
}

// lockTimeOf 返回账期的锁定时点：账期次月的 lockDay+1 日零点。
// 例如 lockDay=8 时，7 月账期的锁定时点是 8 月 9 日零点，即 8 月 8 日整日仍可覆盖。
// 越界的月份由 time.Date 自行归一化，12 月账期的锁定时点会落到次年 1 月。
func lockTimeOf(period billPeriod, lockDay int) time.Time {
	return time.Date(period.Year, time.Month(period.Month)+1, lockDay+1, 0, 0, 0, 0, settleLocation())
}

// isPeriodLocked 判定账期在给定时刻是否已越过锁定时点。
// 这是纯时间闸门：只由账期与当前时间决定，不接受任何外部系统的直接触发入参。
func isPeriodLocked(period billPeriod, lockDay int, now time.Time) bool {
	return !now.Before(lockTimeOf(period, lockDay))
}

// periodOfMonthsAgo 返回 now 所在账期往前推 offset 个月的账期。
// 日期固定取 1 日，越界的月份由 time.Date 自行归一化，因此不会出现「2 月 31 日」这类溢出。
func periodOfMonthsAgo(now time.Time, offset int) billPeriod {
	cur := now.In(settleLocation())
	month := time.Date(cur.Year(), cur.Month()-time.Month(offset), 1, 0, 0, 0, 0, cur.Location())

	return billPeriod{Year: month.Year(), Month: int(month.Month())}
}

// lockedPeriods 返回回溯窗口内全部已锁定的账期，按时间从早到晚排列。
// 窗口从当前账期往前回溯 lookbackMonth 个月，不含当前账期：锁定时点在账期次月，
// 当前账期在当月内不可能已锁定。超窗口的账期不再扫描。
func lockedPeriods(now time.Time, lockDay, lookbackMonth int) []billPeriod {
	periods := make([]billPeriod, 0, lookbackMonth)
	for offset := lookbackMonth; offset >= 1; offset-- {
		period := periodOfMonthsAgo(now, offset)
		if !isPeriodLocked(period, lockDay, now) {
			continue
		}
		periods = append(periods, period)
	}

	return periods
}
