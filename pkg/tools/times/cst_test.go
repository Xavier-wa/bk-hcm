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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEachCSTDate(t *testing.T) {
	got := EachCSTDate("2026-08-16T00:00:00+08:00", "2026-08-18T23:59:59+08:00")
	assert.Equal(t, []string{"2026-08-16", "2026-08-17", "2026-08-18"}, got)
	assert.Nil(t, EachCSTDate("bad", "also-bad"))
	assert.Nil(t, EachCSTDate("2026-08-18T00:00:00+08:00", "2026-08-16T00:00:00+08:00"))
}

func TestCSTDateOf(t *testing.T) {
	assert.Equal(t, "2026-08-16", CSTDateOf("2026-08-16T10:00:00+08:00"))
	assert.Equal(t, "2026-08-20", CSTDateOf("2026-08-20 10:00:00"))
	assert.Equal(t, "", CSTDateOf("not-a-time"))
}

func TestFormatDateTimeCST(t *testing.T) {
	assert.Equal(t, "", FormatDateTimeCST(time.Time{}))

	cst := time.Date(2026, 8, 16, 0, 0, 0, 0, CST())
	assert.Equal(t, "2026-08-16 00:00:00", FormatDateTimeCST(cst))

	utc := time.Date(2026, 8, 15, 16, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-08-16 00:00:00", FormatDateTimeCST(utc))
}
