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

// Package eval defines agent-server eval query, reeval, history sync and dashboard list APIs.
package eval

import (
	"errors"
	"fmt"
	"strings"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/tools/slice"
)

// MaxListFilterValues is the max number of users or bk_biz_ids allowed in one dashboard list query.
const MaxListFilterValues = 100

// ReevalReq is the request for POST /eval/runs/{run_id}/reeval.
type ReevalReq struct {
	// Overwrite is required. false refuses to replace an existing eval row.
	Overwrite *bool `json:"overwrite" validate:"required"`
}

// Validate validates the reeval request.
func (req *ReevalReq) Validate() error {
	return validator.Validate.Struct(req)
}

// PeriodFilter is the common time-range filter for dashboard list queries. Times are RFC3339.
type PeriodFilter struct {
	From  string `json:"from" validate:"required"`
	To    string `json:"to" validate:"required"`
	Scene string `json:"scene"`
}

// Validate validates the period filter.
func (p *PeriodFilter) Validate() error {
	if err := validator.Validate.Struct(p); err != nil {
		return err
	}
	if p.From == "" || p.To == "" {
		return errors.New("from and to are required")
	}
	return nil
}

// DashboardEvalResultsListReq is the request for POST /eval/dashboard/eval_results/list.
type DashboardEvalResultsListReq struct {
	PeriodFilter
	QueryLike  string `json:"query_like"`
	Passed     *bool  `json:"passed"`
	HasRedline *bool  `json:"has_redline"`
	// RunID filters by aiagent_run_eval.run_id. Empty means all.
	RunID string `json:"run_id"`
	// SessionID filters by aiagent_run_eval.session_id. Empty means all.
	SessionID string `json:"session_id"`
	// Users filters by aiagent_run_eval.user IN (...). Empty means all.
	Users []string `json:"users"`
	// BkBizIDs filters by aiagent_run_eval.bk_biz_id IN (...). Empty means all.
	BkBizIDs []int64        `json:"bk_biz_ids"`
	Page     *core.BasePage `json:"page"`
}

// Validate validates the dashboard eval results list request.
func (r *DashboardEvalResultsListReq) Validate() error {
	if err := r.PeriodFilter.Validate(); err != nil {
		return err
	}
	if r.Page == nil {
		r.Page = &core.BasePage{Start: 0, Limit: 20}
	}
	if !r.Page.Count && r.Page.Limit == 0 {
		r.Page.Limit = 20
	}
	if err := r.Page.Validate(); err != nil {
		return err
	}
	// 对 users/bk_biz_ids 做清洗：去空白、去重、剔除零值，并校验数量上限，
	// 清洗后的结果回写到 r.Users/r.BkBizIDs，供后续构造过滤条件使用。
	users, bkBizIDs, err := compactListIdentityFilters(r.Users, r.BkBizIDs)
	if err != nil {
		return err
	}
	r.Users, r.BkBizIDs = users, bkBizIDs
	return nil
}

// DashboardEvalResultsListItem is one row of the dashboard eval results list.
type DashboardEvalResultsListItem struct {
	CreatedAt    string `json:"created_at"`
	RunID        string `json:"run_id"`
	SessionID    string `json:"session_id"`
	User         string `json:"user"`
	BkBizID      int64  `json:"bk_biz_id"`
	Scene        string `json:"scene"`
	Query        string `json:"query"`
	QualityScore int    `json:"quality_score"`
	Passed       bool   `json:"passed"`
}

// DashboardFeedbackListReq is the request for POST /eval/dashboard/feedback/list. It does not
// support filtering by passed/has_redline: those live on aiagent_run_eval, a different ledger
// than aiagent_run_feedback, and filtering by them would require an in-memory join instead of a
// single pushed-down SQL query.
type DashboardFeedbackListReq struct {
	PeriodFilter
	QueryLike string `json:"query_like"`
	// RunID filters by aiagent_run_feedback.run_id. Empty means all.
	RunID string `json:"run_id"`
	// SessionID filters by aiagent_run_feedback.session_id. Empty means all.
	SessionID string `json:"session_id"`
	// Users filters by aiagent_run_feedback.user IN (...). Empty means all.
	Users []string `json:"users"`
	// BkBizIDs filters by aiagent_run_feedback.bk_biz_id IN (...). Empty means all.
	BkBizIDs []int64 `json:"bk_biz_ids"`
	// Reaction filters attitude. Enumeration values such as: like/dislike. Empty means all.
	Reaction enumor.FeedbackReaction `json:"reaction"`
	// Tag filters feedback whose tags JSON array contains this single key.
	Tag  string         `json:"tag"`
	Page *core.BasePage `json:"page"`
}

// Validate validates the dashboard feedback list request.
func (r *DashboardFeedbackListReq) Validate() error {
	if err := r.PeriodFilter.Validate(); err != nil {
		return err
	}
	if r.Reaction != "" {
		if err := r.Reaction.Validate(); err != nil {
			return err
		}
	}
	if r.Page == nil {
		r.Page = &core.BasePage{Start: 0, Limit: 20}
	}
	if !r.Page.Count && r.Page.Limit == 0 {
		r.Page.Limit = 20
	}
	if err := r.Page.Validate(); err != nil {
		return err
	}
	// 对 users/bk_biz_ids 做清洗：去空白、去重、剔除零值，并校验数量上限，
	// 清洗后的结果回写到 r.Users/r.BkBizIDs，供后续构造过滤条件使用。
	users, bkBizIDs, err := compactListIdentityFilters(r.Users, r.BkBizIDs)
	if err != nil {
		return err
	}
	r.Users, r.BkBizIDs = users, bkBizIDs
	return nil
}

// DashboardFeedbackListItem is one row of the dashboard feedback list. QualityScore/Passed are
// display-only (looked up from aiagent_run_eval by run_id after paging): they cannot be used to
// filter or sort this list, see DashboardFeedbackListReq.
type DashboardFeedbackListItem struct {
	CreatedAt    string                  `json:"created_at"`
	RunID        string                  `json:"run_id"`
	SessionID    string                  `json:"session_id"`
	User         string                  `json:"user"`
	BkBizID      int64                   `json:"bk_biz_id"`
	Scene        string                  `json:"scene"`
	Query        string                  `json:"query"`
	Reaction     enumor.FeedbackReaction `json:"reaction"`
	Tags         []string                `json:"tags"`
	Comment      string                  `json:"comment"`
	QualityScore *int                    `json:"quality_score"`
	Passed       *bool                   `json:"passed"`
}

// ListResp is a paged dashboard list response.
type ListResp[T any] struct {
	Count   uint64 `json:"count"`
	Details []T    `json:"details"`
}

const (
	// SortCreatedAt sorts dashboard eval results/feedback rows by created_at.
	SortCreatedAt = "created_at"
	// SortQualityScore sorts dashboard eval results rows by quality_score. Feedback rows do not
	// support this sort field because quality_score is not their sort column.
	SortQualityScore = "quality_score"
)

// compactListIdentityFilters trims/dedupes users, drops zero bk_biz_ids, and caps each at
// MaxListFilterValues. page.sort is intentionally not whitelisted here: an unsupported sort
// column is a real table column name, so it fails naturally at the MySQL layer (unknown column
// in order clause) instead of needing a parallel list kept in sync by hand on every schema change.
func compactListIdentityFilters(users []string, bkBizIDs []int64) ([]string, []int64, error) {
	users = slice.Unique(slice.Filter(
		slice.Map(users, strings.TrimSpace),
		func(user string) bool { return user != "" },
	))
	if len(users) > MaxListFilterValues {
		return nil, nil, fmt.Errorf("users must be less than or equal to %d", MaxListFilterValues)
	}
	bkBizIDs = slice.Unique(slice.Filter(bkBizIDs, func(id int64) bool { return id != 0 }))
	if len(bkBizIDs) > MaxListFilterValues {
		return nil, nil, fmt.Errorf("bk_biz_ids must be less than or equal to %d", MaxListFilterValues)
	}
	return users, bkBizIDs, nil
}

// SyncHistoryReq is the request for POST /eval/runs/history/sync.
type SyncHistoryReq struct {
	// SessionCodes is the session_code list to sync. Required, at most 20 unique values.
	SessionCodes []string `json:"session_codes" validate:"required,min=1,dive,required,max=128"`
}

// Validate validates the history sync request.
func (req *SyncHistoryReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if len(req.SessionCodes) > constant.MaxHistorySyncSessionLimit {
		return fmt.Errorf("session_codes must be <= %d", constant.MaxHistorySyncSessionLimit)
	}
	req.SessionCodes = slice.Unique(req.SessionCodes)
	return nil
}

// SyncHistoryResp is the result of a history sync call.
type SyncHistoryResp struct {
	SessionScanned uint64 `json:"session_scanned"`
	SessionFailed  uint64 `json:"session_failed"`
	RunCreated     uint64 `json:"run_created"`
	RunSkipped     uint64 `json:"run_skipped"`
	RunFailed      uint64 `json:"run_failed"`
}

// RunDetailResp is GET /eval/runs/{run_id}.
type RunDetailResp struct {
	RunID       string         `json:"run_id"`
	SessionCode string         `json:"session_code"`
	User        string         `json:"user"`
	Scene       string         `json:"scene"`
	Status      string         `json:"status"`
	Query       string         `json:"query"`
	Summary     string         `json:"summary"`
	Eval        *EvalDetail    `json:"eval"`
	Window      []WindowDetail `json:"window"`
}

// EvalDetail is the eval block on the detail page.
type EvalDetail struct {
	ProcessScore int            `json:"process_score"`
	OutcomeScore int            `json:"outcome_score"`
	QualityScore int            `json:"quality_score"`
	Passed       bool           `json:"passed"`
	Redlines     []string       `json:"redlines"`
	ReasonCode   string         `json:"reason_code"`
	Dims         map[string]int `json:"dims"`
	Summary      string         `json:"summary"`
	Trace        any            `json:"eval_trace,omitempty"`
}

// WindowDetail is one drill-down row.
type WindowDetail struct {
	RunID      string `json:"run_id"`
	Role       string `json:"role"`
	Brief      string `json:"brief"`
	Query      string `json:"query"`
	Transcript any    `json:"transcript"`
}
