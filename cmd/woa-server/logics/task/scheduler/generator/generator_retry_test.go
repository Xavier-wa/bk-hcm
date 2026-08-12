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

package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"hcm/cmd/woa-server/model/task"
	types "hcm/cmd/woa-server/types/task"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
	"hcm/pkg/serviced"
	cvt "hcm/pkg/tools/converter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDiscover 返回固定的 data-service 地址，真实请求由 fakeHTTPClient 拦截。
type fakeDiscover struct{}

func (fakeDiscover) Discover(cc.Name) ([]string, error) {
	return []string{"http://fake-data-service"}, nil
}

func (fakeDiscover) Services() []cc.Name {
	return []cc.Name{cc.DataServiceName}
}

func (d fakeDiscover) ByLabels([]string) serviced.Discover {
	return d
}

func (fakeDiscover) GetServiceAllNodeKeys(cc.Name) ([]string, error) {
	return nil, nil
}

// fakeHTTPClient 按 URL 后缀拦截 data-service 请求，行为由 listFunc/updateFunc 驱动，按用例替换。
type fakeHTTPClient struct {
	listCalls  int
	listFunc   func(body []byte) (*cvmapplyproto.ZiyanCvmGenerateRecordListResult, error)
	updateFunc func(req *cvmapplyproto.BatchUpdateZiyanCvmGenerateRecordReq) error
}

func (f *fakeHTTPClient) reset() {
	f.listCalls = 0
	f.listFunc = nil
	f.updateFunc = nil
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	switch {
	case strings.HasSuffix(req.URL.Path, "/cvm_apply/generate_records/list"):
		f.listCalls++
		if f.listFunc == nil {
			return nil, fmt.Errorf("unexpected list request: %s", req.URL.Path)
		}
		result, err := f.listFunc(body)
		if err != nil {
			return nil, err
		}
		return jsonHTTPResp(&cvmapplyproto.ZiyanCvmGenerateRecordListResp{Data: result})

	case strings.HasSuffix(req.URL.Path, "/cvm_apply/generate_records/batch"):
		updateReq := new(cvmapplyproto.BatchUpdateZiyanCvmGenerateRecordReq)
		if err := json.Unmarshal(body, updateReq); err != nil {
			return nil, err
		}
		if f.updateFunc == nil {
			return nil, fmt.Errorf("unexpected batch update request: %s", req.URL.Path)
		}
		if err := f.updateFunc(updateReq); err != nil {
			return nil, err
		}
		return jsonHTTPResp(&rest.BaseResp{})

	default:
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}
}

func jsonHTTPResp(v interface{}) (*http.Response, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewReader(data)),
		Header:     make(http.Header),
	}, nil
}

// fakeFinalizer 替代 matcher 记录收尾调用。
type fakeFinalizer struct {
	calls     int
	genRecord *types.GenerateRecord
	order     *types.ApplyOrder
	err       error
}

func (f *fakeFinalizer) FinalApplyStep(_ *kit.Kit, genRecord *types.GenerateRecord,
	order *types.ApplyOrder) error {

	f.calls++
	f.genRecord = genRecord
	f.order = order
	return f.err
}

var (
	testHTTPClient = &fakeHTTPClient{}
	setupModelOnce sync.Once
)

// setupModel 用 fake client set 初始化 model 单例，进程内只生效一次，用例间靠替换 listFunc/updateFunc 隔离。
func setupModel(t *testing.T) {
	setupModelOnce.Do(func() {
		clientSet := client.NewClientSet(testHTTPClient, fakeDiscover{})
		model.InitOperation(clientSet)
	})
	t.Cleanup(testHTTPClient.reset)
}

// newGenRecordRow 构造 data-service 返回的生产记录行，start_at/end_at 需满足转换函数的 RFC3339 解析。
func newGenRecordRow(generateID, suborderID string, status enumor.GenerateStepStatus,
	isMatched bool) *cvmapplytable.ZiyanCvmGenerateRecord {

	return &cvmapplytable.ZiyanCvmGenerateRecord{
		GenerateID: generateID,
		SuborderID: suborderID,
		Status:     cvt.ValToPtr(status),
		IsMatched:  cvt.ValToPtr(isMatched),
		TotalNum:   cvt.ValToPtr(uint(2)),
		StartAt:    "2026-08-01T10:00:00Z",
		EndAt:      "2026-08-01T10:05:00Z",
	}
}

// stubSuborderRecords 按 suborder_id 查询时返回给定记录。
func stubSuborderRecords(records ...*cvmapplytable.ZiyanCvmGenerateRecord) func(body []byte) (
	*cvmapplyproto.ZiyanCvmGenerateRecordListResult, error) {

	return func(body []byte) (*cvmapplyproto.ZiyanCvmGenerateRecordListResult, error) {
		return &cvmapplyproto.ZiyanCvmGenerateRecordListResult{
			Count:   uint64(len(records)),
			Details: records,
		}, nil
	}
}

func newOrder(subOrderID string, totalNum uint) *types.ApplyOrder {
	return &types.ApplyOrder{
		SubOrderId: subOrderID,
		TotalNum:   totalNum,
	}
}

func newDevice(generateID string, isDelivered bool) *types.DeviceInfo {
	return &types.DeviceInfo{
		GenerateId:  generateID,
		IsDelivered: isDelivered,
	}
}

func TestGenerator_RetryMatchDevice(t *testing.T) {
	setupModel(t)

	const subOrderID = "100000-1"

	tests := []struct {
		name            string
		devices         []*types.DeviceInfo
		totalNum        uint
		generatingCount uint
		// listFunc 定制 data-service 查询响应，nil 表示本用例不应触发查询
		listFunc func(body []byte) (*cvmapplyproto.ZiyanCvmGenerateRecordListResult, error)
		// wantUpdateIDs 期望被重置为 is_matched=false 的 generate_id（去重后）
		wantUpdateIDs []string
		wantFinalize  bool
		wantListCalls int
		wantErr       bool
	}{
		{
			name: "部分交付，重置未交付设备对应的生产记录",
			devices: []*types.DeviceInfo{
				newDevice("gen-1", true),
				newDevice("gen-2", false),
				// 同一条生产记录的多台设备去重后只重置一次
				newDevice("gen-2", false),
			},
			totalNum: 3,
			listFunc: func(body []byte) (*cvmapplyproto.ZiyanCvmGenerateRecordListResult, error) {
				// UpdateGenerateRecord 内部先按 generate_id 捞记录再批量更新
				row := newGenRecordRow("gen-2", subOrderID, enumor.GenerateStatusSuccess, true)
				return &cvmapplyproto.ZiyanCvmGenerateRecordListResult{
					Count:   1,
					Details: []*cvmapplytable.ZiyanCvmGenerateRecord{row},
				}, nil
			},
			wantUpdateIDs: []string{"gen-2"},
			wantListCalls: 1,
		},
		{
			name: "全交付且足额，无在途批次，直接收尾",
			devices: []*types.DeviceInfo{
				newDevice("gen-1", true),
				newDevice("gen-1", true),
				newDevice("gen-1", true),
			},
			totalNum:        3,
			generatingCount: 0,
			listFunc: stubSuborderRecords(
				newGenRecordRow("gen-1", subOrderID, enumor.GenerateStatusSuccess, true)),
			wantFinalize:  true,
			wantListCalls: 1,
		},
		{
			name: "全交付但数量不足子单需求，不介入",
			devices: []*types.DeviceInfo{
				newDevice("gen-1", true),
				newDevice("gen-1", true),
			},
			totalNum:        3,
			generatingCount: 0,
			wantListCalls:   0,
		},
		{
			name: "全交付足额但存在在途生产批次，收尾交给在途批次",
			devices: []*types.DeviceInfo{
				newDevice("gen-1", true),
				newDevice("gen-1", true),
				newDevice("gen-1", true),
			},
			totalNum:        3,
			generatingCount: 1,
			wantListCalls:   0,
		},
		{
			name: "查询生产记录失败，错误透传",
			devices: []*types.DeviceInfo{
				newDevice("gen-1", true),
			},
			totalNum: 1,
			listFunc: func(body []byte) (*cvmapplyproto.ZiyanCvmGenerateRecordListResult, error) {
				return nil, errors.New("data-service unavailable")
			},
			wantListCalls: 1,
			wantErr:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testHTTPClient.reset()
			testHTTPClient.listFunc = tc.listFunc

			var updateReqs []cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq
			testHTTPClient.updateFunc = func(req *cvmapplyproto.BatchUpdateZiyanCvmGenerateRecordReq) error {
				updateReqs = append(updateReqs, req.GenerateRecords...)
				return nil
			}

			finalizer := &fakeFinalizer{}
			g := &Generator{matcher: finalizer}
			order := newOrder(subOrderID, tc.totalNum)

			err := g.retryMatchDevice(kit.New(), order, tc.devices, tc.generatingCount)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			var gotUpdateIDs []string
			for _, req := range updateReqs {
				gotUpdateIDs = append(gotUpdateIDs, req.GenerateID)
				assert.NotNil(t, req.IsMatched)
				assert.False(t, *req.IsMatched)
			}
			assert.Equal(t, tc.wantUpdateIDs, gotUpdateIDs)

			if tc.wantFinalize {
				assert.Equal(t, 1, finalizer.calls)
				assert.Same(t, order, finalizer.order)
				require.NotNil(t, finalizer.genRecord)
				assert.Equal(t, subOrderID, finalizer.genRecord.SubOrderId)
			} else {
				assert.Equal(t, 0, finalizer.calls)
			}

			assert.Equal(t, tc.wantListCalls, testHTTPClient.listCalls)
		})
	}
}

func TestGenerator_FinalizeDeliveredOrder(t *testing.T) {
	setupModel(t)

	const subOrderID = "100000-1"

	tests := []struct {
		name string
		// records 子单名下全部生产记录
		records     []*cvmapplytable.ZiyanCvmGenerateRecord
		listErr     error
		finalizeErr error
		wantErr     bool
		wantCalls   int
		// wantGenID 期望传给收尾的生产记录 ID，wantCalls 为 1 时校验
		wantGenID string
	}{
		{
			name: "仍有 Init 批次在途，跳过收尾",
			records: []*cvmapplytable.ZiyanCvmGenerateRecord{
				newGenRecordRow("gen-1", subOrderID, enumor.GenerateStatusSuccess, true),
				newGenRecordRow("gen-2", subOrderID, enumor.GenerateStatusInit, false),
			},
			wantCalls: 0,
		},
		{
			name: "仍有 Handling 批次在途，跳过收尾",
			records: []*cvmapplytable.ZiyanCvmGenerateRecord{
				newGenRecordRow("gen-1", subOrderID, enumor.GenerateStatusHandling, false),
			},
			wantCalls: 0,
		},
		{
			name: "存在未匹配成功记录，留给 informer 收尾，避免并发",
			records: []*cvmapplytable.ZiyanCvmGenerateRecord{
				newGenRecordRow("gen-1", subOrderID, enumor.GenerateStatusSuccess, false),
			},
			wantCalls: 0,
		},
		{
			name: "无成功记录可收尾，仅告警不报错",
			records: []*cvmapplytable.ZiyanCvmGenerateRecord{
				newGenRecordRow("gen-1", subOrderID, enumor.GenerateStatusFailed, true),
				newGenRecordRow("gen-2", subOrderID, enumor.GenerateStatusSuspend, true),
			},
			wantCalls: 0,
		},
		{
			name: "取第一条已匹配成功记录收尾",
			records: []*cvmapplytable.ZiyanCvmGenerateRecord{
				newGenRecordRow("gen-1", subOrderID, enumor.GenerateStatusFailed, true),
				newGenRecordRow("gen-2", subOrderID, enumor.GenerateStatusSuccess, true),
				newGenRecordRow("gen-3", subOrderID, enumor.GenerateStatusSuccess, true),
			},
			wantCalls: 1,
			wantGenID: "gen-2",
		},
		{
			name:      "查询生产记录失败，错误透传",
			listErr:   errors.New("data-service unavailable"),
			wantErr:   true,
			wantCalls: 0,
		},
		{
			name: "收尾执行失败，错误透传",
			records: []*cvmapplytable.ZiyanCvmGenerateRecord{
				newGenRecordRow("gen-1", subOrderID, enumor.GenerateStatusSuccess, true),
			},
			finalizeErr: errors.New("finalize failed"),
			wantErr:     true,
			wantCalls:   1,
			wantGenID:   "gen-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testHTTPClient.reset()
			if tc.listErr != nil {
				testHTTPClient.listFunc = func(body []byte) (*cvmapplyproto.ZiyanCvmGenerateRecordListResult,
					error) {
					return nil, tc.listErr
				}
			} else {
				testHTTPClient.listFunc = stubSuborderRecords(tc.records...)
			}

			finalizer := &fakeFinalizer{err: tc.finalizeErr}
			g := &Generator{matcher: finalizer}
			order := newOrder(subOrderID, uint(len(tc.records)))

			err := g.finalizeDeliveredOrder(kit.New(), order)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantCalls, finalizer.calls)
			if tc.wantCalls == 1 {
				require.NotNil(t, finalizer.genRecord)
				assert.Equal(t, tc.wantGenID, finalizer.genRecord.GenerateId)
				assert.Same(t, order, finalizer.order)
			}
		})
	}
}
