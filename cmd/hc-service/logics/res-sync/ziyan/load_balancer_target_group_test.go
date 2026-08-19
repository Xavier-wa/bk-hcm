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

package ziyan

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"hcm/pkg/adaptor/poller"
	"hcm/pkg/adaptor/tcloud"
	typelb "hcm/pkg/adaptor/types/load-balancer"
	corelb "hcm/pkg/api/core/cloud/load-balancer"
	dataservice "hcm/pkg/client/data-service"
	"hcm/pkg/kit"
	restclient "hcm/pkg/rest/client"
	"hcm/pkg/tools/converter"

	"github.com/stretchr/testify/assert"
	clb "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/clb/v20180317"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	bpaas "hcm/pkg/thirdparty/tencentcloud/bpaas/v20181217"
)

// fakeTCloudZiyan implements TCloudZiyan for testing.
type fakeTCloudZiyan struct {
	tcloud.TCloud
	listTargetsFunc func(kt *kit.Kit, opt *typelb.TCloudListTargetsOption) ([]typelb.TCloudListenerTarget, error)
}

func (f *fakeTCloudZiyan) ListTargets(kt *kit.Kit, opt *typelb.TCloudListTargetsOption) (
	[]typelb.TCloudListenerTarget, error) {

	if f.listTargetsFunc != nil {
		return f.listTargetsFunc(kt, opt)
	}
	return nil, errors.New("ListTargets not implemented")
}

func (f *fakeTCloudZiyan) GetBPaasApplicationDetail(kt *kit.Kit, applicationID uint64) (
	*bpaas.GetBpaasApplicationDetailResponseParams, error) {

	return nil, errors.New("not implemented")
}

func (f *fakeTCloudZiyan) CreateZiyanLoadBalancer(kt *kit.Kit, opt *typelb.TCloudZiyanCreateClbOption) (
	*poller.BaseDoneResult, error) {

	return nil, errors.New("not implemented")
}

func (f *fakeTCloudZiyan) DescribeSlaCapacity(kt *kit.Kit, opt *typelb.TCloudDescribeSlaCapacityOption) (
	*clb.DescribeSlaCapacityResponseParams, error) {

	return nil, errors.New("not implemented")
}

// newTestClient creates a client with fake cloud and real data-service client pointing to test server.
func newTestClient(t *testing.T, cloudCli *fakeTCloudZiyan, handler http.HandlerFunc) *client {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	capability := &restclient.Capability{
		Client:   server.Client(),
		Discover: &staticDiscover{servers: []string{server.URL}},
	}

	return &client{
		cloudCli: cloudCli,
		dbCli:    dataservice.NewClient(capability, "v1"),
	}
}

type staticDiscover struct {
	servers []string
}

func (s *staticDiscover) GetServers() ([]string, error) {
	return s.servers, nil
}

func (s *staticDiscover) Name() string {
	return "static"
}

func TestListTargetsFromCloud_NoCloudIDs(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{CloudLBID: "lb-xxx"}
	param := &SyncBaseParams{Region: "ap-nanjing"}

	expected := []typelb.TCloudListenerTarget{
		{
			ListenerBackend: &clb.ListenerBackend{
				ListenerId: common.StringPtr("lbl-1"),
			},
		},
	}

	cloudCli := &fakeTCloudZiyan{
		listTargetsFunc: func(kt *kit.Kit, opt *typelb.TCloudListTargetsOption) (
			[]typelb.TCloudListenerTarget, error) {

			assert.Equal(t, "ap-nanjing", opt.Region)
			assert.Equal(t, "lb-xxx", opt.LoadBalancerId)
			assert.Empty(t, opt.ListenerIds)
			return expected, nil
		},
	}

	cli := &client{cloudCli: cloudCli}
	targets, err := cli.listTargetsFromCloud(kt, param, opt)
	assert.NoError(t, err)
	assert.Equal(t, expected, targets)
}

func TestListTargetsFromCloud_WithCloudIDs(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{CloudLBID: "lb-xxx"}
	param := &SyncBaseParams{
		Region:   "ap-nanjing",
		CloudIDs: []string{"lbl-1", "lbl-2", "lbl-3", "lbl-4", "lbl-5", "lbl-6", "lbl-7", "lbl-8", "lbl-9", "lbl-10",
			"lbl-11", "lbl-12", "lbl-13", "lbl-14", "lbl-15", "lbl-16", "lbl-17", "lbl-18", "lbl-19", "lbl-20",
			"lbl-21"},
	}

	callCount := 0
	cloudCli := &fakeTCloudZiyan{
		listTargetsFunc: func(kt *kit.Kit, opt *typelb.TCloudListTargetsOption) (
			[]typelb.TCloudListenerTarget, error) {

			callCount++
			assert.Equal(t, "lb-xxx", opt.LoadBalancerId)
			if callCount == 1 {
				assert.Len(t, opt.ListenerIds, 20)
			} else {
				assert.Len(t, opt.ListenerIds, 1)
			}
			return []typelb.TCloudListenerTarget{
				{ListenerBackend: &clb.ListenerBackend{ListenerId: common.StringPtr("lbl-x")}},
			}, nil
		},
	}

	cli := &client{cloudCli: cloudCli}
	targets, err := cli.listTargetsFromCloud(kt, param, opt)
	assert.NoError(t, err)
	assert.Len(t, targets, 2)
	assert.Equal(t, 2, callCount)
}

func TestListTargetsFromCloud_Error(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{CloudLBID: "lb-xxx"}
	param := &SyncBaseParams{Region: "ap-nanjing"}

	cloudCli := &fakeTCloudZiyan{
		listTargetsFunc: func(kt *kit.Kit, opt *typelb.TCloudListTargetsOption) (
			[]typelb.TCloudListenerTarget, error) {

			return nil, errors.New("cloud api error")
		},
	}

	cli := &client{cloudCli: cloudCli}
	targets, err := cli.listTargetsFromCloud(kt, param, opt)
	assert.Error(t, err)
	assert.Nil(t, targets)
}

func TestListTargetsFromDB_Empty(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{LBID: "lb-xxx", CloudLBID: "lb-xxx"}
	param := &SyncBaseParams{}

	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code":    0,
			"message": "",
			"data": map[string]interface{}{
				"count":   0,
				"details": []corelb.BaseTargetListenerRuleRel{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	cli := newTestClient(t, nil, handler)
	relMap, tgRsMap, err := cli.listTargetsFromDB(kt, param, opt)
	assert.NoError(t, err)
	assert.Empty(t, relMap)
	assert.Empty(t, tgRsMap)
}

func TestListTargetsFromDB_WithRels(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{LBID: "lb-xxx", CloudLBID: "lb-xxx"}
	param := &SyncBaseParams{}

	rels := []corelb.BaseTargetListenerRuleRel{
		{ID: "rel-1", TargetGroupID: "tg-1", CloudListenerRuleID: "rule-1"},
		{ID: "rel-2", TargetGroupID: "tg-2", CloudListenerRuleID: "rule-2"},
		{ID: "rel-3", TargetGroupID: "tg-1", CloudListenerRuleID: "rule-3"},
	}

	targets := []corelb.BaseTarget{
		{ID: "rs-1", TargetGroupID: "tg-1", IP: "1.1.1.1", Port: 80},
		{ID: "rs-2", TargetGroupID: "tg-1", IP: "1.1.1.2", Port: 80},
		{ID: "rs-3", TargetGroupID: "tg-2", IP: "2.2.2.2", Port: 80},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		var resp interface{}
		if r.URL.Path == "/api/v1/data/target_group_listener_rels/list" {
			resp = map[string]interface{}{
				"code":    0,
				"message": "",
				"data": map[string]interface{}{
					"count":   uint64(len(rels)),
					"details": rels,
				},
			}
		} else if r.URL.Path == "/api/v1/data/load_balancers/targets/list" {
			resp = map[string]interface{}{
				"code":    0,
				"message": "",
				"data": map[string]interface{}{
					"count":   uint64(len(targets)),
					"details": targets,
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	cli := newTestClient(t, nil, handler)
	relMap, tgRsMap, err := cli.listTargetsFromDB(kt, param, opt)
	assert.NoError(t, err)
	assert.Len(t, relMap, 3)
	assert.Len(t, tgRsMap, 2)
	assert.Len(t, tgRsMap["tg-1"], 2)
	assert.Len(t, tgRsMap["tg-2"], 1)
}

func TestListTargetsFromDB_TGWithoutRS(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{LBID: "lb-xxx", CloudLBID: "lb-xxx"}
	param := &SyncBaseParams{}

	rels := []corelb.BaseTargetListenerRuleRel{
		{ID: "rel-1", TargetGroupID: "tg-1", CloudListenerRuleID: "rule-1"},
		{ID: "rel-2", TargetGroupID: "tg-empty", CloudListenerRuleID: "rule-2"},
	}

	targets := []corelb.BaseTarget{
		{ID: "rs-1", TargetGroupID: "tg-1", IP: "1.1.1.1", Port: 80},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		var resp interface{}
		if r.URL.Path == "/api/v1/data/target_group_listener_rels/list" {
			resp = map[string]interface{}{
				"code":    0,
				"message": "",
				"data": map[string]interface{}{
					"count":   uint64(len(rels)),
					"details": rels,
				},
			}
		} else if r.URL.Path == "/api/v1/data/load_balancers/targets/list" {
			resp = map[string]interface{}{
				"code":    0,
				"message": "",
				"data": map[string]interface{}{
					"count":   uint64(len(targets)),
					"details": targets,
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	cli := newTestClient(t, nil, handler)
	relMap, tgRsMap, err := cli.listTargetsFromDB(kt, param, opt)
	assert.NoError(t, err)
	assert.Len(t, relMap, 2)
	assert.Len(t, tgRsMap, 2)
	assert.Len(t, tgRsMap["tg-1"], 1)
	// 没有rs的目标组保留key，值为nil，与逐目标组查询时的返回语义保持一致
	targetsOfEmptyTG, exists := tgRsMap["tg-empty"]
	assert.True(t, exists)
	assert.Empty(t, targetsOfEmptyTG)
}

func TestListTargetsFromDB_WithCloudIDs(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{LBID: "lb-xxx", CloudLBID: "lb-xxx"}
	param := &SyncBaseParams{CloudIDs: []string{"lbl-1", "lbl-2"}}

	handler := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// 指定cloud ids时，rel查询需带cloud_lbl_id过滤条件
		assert.Contains(t, string(body), "cloud_lbl_id")
		assert.Contains(t, string(body), "lbl-1")
		resp := map[string]interface{}{
			"code":    0,
			"message": "",
			"data": map[string]interface{}{
				"count":   0,
				"details": []corelb.BaseTargetListenerRuleRel{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	cli := newTestClient(t, nil, handler)
	relMap, tgRsMap, err := cli.listTargetsFromDB(kt, param, opt)
	assert.NoError(t, err)
	assert.Empty(t, relMap)
	assert.Empty(t, tgRsMap)
}

func TestListTargetsFromDB_RelError(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{LBID: "lb-xxx", CloudLBID: "lb-xxx"}
	param := &SyncBaseParams{}

	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code":    500,
			"message": "db error",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	cli := newTestClient(t, nil, handler)
	relMap, tgRsMap, err := cli.listTargetsFromDB(kt, param, opt)
	assert.Error(t, err)
	assert.Nil(t, relMap)
	assert.Nil(t, tgRsMap)
}

func TestListTargetsFromDB_TargetError(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{LBID: "lb-xxx", CloudLBID: "lb-xxx"}
	param := &SyncBaseParams{}

	rels := []corelb.BaseTargetListenerRuleRel{
		{ID: "rel-1", TargetGroupID: "tg-1", CloudListenerRuleID: "rule-1"},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		var resp interface{}
		if r.URL.Path == "/api/v1/data/target_group_listener_rels/list" {
			resp = map[string]interface{}{
				"code":    0,
				"message": "",
				"data": map[string]interface{}{
					"count":   uint64(len(rels)),
					"details": rels,
				},
			}
		} else if r.URL.Path == "/api/v1/data/load_balancers/targets/list" {
			resp = map[string]interface{}{
				"code":    500,
				"message": "db error",
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	cli := newTestClient(t, nil, handler)
	relMap, tgRsMap, err := cli.listTargetsFromDB(kt, param, opt)
	assert.Error(t, err)
	assert.Nil(t, relMap)
	assert.Nil(t, tgRsMap)
}

func TestListAllListenerTargetsFromCloud_Success(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{CloudLBID: "lb-xxx"}
	params := &SyncBaseParams{Region: "ap-nanjing", AccountID: "acc-1"}

	expected := []typelb.TCloudListenerTarget{
		{ListenerBackend: &clb.ListenerBackend{ListenerId: common.StringPtr("lbl-1")}},
		{ListenerBackend: &clb.ListenerBackend{ListenerId: common.StringPtr("lbl-2")}},
	}

	cloudCli := &fakeTCloudZiyan{
		listTargetsFunc: func(kt *kit.Kit, opt *typelb.TCloudListTargetsOption) (
			[]typelb.TCloudListenerTarget, error) {

			assert.Empty(t, opt.ListenerIds)
			return expected, nil
		},
	}

	cli := &client{cloudCli: cloudCli}
	listenerTargetMap := cli.listAllListenerTargetsFromCloud(kt, params, opt)
	assert.NotNil(t, listenerTargetMap)
	assert.Len(t, listenerTargetMap, 2)
	assert.Equal(t, "lbl-1", converter.PtrToVal(listenerTargetMap["lbl-1"].ListenerId))
	assert.Equal(t, "lbl-2", converter.PtrToVal(listenerTargetMap["lbl-2"].ListenerId))
}

func TestListAllListenerTargetsFromCloud_Error(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{CloudLBID: "lb-xxx"}
	params := &SyncBaseParams{Region: "ap-nanjing", AccountID: "acc-1"}

	cloudCli := &fakeTCloudZiyan{
		listTargetsFunc: func(kt *kit.Kit, opt *typelb.TCloudListTargetsOption) (
			[]typelb.TCloudListenerTarget, error) {

			return nil, errors.New("cloud api error")
		},
	}

	cli := &client{cloudCli: cloudCli}
	listenerTargetMap := cli.listAllListenerTargetsFromCloud(kt, params, opt)
	assert.Nil(t, listenerTargetMap)
}

func TestMatchListenerTargets_EmptyCloudIDs(t *testing.T) {
	listenerTargetMap := map[string]typelb.TCloudListenerTarget{
		"lbl-1": {ListenerBackend: &clb.ListenerBackend{ListenerId: common.StringPtr("lbl-1")}},
		"lbl-2": {ListenerBackend: &clb.ListenerBackend{ListenerId: common.StringPtr("lbl-2")}},
	}

	listenerTargets, miss := matchListenerTargets(listenerTargetMap, nil)
	assert.Len(t, listenerTargets, 2)
	assert.Empty(t, miss)
}

func TestMatchListenerTargets_WithCloudIDs(t *testing.T) {
	listenerTargetMap := map[string]typelb.TCloudListenerTarget{
		"lbl-1": {ListenerBackend: &clb.ListenerBackend{ListenerId: common.StringPtr("lbl-1")}},
		"lbl-2": {ListenerBackend: &clb.ListenerBackend{ListenerId: common.StringPtr("lbl-2")}},
	}

	listenerTargets, miss := matchListenerTargets(listenerTargetMap, []string{"lbl-1", "lbl-3"})
	assert.Len(t, listenerTargets, 1)
	assert.Len(t, miss, 1)
	assert.Equal(t, "lbl-1", converter.PtrToVal(listenerTargets[0].ListenerId))
	assert.Equal(t, "lbl-3", miss[0])
}

func TestMatchListenerTargets_AllMiss(t *testing.T) {
	listenerTargetMap := map[string]typelb.TCloudListenerTarget{
		"lbl-1": {ListenerBackend: &clb.ListenerBackend{ListenerId: common.StringPtr("lbl-1")}},
	}

	listenerTargets, miss := matchListenerTargets(listenerTargetMap, []string{"lbl-2", "lbl-3"})
	assert.Empty(t, listenerTargets)
	assert.Len(t, miss, 2)
}

// 未全量拉取时（拉取失败或监听器超阈值跳过），回退到按批云拉取，同步继续
func TestListTargetRelated_WithoutListenerTargets(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{
		LBID:      "lb-xxx",
		CloudLBID: "lb-xxx",
		CachedLoadBalancer: &corelb.TCloudLoadBalancer{
			BaseLoadBalancer: corelb.BaseLoadBalancer{CloudID: "lb-xxx"},
		},
	}
	param := &SyncBaseParams{Region: "ap-nanjing", CloudIDs: []string{"lbl-1"}}

	cloudTargets := []typelb.TCloudListenerTarget{
		{ListenerBackend: &clb.ListenerBackend{ListenerId: common.StringPtr("lbl-1")}},
	}
	cloudCli := &fakeTCloudZiyan{
		listTargetsFunc: func(kt *kit.Kit, opt *typelb.TCloudListTargetsOption) (
			[]typelb.TCloudListenerTarget, error) {

			// 回退路径应带监听器id按批拉取
			assert.Equal(t, []string{"lbl-1"}, opt.ListenerIds)
			return cloudTargets, nil
		},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code":    0,
			"message": "",
			"data": map[string]interface{}{
				"count":   0,
				"details": []corelb.BaseTargetListenerRuleRel{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	cli := newTestClient(t, cloudCli, handler)
	targets, relMap, tgRsMap, lb, err := cli.listTargetRelated(kt, param, opt, nil)
	assert.NoError(t, err)
	assert.Equal(t, cloudTargets, targets)
	assert.Empty(t, relMap)
	assert.Empty(t, tgRsMap)
	assert.Equal(t, "lb-xxx", lb.CloudID)
}

// 已有全量拉取结果时直接使用，不再调用云API
func TestListTargetRelated_WithListenerTargets(t *testing.T) {
	kt := kit.New()
	opt := &SyncListenerOption{
		LBID:      "lb-xxx",
		CloudLBID: "lb-xxx",
		CachedLoadBalancer: &corelb.TCloudLoadBalancer{
			BaseLoadBalancer: corelb.BaseLoadBalancer{CloudID: "lb-xxx"},
		},
	}
	param := &SyncBaseParams{Region: "ap-nanjing", CloudIDs: []string{"lbl-1"}}

	listenerTargets := []typelb.TCloudListenerTarget{
		{ListenerBackend: &clb.ListenerBackend{ListenerId: common.StringPtr("lbl-1")}},
	}
	cloudCli := &fakeTCloudZiyan{
		listTargetsFunc: func(kt *kit.Kit, opt *typelb.TCloudListTargetsOption) (
			[]typelb.TCloudListenerTarget, error) {

			t.Error("should not call cloud api when listener targets provided")
			return nil, nil
		},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code":    0,
			"message": "",
			"data": map[string]interface{}{
				"count":   0,
				"details": []corelb.BaseTargetListenerRuleRel{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	cli := newTestClient(t, cloudCli, handler)
	targets, _, _, lb, err := cli.listTargetRelated(kt, param, opt, listenerTargets)
	assert.NoError(t, err)
	assert.Equal(t, listenerTargets, targets)
	assert.Equal(t, "lb-xxx", lb.CloudID)
}
