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

package daoplugin

import (
	"fmt"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	idgenerator "hcm/pkg/dal/dao/id-generator"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	"hcm/pkg/dal/table"
	tableplugin "hcm/pkg/dal/table/plugin"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"

	"github.com/jmoiron/sqlx"
)

// TODO(temp_delete): 临时 iac_plugin_temp_delete DAO，合并注册分支后删除。

// IacPluginTempDelete iac_plugin_temp_delete 表 DAO 接口。
type IacPluginTempDelete interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, model *tableplugin.IacPluginTempDeleteTable) (string, error)
	UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
		model *tableplugin.IacPluginTempDeleteTable) error
	List(kt *kit.Kit, opt *types.ListOption) (*types.ListIacPluginTempDeleteDetails, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
}

var _ IacPluginTempDelete = new(IacPluginTempDeleteDao)

// IacPluginTempDeleteDao 实现 IacPluginTempDelete 接口。
type IacPluginTempDeleteDao struct {
	Orm   orm.Interface
	IDGen idgenerator.IDGenInterface
}

// CreateWithTx 在事务中创建单条 iac_plugin_temp_delete 记录。
func (d *IacPluginTempDeleteDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx,
	model *tableplugin.IacPluginTempDeleteTable) (string, error) {

	if model == nil {
		return "", errf.New(errf.InvalidParameter, "iac plugin temp model is nil")
	}

	if err := model.InsertValidate(); err != nil {
		logs.Errorf("insert validate iac_plugin_temp_delete failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	id, err := d.IDGen.One(kt, model.TableName())
	if err != nil {
		logs.Errorf("generate id for %s failed, err: %v, rid: %s", model.TableName(), err, kt.Rid)
		return "", err
	}
	model.ID = id

	sql := fmt.Sprintf(`INSERT INTO %s (%s) VALUES(%s)`,
		model.TableName(),
		tableplugin.IacPluginTempDeleteTableColumns.ColumnExpr(),
		tableplugin.IacPluginTempDeleteTableColumns.ColonNameExpr())

	if err = d.Orm.Txn(tx).Insert(kt.Ctx, sql, model); err != nil {
		logs.Errorf("insert %s failed, sql: %s, err: %v, rid: %s", model.TableName(), sql, err, kt.Rid)
		return "", fmt.Errorf("insert %s failed, err: %w", model.TableName(), err)
	}

	return id, nil
}

// UpdateWithTx 在事务中按过滤条件更新 iac_plugin_temp_delete 记录。
func (d *IacPluginTempDeleteDao) UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *tableplugin.IacPluginTempDeleteTable) error {

	if filterExpr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is nil")
	}
	if model == nil {
		return errf.New(errf.InvalidParameter, "iac plugin temp update model is nil")
	}

	if err := model.UpdateValidate(); err != nil {
		logs.Errorf("update validate iac_plugin_temp_delete failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	whereExpr, whereValue, err := filterExpr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate update where expr for iac_plugin_temp_delete failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	opts := utils.NewFieldOptions().
		AddIgnoredFields(types.DefaultIgnoredFields...).
		AddIgnoredFields("plugin_name", "plugin_version").
		AddBlankedFields("message", "memo")

	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(model, opts)
	if err != nil {
		logs.Errorf("rearrange sql data for iac_plugin_temp_delete failed, err: %v, rid: %s", err, kt.Rid)
		return fmt.Errorf("prepare parsed sql set filter expr failed, err: %w", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, model.TableName(), setExpr, whereExpr)
	if _, err = d.Orm.Txn(tx).Update(kt.Ctx, sql, tools.MapMerge(toUpdate, whereValue)); err != nil {
		logs.ErrorJson("update iac_plugin_temp_delete failed, filter: %v, err: %v, rid: %s", filterExpr, err, kt.Rid)
		return err
	}

	return nil
}

// List 查询 iac_plugin_temp_delete 记录。
func (d *IacPluginTempDeleteDao) List(kt *kit.Kit, opt *types.ListOption) (
	*types.ListIacPluginTempDeleteDetails, error) {

	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list iac plugin temp option is nil")
	}

	exprOpt := filter.NewExprOption(filter.RuleFields(tableplugin.IacPluginTempDeleteTableColumns.ColumnTypes()))
	if err := opt.Validate(exprOpt, core.NewDefaultPageOption()); err != nil {
		logs.Errorf("validate list iac_plugin_temp_delete option failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate where expr for iac_plugin_temp_delete failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if opt.Page.Count {
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.IacPluginTempDeleteTable, whereExpr)
		count, err := d.Orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count iac_plugin_temp_delete failed, err: %v, filter: %v, rid: %s",
				err, opt.Filter, kt.Rid)
			return nil, err
		}
		return &types.ListIacPluginTempDeleteDetails{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		logs.Errorf("generate page sql expr for iac_plugin_temp_delete failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`,
		tableplugin.IacPluginTempDeleteTableColumns.FieldsNamedExpr(opt.Fields),
		table.IacPluginTempDeleteTable, whereExpr, pageExpr)

	details := make([]tableplugin.IacPluginTempDeleteTable, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.ErrorJson("select iac_plugin_temp_delete failed, sql: %s, err: %v, filter: %v, rid: %s",
			sql, err, opt.Filter, kt.Rid)
		return nil, err
	}

	return &types.ListIacPluginTempDeleteDetails{Details: details}, nil
}

// DeleteWithTx 在事务中按过滤条件删除 iac_plugin_temp_delete 记录。
func (d *IacPluginTempDeleteDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate delete where expr for iac_plugin_temp_delete failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.IacPluginTempDeleteTable, whereExpr)
	if _, err = d.Orm.Txn(tx).Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete iac_plugin_temp_delete failed, err: %v, filter: %v, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}
