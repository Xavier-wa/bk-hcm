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

// Package host ...
package host

import (
	"fmt"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/audit"
	idgenerator "hcm/pkg/dal/dao/id-generator"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	"hcm/pkg/dal/dao/types/dissolve/host"
	"hcm/pkg/dal/table"
	define "hcm/pkg/dal/table/dissolve/host"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"

	"github.com/jmoiron/sqlx"
)

// RecycleHost defines recycle host dao operations.
type RecycleHost interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []define.RecycleHostTable) ([]string, error)
	UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression, Host *define.RecycleHostTable) error
	List(kt *kit.Kit, opt *types.ListOption, whereOpts ...*filter.SQLWhereOption) (*host.ListRecycleHostDetails,
		error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
	ListExpectAbolishTime(kt *kit.Kit, expr *filter.Expression) ([]string, error)
}

var _ RecycleHost = new(Dao)

// Dao recycle host dao.
type Dao struct {
	orm   orm.Interface
	idGen idgenerator.IDGenInterface
	audit audit.Interface
}

// NewRecycleHostDao create a recycle host dao.
func NewRecycleHostDao(orm orm.Interface, idGen idgenerator.IDGenInterface, audit audit.Interface) RecycleHost {
	return &Dao{
		orm:   orm,
		idGen: idGen,
		audit: audit,
	}
}

// CreateWithTx create recycle host with transaction.
func (d *Dao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, hosts []define.RecycleHostTable) ([]string, error) {
	if len(hosts) == 0 {
		return nil, errf.New(errf.InvalidParameter, "hosts to create cannot be empty")
	}

	ids, err := d.idGen.Batch(kt, table.RecycleHostInfo, len(hosts))
	if err != nil {
		return nil, err
	}

	for idx := range hosts {
		hosts[idx].ID = ids[idx]
		hosts[idx].Creator = kt.User
		if err = hosts[idx].InsertValidate(); err != nil {
			return nil, err
		}
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s)	VALUES(%s)`, table.RecycleHostInfo,
		define.RecycleHostColumns.ColumnExpr(), define.RecycleHostColumns.ColonNameExpr())

	err = d.orm.Txn(tx).BulkInsert(kt.Ctx, sql, hosts)
	if err != nil {
		return nil, fmt.Errorf("insert %s failed, err: %v", table.RecycleHostInfo, err)
	}

	return ids, nil
}

// UpdateWithTx update recycle host with transaction.
func (d *Dao) UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression, host *define.RecycleHostTable) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is nil")
	}

	host.Reviser = kt.User
	if err := host.UpdateValidate(); err != nil {
		return err
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	opts := utils.NewFieldOptions().AddIgnoredFields(types.DefaultIgnoredFields...)
	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(host, opts)
	if err != nil {
		return fmt.Errorf("prepare parsed sql set filter expr failed, err: %v", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, host.TableName(), setExpr, whereExpr)

	effected, err := d.orm.Txn(tx).Update(kt.Ctx, sql, tools.MapMerge(toUpdate, whereValue))
	if err != nil {
		logs.ErrorJson("update resource recycle host failed, err: %v, filter: %v, rid: %s", err, expr, kt.Rid)
		return err
	}

	if effected == 0 {
		logs.ErrorJson("update resource recycle host, but data not found, filter: %v, rid: %s", expr, kt.Rid)
		return errf.New(errf.RecordNotFound, orm.ErrRecordNotFound.Error())
	}

	return nil
}

// List recycle hosts.
func (d *Dao) List(kt *kit.Kit, opt *types.ListOption, whereOpts ...*filter.SQLWhereOption) (
	*host.ListRecycleHostDetails, error) {

	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list recycle host options is nil")
	}

	if err := opt.Validate(filter.NewExprOption(filter.RuleFields(define.RecycleHostColumns.ColumnTypes())),
		core.NewDefaultPageOption()); err != nil {
		return nil, err
	}

	whereOpt := tools.DefaultSqlWhereOption
	if len(whereOpts) != 0 && whereOpts[0] != nil {
		err := whereOpts[0].Validate()
		if err != nil {
			return nil, err
		}
		whereOpt = whereOpts[0]
	}

	if opt.Filter == nil {
		opt.Filter = tools.AllExpression()
	}
	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(whereOpt)
	if err != nil {
		return nil, err
	}

	if opt.Page.Count {
		// this is a count request, do count operation only.
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.RecycleHostInfo, whereExpr)

		count, err := d.orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count recycle hosts failed, err: %v, filter: %v, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &host.ListRecycleHostDetails{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`, define.RecycleHostColumns.FieldsNamedExpr(opt.Fields),
		table.RecycleHostInfo, whereExpr, pageExpr)

	details := make([]define.RecycleHostTable, 0)
	if err = d.orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		return nil, err
	}

	return &host.ListRecycleHostDetails{Details: details}, nil
}

// DeleteWithTx delete recycle host with transaction.
func (d *Dao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression) error {
	if filterExpr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := filterExpr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.RecycleHostInfo, whereExpr)
	if _, err = d.orm.Txn(tx).Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete recycle host failed, err: %v, filter: %v, rid: %s", err, filterExpr, kt.Rid)
		return err
	}

	return nil
}

// ListExpectAbolishTime 按 expect_abolish_time 分组去重、升序返回非空的裁撤截止时间列表。
func (d *Dao) ListExpectAbolishTime(kt *kit.Kit, expr *filter.Expression) ([]string, error) {
	// 恒定排除空串截止时间
	rules := []filter.RuleFactory{tools.RuleNotEqual("expect_abolish_time", "")}
	if expr != nil {
		rules = append(rules, expr)
	}
	finalExpr := &filter.Expression{Op: filter.And, Rules: rules}

	if err := finalExpr.Validate(filter.NewExprOption(
		filter.RuleFields(define.RecycleHostColumns.ColumnTypes()))); err != nil {
		return nil, err
	}

	whereExpr, whereValue, err := finalExpr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT expect_abolish_time FROM %s %s GROUP BY expect_abolish_time `+
		`ORDER BY expect_abolish_time ASC`, table.RecycleHostInfo, whereExpr)

	rows := make([]struct {
		ExpectAbolishTime string `db:"expect_abolish_time"`
	}, 0)
	if err = d.orm.Do().Select(kt.Ctx, &rows, sql, whereValue); err != nil {
		logs.ErrorJson("list recycle host expect abolish time failed, err: %v, filter: %v, rid: %s", err,
			finalExpr, kt.Rid)
		return nil, err
	}

	times := make([]string, 0, len(rows))
	for _, row := range rows {
		times = append(times, row.ExpectAbolishTime)
	}

	return times, nil
}
