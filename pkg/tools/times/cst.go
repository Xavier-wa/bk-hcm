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

	"hcm/pkg/criteria/constant"
)

const cstOffsetSec = 8 * 3600

// CST returns the Asia/Shanghai location, falling back to a fixed UTC+8 zone.
func CST() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", cstOffsetSec)
	}
	return loc
}

// FormatDateTimeCST formats t as naive DATETIME in Asia/Shanghai.
// Empty input returns an empty string. Use this when comparing or writing
// MySQL DATETIME columns that store CST wall-clock values.
func FormatDateTimeCST(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(CST()).Format(constant.DateTimeLayout)
}

// EachCSTDate returns every CST calendar day in [from, to] as 2006-01-02.
// from and to are RFC3339. Invalid range returns nil.
func EachCSTDate(from, to string) []string {
	loc := CST()
	start, err := time.Parse(time.RFC3339, from)
	if err != nil {
		return nil
	}
	end, err := time.Parse(time.RFC3339, to)
	if err != nil {
		return nil
	}
	sy, sm, sd := start.In(loc).Date()
	ey, em, ed := end.In(loc).Date()
	cur := time.Date(sy, sm, sd, 0, 0, 0, 0, loc)
	last := time.Date(ey, em, ed, 0, 0, 0, 0, loc)
	if cur.After(last) {
		return nil
	}
	out := make([]string, 0)
	for !cur.After(last) {
		out = append(out, cur.Format(constant.DateLayout))
		cur = cur.AddDate(0, 0, 1)
	}
	return out
}

// CSTDateOf converts an RFC3339 or "2006-01-02 15:04:05" timestamp to a CST
// calendar day. Unparseable input returns an empty string.
func CSTDateOf(raw string) string {
	loc := CST()
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t, err = time.ParseInLocation(constant.DateTimeLayout, raw, loc)
	}
	if err != nil {
		return ""
	}
	return t.In(loc).Format(constant.DateLayout)
}
