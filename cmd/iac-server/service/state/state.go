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

// Package state 提供 iac-server 受管资源状态摘要列表与脱敏详情只读查询接口。
package state

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	logicstate "hcm/cmd/iac-server/logics/state"
	"hcm/cmd/iac-server/service/capability"
	"hcm/pkg/api/core"
	coreiac "hcm/pkg/api/core/iac"
	dataiac "hcm/pkg/api/data-service/iac"
	iacproto "hcm/pkg/api/iac-server"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iac/iacstate"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"

	tfjson "github.com/hashicorp/terraform-json"
)

type service struct {
	client   *client.ClientSet
	resolver *logicstate.SchemaResolver
}

// InitService 初始化并注册受管资源状态只读查询路由。
func InitService(cap *capability.Capability) {
	var resolver *logicstate.SchemaResolver
	if cap.ApiClient != nil {
		resolver, _ = logicstate.NewSchemaResolver(cap.ApiClient)
	}
	svc := &service{
		client:   cap.ApiClient,
		resolver: resolver,
	}

	h := rest.NewHandler()
	h.Add("ListState", http.MethodPost, "/repository/{repository_id}/state/list", svc.ListState)
	h.Add("GetState", http.MethodGet, "/repository/{repository_id}/state/{state_id}", svc.GetState)
	h.Load(cap.WebService)
}

// ListState 分页查询指定仓库下已生效受管资源的状态摘要列表，不返回敏感属性载荷。
func (svc *service) ListState(cts *rest.Contexts) (any, error) {
	repositoryID := cts.PathParameter("repository_id").String()
	if len(repositoryID) == 0 {
		return nil, errf.New(errf.InvalidParameter, "repository_id is required")
	}

	req := new(iacproto.ListStateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("list iac state decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("list iac state validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := svc.ensureRepositoryExists(cts.Kit, repositoryID); err != nil {
		return nil, err
	}

	baseRules := []filter.RuleFactory{
		tools.RuleEqual("repository_id", repositoryID),
		tools.RuleEqual("import_from_spec_id", ""),
	}
	expr, err := mergeFilterExpr(baseRules, req.Filter)
	if err != nil {
		logs.Errorf("merge list iac state filter failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	res, err := svc.client.DataService().Global.IacState.List(cts.Kit, &dataiac.IacStateListReq{
		Filter: expr,
		Page:   req.Page,
	})
	if err != nil {
		logs.Errorf("list iac state from data-service failed, repository_id: %s, err: %v, rid: %s",
			repositoryID, err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	if req.Page.Count {
		return &iacproto.ListStateResp{Count: res.Count}, nil
	}

	details := make([]iacproto.StateListItem, 0, len(res.Details))
	for i := range res.Details {
		item, buildErr := buildStateListItem(&res.Details[i])
		if buildErr != nil {
			logs.Errorf("build state list item failed, id: %s, err: %v, rid: %s",
				res.Details[i].ID, buildErr, cts.Kit.Rid)
			return nil, errf.NewFromErr(errf.Aborted, buildErr)
		}
		details = append(details, item)
	}

	return &iacproto.ListStateResp{Details: details}, nil
}

// GetState 查询指定仓库下单条已生效受管资源状态的脱敏详情。
func (svc *service) GetState(cts *rest.Contexts) (any, error) {
	repositoryID := cts.PathParameter("repository_id").String()
	stateID := cts.PathParameter("state_id").String()
	if len(repositoryID) == 0 || len(stateID) == 0 {
		return nil, errf.New(errf.InvalidParameter, "repository_id and state_id are required")
	}

	if err := svc.ensureRepositoryExists(cts.Kit, repositoryID); err != nil {
		return nil, err
	}

	expr := tools.ExpressionAnd(tools.RuleEqual("id", stateID), tools.RuleEqual("repository_id", repositoryID),
		tools.RuleEqual("import_from_spec_id", ""))
	res, err := svc.client.DataService().Global.IacState.List(cts.Kit, &dataiac.IacStateListReq{
		Filter: expr,
		Page:   &core.BasePage{Start: 0, Limit: 1},
	})
	if err != nil {
		logs.Errorf("get iac state from data-service failed, repository_id: %s, state_id: %s, err: %v, rid: %s",
			repositoryID, stateID, err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}
	if res == nil || len(res.Details) == 0 {
		return nil, errf.Newf(errf.RecordNotFound, "iac state %s not found", stateID)
	}

	row := &res.Details[0]
	currentObj, deposedMap, err := decodeStateRowObjects(row)
	if err != nil {
		logs.Errorf("decode iac state objects failed, state_id: %s, err: %v, rid: %s", stateID, err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	// 单次请求内复用同 (kind, plugin, type) 的 SchemaBlock，避免重复跨服务查询，不做跨请求缓存。
	reqBlocks := make(map[string]*tfjson.SchemaBlock)
	resolveOnce := func(obj *iacstate.StateObject) (*tfjson.SchemaBlock, error) {
		key := fmt.Sprintf("%s|%s|%s", row.Kind, obj.Plugin, obj.Type)
		if b, ok := reqBlocks[key]; ok {
			return b, nil
		}
		b, resolveErr := svc.resolver.ResolveBlock(cts.Kit, row.Kind, obj.Plugin, obj.Type)
		if resolveErr != nil {
			return nil, resolveErr
		}
		reqBlocks[key] = b
		return b, nil
	}

	var maskedCurrent *iacproto.MaskedStateObject
	if currentObj != nil {
		block, resolveErr := resolveOnce(currentObj)
		if resolveErr != nil {
			logs.Errorf("resolve schema block for current state failed, state_id: %s, err: %v, rid: %s",
				stateID, resolveErr, cts.Kit.Rid)
			return nil, resolveErr
		}
		maskedCurrent, err = logicstate.MaskStateObject(currentObj, block)
		if err != nil {
			return nil, err
		}
	}

	maskedDeposed := make(map[string]*iacproto.MaskedStateObject, len(deposedMap))
	for depKey, depObj := range deposedMap {
		if depObj == nil {
			continue
		}
		block, resolveErr := resolveOnce(depObj)
		if resolveErr != nil {
			logs.Errorf("resolve schema block for deposed state failed, state_id: %s, "+
				"deposed_key: %s, err: %v, rid: %s", stateID, depKey, resolveErr, cts.Kit.Rid)
			return nil, resolveErr
		}
		masked, maskErr := logicstate.MaskStateObject(depObj, block)
		if maskErr != nil {
			return nil, maskErr
		}
		maskedDeposed[depKey] = masked
	}

	var createdAt, updatedAt string
	if row.Revision != nil {
		createdAt = row.Revision.CreatedAt
		updatedAt = row.Revision.UpdatedAt
	}

	return &iacproto.GetStateResp{
		ID:           row.ID,
		RepositoryID: row.RepositoryID,
		BlockID:      row.BlockID,
		Kind:         row.Kind,
		Current:      maskedCurrent,
		Deposed:      maskedDeposed,
		LastFlowID:   row.LastFlowID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

func (svc *service) ensureRepositoryExists(kt *kit.Kit, repositoryID string) error {
	if svc.client == nil || svc.client.DataService() == nil || svc.client.DataService().Global == nil {
		return errf.New(errf.Aborted, "iac data service client is not initialized")
	}

	res, err := svc.client.DataService().Global.IacRepository.List(kt, &dataiac.IacRepositoryListReq{
		Filter: tools.EqualExpression("id", repositoryID),
		Page:   &core.BasePage{Count: true},
	})
	if err != nil {
		logs.Errorf("check iac repository exists failed, repository_id: %s, err: %v, rid: %s",
			repositoryID, err, kt.Rid)
		return errf.NewFromErr(errf.Aborted, err)
	}
	if res == nil || res.Count == 0 {
		return errf.Newf(errf.RecordNotFound, "iac repository %s not found", repositoryID)
	}
	return nil
}

func mergeFilterExpr(baseRules []filter.RuleFactory, userFilter *filter.Expression) (*filter.Expression, error) {
	if userFilter == nil || len(userFilter.Rules) == 0 {
		return &filter.Expression{
			Op:    filter.And,
			Rules: baseRules,
		}, nil
	}
	rules := make([]filter.RuleFactory, 0, len(baseRules)+1)
	rules = append(rules, baseRules...)
	rules = append(rules, userFilter)
	return &filter.Expression{
		Op:    filter.And,
		Rules: rules,
	}, nil
}

// stateObjectSummary 是列表接口从 current 或 deposed 的状态 JSON 中抽取的展示字段。
// 完整状态对象还包含 state、private、provider_config 等载荷；反序列化到该结构时这些字段会被忽略。
// current 有内容时用它填充列表的 plugin、plugin_instance_id、type；current 为空时改取 deposed 中 key 字典序的第一条。
type stateObjectSummary struct {
	Plugin           string `json:"plugin"`
	PluginInstanceID string `json:"plugin_instance_id"`
	Type             string `json:"type"`
}

func buildStateListItem(row *coreiac.IacState) (iacproto.StateListItem, error) {
	var currentSummary *stateObjectSummary
	if row.Current != nil && len(*row.Current) > 0 && !row.Current.IsNull() {
		s := new(stateObjectSummary)
		if err := json.Unmarshal([]byte(*row.Current), s); err != nil {
			return iacproto.StateListItem{}, fmt.Errorf("unmarshal current summary failed: %w", err)
		}
		currentSummary = s
	}

	deposedSummaries := make(map[string]*stateObjectSummary)
	if len(row.Deposed) > 0 && string(row.Deposed) != constant.IacStateEmptyDeposedJSON && !row.Deposed.IsNull() {
		if err := json.Unmarshal([]byte(row.Deposed), &deposedSummaries); err != nil {
			return iacproto.StateListItem{}, fmt.Errorf("unmarshal deposed summary failed: %w", err)
		}
	}

	var primary *stateObjectSummary
	if currentSummary != nil {
		primary = currentSummary
	} else if len(deposedSummaries) > 0 {
		keys := make([]string, 0, len(deposedSummaries))
		for k := range deposedSummaries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		primary = deposedSummaries[keys[0]]
	}

	var pluginName, pluginInstanceID, typeName string
	if primary != nil {
		pluginName = primary.Plugin
		pluginInstanceID = primary.PluginInstanceID
		typeName = primary.Type
	}

	var updatedAt string
	if row.Revision != nil {
		updatedAt = row.Revision.UpdatedAt
	}

	return iacproto.StateListItem{
		ID:               row.ID,
		BlockID:          row.BlockID,
		PluginInstanceID: pluginInstanceID,
		Kind:             row.Kind,
		Plugin:           pluginName,
		Type:             typeName,
		HasCurrent:       currentSummary != nil,
		DeposedCount:     len(deposedSummaries),
		LastFlowID:       row.LastFlowID,
		UpdatedAt:        updatedAt,
	}, nil
}

func decodeStateRowObjects(row *coreiac.IacState) (*iacstate.StateObject, map[string]*iacstate.StateObject, error) {
	var currentObj *iacstate.StateObject
	if row.Current != nil && len(*row.Current) > 0 && !row.Current.IsNull() {
		obj := new(iacstate.StateObject)
		if err := json.Unmarshal([]byte(*row.Current), obj); err != nil {
			return nil, nil, fmt.Errorf("unmarshal current object failed: %w", err)
		}
		currentObj = obj
	}

	deposedMap := make(map[string]*iacstate.StateObject)
	if len(row.Deposed) > 0 && string(row.Deposed) != constant.IacStateEmptyDeposedJSON && !row.Deposed.IsNull() {
		if err := json.Unmarshal([]byte(row.Deposed), &deposedMap); err != nil {
			return nil, nil, fmt.Errorf("unmarshal deposed objects failed: %w", err)
		}
	}

	return currentObj, deposedMap, nil
}
