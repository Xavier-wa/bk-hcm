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
	"strings"

	"hcm/pkg/api/core"
	coreiac "hcm/pkg/api/core/iac"
	dataiac "hcm/pkg/api/data-service/iac"
	dataservice "hcm/pkg/client/data-service"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
)

// StateManager 无状态 IaC 受管实例状态管理器，所有操作直接通过 data-service 持久化，并发安全。
type StateManager struct {
	dataCli *dataservice.Client
}

// NewStateManager 创建无状态 StateManager 实例。dataCli 及其 Global.IacState 未初始化时返回错误。
func NewStateManager(dataCli *dataservice.Client) (*StateManager, error) {
	if dataCli == nil || dataCli.Global == nil || dataCli.Global.IacState == nil {
		return nil, errf.New(errf.InvalidParameter, "data service iac state client is required")
	}
	return &StateManager{dataCli: dataCli}, nil
}

func (m *StateManager) ensureClient() error {
	if m.dataCli == nil || m.dataCli.Global == nil || m.dataCli.Global.IacState == nil {
		return errf.New(errf.Aborted, "iac state data client is not initialized")
	}
	return nil
}

// GetRepositoryStates 查询仓库下已生效的受管状态记录（默认排除暂存态记录）。
// 当 blockIDs 为空时全量分页拉取，非空时按 block_id 分批定向拉取；无匹配记录时返回 (nil, nil)。
func (m *StateManager) GetRepositoryStates(kt *kit.Kit, repositoryID string, blockIDs []string) (
	[]*StateRecord, error) {

	if err := m.ensureClient(); err != nil {
		return nil, err
	}
	if len(repositoryID) == 0 {
		return nil, errf.New(errf.InvalidParameter, "repository_id is required")
	}

	if len(blockIDs) > 0 {
		return m.listRepositoryStatesByBlockIDs(kt, repositoryID, blockIDs)
	}

	// import_from_spec_id 非空是导入中的暂存行，编排只读已生效的。
	expr := tools.ExpressionAnd(tools.RuleEqual("repository_id", repositoryID),
		tools.RuleEqual("import_from_spec_id", ""))

	// 按 id 升序翻页，offset 才不会跨页漏行或重复。
	var result []*StateRecord
	var start uint32
	for {
		resp, err := m.dataCli.Global.IacState.List(kt, &dataiac.IacStateListReq{
			Filter: expr,
			Page: &core.BasePage{
				Start: start,
				Limit: core.DefaultMaxPageLimit,
				Sort:  "id",
				Order: core.Ascending,
			},
		})
		if err != nil {
			logs.Errorf("list iac state for repository failed, repository_id: %s, err: %v, rid: %s",
				repositoryID, err, kt.Rid)
			return nil, err
		}

		for i := range resp.Details {
			rec, convErr := decodeStateRecord(&resp.Details[i])
			if convErr != nil {
				logs.Errorf("decode iac state record failed, id: %s, err: %v, rid: %s",
					resp.Details[i].ID, convErr, kt.Rid)
				return nil, convErr
			}
			result = append(result, rec)
		}

		if uint(len(resp.Details)) < core.DefaultMaxPageLimit {
			break
		}
		start += uint32(core.DefaultMaxPageLimit)
	}

	// 空仓库返回 nil，调用方用 nil 判断没有 state。
	if len(result) == 0 {
		return nil, nil
	}

	return result, nil
}

func (m *StateManager) listRepositoryStatesByBlockIDs(kt *kit.Kit, repositoryID string, blockIDs []string) (
	[]*StateRecord, error) {

	uniqueIDs := slice.Unique(blockIDs)
	result := make([]*StateRecord, 0, len(uniqueIDs))
	for _, batch := range slice.Split(uniqueIDs, int(filter.DefaultMaxInLimit)) {
		expr := tools.ExpressionAnd(tools.RuleEqual("repository_id", repositoryID),
			tools.RuleEqual("import_from_spec_id", ""), tools.RuleIn("block_id", batch))
		resp, err := m.dataCli.Global.IacState.List(kt, &dataiac.IacStateListReq{
			Filter: expr,
			Page: &core.BasePage{
				Start: 0,
				Limit: core.DefaultMaxPageLimit,
				Sort:  "id",
				Order: core.Ascending,
			},
		})
		if err != nil {
			logs.Errorf("list iac states by block_ids failed, repository_id: %s, err: %v, rid: %s",
				repositoryID, err, kt.Rid)
			return nil, err
		}

		for idx := range resp.Details {
			rec, convErr := decodeStateRecord(&resp.Details[idx])
			if convErr != nil {
				logs.Errorf("decode iac state failed, id: %s, err: %v, rid: %s",
					resp.Details[idx].ID, convErr, kt.Rid)
				return nil, convErr
			}
			result = append(result, rec)
		}
	}

	if len(result) == 0 {
		return nil, nil
	}

	return result, nil
}

// GetDependencyStates 按 blockIDs 定向拉取依赖资源的已生效 Current.State，任意缺失或 current 为空时返回错误。
func (m *StateManager) GetDependencyStates(kt *kit.Kit, repositoryID string, blockIDs []string) (
	map[string]map[string]any, error) {

	if err := m.ensureClient(); err != nil {
		return nil, err
	}
	if len(repositoryID) == 0 {
		return nil, errf.New(errf.InvalidParameter, "repository_id is required")
	}
	if len(blockIDs) == 0 {
		return map[string]map[string]any{}, nil
	}

	uniqueIDs := slice.Unique(blockIDs)
	records, err := m.GetRepositoryStates(kt, repositoryID, uniqueIDs)
	if err != nil {
		return nil, err
	}

	found := make(map[string]*StateRecord, len(records))
	for _, rec := range records {
		found[rec.BlockID] = rec
	}

	// 行不存在，或 current 为空（只剩 deposed），依赖都用不了。
	result := make(map[string]map[string]any, len(uniqueIDs))
	var missing []string
	for _, blockID := range uniqueIDs {
		rec, ok := found[blockID]
		cur := rec.CurrentOrNil()
		if !ok || cur == nil {
			missing = append(missing, blockID)
			continue
		}
		result[blockID] = cur.State
	}
	if len(missing) > 0 {
		return nil, errf.Newf(errf.RecordNotFound,
			"dependency states missing or current is null: %s", strings.Join(missing, ", "))
	}

	return result, nil
}

// UpsertAppliedState 写回已执行的资源或数据源实例状态，支持先建后删轮转、import 前缀剥离与半成品状态落库。
func (m *StateManager) UpsertAppliedState(kt *kit.Kit, req *UpsertAppliedReq) error {
	if err := m.ensureClient(); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return errf.NewFromErr(errf.InvalidParameter, err)
	}

	// import. 前缀是导入节点，落到去掉前缀的资源地址，kind 固定 resource。
	targetBlockID := req.BlockID
	kind := req.Kind
	if strings.HasPrefix(targetBlockID, constant.IacStateImportBlockPrefix) {
		targetBlockID = strings.TrimPrefix(targetBlockID, constant.IacStateImportBlockPrefix)
		kind = enumor.IacStateKindResource
	}

	currentJSON, err := tabletypes.NewJsonField(req.Object)
	if err != nil {
		return errf.NewFromErr(errf.InvalidParameter, err)
	}

	blockState, err := m.getStateByBlockID(kt, req.RepositoryID, targetBlockID)
	if err != nil {
		return err
	}

	if blockState == nil {
		return m.createAppliedState(kt, req, targetBlockID, kind, currentJSON)
	}
	return m.updateAppliedState(kt, req, blockState, kind, currentJSON)
}

func (m *StateManager) createAppliedState(kt *kit.Kit, req *UpsertAppliedReq, targetBlockID string,
	kind enumor.IacStateKind, currentJSON tabletypes.JsonField) error {

	// 没有这行就新建。deposed 用 {}，import_from_spec_id 留空表示已生效。
	_, err := m.dataCli.Global.IacState.BatchCreate(kt, &dataiac.IacStateBatchCreateReq{
		Items: []dataiac.IacStateCreateField{
			{
				RepositoryID:     req.RepositoryID,
				BlockID:          targetBlockID,
				Kind:             kind,
				Current:          currentJSON,
				Deposed:          tabletypes.JsonField(constant.IacStateEmptyDeposedJSON),
				LastFlowID:       req.FlowID,
				ImportFromSpecID: "",
			},
		},
	})
	if err != nil {
		logs.Errorf("create iac state failed, repository_id: %s, block_id: %s, err: %v, rid: %s",
			req.RepositoryID, targetBlockID, err, kt.Rid)
		return err
	}
	return nil
}

func (m *StateManager) updateAppliedState(kt *kit.Kit, req *UpsertAppliedReq, blockState *StateRecord,
	kind enumor.IacStateKind, currentJSON tabletypes.JsonField) error {

	upd := dataiac.IacStateUpdateField{
		ID:         blockState.ID,
		Kind:       kind,
		Current:    currentJSON,
		LastFlowID: req.FlowID,
	}
	// 暂存行被正式写回后，清掉 import_from_spec_id。
	if blockState.ImportFromSpecID != "" {
		upd.ImportFromSpecID = converter.ValToPtr("")
	}

	// 先建后删：若 deposed_key 已存在说明上游分配不唯一，直接拒绝覆盖；否则将旧 current 挂到 deposed_key 下。
	if req.DeposedKey != "" {
		deposedMap := blockState.Deposed
		if deposedMap == nil {
			deposedMap = make(map[string]*StateObject)
		}
		if _, exists := deposedMap[req.DeposedKey]; exists {
			logs.Errorf("upsert iac state failed, deposed key %s already exists, id: %s, block_id: %s, rid: %s",
				req.DeposedKey, blockState.ID, blockState.BlockID, kt.Rid)
			return errf.Newf(errf.RecordDuplicated, "deposed_key %q already exists for block %q",
				req.DeposedKey, blockState.BlockID)
		}
		if blockState.Current != nil {
			deposedMap[req.DeposedKey] = blockState.Current
		}
		deposedJSON, err := marshalDeposedMap(deposedMap)
		if err != nil {
			return errf.NewFromErr(errf.Aborted, err)
		}
		upd.Deposed = deposedJSON
	}

	if err := m.dataCli.Global.IacState.BatchUpdate(kt, &dataiac.IacStateBatchUpdateReq{
		Items: []dataiac.IacStateUpdateField{upd},
	}); err != nil {
		logs.Errorf("update iac state failed, id: %s, block_id: %s, err: %v, rid: %s",
			blockState.ID, blockState.BlockID, err, kt.Rid)
		return err
	}
	return nil
}

// RemoveDestroyedState 清理已销毁的实例状态，按需仅清除指定 deposed 键、置 current 为 NULL 或物理删除整行。
func (m *StateManager) RemoveDestroyedState(kt *kit.Kit, req *RemoveDestroyedReq) error {
	if err := m.ensureClient(); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return errf.NewFromErr(errf.InvalidParameter, err)
	}

	blockState, err := m.getStateByBlockID(kt, req.RepositoryID, req.BlockID)
	if err != nil {
		return err
	}
	// 没有这行，当作已经清掉。
	if blockState == nil {
		logs.Infof("skip remove destroyed iac state, record not found, repository_id: %s, block_id: %s, rid: %s",
			req.RepositoryID, req.BlockID, kt.Rid)
		return nil
	}

	// 只删指定的 deposed 键；键不在也当作已经清掉。
	if req.DeposedKey != "" {
		if _, exists := blockState.Deposed[req.DeposedKey]; !exists {
			logs.Infof("skip remove destroyed iac state, deposed key not found, id: %s, deposed_key: %s, rid: %s",
				blockState.ID, req.DeposedKey, kt.Rid)
			return nil
		}
		delete(blockState.Deposed, req.DeposedKey)

		// current 和 deposed 都空了，整行删掉。
		if blockState.Current == nil && len(blockState.Deposed) == 0 {
			logs.Infof("remove destroyed iac state, current and deposed are empty, id: %s, rid: %s",
				blockState.ID, kt.Rid)
			return m.deleteStateByID(kt, blockState.ID)
		}

		deposedJSON, marshalErr := marshalDeposedMap(blockState.Deposed)
		if marshalErr != nil {
			return errf.NewFromErr(errf.Aborted, marshalErr)
		}
		if err = m.dataCli.Global.IacState.BatchUpdate(kt, &dataiac.IacStateBatchUpdateReq{
			Items: []dataiac.IacStateUpdateField{
				{
					ID:         blockState.ID,
					Deposed:    deposedJSON,
					LastFlowID: req.FlowID,
				},
			},
		}); err != nil {
			logs.Errorf("remove deposed state key failed, id: %s, deposed_key: %s, err: %v, rid: %s",
				blockState.ID, req.DeposedKey, err, kt.Rid)
			return err
		}
		return nil
	}

	// 销毁的是 current。没有 deposed 就删行，还有旧实例就把 current 写成 null。
	if len(blockState.Deposed) == 0 {
		return m.deleteStateByID(kt, blockState.ID)
	}

	if err = m.dataCli.Global.IacState.BatchUpdate(kt, &dataiac.IacStateBatchUpdateReq{
		Items: []dataiac.IacStateUpdateField{
			{
				ID:         blockState.ID,
				Current:    tabletypes.JsonField(constant.IacStateNullJSON),
				LastFlowID: req.FlowID,
			},
		},
	}); err != nil {
		logs.Errorf("clear current iac state failed, id: %s, block_id: %s, err: %v, rid: %s",
			blockState.ID, req.BlockID, err, kt.Rid)
		return err
	}

	return nil
}

func (m *StateManager) getStateByBlockID(kt *kit.Kit, repositoryID, blockID string) (*StateRecord, error) {
	expr := tools.ExpressionAnd(tools.RuleEqual("repository_id", repositoryID), tools.RuleEqual("block_id", blockID))
	resp, err := m.dataCli.Global.IacState.List(kt, &dataiac.IacStateListReq{
		Filter: expr,
		Page: &core.BasePage{
			Start: 0,
			Limit: 1,
		},
	})
	if err != nil {
		logs.Errorf("query iac state by block_id failed, repository_id: %s, block_id: %s, err: %v, rid: %s",
			repositoryID, blockID, err, kt.Rid)
		return nil, err
	}
	if len(resp.Details) == 0 {
		return nil, nil
	}
	return decodeStateRecord(&resp.Details[0])
}

func (m *StateManager) deleteStateByID(kt *kit.Kit, id string) error {
	if err := m.dataCli.Global.IacState.BatchDelete(kt, &dataiac.IacStateBatchDeleteReq{
		Filter: tools.EqualExpression("id", id),
	}); err != nil {
		logs.Errorf("delete iac state row failed, id: %s, err: %v, rid: %s", id, err, kt.Rid)
		return err
	}
	return nil
}

func decodeStateRecord(raw *coreiac.IacState) (*StateRecord, error) {
	rec := &StateRecord{
		ID:               raw.ID,
		RepositoryID:     raw.RepositoryID,
		BlockID:          raw.BlockID,
		Kind:             raw.Kind,
		Deposed:          make(map[string]*StateObject),
		LastFlowID:       raw.LastFlowID,
		ImportFromSpecID: raw.ImportFromSpecID,
	}

	if raw.Current != nil && len(*raw.Current) > 0 && !raw.Current.IsNull() {
		obj := new(StateObject)
		if err := json.Unmarshal([]byte(*raw.Current), obj); err != nil {
			return nil, fmt.Errorf("unmarshal current state for id %s failed: %w", raw.ID, err)
		}
		rec.Current = obj
	}

	if len(raw.Deposed) > 0 && string(raw.Deposed) != constant.IacStateEmptyDeposedJSON && !raw.Deposed.IsNull() {
		var deposed map[string]*StateObject
		if err := json.Unmarshal([]byte(raw.Deposed), &deposed); err != nil {
			return nil, fmt.Errorf("unmarshal deposed state for id %s failed: %w", raw.ID, err)
		}
		if deposed != nil {
			rec.Deposed = deposed
		}
	}

	return rec, nil
}

func marshalDeposedMap(deposed map[string]*StateObject) (tabletypes.JsonField, error) {
	if len(deposed) == 0 {
		return tabletypes.JsonField(constant.IacStateEmptyDeposedJSON), nil
	}
	b, err := json.Marshal(deposed)
	if err != nil {
		return "", fmt.Errorf("marshal deposed map failed: %w", err)
	}
	return tabletypes.JsonField(b), nil
}
