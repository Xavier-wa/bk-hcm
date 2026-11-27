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

package iacstate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"hcm/pkg/api/core"
	coreiac "hcm/pkg/api/core/iac"
	dataiac "hcm/pkg/api/data-service/iac"
	dataservice "hcm/pkg/client/data-service"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	restcli "hcm/pkg/rest/client"
	"hcm/pkg/runtime/filter"
)

type fakeStateDataClient struct {
	mu          sync.Mutex
	seq         int
	rows        map[string]coreiac.IacState
	lastUpdates []dataiac.IacStateUpdateField
}

func newFakeStateDataClient() *fakeStateDataClient {
	return &fakeStateDataClient{
		rows: make(map[string]coreiac.IacState),
	}
}

type fakeDiscovery struct{}

func (fakeDiscovery) GetServers() ([]string, error) {
	return []string{"http://127.0.0.1"}, nil
}

type inMemoryHTTPClient struct {
	handler http.HandlerFunc
}

func (c *inMemoryHTTPClient) Do(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	c.handler(rec, req)
	return rec.Result(), nil
}

func newMockDataServiceClient(fake *fakeStateDataClient) *dataservice.Client {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		kt := kit.New()
		switch {
		case strings.HasSuffix(r.URL.Path, "/iac_state/batch/create"):
			req := new(dataiac.IacStateBatchCreateReq)
			_ = json.NewDecoder(r.Body).Decode(req)
			res, err := fake.BatchCreate(kt, req)
			writeMockResp(w, res, err)
		case strings.HasSuffix(r.URL.Path, "/iac_state/batch/update"):
			req := new(dataiac.IacStateBatchUpdateReq)
			_ = json.NewDecoder(r.Body).Decode(req)
			writeMockResp(w, nil, fake.BatchUpdate(kt, req))
		case strings.HasSuffix(r.URL.Path, "/iac_state/batch"):
			req := new(dataiac.IacStateBatchDeleteReq)
			_ = json.NewDecoder(r.Body).Decode(req)
			writeMockResp(w, nil, fake.BatchDelete(kt, req))
		case strings.HasSuffix(r.URL.Path, "/iac_state/list"):
			req := new(dataiac.IacStateListReq)
			_ = json.NewDecoder(r.Body).Decode(req)
			res, err := fake.List(kt, req)
			writeMockResp(w, res, err)
		default:
			writeMockResp(w, nil, fmt.Errorf("unexpected path: %s", r.URL.Path))
		}
	}
	return dataservice.NewClient(&restcli.Capability{
		Client:   &inMemoryHTTPClient{handler: handler},
		Discover: fakeDiscovery{},
	}, "v1")
}

func writeMockResp(w http.ResponseWriter, data any, err error) {
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": errf.Aborted, "message": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"code": errf.OK, "message": "", "data": data})
}

func (f *fakeStateDataClient) BatchCreate(_ *kit.Kit, req *dataiac.IacStateBatchCreateReq) (
	*core.BatchCreateResult, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	ids := make([]string, 0, len(req.Items))
	for _, item := range req.Items {
		f.seq++
		id := fmt.Sprintf("st-%d", f.seq)
		var curPtr *tabletypes.JsonField
		if len(item.Current) > 0 && !item.Current.IsNull() {
			c := item.Current
			curPtr = &c
		}
		dep := item.Deposed
		if len(dep) == 0 {
			dep = tabletypes.JsonField(constant.IacStateEmptyDeposedJSON)
		}
		f.rows[id] = coreiac.IacState{
			ID:               id,
			RepositoryID:     item.RepositoryID,
			BlockID:          item.BlockID,
			Kind:             item.Kind,
			Current:          curPtr,
			Deposed:          dep,
			LastFlowID:       item.LastFlowID,
			ImportFromSpecID: item.ImportFromSpecID,
		}
		ids = append(ids, id)
	}
	return &core.BatchCreateResult{IDs: ids}, nil
}

func (f *fakeStateDataClient) BatchUpdate(_ *kit.Kit, req *dataiac.IacStateBatchUpdateReq) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, item := range req.Items {
		f.lastUpdates = append(f.lastUpdates, item)
		row, ok := f.rows[item.ID]
		if !ok {
			continue
		}
		if item.BlockID != "" {
			row.BlockID = item.BlockID
		}
		if item.Kind != "" {
			row.Kind = item.Kind
		}
		if item.Current != "" {
			if item.Current.IsNull() {
				row.Current = nil
			} else {
				c := item.Current
				row.Current = &c
			}
		}
		if item.Deposed != "" {
			row.Deposed = item.Deposed
		}
		if item.LastFlowID != "" {
			row.LastFlowID = item.LastFlowID
		}
		if item.ImportFromSpecID != nil {
			row.ImportFromSpecID = *item.ImportFromSpecID
		}
		f.rows[item.ID] = row
	}
	return nil
}

func (f *fakeStateDataClient) BatchDelete(_ *kit.Kit, req *dataiac.IacStateBatchDeleteReq) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for id, row := range f.rows {
		if matchExpr(req.Filter, row) {
			delete(f.rows, id)
		}
	}
	return nil
}

func (f *fakeStateDataClient) List(_ *kit.Kit, req *dataiac.IacStateListReq) (*dataiac.IacStateListResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	matched := make([]coreiac.IacState, 0)
	for _, row := range f.rows {
		if matchExpr(req.Filter, row) {
			matched = append(matched, row)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })

	if req.Page != nil && req.Page.Count {
		return &dataiac.IacStateListResult{Count: uint64(len(matched))}, nil
	}

	start := 0
	limit := len(matched)
	if req.Page != nil {
		start = int(req.Page.Start)
		if req.Page.Limit > 0 {
			limit = int(req.Page.Limit)
		}
	}
	if start >= len(matched) {
		return &dataiac.IacStateListResult{Details: []coreiac.IacState{}}, nil
	}
	end := start + limit
	if end > len(matched) {
		end = len(matched)
	}
	return &dataiac.IacStateListResult{Details: matched[start:end]}, nil
}

func matchExpr(expr *filter.Expression, row coreiac.IacState) bool {
	if expr == nil {
		return true
	}
	for _, r := range expr.Rules {
		atom, ok := r.(*filter.AtomRule)
		if !ok {
			continue
		}
		var fieldVal string
		switch atom.Field {
		case "id":
			fieldVal = row.ID
		case "repository_id":
			fieldVal = row.RepositoryID
		case "block_id":
			fieldVal = row.BlockID
		case "import_from_spec_id":
			fieldVal = row.ImportFromSpecID
		default:
			continue
		}
		switch atom.Op {
		case filter.Equal.Factory():
			if fmt.Sprint(atom.Value) != fieldVal {
				return false
			}
		case filter.In.Factory():
			hit := false
			switch vals := atom.Value.(type) {
			case []string:
				for _, v := range vals {
					if v == fieldVal {
						hit = true
						break
					}
				}
			case []any:
				for _, v := range vals {
					if fmt.Sprint(v) == fieldVal {
						hit = true
						break
					}
				}
			default:
				return false
			}
			if !hit {
				return false
			}
		}
	}
	return true
}

func TestStateManager_GetRepositoryAndDependencyStates(t *testing.T) {
	if _, err := NewStateManager(nil); err == nil {
		t.Fatal("expected error when dataCli is nil")
	}

	kt := kit.New()
	fake := newFakeStateDataClient()
	mgr, err := NewStateManager(newMockDataServiceClient(fake))
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// 空仓库返回 nil
	empty, err := mgr.GetRepositoryStates(kt, "repo-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if empty != nil {
		t.Fatalf("expected nil for empty repository, got %v", empty)
	}

	// 写入两条正常态记录与一条暂存态记录
	if err := mgr.UpsertAppliedState(kt, &UpsertAppliedReq{
		RepositoryID: "repo-1",
		BlockID:      "tencentcloud_vpc.main",
		Kind:         enumor.IacStateKindResource,
		Object: &StateObject{Plugin: "tcloud", PluginInstanceID: "inst-1", Type: "tencentcloud_vpc",
			State: map[string]any{"id": "vpc-1"}},
		FlowID: "flow-1",
	}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	if err := mgr.UpsertAppliedState(kt, &UpsertAppliedReq{
		RepositoryID: "repo-1",
		BlockID:      "tencentcloud_subnet.app",
		Kind:         enumor.IacStateKindResource,
		Object: &StateObject{Plugin: "tcloud", PluginInstanceID: "inst-1", Type: "tencentcloud_subnet",
			State: map[string]any{"id": "sub-app"}},
		FlowID: "flow-1",
	}); err != nil {
		t.Fatalf("upsert subnet failed: %v", err)
	}

	// 插入一条 import 暂存记录，应被默认过滤
	_, _ = fake.BatchCreate(kt, &dataiac.IacStateBatchCreateReq{
		Items: []dataiac.IacStateCreateField{
			{
				RepositoryID: "repo-1",
				BlockID:      "tencentcloud_subnet.staging",
				Kind:         enumor.IacStateKindResource,
				Current: tabletypes.JsonField(`{"plugin":"tcloud","type":"tencentcloud_subnet",` +
					`"state":{"id":"sub-1"}}`),
				Deposed:          tabletypes.JsonField(`{}`),
				ImportFromSpecID: "spec-9",
			},
		},
	})

	// blockIDs 为空时返回全部已生效记录
	states, err := mgr.GetRepositoryStates(kt, "repo-1", nil)
	if err != nil {
		t.Fatalf("GetRepositoryStates failed: %v", err)
	}
	if len(states) != 2 || states[0].BlockID != "tencentcloud_vpc.main" {
		t.Fatalf("expected 2 active states, got %+v", states)
	}
	if states[0].CurrentOrNil() == nil || states[0].CurrentOrNil().State["id"] != "vpc-1" {
		t.Fatalf("unexpected current state: %+v", states[0].CurrentOrNil())
	}

	// blockIDs 非空时仅返回指定 block_id 的已生效记录，且排除暂存记录
	filtered, err := mgr.GetRepositoryStates(kt, "repo-1",
		[]string{"tencentcloud_vpc.main", "tencentcloud_subnet.staging", "missing.block"})
	if err != nil {
		t.Fatalf("GetRepositoryStates with blockIDs failed: %v", err)
	}
	if len(filtered) != 1 || filtered[0].BlockID != "tencentcloud_vpc.main" {
		t.Fatalf("expected only tencentcloud_vpc.main, got %+v", filtered)
	}

	// blockIDs 均未命中时返回 nil
	noMatch, err := mgr.GetRepositoryStates(kt, "repo-1", []string{"missing.block"})
	if err != nil {
		t.Fatalf("GetRepositoryStates with missing blockIDs failed: %v", err)
	}
	if noMatch != nil {
		t.Fatalf("expected nil when no blockIDs matched, got %+v", noMatch)
	}

	// 定向依赖查询成功，直接返回 Current.State 字典
	deps, err := mgr.GetDependencyStates(kt, "repo-1", []string{"tencentcloud_vpc.main"})
	if err != nil {
		t.Fatalf("GetDependencyStates failed: %v", err)
	}
	if len(deps) != 1 || deps["tencentcloud_vpc.main"]["id"] != "vpc-1" {
		t.Fatalf("expected dependency state vpc-1, got %+v", deps)
	}

	// 缺失依赖或暂存态依赖应报错并包含缺失 block_id
	_, err = mgr.GetDependencyStates(kt, "repo-1", []string{"tencentcloud_vpc.main", "tencentcloud_subnet.staging",
		"missing.block"})
	if err == nil {
		t.Fatal("expected error for missing dependencies")
	}
	if !strings.Contains(err.Error(), "tencentcloud_subnet.staging") ||
		!strings.Contains(err.Error(), "missing.block") {
		t.Fatalf("expected error to list missing block_ids, got: %v", err)
	}
}

func TestStateManager_UpsertImportAndCreateBeforeDestroyLifecycle(t *testing.T) {
	kt := kit.New()
	fake := newFakeStateDataClient()
	mgr, err := NewStateManager(newMockDataServiceClient(fake))
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}

	// 1. import. 前缀剥离并强制 kind=resource，同时验证 plugin_instance_id、depends_on 与 identity 落库还原
	if err := mgr.UpsertAppliedState(kt, &UpsertAppliedReq{
		RepositoryID: "repo-1",
		BlockID:      "import.tencentcloud_instance.web",
		Kind:         enumor.IacStateKindData,
		Object: &StateObject{
			Plugin:           "tcloud",
			PluginInstanceID: "inst-1",
			Type:             "tencentcloud_instance",
			State:            map[string]any{"id": "ins-v1"},
			DependsOn:        []string{"tencentcloud_vpc.main"},
			Identity:         map[string]any{"instance_id": "ins-v1"},
		},
		FlowID: "flow-import",
	}); err != nil {
		t.Fatalf("import upsert failed: %v", err)
	}

	states, err := mgr.GetRepositoryStates(kt, "repo-1", nil)
	if err != nil || len(states) != 1 {
		t.Fatalf("expected 1 state after import, got %v, err=%v", states, err)
	}
	if states[0].BlockID != "tencentcloud_instance.web" || states[0].Kind != enumor.IacStateKindResource {
		t.Fatalf("expected stripped block_id and resource kind, got %+v", states[0])
	}
	if states[0].Current.PluginInstanceID != "inst-1" ||
		len(states[0].Current.DependsOn) != 1 || states[0].Current.DependsOn[0] != "tencentcloud_vpc.main" ||
		states[0].Current.Identity["instance_id"] != "ins-v1" {
		t.Fatalf("expected plugin_instance_id, depends_on and identity preserved, got %+v", states[0].Current)
	}

	// 2. 先建后删轮转：旧 current(inst-1) 转入 deposed["a1b2c3d4"]，新对象(inst-2) 写入 current
	if err := mgr.UpsertAppliedState(kt, &UpsertAppliedReq{
		RepositoryID: "repo-1",
		BlockID:      "tencentcloud_instance.web",
		Kind:         enumor.IacStateKindResource,
		DeposedKey:   "a1b2c3d4",
		Object: &StateObject{
			Plugin:           "tcloud",
			PluginInstanceID: "inst-2",
			Type:             "tencentcloud_instance",
			State:            map[string]any{"id": "ins-v2"},
		},
		FlowID: "flow-cbd",
	}); err != nil {
		t.Fatalf("cbd upsert failed: %v", err)
	}

	// 2.1 重复传入已存在的 deposed_key 必须报错拒绝覆盖
	dupErr := mgr.UpsertAppliedState(kt, &UpsertAppliedReq{
		RepositoryID: "repo-1",
		BlockID:      "tencentcloud_instance.web",
		Kind:         enumor.IacStateKindResource,
		DeposedKey:   "a1b2c3d4",
		Object: &StateObject{
			Plugin:           "tcloud",
			PluginInstanceID: "inst-3",
			Type:             "tencentcloud_instance",
			State:            map[string]any{"id": "ins-v3"},
		},
		FlowID: "flow-cbd-dup",
	})
	if dupErr == nil {
		t.Fatal("expected error when deposed_key already exists")
	}

	states, _ = mgr.GetRepositoryStates(kt, "repo-1", nil)
	if states[0].Current.State["id"] != "ins-v2" || states[0].Current.PluginInstanceID != "inst-2" {
		t.Fatalf("expected new current ins-v2 with inst-2, got %+v", states[0].Current)
	}
	if states[0].Deposed["a1b2c3d4"] == nil || states[0].Deposed["a1b2c3d4"].State["id"] != "ins-v1" ||
		states[0].Deposed["a1b2c3d4"].PluginInstanceID != "inst-1" {
		t.Fatalf("expected old instance ins-v1 with inst-1 in deposed, got %+v", states[0].Deposed)
	}

	// 3. 销毁 current 但 deposed 非空：只置 current = SQL NULL，保留整行
	if err := mgr.RemoveDestroyedState(kt, &RemoveDestroyedReq{
		RepositoryID: "repo-1",
		BlockID:      "tencentcloud_instance.web",
		FlowID:       "flow-destroy-cur",
	}); err != nil {
		t.Fatalf("remove current failed: %v", err)
	}

	lastUpd := fake.lastUpdates[len(fake.lastUpdates)-1]
	if !lastUpd.Current.IsNull() {
		t.Fatalf("expected Current == \"null\" in BatchUpdate, got %q", lastUpd.Current)
	}
	states, _ = mgr.GetRepositoryStates(kt, "repo-1", nil)
	if len(states) != 1 || states[0].CurrentOrNil() != nil || len(states[0].Deposed) != 1 {
		t.Fatalf("expected row preserved with nil current and 1 deposed, got %+v", states)
	}

	// current == nil 时作为依赖查询必须报错
	if _, err := mgr.GetDependencyStates(kt, "repo-1", []string{"tencentcloud_instance.web"}); err == nil {
		t.Fatal("expected dependency query to fail when current is nil")
	}

	// 4. 清理最后一个 deposed 键后 current 与 deposed 均空，整行物理删除
	if err := mgr.RemoveDestroyedState(kt, &RemoveDestroyedReq{
		RepositoryID: "repo-1",
		BlockID:      "tencentcloud_instance.web",
		DeposedKey:   "a1b2c3d4",
		FlowID:       "flow-destroy-dep",
	}); err != nil {
		t.Fatalf("remove deposed failed: %v", err)
	}

	states, _ = mgr.GetRepositoryStates(kt, "repo-1", nil)
	if states != nil {
		t.Fatalf("expected row deleted after all instances destroyed, got %+v", states)
	}

	// 5. 对不存在的记录执行 RemoveDestroyedState 幂等返回 nil
	if err := mgr.RemoveDestroyedState(kt, &RemoveDestroyedReq{
		RepositoryID: "repo-1",
		BlockID:      "tencentcloud_instance.web",
		FlowID:       "flow-idempotent",
	}); err != nil {
		t.Fatalf("expected idempotent nil on missing row, got %v", err)
	}
}
