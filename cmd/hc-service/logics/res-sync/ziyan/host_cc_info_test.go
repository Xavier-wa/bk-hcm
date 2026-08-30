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

package ziyan

import (
	"strconv"
	"testing"

	"hcm/pkg/api/core/cloud/cvm"
	"hcm/pkg/api/data-service/cloud"
	"hcm/pkg/kit"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/tools/converter"

	"github.com/stretchr/testify/assert"
)

func TestClassifyHost(t *testing.T) {
	testCases := []struct {
		name          string
		ccHosts       []cmdb.Host
		dbHosts       []cvm.Cvm[cvm.TCloudZiyanHostExtension]
		hostBizIDMap  map[int64]int64
		wantCreateIDs []int64
		wantUpdateIDs []int64
		wantDelIDs    []string
	}{
		{
			name:          "cc exists and db missing goes to create",
			ccHosts:       []cmdb.Host{{BkHostID: 1, BkCloudInstID: "ins-1"}},
			dbHosts:       nil,
			wantCreateIDs: []int64{1},
			wantUpdateIDs: []int64{},
			wantDelIDs:    []string{},
		},
		{
			name:          "cc and db both exist goes to update",
			ccHosts:       []cmdb.Host{{BkHostID: 1, BkCloudInstID: "ins-1"}},
			dbHosts:       []cvm.Cvm[cvm.TCloudZiyanHostExtension]{{BaseCvm: cvm.BaseCvm{CloudID: "ins-1"}}},
			hostBizIDMap:  map[int64]int64{1: 100},
			wantCreateIDs: []int64{},
			wantUpdateIDs: []int64{1},
			wantDelIDs:    []string{},
		},
		{
			name:          "cc missing and db exists goes to delete",
			ccHosts:       nil,
			dbHosts:       []cvm.Cvm[cvm.TCloudZiyanHostExtension]{{BaseCvm: cvm.BaseCvm{CloudID: "ins-1"}}},
			wantCreateIDs: []int64{},
			wantUpdateIDs: []int64{},
			wantDelIDs:    []string{"ins-1"},
		},
		{
			name:          "absent on both sides is skipped",
			ccHosts:       nil,
			dbHosts:       nil,
			wantCreateIDs: []int64{},
			wantUpdateIDs: []int64{},
			wantDelIDs:    []string{},
		},
		{
			// cc 主机重建后 bk_host_id 变化但 cloud_id 不变，必须命中旧记录走更新而不是新增
			name:          "rebuilt cc host with new host id but same cloud id goes to update",
			ccHosts:       []cmdb.Host{{BkHostID: 99, BkCloudInstID: "ins-1"}},
			dbHosts:       []cvm.Cvm[cvm.TCloudZiyanHostExtension]{{BaseCvm: cvm.BaseCvm{CloudID: "ins-1", BkHostID: 1}}},
			hostBizIDMap:  map[int64]int64{99: 100},
			wantCreateIDs: []int64{},
			wantUpdateIDs: []int64{99},
			wantDelIDs:    []string{},
		},
		{
			// 没有 bk_cloud_inst_id 时用固资号兜底作为 cloud id
			name:          "cloud id falls back to asset id",
			ccHosts:       []cmdb.Host{{BkHostID: 1, BkAssetID: "asset-1"}},
			dbHosts:       []cvm.Cvm[cvm.TCloudZiyanHostExtension]{{BaseCvm: cvm.BaseCvm{CloudID: "asset-1"}}},
			hostBizIDMap:  map[int64]int64{1: 100},
			wantCreateIDs: []int64{},
			wantUpdateIDs: []int64{1},
			wantDelIDs:    []string{},
		},
		{
			// 两个标识都为空的脏数据不参与任何分类，也不能把 db 记录判成删除
			name:          "dirty cc host without any identifier is dropped",
			ccHosts:       []cmdb.Host{{BkHostID: 1}},
			dbHosts:       nil,
			wantCreateIDs: []int64{},
			wantUpdateIDs: []int64{},
			wantDelIDs:    []string{},
		},
		{
			name: "mixed batch is split into three sets",
			ccHosts: []cmdb.Host{
				{BkHostID: 1, BkCloudInstID: "ins-1"},
				{BkHostID: 2, BkCloudInstID: "ins-2"},
			},
			dbHosts: []cvm.Cvm[cvm.TCloudZiyanHostExtension]{
				{BaseCvm: cvm.BaseCvm{CloudID: "ins-2"}},
				{BaseCvm: cvm.BaseCvm{CloudID: "ins-3"}},
			},
			hostBizIDMap:  map[int64]int64{2: 100},
			wantCreateIDs: []int64{1},
			wantUpdateIDs: []int64{2},
			wantDelIDs:    []string{"ins-3"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ccHostMap := buildCCHostMapByCloudID(kit.New(), tc.ccHosts)

			createIDs, updates, delCloudIDs := classifyHost(kit.New(), "account-1", ccHostMap,
				tc.dbHosts, tc.hostBizIDMap)

			updateIDs := make([]int64, 0, len(updates))
			for i := range updates {
				updateIDs = append(updateIDs, updates[i].BkHostID)
			}

			assert.ElementsMatch(t, tc.wantCreateIDs, createIDs)
			assert.ElementsMatch(t, tc.wantUpdateIDs, updateIDs)
			assert.ElementsMatch(t, tc.wantDelIDs, delCloudIDs)
		})
	}
}

// TestConvToCCOnlyUpdate_KeepsCloudFields 增量更新只覆盖 cc 来源字段，
// 云来源的基表字段留空（data-service 只更新非零字段），extension 中的云上槽位保留 db 现值。
func TestConvToCCOnlyUpdate_KeepsCloudFields(t *testing.T) {
	dbHost := cvm.Cvm[cvm.TCloudZiyanHostExtension]{
		BaseCvm: cvm.BaseCvm{
			ID:               "00000001",
			CloudID:          "ins-1",
			Name:             "cloud-instance-name",
			BkBizID:          100,
			BkHostID:         1,
			Zone:             "ap-guangzhou-3",
			Status:           "RUNNING",
			ImageID:          "image-old",
			CloudVpcIDs:      []string{"vpc-cloud"},
			VpcIDs:           []string{"00000010"},
			CloudSubnetIDs:   []string{"subnet-cloud"},
			SubnetIDs:        []string{"00000020"},
			CloudCreatedTime: "2024-01-01T00:00:00Z",
			CloudExpiredTime: "2025-01-01T00:00:00Z",
		},
		Extension: &cvm.TCloudZiyanHostExtension{
			TCloudCvmExtension: &cvm.TCloudCvmExtension{
				InstanceChargeType: converter.ValToPtr("POSTPAID_BY_HOUR"),
				Cpu:                converter.ValToPtr(int64(4)),
			},
			HostName:  "old-host-name",
			Operator:  "old-operator",
			SrvStatus: "测试中",
			BkCpu:     4,
		},
	}

	ccExpect := cvm.Cvm[cvm.TCloudZiyanHostExtension]{
		BaseCvm: cvm.BaseCvm{
			CloudID:              "ins-1",
			Name:                 "cc-host-name",
			BkBizID:              200,
			BkHostID:             1,
			BkAssetID:            "asset-1",
			Region:               "ap-guangzhou",
			Zone:                 "ap-guangzhou-6",
			OsName:               "TencentOS 3.1",
			MachineType:          "SA2.MEDIUM4",
			CloudVpcIDs:          []string{"vpc-cc"},
			CloudSubnetIDs:       []string{"subnet-cc"},
			PrivateIPv4Addresses: []string{"10.0.0.1"},
			PublicIPv4Addresses:  []string{"1.1.1.1"},
		},
		Extension: &cvm.TCloudZiyanHostExtension{
			HostName:  "cc-host-name",
			Operator:  "new-operator",
			SrvStatus: "运营中",
			BkCpu:     8,
		},
	}

	update := convToCCOnlyUpdate(ccExpect, dbHost)

	// 主键取 db 记录 id，cc 字段全部刷新
	assert.Equal(t, "00000001", update.ID)
	assert.Equal(t, int64(200), update.BkBizID)
	assert.Equal(t, int64(1), update.BkHostID)
	assert.Equal(t, "asset-1", update.BkAssetID)
	assert.Equal(t, "ap-guangzhou", update.Region)
	assert.Equal(t, "TencentOS 3.1", update.OsName)
	assert.Equal(t, "SA2.MEDIUM4", update.MachineType)
	assert.Equal(t, []string{"10.0.0.1"}, update.PrivateIPv4Addresses)
	assert.Equal(t, []string{"1.1.1.1"}, update.PublicIPv4Addresses)

	// 云来源基表字段留空，不会覆盖 db 现值
	assert.Empty(t, update.Name)
	assert.Empty(t, update.Zone)
	assert.Empty(t, update.ImageID)
	assert.Empty(t, update.CloudImageID)
	assert.Empty(t, update.CloudVpcIDs)
	assert.Empty(t, update.VpcIDs)
	assert.Empty(t, update.CloudSubnetIDs)
	assert.Empty(t, update.SubnetIDs)
	assert.Empty(t, update.CloudCreatedTime)
	assert.Empty(t, update.CloudExpiredTime)

	// status 是必填校验字段，回填 db 现值，写回原值不改变数据
	assert.Equal(t, "RUNNING", update.Status)

	// extension 的 cc 槽位刷新，云上槽位保留 db 现值不被冲成零值
	assert.NotNil(t, update.Extension)
	assert.Equal(t, "cc-host-name", update.Extension.HostName)
	assert.Equal(t, "new-operator", update.Extension.Operator)
	assert.Equal(t, "运营中", update.Extension.SrvStatus)
	assert.Equal(t, int64(8), update.Extension.BkCpu)
	assert.NotNil(t, update.Extension.TCloudCvmExtension)
	assert.Equal(t, "POSTPAID_BY_HOUR", converter.PtrToVal(update.Extension.InstanceChargeType))
	assert.Equal(t, int64(4), converter.PtrToVal(update.Extension.Cpu))
}

// TestConvToCCOnlyUpdate_DBExtensionNil db 侧 extension 为空时不应 panic
func TestConvToCCOnlyUpdate_DBExtensionNil(t *testing.T) {
	dbHost := cvm.Cvm[cvm.TCloudZiyanHostExtension]{
		BaseCvm: cvm.BaseCvm{ID: "00000001", CloudID: "ins-1", Status: "RUNNING"},
	}
	ccExpect := cvm.Cvm[cvm.TCloudZiyanHostExtension]{
		BaseCvm:   cvm.BaseCvm{CloudID: "ins-1", BkBizID: 100, BkHostID: 1},
		Extension: &cvm.TCloudZiyanHostExtension{HostName: "cc-host-name", Operator: "op"},
	}

	update := convToCCOnlyUpdate(ccExpect, dbHost)

	assert.NotNil(t, update.Extension)
	assert.Equal(t, "cc-host-name", update.Extension.HostName)
	assert.Equal(t, "op", update.Extension.Operator)
	assert.Nil(t, update.Extension.TCloudCvmExtension)
}

func TestIsHostCCFieldChange(t *testing.T) {
	newDBHost := func() cvm.Cvm[cvm.TCloudZiyanHostExtension] {
		return cvm.Cvm[cvm.TCloudZiyanHostExtension]{
			BaseCvm: cvm.BaseCvm{
				CloudID:              "ins-1",
				BkBizID:              100,
				BkHostID:             1,
				BkCloudID:            0,
				BkAssetID:            "asset-1",
				AccountID:            "account-1",
				Region:               "ap-guangzhou",
				OsName:               "TencentOS 3.1",
				MachineType:          "SA2.MEDIUM4",
				PrivateIPv4Addresses: []string{"10.0.0.1"},
			},
			Extension: &cvm.TCloudZiyanHostExtension{
				HostName:  "host-1",
				Operator:  "op",
				SrvStatus: "运营中",
				BkCpu:     8,
			},
		}
	}

	testCases := []struct {
		name     string
		mutateCC func(host *cvm.Cvm[cvm.TCloudZiyanHostExtension])
		expected bool
	}{
		{
			name:     "identical cc fields report no change",
			mutateCC: func(host *cvm.Cvm[cvm.TCloudZiyanHostExtension]) {},
			expected: false,
		},
		{
			name:     "biz id change is detected",
			mutateCC: func(host *cvm.Cvm[cvm.TCloudZiyanHostExtension]) { host.BkBizID = 200 },
			expected: true,
		},
		{
			name:     "region change is detected",
			mutateCC: func(host *cvm.Cvm[cvm.TCloudZiyanHostExtension]) { host.Region = "ap-shanghai" },
			expected: true,
		},
		{
			name:     "machine type change is detected",
			mutateCC: func(host *cvm.Cvm[cvm.TCloudZiyanHostExtension]) { host.MachineType = "S5.LARGE8" },
			expected: true,
		},
		{
			name:     "private ip change is detected",
			mutateCC: func(host *cvm.Cvm[cvm.TCloudZiyanHostExtension]) { host.PrivateIPv4Addresses = []string{"10.0.0.2"} },
			expected: true,
		},
		{
			name:     "operator change is detected",
			mutateCC: func(host *cvm.Cvm[cvm.TCloudZiyanHostExtension]) { host.Extension.Operator = "new-op" },
			expected: true,
		},
		{
			name:     "srv status change is detected",
			mutateCC: func(host *cvm.Cvm[cvm.TCloudZiyanHostExtension]) { host.Extension.SrvStatus = "测试中" },
			expected: true,
		},
		{
			// 云来源字段不参与比较，否则会与全量同步来回覆盖
			name: "cloud sourced fields are ignored",
			mutateCC: func(host *cvm.Cvm[cvm.TCloudZiyanHostExtension]) {
				host.Name = "cc-host-name"
				host.Zone = "ap-guangzhou-6"
				host.Status = "STOPPED"
				host.ImageID = "image-new"
				host.CloudVpcIDs = []string{"vpc-cc"}
				host.CloudSubnetIDs = []string{"subnet-cc"}
				host.CloudCreatedTime = "2026-01-01T00:00:00Z"
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dbHost := newDBHost()
			ccExpect := newDBHost()
			tc.mutateCC(&ccExpect)

			assert.Equal(t, tc.expected, isHostCCChange(ccExpect, dbHost))
		})
	}
}

// TestClassifyHostUpdate 覆盖增量更新分支的编排：biz 归属缺失跳过、无变化跳过、
// 有变化才构造更新，以及多台混合时各自的走向。
func TestClassifyHostUpdate(t *testing.T) {
	// 构造一对 cc/db 均存在的主机。dbHost 直接由 convertToHost 产出，保证与 ccHost 完全对齐，
	// 再补上 db 才有的主键 id 与云权威 status（这两个字段 convertToHost 不产出）。
	// 这样「无变化」才是真的无变化，差异只来自 mutateCC 显式改的那些字段。
	newHostPair := func(hostID int64, ccBizID int64, mutateCC func(h *cmdb.Host)) (cmdb.Host,
		cvm.Cvm[cvm.TCloudZiyanHostExtension]) {

		ccHost := cmdb.Host{
			BkHostID:           hostID,
			BkCloudInstID:      "ins-" + strconv.FormatInt(hostID, 10),
			BkAssetID:          "asset-" + strconv.FormatInt(hostID, 10),
			BkHostName:         "host-" + strconv.FormatInt(hostID, 10),
			BkCloudRegion:      "ap-guangzhou",
			BkOSName:           "TencentOS 3.1",
			BkHostInnerIP:      "10.0.0.1",
			Operator:           "op",
			SrvStatus:          "运营中",
			BkCpu:              8,
			BkDisk:             100,
			BkBakOperator:      "bak-op",
			SvrDeviceClassName: "SA2.MEDIUM4",
		}
		// dbHost 与「未 mutate 的 ccHost」对齐：先留一份原始副本用于重建 dbHost，
		// 这样 mutateCC 改什么，就只有什么字段与 db 不一致
		baseCC := ccHost
		if mutateCC != nil {
			mutateCC(&ccHost)
		}

		dbHost := convertToHost(&baseCC, "account-1", ccBizID)
		dbHost.ID = "id-" + strconv.FormatInt(hostID, 10)
		dbHost.Status = "RUNNING"

		return ccHost, dbHost
	}

	classifyUpdates := func(bizMap map[int64]int64, ccHosts []cmdb.Host,
		dbHosts []cvm.Cvm[cvm.TCloudZiyanHostExtension]) []cloud.CvmBatchUpdateWithExtension[cvm.TCloudZiyanHostExtension] {

		_, updates, _ := classifyHost(kit.New(), "account-1", buildCCHostMapByCloudID(kit.New(), ccHosts),
			dbHosts, bizMap)
		return updates
	}

	t.Run("no change produces empty updates", func(t *testing.T) {
		ccHost, dbHost := newHostPair(1, 100, nil)
		updates := classifyUpdates(map[int64]int64{1: 100}, []cmdb.Host{ccHost},
			[]cvm.Cvm[cvm.TCloudZiyanHostExtension]{dbHost})
		assert.Empty(t, updates)
	})

	t.Run("host missing biz id is skipped", func(t *testing.T) {
		ccHost, dbHost := newHostPair(1, 100, func(h *cmdb.Host) { h.Operator = "new-op" })
		// hostBizIDMap 里没有 hostID 1，模拟 cc 查不到业务关系
		updates := classifyUpdates(map[int64]int64{}, []cmdb.Host{ccHost},
			[]cvm.Cvm[cvm.TCloudZiyanHostExtension]{dbHost})
		assert.Empty(t, updates)
	})

	t.Run("cc field change produces update", func(t *testing.T) {
		ccHost, dbHost := newHostPair(1, 100, func(h *cmdb.Host) { h.Operator = "new-op" })
		updates := classifyUpdates(map[int64]int64{1: 100}, []cmdb.Host{ccHost},
			[]cvm.Cvm[cvm.TCloudZiyanHostExtension]{dbHost})

		assert.Len(t, updates, 1)
		assert.Equal(t, "id-1", updates[0].ID)
		assert.Equal(t, "new-op", updates[0].Extension.Operator)
		// 云权威 status 回填 db 现值
		assert.Equal(t, "RUNNING", updates[0].Status)
	})

	t.Run("mixed batch splits changed and unchanged", func(t *testing.T) {
		ccHost1, dbHost1 := newHostPair(1, 100, func(h *cmdb.Host) { h.Operator = "new-op" })
		ccHost2, dbHost2 := newHostPair(2, 100, nil)
		ccHost3, dbHost3 := newHostPair(3, 100, func(h *cmdb.Host) { h.Operator = "another-op" })

		updates := classifyUpdates(map[int64]int64{1: 100, 2: 100}, []cmdb.Host{ccHost1, ccHost2, ccHost3},
			[]cvm.Cvm[cvm.TCloudZiyanHostExtension]{dbHost1, dbHost2, dbHost3})

		// 只有 hostID 1 有变化且有 biz，应产出唯一一条更新
		assert.Len(t, updates, 1)
		assert.Equal(t, "id-1", updates[0].ID)
	})

	t.Run("biz reassignment is detected and updated", func(t *testing.T) {
		// db 里 biz=100，cc 关系查询返回 biz=200 → 分配业务场景
		ccHost, dbHost := newHostPair(1, 100, nil)
		updates := classifyUpdates(map[int64]int64{1: 200}, []cmdb.Host{ccHost},
			[]cvm.Cvm[cvm.TCloudZiyanHostExtension]{dbHost})

		assert.Len(t, updates, 1)
		assert.Equal(t, int64(200), updates[0].BkBizID)
	})
}
