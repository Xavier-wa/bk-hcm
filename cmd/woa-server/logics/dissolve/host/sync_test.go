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

package host

import (
	"testing"

	define "hcm/pkg/dal/table/dissolve/host"
	tabletypes "hcm/pkg/dal/table/types"
	cvt "hcm/pkg/tools/converter"

	"github.com/stretchr/testify/assert"
)

// TestFillBizFields 测试业务字段补全的优先级（CC 优先、ES 兜底、DB 旧值保底）与 ignore 计算。
func TestFillBizFields(t *testing.T) {
	const (
		assetID   = "ASSET-1"
		projectID = 100
	)
	dbKey := compositeKey(projectID, assetID)

	tests := []struct {
		name          string
		ccMap         map[string]bizInfo
		esMap         map[string]bizInfo
		dbMap         map[string]define.RecycleHostTable
		ignoreSet     map[int64]struct{}
		wantBizID     int64
		wantGroupID   int64
		wantOperators []string
		wantIgnore    bool
	}{
		{
			name: "CC 命中（旧值无）优先用 CC",
			ccMap: map[string]bizInfo{assetID: {bizID: 10, groupID: 20, operators: []string{"cc-op"},
				found: true}},
			esMap:         map[string]bizInfo{},
			dbMap:         map[string]define.RecycleHostTable{},
			ignoreSet:     map[int64]struct{}{},
			wantBizID:     10,
			wantGroupID:   20,
			wantOperators: []string{"cc-op"},
			wantIgnore:    false,
		},
		{
			name:  "CC 命中（旧值有）仍优先用 CC",
			ccMap: map[string]bizInfo{assetID: {bizID: 10, groupID: 20, operators: []string{"cc-op"}, found: true}},
			esMap: map[string]bizInfo{assetID: {bizID: 30, groupID: 40, operators: []string{"es-op"}, found: true}},
			dbMap: map[string]define.RecycleHostTable{
				dbKey: {BkBizID: cvt.ValToPtr(int64(50)), GroupID: cvt.ValToPtr(int64(60)),
					Operators: tabletypes.StringArray{"db-op"}},
			},
			ignoreSet:     map[int64]struct{}{},
			wantBizID:     10,
			wantGroupID:   20,
			wantOperators: []string{"cc-op"},
			wantIgnore:    false,
		},
		{
			name:  "CC 未命中、ES 命中（旧值无）用 ES",
			ccMap: map[string]bizInfo{},
			esMap: map[string]bizInfo{assetID: {bizID: 30, groupID: 40, operators: []string{"es-op"},
				found: true}},
			dbMap:         map[string]define.RecycleHostTable{},
			ignoreSet:     map[int64]struct{}{},
			wantBizID:     30,
			wantGroupID:   40,
			wantOperators: []string{"es-op"},
			wantIgnore:    false,
		},
		{
			name:  "CC 未命中、ES 未命中、旧值有，用 DB 旧值保底",
			ccMap: map[string]bizInfo{},
			esMap: map[string]bizInfo{},
			dbMap: map[string]define.RecycleHostTable{
				dbKey: {BkBizID: cvt.ValToPtr(int64(50)), GroupID: cvt.ValToPtr(int64(60)),
					Operators: tabletypes.StringArray{"db-op"}},
			},
			ignoreSet:     map[int64]struct{}{},
			wantBizID:     50,
			wantGroupID:   60,
			wantOperators: []string{"db-op"},
			wantIgnore:    false,
		},
		{
			name:          "CC 未命中、ES 未命中、旧值无，补空值",
			ccMap:         map[string]bizInfo{},
			esMap:         map[string]bizInfo{},
			dbMap:         map[string]define.RecycleHostTable{},
			ignoreSet:     map[int64]struct{}{},
			wantBizID:     0,
			wantGroupID:   0,
			wantOperators: nil,
			wantIgnore:    false,
		},
		{
			name: "CC 命中且命中忽略业务，ignore 置 true",
			ccMap: map[string]bizInfo{assetID: {bizID: 10, groupID: 20, operators: []string{"cc-op"},
				found: true}},
			esMap:         map[string]bizInfo{},
			dbMap:         map[string]define.RecycleHostTable{},
			ignoreSet:     map[int64]struct{}{10: {}},
			wantBizID:     10,
			wantGroupID:   20,
			wantOperators: []string{"cc-op"},
			wantIgnore:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := define.RecycleHostTable{
				AssetID:   cvt.ValToPtr(assetID),
				ProjectID: cvt.ValToPtr(projectID),
			}

			host = fillBizFields(host, tt.ccMap, tt.esMap, tt.dbMap, tt.ignoreSet)

			assert.Equal(t, tt.wantBizID, cvt.PtrToVal(host.BkBizID))
			assert.Equal(t, tt.wantGroupID, cvt.PtrToVal(host.GroupID))
			assert.Equal(t, tabletypes.StringArray(tt.wantOperators), host.Operators)
			assert.Equal(t, tt.wantIgnore, cvt.PtrToVal(host.IsIgnore))
		})
	}
}

// TestDiff 测试基于 (project_id, asset_id) 复合键的新增/更新/删除比对。
func TestDiff(t *testing.T) {
	mkHost := func(id string, projectID int, assetID, ip string) define.RecycleHostTable {
		return define.RecycleHostTable{
			ID:        id,
			ProjectID: cvt.ValToPtr(projectID),
			AssetID:   cvt.ValToPtr(assetID),
			InnerIP:   cvt.ValToPtr(ip),
		}
	}

	dbHosts := []define.RecycleHostTable{
		mkHost("1", 100, "A", "1.1.1.1"), // 将被更新（ip 变化）
		mkHost("2", 100, "B", "2.2.2.2"), // 不变
		mkHost("3", 100, "C", "3.3.3.3"), // 将被删除
	}
	caicheHosts := []define.RecycleHostTable{
		mkHost("", 100, "A", "1.1.1.9"), // 更新
		mkHost("", 100, "B", "2.2.2.2"), // 不变
		mkHost("", 100, "D", "4.4.4.4"), // 新增
		mkHost("", 200, "A", "5.5.5.5"), // 新增（同 assetID 不同 project）
	}

	create, update, deleteIDs := diff(dbHosts, caicheHosts)

	assert.Len(t, create, 2)
	assert.Len(t, update, 1)
	assert.Equal(t, "1", update[0].ID, "update 应继承 DB 记录 ID")
	assert.Equal(t, []string{"3"}, deleteIDs)
}
