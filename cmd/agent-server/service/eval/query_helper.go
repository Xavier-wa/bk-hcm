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

package eval

import (
	"encoding/json"
	"fmt"
	"time"

	evalogic "hcm/cmd/agent-server/logics/eval"
	proto "hcm/pkg/api/agent-server/eval"
	"hcm/pkg/api/core"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/dal/dao/tools"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/times"
)

func passThreshold() int {
	// passThreshold 只用于查询侧「通过」判定，不写库。
	return cc.AgentServer().Eval.PassThreshold
}

func isPassed(quality int, redlines []string) bool {
	return quality >= passThreshold() && len(redlines) == 0
}

func (svc *service) buildWindow(kt *kit.Kit, snapshot evalogic.ContextSnapshot) []proto.WindowDetail {
	ids := make([]string, 0, len(snapshot.Runs))
	for _, r := range snapshot.Runs {
		ids = append(ids, r.RunID)
	}
	runMap, err := svc.loadRunsByIDs(kt, ids, nil)
	if err != nil {
		logs.Warnf("load window runs failed, err: %v, rid: %s", err, kt.Rid)
		runMap = map[string]tableaiagent.RunTable{}
	}
	out := make([]proto.WindowDetail, 0, len(snapshot.Runs))
	for _, r := range snapshot.Runs {
		item := proto.WindowDetail{RunID: r.RunID, Role: string(r.Role), Brief: r.Brief}
		if run, ok := runMap[r.RunID]; ok {
			item.Query = run.Query
			item.Transcript = jsonItems(run.Transcript)
		}
		out = append(out, item)
	}
	return out
}

func (svc *service) loadRunsByIDs(kt *kit.Kit, ids, fields []string) (
	map[string]tableaiagent.RunTable, error) {

	out := make(map[string]tableaiagent.RunTable, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	for start := 0; start < len(ids); start += int(core.DefaultMaxPageLimit) {
		end := start + int(core.DefaultMaxPageLimit)
		if end > len(ids) {
			end = len(ids)
		}
		result, err := svc.cli.DataService().Aiagent.Run.List(kt, &core.ListReq{
			Filter: tools.ContainersExpression("run_id", ids[start:end]),
			Fields: fields,
			Page:   &core.BasePage{Start: 0, Limit: core.DefaultMaxPageLimit},
		})
		if err != nil {
			logs.Errorf("list runs by ids failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		for _, r := range result.Details {
			out[r.RunID] = r
		}
	}
	return out, nil
}

// periodExpr builds the common [from, to] + optional scene filter used by dashboard list queries.
// from/to are RFC3339 request strings; they are normalized to CST wall-clock RFC3339
// (constant.TimeStdFormat) before being put into the filter. data-service Time columns reject
// naive DATETIME strings like "2006-01-02 15:04:05", so the value must keep the T separator and
// timezone offset; converting into Asia/Shanghai first keeps the intended CST range.
func periodExpr(field, from, to, scene string, extra ...*filter.AtomRule) (*filter.Expression, error) {
	fromCST, err := rfc3339ToCSTDateTime(from)
	if err != nil {
		return nil, fmt.Errorf("invalid from: %w", err)
	}
	toCST, err := rfc3339ToCSTDateTime(to)
	if err != nil {
		return nil, fmt.Errorf("invalid to: %w", err)
	}

	rules := []*filter.AtomRule{
		tools.RuleGreaterThanEqual(field, fromCST),
		tools.RuleLessThanEqual(field, toCST),
	}
	if scene != "" {
		rules = append(rules, tools.RuleEqual("scene", scene))
	}
	rules = append(rules, extra...)
	return tools.ExpressionAnd(rules...), nil
}

// rfc3339ToCSTDateTime parses an RFC3339 timestamp and formats it as CST wall-clock TimeStdFormat.
func rfc3339ToCSTDateTime(raw string) (string, error) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "", err
	}
	return t.In(times.CST()).Format(constant.TimeStdFormat), nil
}

func jsonItems(raw types.JsonField) any {
	if raw == "" || raw == "{}" {
		return evalogic.Transcript{Items: []evalogic.TranscriptItem{}}
	}
	var t evalogic.Transcript
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return json.RawMessage(raw)
	}
	return t
}
