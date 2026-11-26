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
	"hcm/pkg/criteria/enumor"
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

// IacState iac_state 表 DAO 接口。
type IacState interface {
	BatchCreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []tableiac.IacStateTable) ([]string, error)
	UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression, model *tableiac.IacStateTable) error
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
	List(kt *kit.Kit, opt *types.ListOption) (*types.ListIacStateDetails, error)
}

var _ IacState = new(IacStateDao)

// IacStateDao 实现 IacState 接口。
type IacStateDao struct {
	Orm   orm.Interface
	IDGen idgenerator.IDGenInterface
}

// BatchCreateWithTx 在事务中批量创建 iac_state 记录。
func (d *IacStateDao) BatchCreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []tableiac.IacStateTable) ([]string, error) {
	if len(models) == 0 {
		return nil, errf.New(errf.InvalidParameter, "iac state models to create cannot be empty")
	}

	for index := range models {
		if err := models[index].InsertValidate(); err != nil {
			logs.Errorf("insert validate iac_state failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
	}

	ids, err := d.IDGen.Batch(kt, table.IacStateTable, len(models))
	if err != nil {
		logs.Errorf("generate id for %s failed, err: %v, rid: %s", table.IacStateTable, err, kt.Rid)
		return nil, err
	}
	for index := range models {
		models[index].ID = ids[index]
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s) VALUES(%s)`, table.IacStateTable,
		tableiac.IacStateTableColumns.ColumnExpr(), tableiac.IacStateTableColumns.ColonNameExpr())

	err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Txn(tx).BulkInsert(kt.Ctx, sql, models)
	if err != nil {
		logs.Errorf("insert %s failed, sql: %s, err: %v, rid: %s", table.IacStateTable, sql, err, kt.Rid)
		return nil, fmt.Errorf("insert %s failed, err: %w", table.IacStateTable, err)
	}

	return ids, nil
}

// buildIacStateUpdateSQL 构造 iac_state 更新 SET 表达式与参数映射。
// 当 model.Current 为 JSON null 字面量时，将参数值显式置为 nil，使数据库写入 SQL NULL。
func buildIacStateUpdateSQL(model *tableiac.IacStateTable) (string, map[string]interface{}, error) {
	opts := utils.NewFieldOptions().
		AddIgnoredFields(types.DefaultIgnoredFields...).
		AddIgnoredFields("repository_id", "tenant_id").
		AddBlankedFields("import_from_spec_id")

	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(model, opts)
	if err != nil {
		return "", nil, err
	}

	if model.Current.IsNull() {
		toUpdate["current"] = nil
	}

	return setExpr, toUpdate, nil
}

// UpdateWithTx 在事务中按过滤条件更新 iac_state 记录。
func (d *IacStateDao) UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *tableiac.IacStateTable) error {

	if filterExpr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is nil")
	}
	if model == nil {
		return errf.New(errf.InvalidParameter, "iac state update model is nil")
	}

	if err := model.UpdateValidate(); err != nil {
		logs.Errorf("update validate iac_state failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	whereExpr, whereValue, err := filterExpr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate update where expr for iac_state failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	setExpr, toUpdate, err := buildIacStateUpdateSQL(model)
	if err != nil {
		logs.Errorf("rearrange sql data for iac_state failed, err: %v, rid: %s", err, kt.Rid)
		return fmt.Errorf("prepare parsed sql set filter expr failed, err: %w", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, model.TableName(), setExpr, whereExpr)
	_, err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Txn(tx).Update(
		kt.Ctx, sql, tools.MapMerge(toUpdate, whereValue))
	if err != nil {
		logs.ErrorJson("update iac_state failed, filter: %v, err: %v, rid: %s", filterExpr, err, kt.Rid)
		return err
	}

	return nil
}

// DeleteWithTx 在事务中按过滤条件删除 iac_state 记录。
func (d *IacStateDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate delete where expr for iac_state failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.IacStateTable, whereExpr)
	if _, err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Txn(tx).Delete(
		kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete iac_state failed, err: %v, filter: %v, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}

// List 查询 iac_state 记录。
func (d *IacStateDao) List(kt *kit.Kit, opt *types.ListOption) (*types.ListIacStateDetails, error) {
	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list iac state option is nil")
	}

	columnTypes := tableiac.IacStateTableColumns.ColumnTypes()
	columnTypes["current.*"] = enumor.Json
	columnTypes["deposed.*"] = enumor.Json
	exprOpt := filter.NewExprOption(filter.RuleFields(columnTypes))
	if err := opt.Validate(exprOpt, core.NewDefaultPageOption()); err != nil {
		logs.Errorf("validate list iac_state option failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate where expr for iac_state failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if opt.Page.Count {
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.IacStateTable, whereExpr)
		count, err := d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count iac_state failed, err: %v, filter: %v, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}
		return &types.ListIacStateDetails{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		logs.Errorf("generate page sql expr for iac_state failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`, tableiac.IacStateTableColumns.FieldsNamedExpr(opt.Fields),
		table.IacStateTable, whereExpr, pageExpr)

	details := make([]tableiac.IacStateTable, 0)
	err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Do().Select(kt.Ctx, &details, sql, whereValue)
	if err != nil {
		logs.ErrorJson("select iac_state failed, sql: %s, err: %v, filter: %v, rid: %s", sql, err, opt.Filter, kt.Rid)
		return nil, err
	}

	return &types.ListIacStateDetails{Details: details}, nil
}
