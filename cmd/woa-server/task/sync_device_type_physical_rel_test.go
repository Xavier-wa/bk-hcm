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

package task

import (
	"context"
	"testing"

	"hcm/pkg/kit"
	"hcm/pkg/thirdparty/cvmapi"

	"github.com/stretchr/testify/assert"
)

func testKit() *kit.Kit {
	return &kit.Kit{Ctx: context.Background(), Rid: "test-rid"}
}

// TestBuildSnapshotFromCvmTypeItems 验证字段过滤与去重逻辑。
// 覆盖：空字段过滤、相同 cvmInstanceModel 去重（后者覆盖前者）。
func TestBuildSnapshotFromCvmTypeItems(t *testing.T) {
	cases := []struct {
		name  string
		input []cvmapi.CvmTypeItem
		want  map[string]string
	}{
		{
			name:  "empty input",
			input: []cvmapi.CvmTypeItem{},
			want:  map[string]string{},
		},
		{
			name: "normal records",
			input: []cvmapi.CvmTypeItem{
				{CvmInstanceModel: "SA4t.32XLARGE576", DeviceFamily: "云上计算标准"},
				{CvmInstanceModel: "S5.SMALL2", DeviceFamily: "云上计算通用"},
			},
			want: map[string]string{
				"SA4t.32XLARGE576": "云上计算标准",
				"S5.SMALL2":        "云上计算通用",
			},
		},
		{
			name: "skip empty cvm instance model",
			input: []cvmapi.CvmTypeItem{
				{CvmInstanceModel: "", DeviceFamily: "云上计算标准"},
				{CvmInstanceModel: "S5.SMALL2", DeviceFamily: "云上计算通用"},
			},
			want: map[string]string{"S5.SMALL2": "云上计算通用"},
		},
		{
			name: "skip empty device family",
			input: []cvmapi.CvmTypeItem{
				{CvmInstanceModel: "SA4t.32XLARGE576", DeviceFamily: ""},
				{CvmInstanceModel: "S5.SMALL2", DeviceFamily: "云上计算通用"},
			},
			want: map[string]string{"S5.SMALL2": "云上计算通用"},
		},
		{
			name: "duplicate key - later wins",
			input: []cvmapi.CvmTypeItem{
				{CvmInstanceModel: "SA4t.32XLARGE576", DeviceFamily: "云上计算标准"},
				{CvmInstanceModel: "SA4t.32XLARGE576", DeviceFamily: "云上计算高阶"},
			},
			want: map[string]string{"SA4t.32XLARGE576": "云上计算高阶"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildSnapshotFromCvmTypeItems(testKit(), c.input)
			assert.Equal(t, c.want, got)
		})
	}
}

// TestDiffSnapshots 验证 diff 计算逻辑。
// 覆盖：纯新增、纯更新、混合、完全一致、双方都为空。
func TestDiffSnapshots(t *testing.T) {
	cases := []struct {
		name       string
		crpMap     map[string]string
		localMap   map[string]localRecord
		wantCreate map[string]string
		wantUpdate map[string]string
	}{
		{
			name:       "both empty",
			crpMap:     map[string]string{},
			localMap:   map[string]localRecord{},
			wantCreate: map[string]string{},
			wantUpdate: map[string]string{},
		},
		{
			name: "pure create - crp has all, local empty",
			crpMap: map[string]string{
				"SA4t.32XLARGE576": "云上计算标准",
				"S5.SMALL2":        "云上计算通用",
			},
			localMap: map[string]localRecord{},
			wantCreate: map[string]string{
				"SA4t.32XLARGE576": "云上计算标准",
				"S5.SMALL2":        "云上计算通用",
			},
			wantUpdate: map[string]string{},
		},
		{
			name: "pure update - same keys but different value",
			crpMap: map[string]string{
				"SA4t.32XLARGE576": "云上计算高阶",
			},
			localMap: map[string]localRecord{
				"SA4t.32XLARGE576": {ID: "id-1", PhysicalDeviceFamily: "云上计算标准"},
			},
			wantCreate: map[string]string{},
			wantUpdate: map[string]string{"id-1": "云上计算高阶"},
		},
		{
			name:   "pure delete - crp empty, local has data",
			crpMap: map[string]string{},
			localMap: map[string]localRecord{
				"SA4t.32XLARGE576": {ID: "id-1", PhysicalDeviceFamily: "云上计算标准"},
				"S5.SMALL2":        {ID: "id-2", PhysicalDeviceFamily: "云上计算通用"},
			},
			wantCreate: map[string]string{},
			wantUpdate: map[string]string{},
		},
		{
			name: "mixed - create + update + delete",
			crpMap: map[string]string{
				"NEW.MODEL":        "新机型族",
				"SA4t.32XLARGE576": "云上计算高阶",
			},
			localMap: map[string]localRecord{
				"SA4t.32XLARGE576": {ID: "id-1", PhysicalDeviceFamily: "云上计算标准"},
				"OLD.MODEL":        {ID: "id-3", PhysicalDeviceFamily: "云上计算淘汰"},
			},
			wantCreate: map[string]string{"NEW.MODEL": "新机型族"},
			wantUpdate: map[string]string{"id-1": "云上计算高阶"},
		},
		{
			name: "fully equal - no write",
			crpMap: map[string]string{
				"SA4t.32XLARGE576": "云上计算标准",
				"S5.SMALL2":        "云上计算通用",
			},
			localMap: map[string]localRecord{
				"SA4t.32XLARGE576": {ID: "id-1", PhysicalDeviceFamily: "云上计算标准"},
				"S5.SMALL2":        {ID: "id-2", PhysicalDeviceFamily: "云上计算通用"},
			},
			wantCreate: map[string]string{},
			wantUpdate: map[string]string{},
		},
	}

	task := &SyncDeviceTypePhysicalRelTask{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			toCreate, toUpdate := task.diffSnapshots(c.crpMap, c.localMap)
			assert.Equal(t, c.wantCreate, toCreate)
			assert.Equal(t, c.wantUpdate, toUpdate)
		})
	}
}

// TestSyncDeviceTypePhysicalRelTask_Meta 验证任务元信息：名称、URL、Next 间隔。
func TestSyncDeviceTypePhysicalRelTask_Meta(t *testing.T) {
	task := &SyncDeviceTypePhysicalRelTask{}
	assert.Equal(t, "sync_device_type_physical_rel", task.Name())
	assert.Equal(t, "/device_type_physical_rels/sync", task.GetURL())
}
