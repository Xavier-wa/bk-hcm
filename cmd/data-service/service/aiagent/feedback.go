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

package aiagent

import (
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/converter"
)

// CreateAgentRunFeedback creates a new aiagent run feedback row.
func (svc *service) CreateAgentRunFeedback(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.CreateAgentRunFeedbackReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	feedback := &tableaiagent.FeedbackTable{
		RunID:     req.RunID,
		SessionID: req.SessionID,
		User:      req.User,
		BkBizID:   req.BkBizID,
		Tags:      req.Tags,
		Reaction:  req.Reaction,
		Comment:   converter.ValToPtr(req.Comment),
	}

	id, err := svc.dao.AiagentRunFeedback().Create(cts.Kit, feedback)
	if err != nil {
		logs.Errorf("create aiagent run feedback failed, err: %v, run_id: %s, rid: %s",
			err, req.RunID, cts.Kit.Rid)
		return nil, err
	}

	return &dsaiagent.CreateAgentRunFeedbackResult{ID: id}, nil
}

// UpdateAgentRunFeedback overwrites tags/reaction/comment of an existing feedback row by id.
func (svc *service) UpdateAgentRunFeedback(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.UpdateAgentRunFeedbackReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	expr, err := tools.And(filter.AtomRule{Field: "id", Op: filter.Equal.Factory(), Value: req.ID})
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	tags := req.Tags
	if tags == nil {
		// overwrite semantics: omitted tags must clear the stored array, not skip SET.
		tags = tabletypes.StringArray{}
	}

	feedback := &tableaiagent.FeedbackTable{
		Tags:     tags,
		Reaction: req.Reaction,
		Comment:  converter.ValToPtr(req.Comment),
	}

	if err := svc.dao.AiagentRunFeedback().Update(cts.Kit, expr, feedback); err != nil {
		logs.Errorf("update aiagent run feedback failed, err: %v, id: %s, rid: %s", err, req.ID, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// ListAgentRunFeedback queries aiagent run feedback rows with filter and pagination.
func (svc *service) ListAgentRunFeedback(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.ListAgentRunFeedbackReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.ListOption{
		Filter: req.Filter,
		Page:   req.Page,
		Fields: req.Fields,
	}

	result, err := svc.dao.AiagentRunFeedback().List(cts.Kit, opt)
	if err != nil {
		logs.Errorf("list aiagent run feedback failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &dsaiagent.ListAgentRunFeedbackResult{
		Count:   result.Count,
		Details: result.Details,
	}, nil
}

// BatchDeleteAgentRunFeedback deletes aiagent run feedback rows by filter.
func (svc *service) BatchDeleteAgentRunFeedback(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.DeleteAgentRunFeedbackReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := svc.dao.AiagentRunFeedback().Delete(cts.Kit, req.Filter); err != nil {
		logs.Errorf("batch delete aiagent run feedback failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
