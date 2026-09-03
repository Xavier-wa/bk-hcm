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

package feedback

import (
	"encoding/json"

	proto "hcm/pkg/api/agent-server/feedback"
	"hcm/pkg/api/core"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	datagconf "hcm/pkg/api/data-service/global_config"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// BizCommitFeedback upserts the current user's feedback (like/dislike) on one
// Agent run. It inserts a row when the run has no feedback yet, or overwrites
// the existing row (including on reaction change) otherwise.
//
// POST /api/v1/agent/bizs/{bk_biz_id}/feedback/commit
func (svc *service) BizCommitFeedback(cts *rest.Contexts) (interface{}, error) {
	bizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bizID <= 0 {
		return nil, errf.New(errf.InvalidParameter, "bk_biz_id must be greater than 0")
	}

	if err := svc.authorizeBizSession(cts.Kit, bizID); err != nil {
		return nil, err
	}

	req := new(proto.CommitFeedbackReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	sess, err := findOwnedSession(cts.Kit, svc.cli, req.SessionID, bizID)
	if err != nil {
		return nil, err
	}

	if err := ensureRunMatchesSession(cts.Kit, svc.cli, req.RunID, sess.SessionCode); err != nil {
		return nil, err
	}

	if err := svc.validateFeedbackTags(cts.Kit, req.Reaction, req.Tags); err != nil {
		return nil, err
	}

	id, err := svc.upsertFeedback(cts.Kit, bizID, req)
	if err != nil {
		logs.Errorf("upsert feedback failed, err: %v, run_id: %s, rid: %s", err, req.RunID, cts.Kit.Rid)
		return nil, err
	}

	return &proto.CommitFeedbackResp{ID: id}, nil
}

// validateFeedbackTags checks that every tag in tags is a key of the global_config
// agent_feedback_tag map for the given reaction. Empty tags are always allowed. When the
// map is missing for this reaction, only empty tags are allowed.
func (svc *service) validateFeedbackTags(kt *kit.Kit, reaction enumor.FeedbackReaction, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	listReq := &datagconf.ListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("config_type", enumor.GlobalConfigTypeAgentFeedbackTag),
			tools.RuleEqual("config_key", string(reaction)),
		),
		Page: core.NewDefaultBasePage(),
	}
	resp, err := svc.cli.DataService().Global.GlobalConfig.List(kt, listReq)
	if err != nil {
		logs.Errorf("list agent_feedback_tag config failed, err: %v, reaction: %s, rid: %s", err, reaction, kt.Rid)
		return errf.NewFromErr(errf.Aborted, err)
	}
	if len(resp.Details) == 0 {
		return errf.Newf(errf.InvalidParameter, "feedback tags are not configured for reaction: %s", reaction)
	}

	allowed := make(map[string]string)
	if err := json.Unmarshal([]byte(resp.Details[0].ConfigValue), &allowed); err != nil {
		logs.Errorf("parse agent_feedback_tag config_value failed, err: %v, reaction: %s, rid: %s", err, reaction, kt.Rid)
		return errf.NewFromErr(errf.Aborted, err)
	}

	for _, tag := range tags {
		if _, ok := allowed[tag]; !ok {
			return errf.Newf(errf.InvalidParameter, "tag %q is not allowed for reaction: %s", tag, reaction)
		}
	}
	return nil
}

// upsertFeedback writes the feedback row for req.RunID: it overwrites the existing
// row when one already exists (which also implements the "clear old tags/comment on
// reaction change" rule, since the whole row is always rewritten), otherwise it
// inserts a new row.
func (svc *service) upsertFeedback(kt *kit.Kit, bizID int64, req *proto.CommitFeedbackReq) (
	string, error) {

	feedbackCli := svc.cli.DataService().Aiagent.Feedback

	existing, err := findFeedbackByRunID(kt, svc.cli, req.RunID)
	if err != nil {
		return "", err
	}

	if existing != nil {
		if existing.User != kt.User || existing.BkBizID != bizID {
			logs.Errorf("update feedback denied, feedback does not belong to caller, run_id: %s, "+
				"row_user: %s, row_biz: %d, user: %s, bk_biz_id: %d, rid: %s",
				req.RunID, existing.User, existing.BkBizID, kt.User, bizID, kt.Rid)
			return "", errf.Newf(errf.PermissionDenied, "permission denied, run_id: %s", req.RunID)
		}
		updateReq := &dsaiagent.UpdateAgentRunFeedbackReq{
			ID:       existing.ID,
			Tags:     types.StringArray(req.Tags),
			Reaction: req.Reaction,
			Comment:  req.Comment,
		}
		if err := feedbackCli.Update(kt, updateReq); err != nil {
			logs.Errorf("update feedback failed, err: %v, id: %s, rid: %s", err, existing.ID, kt.Rid)
			return "", err
		}
		return existing.ID, nil
	}

	createReq := &dsaiagent.CreateAgentRunFeedbackReq{
		RunID:     req.RunID,
		SessionID: req.SessionID,
		User:      kt.User,
		BkBizID:   bizID,
		Tags:      types.StringArray(req.Tags),
		Reaction:  req.Reaction,
		Comment:   req.Comment,
	}
	result, err := feedbackCli.Create(kt, createReq)
	if err == nil {
		return result.ID, nil
	}

	// Concurrent insert may race and hit uk_run_id; fall back to update.
	if !errf.IsDuplicated(err) {
		logs.Errorf("create feedback failed, err: %v, run_id: %s, rid: %s", err, req.RunID, kt.Rid)
		return "", err
	}

	existing, findErr := findFeedbackByRunID(kt, svc.cli, req.RunID)
	if findErr != nil {
		return "", findErr
	}
	if existing == nil {
		return "", errf.Newf(errf.Aborted, "feedback row for run_id %q not found after duplicate insert",
			req.RunID)
	}

	updateReq := &dsaiagent.UpdateAgentRunFeedbackReq{
		ID:       existing.ID,
		Tags:     types.StringArray(req.Tags),
		Reaction: req.Reaction,
		Comment:  req.Comment,
	}
	if err := feedbackCli.Update(kt, updateReq); err != nil {
		logs.Errorf("update feedback after duplicate insert failed, err: %v, id: %s, rid: %s",
			err, existing.ID, kt.Rid)
		return "", err
	}
	return existing.ID, nil
}
