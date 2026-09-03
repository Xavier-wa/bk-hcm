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
	"fmt"
	"strings"
	"time"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/audit"
	idgenerator "hcm/pkg/dal/dao/id-generator"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	daoaiagent "hcm/pkg/dal/dao/types/aiagent"
	"hcm/pkg/dal/table"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/times"

	"github.com/jmoiron/sqlx"
)

// AiagentRunEvalGapOption is the option of listing unevaluated terminal runs.
// From+To and Lookback are mutually exclusive windows; From+To wins when both are set.
// All timestamps are formatted as CST DATETIME before binding to MySQL.
type AiagentRunEvalGapOption struct {
	Lookback time.Duration
	From     time.Time
	To       time.Time
	Limit    uint
	Count    bool
	Start    uint32
}

// AiagentRunEval defines aiagent run eval DAO operations.
type AiagentRunEval interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, model *tableaiagent.RunEvalTable) (string, error)
	GetByRunID(kt *kit.Kit, runID string) (*tableaiagent.RunEvalTable, error)
	List(kt *kit.Kit, opt *types.ListOption) (*daoaiagent.ListAiagentRunEvals, error)
	ListGap(kt *kit.Kit, opt *AiagentRunEvalGapOption) (*daoaiagent.ListAiagentRuns, error)
	UpdateByRunIDWithTx(kt *kit.Kit, tx *sqlx.Tx, model *tableaiagent.RunEvalTable) error
}

var _ AiagentRunEval = new(AiagentRunEvalDao)

// AiagentRunEvalDao is the DAO implementation for aiagent run eval.
type AiagentRunEvalDao struct {
	orm   orm.Interface
	idGen idgenerator.IDGenInterface
	audit audit.Interface
}

// NewAiagentRunEvalDao creates a new AiagentRunEvalDao.
func NewAiagentRunEvalDao(orm orm.Interface, idGen idgenerator.IDGenInterface,
	audit audit.Interface) AiagentRunEval {

	return &AiagentRunEvalDao{orm: orm, idGen: idGen, audit: audit}
}

// CreateWithTx creates an eval row within a transaction.
func (d *AiagentRunEvalDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, model *tableaiagent.RunEvalTable) (
	string, error) {

	id, err := d.idGen.One(kt, table.AiagentRunEvalTable)
	if err != nil {
		return "", err
	}
	model.ID = id
	model.Creator = kt.User
	model.Reviser = kt.User
	if model.Redlines == nil {
		model.Redlines = []string{}
	}
	if err = model.InsertValidate(); err != nil {
		return "", err
	}

	sqlStr := fmt.Sprintf(`INSERT INTO %s (%s) VALUES(%s)`, table.AiagentRunEvalTable,
		tableaiagent.RunEvalColumns.ColumnExpr(), tableaiagent.RunEvalColumns.ColonNameExpr())

	if err = d.orm.Txn(tx).BulkInsert(kt.Ctx, sqlStr, []tableaiagent.RunEvalTable{*model}); err != nil {
		if em := errf.GetMySQLDuplicated(err); em != nil {
			logs.Warnf("insert aiagent_run_eval hit duplicated key, err: %v, run_id: %s, rid: %s",
				err, model.RunID, kt.Rid)
			return "", errf.New(errf.RecordDuplicated, em.Message)
		}
		return "", fmt.Errorf("insert %s failed, err: %v", table.AiagentRunEvalTable, err)
	}
	return id, nil
}

// GetByRunID returns one eval row by run_id.
func (d *AiagentRunEvalDao) GetByRunID(kt *kit.Kit, runID string) (*tableaiagent.RunEvalTable, error) {
	if runID == "" {
		return nil, errf.New(errf.InvalidParameter, "run_id can not be empty")
	}

	sqlStr := fmt.Sprintf(`SELECT %s FROM %s WHERE run_id = :run_id`,
		tableaiagent.RunEvalColumns.FieldsNamedExpr(nil), table.AiagentRunEvalTable)

	details := make([]tableaiagent.RunEvalTable, 0)
	if err := d.orm.Do().Select(kt.Ctx, &details, sqlStr, map[string]interface{}{"run_id": runID}); err != nil {
		return nil, err
	}
	if len(details) == 0 {
		return nil, errf.New(errf.RecordNotFound, "record not found")
	}
	return &details[0], nil
}

// List queries eval rows with filter and pagination.
func (d *AiagentRunEvalDao) List(kt *kit.Kit, opt *types.ListOption) (*daoaiagent.ListAiagentRunEvals, error) {
	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list aiagent run eval options is nil")
	}

	if err := opt.Validate(filter.NewExprOption(filter.RuleFields(tableaiagent.RunEvalColumns.ColumnTypes())),
		core.NewDefaultPageOption()); err != nil {
		return nil, err
	}

	if opt.Filter == nil {
		opt.Filter = tools.AllExpression()
	}
	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	if opt.Page.Count {
		sqlStr := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.AiagentRunEvalTable, whereExpr)
		count, err := d.orm.Do().Count(kt.Ctx, sqlStr, whereValue)
		if err != nil {
			logs.ErrorJson("count aiagent run evals failed, err: %v, filter: %s, rid: %s",
				err, opt.Filter, kt.Rid)
			return nil, err
		}
		return &daoaiagent.ListAiagentRunEvals{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		return nil, err
	}

	sqlStr := fmt.Sprintf(`SELECT %s FROM %s %s %s`,
		tableaiagent.RunEvalColumns.FieldsNamedExpr(opt.Fields),
		table.AiagentRunEvalTable, whereExpr, pageExpr)

	details := make([]tableaiagent.RunEvalTable, 0)
	if err = d.orm.Do().Select(kt.Ctx, &details, sqlStr, whereValue); err != nil {
		return nil, err
	}
	return &daoaiagent.ListAiagentRunEvals{Details: details}, nil
}

// ListGap lists terminal runs that have no eval row.
func (d *AiagentRunEvalDao) ListGap(kt *kit.Kit, opt *AiagentRunEvalGapOption) (*daoaiagent.ListAiagentRuns, error) {
	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "gap list option is nil")
	}

	sqlStr, args, err := buildGapSQL(opt, time.Now())
	if err != nil {
		return nil, err
	}

	if opt.Count {
		count, err := d.orm.Do().Count(kt.Ctx, sqlStr, args)
		if err != nil {
			logs.Errorf("count aiagent_run eval gap failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		return &daoaiagent.ListAiagentRuns{Count: count}, nil
	}

	details := make([]tableaiagent.RunTable, 0)
	if err := d.orm.Do().Select(kt.Ctx, &details, sqlStr, args); err != nil {
		logs.Errorf("list aiagent_run eval gap failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	return &daoaiagent.ListAiagentRuns{Details: details}, nil
}

// UpdateByRunIDWithTx overwrites an existing eval row in a transaction.
func (d *AiagentRunEvalDao) UpdateByRunIDWithTx(kt *kit.Kit, tx *sqlx.Tx,
	model *tableaiagent.RunEvalTable) error {
	if model == nil || model.RunID == "" {
		return errf.New(errf.InvalidParameter, "run_id can not be empty")
	}
	model.Creator = ""
	if model.Reviser == "" {
		model.Reviser = kt.User
	}
	if err := model.UpdateValidate(); err != nil {
		return err
	}

	sqlStr := fmt.Sprintf(`UPDATE %s SET session_id=:session_id, user=:user, bk_biz_id=:bk_biz_id,
start_run_id=:start_run_id, process_score=:process_score, outcome_score=:outcome_score,
quality_score=:quality_score, redlines=:redlines, reason_code=:reason_code,
rubric_version=:rubric_version, eval_result=:eval_result, context_snapshot=:context_snapshot,
eval_trace=:eval_trace, reviser=:reviser WHERE run_id=:run_id`, table.AiagentRunEvalTable)

	effected, err := d.orm.Txn(tx).Update(kt.Ctx, sqlStr, map[string]interface{}{
		"session_id":       model.SessionID,
		"user":             model.User,
		"bk_biz_id":        model.BkBizID,
		"start_run_id":     model.StartRunID,
		"process_score":    model.ProcessScore,
		"outcome_score":    model.OutcomeScore,
		"quality_score":    model.QualityScore,
		"redlines":         model.Redlines,
		"reason_code":      model.ReasonCode,
		"rubric_version":   model.RubricVersion,
		"eval_result":      model.EvalResult,
		"context_snapshot": model.ContextSnapshot,
		"eval_trace":       model.EvalTrace,
		"reviser":          model.Reviser,
		"run_id":           model.RunID,
	})
	if err != nil {
		logs.Errorf("update aiagent_run_eval failed, err: %v, run_id: %s, rid: %s", err, model.RunID, kt.Rid)
		return err
	}
	if effected == 0 {
		return errf.New(errf.RecordNotFound, "record not found")
	}
	return nil
}

func buildGapSQL(opt *AiagentRunEvalGapOption, now time.Time) (string, map[string]interface{}, error) {
	args := map[string]interface{}{}
	statusIn, err := terminalStatusInClause(args)
	if err != nil {
		return "", nil, err
	}
	where := statusIn + ` AND e.run_id IS NULL`
	if !opt.From.IsZero() && !opt.To.IsZero() {
		where += ` AND r.updated_at >= :from_tm AND r.updated_at <= :to_tm`
		args["from_tm"] = times.FormatDateTimeCST(opt.From)
		args["to_tm"] = times.FormatDateTimeCST(opt.To)
	} else {
		if opt.Lookback <= 0 {
			return "", nil, errf.New(errf.InvalidParameter, "lookback must be positive")
		}
		where += ` AND r.updated_at >= :since`
		args["since"] = times.FormatDateTimeCST(now.Add(-opt.Lookback))
	}

	if opt.Count {
		sqlStr := fmt.Sprintf(`SELECT COUNT(*) FROM %s r LEFT JOIN %s e ON e.run_id = r.run_id WHERE %s`,
			table.AiagentRunTable, table.AiagentRunEvalTable, where)
		return sqlStr, args, nil
	}

	limit := constant.AiagentRunEvalGapLimit(opt.Limit)
	sqlStr := fmt.Sprintf(`SELECT r.* FROM %s r LEFT JOIN %s e ON e.run_id = r.run_id WHERE %s
ORDER BY r.updated_at ASC LIMIT :start, :limit`,
		table.AiagentRunTable, table.AiagentRunEvalTable, where)
	args["start"] = opt.Start
	args["limit"] = limit
	return sqlStr, args, nil
}

// terminalStatusInClause 按终态列表拼 named 占位符，例如 r.status IN (:term_status_0, :term_status_1)。
func terminalStatusInClause(args map[string]interface{}) (string, error) {
	statuses := enumor.TerminalAiagentRunStatuses()
	if len(statuses) == 0 {
		return "", errf.New(errf.InvalidParameter, "terminal aiagent run statuses is empty")
	}
	holders := make([]string, 0, len(statuses))
	for i, status := range statuses {
		key := fmt.Sprintf("term_status_%d", i)
		holders = append(holders, ":"+key)
		args[key] = string(status)
	}
	return fmt.Sprintf("r.status IN (%s)", strings.Join(holders, ",")), nil
}
