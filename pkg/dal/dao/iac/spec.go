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

package daoiac

import (
	"fmt"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	idgenerator "hcm/pkg/dal/dao/id-generator"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	"hcm/pkg/dal/table"
	tableiac "hcm/pkg/dal/table/iac"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"

	"github.com/jmoiron/sqlx"
)

// IacSpec iac_spec 表 DAO 接口。
type IacSpec interface {
	BatchCreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []tableiac.IacSpecTable) ([]string, error)
	UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression, model *tableiac.IacSpecTable) error
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
	List(kt *kit.Kit, opt *types.ListOption) (*types.ListIacSpecDetails, error)
}

var _ IacSpec = new(IacSpecDao)

// IacSpecDao 实现 IacSpec 接口。
type IacSpecDao struct {
	Orm   orm.Interface
	IDGen idgenerator.IDGenInterface
}

// BatchCreateWithTx 在事务中批量创建 iac_spec 记录。
func (d *IacSpecDao) BatchCreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []tableiac.IacSpecTable) ([]string, error) {
	if len(models) == 0 {
		return nil, errf.New(errf.InvalidParameter, "iac spec models to create cannot be empty")
	}

	for index := range models {
		if err := models[index].InsertValidate(); err != nil {
			logs.Errorf("insert validate iac_spec failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
	}

	ids, err := d.IDGen.Batch(kt, table.IacSpecTable, len(models))
	if err != nil {
		logs.Errorf("generate id for %s failed, err: %v, rid: %s", table.IacSpecTable, err, kt.Rid)
		return nil, err
	}
	for index := range models {
		models[index].ID = ids[index]
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s) VALUES(%s)`, table.IacSpecTable, tableiac.IacSpecTableColumns.ColumnExpr(),
		tableiac.IacSpecTableColumns.ColonNameExpr())

	err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Txn(tx).BulkInsert(kt.Ctx, sql, models)
	if err != nil {
		logs.Errorf("insert %s failed, sql: %s, err: %v, rid: %s", table.IacSpecTable, sql, err, kt.Rid)
		return nil, fmt.Errorf("insert %s failed, err: %w", table.IacSpecTable, err)
	}

	return ids, nil
}

// UpdateWithTx 在事务中按过滤条件更新 iac_spec 记录。
func (d *IacSpecDao) UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *tableiac.IacSpecTable) error {

	if filterExpr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is nil")
	}
	if model == nil {
		return errf.New(errf.InvalidParameter, "iac spec update model is nil")
	}

	if err := model.UpdateValidate(); err != nil {
		logs.Errorf("update validate iac_spec failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	whereExpr, whereValue, err := filterExpr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate update where expr for iac_spec failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	opts := utils.NewFieldOptions().
		AddIgnoredFields(types.DefaultIgnoredFields...).
		AddIgnoredFields("bk_biz_id", "repository_id", "version", "tenant_id").
		AddBlankedFields("memo")

	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(model, opts)
	if err != nil {
		logs.Errorf("rearrange sql data for iac_spec failed, err: %v, rid: %s", err, kt.Rid)
		return fmt.Errorf("prepare parsed sql set filter expr failed, err: %w", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, model.TableName(), setExpr, whereExpr)
	_, err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Txn(tx).Update(
		kt.Ctx, sql, tools.MapMerge(toUpdate, whereValue))
	if err != nil {
		logs.ErrorJson("update iac_spec failed, filter: %v, err: %v, rid: %s", filterExpr, err, kt.Rid)
		return err
	}

	return nil
}

// DeleteWithTx 在事务中按过滤条件删除 iac_spec 记录。
func (d *IacSpecDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate delete where expr for iac_spec failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.IacSpecTable, whereExpr)
	if _, err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Txn(tx).Delete(
		kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete iac_spec failed, err: %v, filter: %v, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}

// List 查询 iac_spec 记录。
func (d *IacSpecDao) List(kt *kit.Kit, opt *types.ListOption) (*types.ListIacSpecDetails, error) {
	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list iac spec option is nil")
	}

	columnTypes := tableiac.IacSpecTableColumns.ColumnTypes()
	exprOpt := filter.NewExprOption(filter.RuleFields(columnTypes))
	if err := opt.Validate(exprOpt, core.NewDefaultPageOption()); err != nil {
		logs.Errorf("validate list iac_spec option failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate where expr for iac_spec failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if opt.Page.Count {
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.IacSpecTable, whereExpr)
		count, err := d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count iac_spec failed, err: %v, filter: %v, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}
		return &types.ListIacSpecDetails{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		logs.Errorf("generate page sql expr for iac_spec failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`, tableiac.IacSpecTableColumns.FieldsNamedExpr(opt.Fields),
		table.IacSpecTable, whereExpr, pageExpr)

	details := make([]tableiac.IacSpecTable, 0)
	err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Do().Select(kt.Ctx, &details, sql, whereValue)
	if err != nil {
		logs.ErrorJson("select iac_spec failed, sql: %s, err: %v, filter: %v, rid: %s", sql, err, opt.Filter, kt.Rid)
		return nil, err
	}

	return &types.ListIacSpecDetails{Details: details}, nil
}
