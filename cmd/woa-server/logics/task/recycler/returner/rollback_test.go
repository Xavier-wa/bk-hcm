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

package returner

import (
	"errors"
	"testing"

	"hcm/cmd/woa-server/dal/task/table"
	"hcm/pkg/kit"
	"hcm/pkg/thirdparty/api-gateway/cmdb"

	"github.com/stretchr/testify/assert"
)

// fakeCmdbClient only implements the CMDB methods used by the rollback flow.
// The embedded interface stays nil on purpose: calling any other method panics,
// which proves the rollback flow does not reach beyond its declared dependencies.
type fakeCmdbClient struct {
	cmdb.Client

	internalModuleIDs []int64
	moduleIDsErr      error

	// settledAssets are the asset IDs reported as already in a business internal module.
	settledAssets []string
	listErr       error
	listCalls     int

	transferErr     error
	transferBatches [][]string
}

func (f *fakeCmdbClient) GetBizInternalModuleIDs(_ *kit.Kit, _ int64) ([]int64, error) {
	if f.moduleIDsErr != nil {
		return nil, f.moduleIDsErr
	}
	return f.internalModuleIDs, nil
}

func (f *fakeCmdbClient) ListBizHost(_ *kit.Kit, _ *cmdb.ListBizHostParams) (*cmdb.ListBizHostResult, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	f.listCalls++

	info := make([]cmdb.Host, 0, len(f.settledAssets))
	for i, assetID := range f.settledAssets {
		info = append(info, cmdb.Host{BkHostID: int64(i + 1), BkAssetID: assetID})
	}

	return &cmdb.ListBizHostResult{Count: int64(len(info)), Info: info}, nil
}

func (f *fakeCmdbClient) HostsCrTransit2Idle(_ *kit.Kit, req *cmdb.CrTransitIdleReq) error {
	f.transferBatches = append(f.transferBatches, req.AssetIDs)
	return f.transferErr
}

func newTestReturner(cli *fakeCmdbClient) *Returner {
	return &Returner{cmdbCli: cli}
}

func testOrder() *table.RecycleOrder {
	return &table.RecycleOrder{
		OrderID:    1,
		SuborderID: "sub-1",
		BizID:      100,
		Status:     table.RecycleStatusReturnFailed,
	}
}

func testHosts(assetIDs ...string) []*table.RecycleHost {
	hosts := make([]*table.RecycleHost, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		hosts = append(hosts, &table.RecycleHost{
			SuborderID: "sub-1",
			BizID:      100,
			AssetID:    assetID,
			Status:     table.RecycleStatusReturnFailed,
		})
	}
	return hosts
}

// TestReturnFailedHostFilter verifies the host level filter keeps the status condition,
// without which hosts already recycled by the company side would be rolled back too.
func TestReturnFailedHostFilter(t *testing.T) {
	filter := returnFailedHostFilter("sub-1")

	assert.Equal(t, "sub-1", filter["suborder_id"])
	assert.Equal(t, table.RecycleStatusReturnFailed, filter["status"])
	assert.Len(t, filter, 2)
}

func TestFilterUnsettledAssets(t *testing.T) {
	testCases := []struct {
		name     string
		assetIDs []string
		settled  map[string]struct{}
		expected []string
	}{
		{
			name:     "none settled",
			assetIDs: []string{"a", "b"},
			settled:  map[string]struct{}{},
			expected: []string{"a", "b"},
		},
		{
			name:     "all settled",
			assetIDs: []string{"a", "b"},
			settled:  map[string]struct{}{"a": {}, "b": {}},
			expected: []string{},
		},
		{
			name:     "partially settled",
			assetIDs: []string{"a", "b", "c"},
			settled:  map[string]struct{}{"b": {}},
			expected: []string{"a", "c"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, filterUnsettledAssets(tc.assetIDs, tc.settled))
		})
	}
}

// TestRollbackHostsToBizIdle_AllSettled covers the idempotent branch: every host already
// sits in a business internal module, so no CMDB transfer may be issued.
func TestRollbackHostsToBizIdle_AllSettled(t *testing.T) {
	cli := &fakeCmdbClient{
		internalModuleIDs: []int64{1, 2, 3},
		settledAssets:     []string{"asset-1", "asset-2"},
	}

	err := newTestReturner(cli).rollbackHostsToBizIdle(kit.New(), testOrder(),
		testHosts("asset-1", "asset-2"))

	assert.NoError(t, err)
	assert.Empty(t, cli.transferBatches, "no transfer should be issued for settled hosts")
}

// TestRollbackHostsToBizIdle_PartiallySettled verifies only the hosts still stuck in the
// transit pool are transferred.
func TestRollbackHostsToBizIdle_PartiallySettled(t *testing.T) {
	cli := &fakeCmdbClient{
		internalModuleIDs: []int64{1, 2, 3},
		settledAssets:     []string{"asset-1"},
	}

	err := newTestReturner(cli).rollbackHostsToBizIdle(kit.New(), testOrder(),
		testHosts("asset-1", "asset-2"))

	assert.NoError(t, err)
	assert.Equal(t, [][]string{{"asset-2"}}, cli.transferBatches)
}

func TestRollbackHostsToBizIdle_TransferFailed(t *testing.T) {
	cli := &fakeCmdbClient{
		internalModuleIDs: []int64{1, 2, 3},
		transferErr:       errors.New("cmdb unavailable"),
	}

	err := newTestReturner(cli).rollbackHostsToBizIdle(kit.New(), testOrder(),
		testHosts("asset-1"))

	assert.Error(t, err)
	assert.Equal(t, [][]string{{"asset-1"}}, cli.transferBatches)
}

func TestRollbackHostsToBizIdle_ModuleQueryFailed(t *testing.T) {
	cli := &fakeCmdbClient{moduleIDsErr: errors.New("topo unavailable")}

	err := newTestReturner(cli).rollbackHostsToBizIdle(kit.New(), testOrder(),
		testHosts("asset-1"))

	assert.Error(t, err)
	assert.Empty(t, cli.transferBatches, "no transfer should be issued when module query failed")
}

func TestRollbackHostsToBizIdle_ListFailed(t *testing.T) {
	cli := &fakeCmdbClient{
		internalModuleIDs: []int64{1, 2, 3},
		listErr:           errors.New("cmdb unavailable"),
	}

	err := newTestReturner(cli).rollbackHostsToBizIdle(kit.New(), testOrder(),
		testHosts("asset-1"))

	assert.Error(t, err)
	assert.Empty(t, cli.transferBatches, "no transfer should be issued when location query failed")
}
