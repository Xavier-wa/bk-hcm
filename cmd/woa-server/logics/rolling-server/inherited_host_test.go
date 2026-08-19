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

package rollingserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalcRemainMonths(t *testing.T) {
	from := time.Date(2026, 8, 11, 10, 30, 0, 0, time.UTC)
	tests := []struct {
		name   string
		from   time.Time
		expire time.Time
		want   int
	}{
		{
			name:   "整月：同一日期，主体算法命中，兜底不生效",
			from:   from,
			expire: time.Date(2027, 2, 11, 10, 30, 0, 0, time.UTC),
			want:   6,
		},
		{
			name:   "不足一月补一月：8 个月零 3 天算 9 个月",
			from:   from,
			expire: time.Date(2027, 4, 14, 10, 30, 0, 0, time.UTC),
			want:   9,
		},
		{
			// 同日但到期时刻更晚，主体算法算出的月数覆盖不到到期时间，由兜底补足
			name:   "不足一月补一月：兜底分支生效",
			from:   from,
			expire: time.Date(2027, 2, 11, 18, 0, 0, 0, time.UTC),
			want:   7,
		},
		{
			// 按量计费固资无到期时间，返回负数与提取前的现网行为一致，不做零值兜底
			name:   "按量计费无到期时间",
			from:   from,
			expire: time.Time{},
			want:   -24307,
		},
		{
			name:   "已到期：到期时间早于当前时间",
			from:   from,
			expire: time.Date(2026, 6, 11, 10, 30, 0, 0, time.UTC),
			want:   -2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CalcRemainMonths(tt.from, tt.expire))
		})
	}
}
