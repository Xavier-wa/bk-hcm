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

// Package aiagent defines DAO operations for the aiagent module.
package aiagent

import (
	"crypto/md5" // #nosec G501
	"fmt"
	"time"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/audit"
	idgenerator "hcm/pkg/dal/dao/id-generator"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	daoaiagent "hcm/pkg/dal/dao/types/aiagent"
	"hcm/pkg/dal/table"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"

	"github.com/jmoiron/sqlx"
)

// AiagentSession defines aiagent session DAO operations.
type AiagentSession interface {
	// CreateWithTx creates a session and returns (id, sessionCode, error).
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, session *tableaiagent.SessionTable) (id, sessionCode string, err error)
	Update(kt *kit.Kit, expr *filter.Expression, session *tableaiagent.SessionTable) error
	List(kt *kit.Kit, opt *types.ListOption) (*daoaiagent.ListAiagentSessions, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
	IncrContentCount(kt *kit.Kit, sessionCode string) error
}

var _ AiagentSession = new(AiagentSessionDao)

// AiagentSessionDao is the DAO implementation for aiagent session.
type AiagentSessionDao struct {
	orm   orm.Interface
	idGen idgenerator.IDGenInterface
	audit audit.Interface
}

// NewAiagentSessionDao creates a new AiagentSessionDao.
func NewAiagentSessionDao(orm orm.Interface, idGen idgenerator.IDGenInterface, audit audit.Interface) AiagentSession {
	return &AiagentSessionDao{
		orm:   orm,
		idGen: idGen,
		audit: audit,
	}
}

// CreateWithTx creates an aiagent session within a transaction.
// It auto-generates id, thread_id, and session_code (md5(appName+user+id) + "-" + YYYYMMDDHH).
// Returns the generated id and session_code.
func (d *AiagentSessionDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, sess *tableaiagent.SessionTable) (
	id, sessionCode string, err error) {

	id, err = d.idGen.One(kt, table.AiagentSessionTable)
	if err != nil {
		return "", "", err
	}

	sess.ID = id
	sess.ThreadID = id
	sess.SessionCode = generateSessionCode(sess.AppName, sess.User, id)
	sess.Creator = kt.User
	sess.Reviser = kt.User
	sessionCode = sess.SessionCode

	if err = sess.InsertValidate(); err != nil {
		return "", "", err
	}

	sqlStr := fmt.Sprintf(`INSERT INTO %s (%s) VALUES(%s)`, table.AiagentSessionTable,
		tableaiagent.SessionColumns.ColumnExpr(), tableaiagent.SessionColumns.ColonNameExpr())

	if err = d.orm.Txn(tx).BulkInsert(kt.Ctx, sqlStr, []tableaiagent.SessionTable{*sess}); err != nil {
		return "", "", fmt.Errorf("insert %s failed, err: %v", table.AiagentSessionTable, err)
	}

	return id, sessionCode, nil
}

// generateSessionCode generates session_code = hex(md5(appName+user+id)) + "-" + YYYYMMDDHH.
func generateSessionCode(appName, user, id string) string {
	hash := md5.Sum([]byte(appName + user + id)) // nolint:gosec
	ts := time.Now().Format("2006010215")
	return fmt.Sprintf("%x-%s", hash, ts)
}

// Update updates an aiagent session.
func (d *AiagentSessionDao) Update(kt *kit.Kit, expr *filter.Expression, sess *tableaiagent.SessionTable) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is nil")
	}

	sess.Reviser = kt.User
	if err := sess.UpdateValidate(); err != nil {
		return err
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	opts := utils.NewFieldOptions().AddIgnoredFields(types.DefaultIgnoredFields...)
	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(sess, opts)
	if err != nil {
		return fmt.Errorf("prepare parsed sql set filter expr failed, err: %v", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, sess.TableName(), setExpr, whereExpr)

	effected, err := d.orm.Do().Update(kt.Ctx, sql, tools.MapMerge(toUpdate, whereValue))
	if err != nil {
		logs.ErrorJson("update aiagent session failed, err: %v, filter: %s, rid: %v", err, expr, kt.Rid)
		return err
	}

	if effected == 0 {
		logs.ErrorJson("update aiagent session, but data not found, filter: %v, rid: %v", expr, kt.Rid)
		return errf.New(errf.RecordNotFound, orm.ErrRecordNotFound.Error())
	}

	return nil
}

// List queries aiagent sessions with filter and pagination.
func (d *AiagentSessionDao) List(kt *kit.Kit, opt *types.ListOption) (*daoaiagent.ListAiagentSessions, error) {
	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list aiagent session options is nil")
	}

	if err := opt.Validate(filter.NewExprOption(filter.RuleFields(tableaiagent.SessionColumns.ColumnTypes())),
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
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.AiagentSessionTable, whereExpr)

		count, err := d.orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count aiagent sessions failed, err: %v, filter: %s, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &daoaiagent.ListAiagentSessions{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`,
		tableaiagent.SessionColumns.FieldsNamedExpr(opt.Fields),
		table.AiagentSessionTable, whereExpr, pageExpr)

	sessions := make([]tableaiagent.SessionTable, 0)
	if err = d.orm.Do().Select(kt.Ctx, &sessions, sql, whereValue); err != nil {
		return nil, err
	}

	return &daoaiagent.ListAiagentSessions{Details: sessions}, nil
}

// DeleteWithTx deletes aiagent sessions within a transaction.
func (d *AiagentSessionDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression) error {
	if filterExpr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := filterExpr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.AiagentSessionTable, whereExpr)
	_, err = d.orm.Txn(tx).Delete(kt.Ctx, sql, whereValue)
	if err != nil {
		logs.ErrorJson("delete aiagent session failed, err: %v, filter: %v, rid: %s", err, filterExpr, kt.Rid)
		return err
	}

	return nil
}

// IncrContentCount atomically increments session_content_count by session_code.
func (d *AiagentSessionDao) IncrContentCount(kt *kit.Kit, sessionCode string) error {
	if sessionCode == "" {
		return errf.New(errf.InvalidParameter, "session_code can not be empty")
	}

	sql := fmt.Sprintf(
		`UPDATE %s SET session_content_count = session_content_count + 1 WHERE session_code = :session_code`,
		table.AiagentSessionTable,
	)

	effected, err := d.orm.Do().Update(kt.Ctx, sql, map[string]interface{}{"session_code": sessionCode})
	if err != nil {
		logs.ErrorJson("incr aiagent session content count failed, session_code: %s, err: %v, rid: %s",
			sessionCode, err, kt.Rid)
		return err
	}

	if effected == 0 {
		return errf.New(errf.RecordNotFound, "session not found by session_code")
	}

	return nil
}
