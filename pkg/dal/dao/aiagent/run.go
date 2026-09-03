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

	"hcm/pkg/api/core"
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
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"

	"github.com/jmoiron/sqlx"
)

// AiagentRun defines aiagent run DAO operations.
type AiagentRun interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, run *tableaiagent.RunTable) (string, error)
	UpdateStatusCAS(kt *kit.Kit, runID string, status enumor.AiagentRunStatus, reason, query string,
		transcript tabletypes.JsonField, reviser string) error
	PatchTranscript(kt *kit.Kit, runID, query string, transcript tabletypes.JsonField, reviser string) error
	PatchOccurredAtWithTx(kt *kit.Kit, tx *sqlx.Tx, runID, occurredAt, endedAt, reviser string) error
	List(kt *kit.Kit, opt *types.ListOption) (*daoaiagent.ListAiagentRuns, error)
}

var _ AiagentRun = new(AiagentRunDao)

// AiagentRunDao is the DAO implementation for aiagent run.
type AiagentRunDao struct {
	orm   orm.Interface
	idGen idgenerator.IDGenInterface
	audit audit.Interface
}

// NewAiagentRunDao creates a new AiagentRunDao.
func NewAiagentRunDao(orm orm.Interface, idGen idgenerator.IDGenInterface, audit audit.Interface) AiagentRun {
	return &AiagentRunDao{orm: orm, idGen: idGen, audit: audit}
}

// CreateWithTx creates an aiagent run within a transaction.
func (d *AiagentRunDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, run *tableaiagent.RunTable) (string, error) {
	id, err := d.idGen.One(kt, table.AiagentRunTable)
	if err != nil {
		return "", err
	}

	run.ID = id
	if run.Status == "" {
		run.Status = enumor.AiagentRunStatusRunning
	}
	if run.Scene == "" {
		run.Scene = enumor.IntentTypeUnsupported
	}
	run.Creator = kt.User
	run.Reviser = kt.User

	if err = run.InsertValidate(); err != nil {
		return "", err
	}

	sqlStr := fmt.Sprintf(`INSERT INTO %s (%s) VALUES(%s)`, table.AiagentRunTable,
		tableaiagent.RunColumns.ColumnExpr(), tableaiagent.RunColumns.ColonNameExpr())

	if err = d.orm.Txn(tx).BulkInsert(kt.Ctx, sqlStr, []tableaiagent.RunTable{*run}); err != nil {
		if em := errf.GetMySQLDuplicated(err); em != nil {
			logs.Warnf("insert aiagent_run hit duplicated key, err: %v, run_id: %s, rid: %s",
				err, run.RunID, kt.Rid)
			return "", errf.New(errf.RecordDuplicated, em.Message)
		}
		return "", fmt.Errorf("insert %s failed, err: %v", table.AiagentRunTable, err)
	}

	return id, nil
}

// UpdateStatusCAS CAS-updates a running row to a terminal status.
// Zero affected rows returns RecordNotUpdate. Empty query keeps the existing column
// so /cancel and orphan sweep do not wipe the create-time user text.
func (d *AiagentRunDao) UpdateStatusCAS(kt *kit.Kit, runID string, status enumor.AiagentRunStatus,
	reason, query string, transcript tabletypes.JsonField, reviser string) error {

	if runID == "" {
		return errf.New(errf.InvalidParameter, "run_id can not be empty")
	}
	if !status.IsTerminal() {
		return errf.New(errf.InvalidParameter, "status must be a terminal state")
	}
	if reviser == "" {
		reviser = kt.User
	}

	sqlStr := casUpdateSQL()

	effected, err := d.orm.Do().Update(kt.Ctx, sqlStr, map[string]interface{}{
		"status":      status,
		"reason":      reason,
		"query":       query,
		"transcript":  transcript,
		"reviser":     reviser,
		"run_id":      runID,
		"from_status": enumor.AiagentRunStatusRunning,
	})
	if err != nil {
		logs.Errorf("cas update aiagent_run failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
		return err
	}
	return errIfCASZeroRows(effected)
}

func casUpdateSQL() string {
	return fmt.Sprintf(`UPDATE %s SET status=:status, reason=:reason,
query=IF(:query='', query, :query), transcript=:transcript,
reviser=:reviser WHERE run_id=:run_id AND status=:from_status`, table.AiagentRunTable)
}

func errIfCASZeroRows(effected int64) error {
	if effected == 0 {
		return errf.New(errf.RecordNotUpdate, "record not update")
	}
	return nil
}

// PatchTranscript writes query/transcript without refreshing updated_at.
func (d *AiagentRunDao) PatchTranscript(kt *kit.Kit, runID, query string, transcript tabletypes.JsonField,
	reviser string) error {

	if runID == "" {
		return errf.New(errf.InvalidParameter, "run_id can not be empty")
	}
	if reviser == "" {
		reviser = kt.User
	}

	sqlStr := patchTranscriptSQL()

	effected, err := d.orm.Do().Update(kt.Ctx, sqlStr, map[string]interface{}{
		"query":      query,
		"transcript": transcript,
		"reviser":    reviser,
		"run_id":     runID,
	})
	if err != nil {
		logs.Errorf("patch aiagent_run transcript failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
		return err
	}
	if effected == 0 {
		return errf.New(errf.RecordNotFound, "record not found")
	}
	return nil
}

// List queries aiagent runs with filter and pagination.
func (d *AiagentRunDao) List(kt *kit.Kit, opt *types.ListOption) (*daoaiagent.ListAiagentRuns, error) {
	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list aiagent run options is nil")
	}

	if err := opt.Validate(filter.NewExprOption(filter.RuleFields(tableaiagent.RunColumns.ColumnTypes())),
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
		sqlStr := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.AiagentRunTable, whereExpr)
		count, err := d.orm.Do().Count(kt.Ctx, sqlStr, whereValue)
		if err != nil {
			logs.ErrorJson("count aiagent runs failed, err: %v, filter: %s, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}
		return &daoaiagent.ListAiagentRuns{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		return nil, err
	}

	sqlStr := fmt.Sprintf(`SELECT %s FROM %s %s %s`,
		tableaiagent.RunColumns.FieldsNamedExpr(opt.Fields),
		table.AiagentRunTable, whereExpr, pageExpr)

	details := make([]tableaiagent.RunTable, 0)
	if err = d.orm.Do().Select(kt.Ctx, &details, sqlStr, whereValue); err != nil {
		return nil, err
	}
	return &daoaiagent.ListAiagentRuns{Details: details}, nil
}

func patchTranscriptSQL() string {
	return fmt.Sprintf(`UPDATE %s SET query=:query, transcript=:transcript, reviser=:reviser,
updated_at=updated_at WHERE run_id=:run_id`, table.AiagentRunTable)
}

// PatchOccurredAtWithTx 在历史插入后回写 created_at / updated_at。
// 实时 Ledger 创建不会调用本方法，时间戳保持库默认 now()。
func (d *AiagentRunDao) PatchOccurredAtWithTx(kt *kit.Kit, tx *sqlx.Tx, runID, occurredAt, endedAt,
	reviser string) error {

	if runID == "" {
		return errf.New(errf.InvalidParameter, "run_id can not be empty")
	}
	if occurredAt == "" {
		return errf.New(errf.InvalidParameter, "occurred_at can not be empty")
	}
	if endedAt == "" {
		endedAt = occurredAt
	}
	if reviser == "" {
		reviser = kt.User
	}

	effected, err := d.orm.Txn(tx).Update(kt.Ctx, patchOccurredAtSQL(), map[string]interface{}{
		"created_at": occurredAt,
		"updated_at": endedAt,
		"reviser":    reviser,
		"run_id":     runID,
	})
	if err != nil {
		logs.Errorf("patch aiagent_run occurred_at failed, err: %v, run_id: %s, rid: %s",
			err, runID, kt.Rid)
		return err
	}
	if effected == 0 {
		return errf.New(errf.RecordNotFound, "record not found")
	}
	return nil
}

func patchOccurredAtSQL() string {
	return fmt.Sprintf(`UPDATE %s SET created_at=:created_at, updated_at=:updated_at, reviser=:reviser
WHERE run_id=:run_id`, table.AiagentRunTable)
}
