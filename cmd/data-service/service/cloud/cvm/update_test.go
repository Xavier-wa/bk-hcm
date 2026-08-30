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

package cvm

import (
	"testing"

	corecvm "hcm/pkg/api/core/cloud/cvm"
	protocloud "hcm/pkg/api/data-service/cloud"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"

	"github.com/stretchr/testify/assert"
)

func TestSortByIDForLockOrder(t *testing.T) {
	testCases := []struct {
		name    string
		ids     []string
		expects []string
	}{
		{
			name:    "unordered ids are sorted ascending",
			ids:     []string{"00000003", "00000001", "00000002"},
			expects: []string{"00000001", "00000002", "00000003"},
		},
		{
			name:    "already sorted ids stay unchanged",
			ids:     []string{"00000001", "00000002", "00000003"},
			expects: []string{"00000001", "00000002", "00000003"},
		},
		{
			name:    "reversed ids are fully reordered",
			ids:     []string{"cvm-c", "cvm-b", "cvm-a"},
			expects: []string{"cvm-a", "cvm-b", "cvm-c"},
		},
		{
			name:    "single element",
			ids:     []string{"cvm-a"},
			expects: []string{"cvm-a"},
		},
		{
			name:    "empty slice",
			ids:     []string{},
			expects: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 带 extension 的全量更新入口
			cvms := buildZiyanCvmUpdates(tc.ids)
			cvms = slice.SortByKey(cvms, func(one protocloud.CvmBatchUpdateWithExtension[corecvm.TCloudZiyanHostExtension]) string {
				return one.ID
			})

			actual := make([]string, 0, len(cvms))
			for _, one := range cvms {
				actual = append(actual, one.ID)
			}
			assert.Equal(t, tc.expects, actual)

			// 窄字段更新入口，两个入口必须产出同一加锁顺序，否则彼此之间仍会死锁
			commonInfos := buildCvmCommonInfoUpdates(tc.ids)
			commonInfos = slice.SortByKey(commonInfos, func(one protocloud.CvmCommonInfoBatchUpdateData) string {
				return one.ID
			})

			commonActual := make([]string, 0, len(commonInfos))
			for _, one := range commonInfos {
				commonActual = append(commonActual, one.ID)
			}
			assert.Equal(t, tc.expects, commonActual)
		})
	}
}

// TestSortByIDForLockOrder_PreservesContent 排序只改变批内顺序，不得改变任一主机的更新内容，
// 保证 extension merge、is_gpu 重算、非零字段过滤等下游行为与排序前完全等价。
func TestSortByIDForLockOrder_PreservesContent(t *testing.T) {
	ids := []string{"00000003", "00000001", "00000002"}
	origin := buildZiyanCvmUpdates(ids)

	// 深拷贝一份做基准，按 id 建索引后逐项比对
	expected := make(map[string]protocloud.CvmBatchUpdateWithExtension[corecvm.TCloudZiyanHostExtension],
		len(origin))
	for _, one := range origin {
		expected[one.ID] = one
	}

	cvms := buildZiyanCvmUpdates(ids)
	cvms = slice.SortByKey(cvms, func(one protocloud.CvmBatchUpdateWithExtension[corecvm.TCloudZiyanHostExtension]) string {
		return one.ID
	})

	assert.Len(t, cvms, len(ids))
	for _, one := range cvms {
		want, exist := expected[one.ID]
		assert.True(t, exist, "sorted result contains unexpected id: %s", one.ID)
		assert.Equal(t, want.Name, one.Name)
		assert.Equal(t, want.BkBizID, one.BkBizID)
		assert.Equal(t, want.BkHostID, one.BkHostID)
		assert.Equal(t, want.MachineType, one.MachineType)
		assert.Equal(t, want.Status, one.Status)
		assert.Equal(t, want.PrivateIPv4Addresses, one.PrivateIPv4Addresses)
		assert.NotNil(t, one.Extension)
		assert.Equal(t, want.Extension.Operator, one.Extension.Operator)
		assert.Equal(t, want.Extension.SrvStatus, one.Extension.SrvStatus)
	}
}

func buildZiyanCvmUpdates(ids []string) []protocloud.CvmBatchUpdateWithExtension[corecvm.TCloudZiyanHostExtension] {
	cvms := make([]protocloud.CvmBatchUpdateWithExtension[corecvm.TCloudZiyanHostExtension], 0, len(ids))
	for i, id := range ids {
		cvms = append(cvms, protocloud.CvmBatchUpdateWithExtension[corecvm.TCloudZiyanHostExtension]{
			CvmBatchUpdate: protocloud.CvmBatchUpdate{
				ID:                   id,
				Name:                 "host-" + id,
				BkBizID:              int64(100 + i),
				BkHostID:             int64(1000 + i),
				MachineType:          "S5.LARGE8",
				Status:               "RUNNING",
				PrivateIPv4Addresses: []string{"127.0.0." + id},
			},
			Extension: &corecvm.TCloudZiyanHostExtension{
				Operator:  "tester-" + id,
				SrvStatus: "运营中",
			},
		})
	}

	return cvms
}

func buildCvmCommonInfoUpdates(ids []string) []protocloud.CvmCommonInfoBatchUpdateData {
	cvms := make([]protocloud.CvmCommonInfoBatchUpdateData, 0, len(ids))
	for i, id := range ids {
		cvms = append(cvms, protocloud.CvmCommonInfoBatchUpdateData{
			ID:      id,
			BkBizID: converter.ValToPtr(int64(100 + i)),
			Name:    converter.ValToPtr("host-" + id),
		})
	}

	return cvms
}
