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
	"hcm/pkg/criteria/enumor"
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

// IacPluginInstance iac_plugin_instance 表 DAO 接口。
type IacPluginInstance interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, model *tableplugin.IacPluginInstanceTable) (string, error)
	UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
		model *tableplugin.IacPluginInstanceTable) error
	List(kt *kit.Kit, opt *types.ListOption) (*types.ListIacPluginInstanceDetails, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
}

var _ IacPluginInstance = new(IacPluginInstanceDao)

// IacPluginInstanceDao 实现 IacPluginInstance 接口。
type IacPluginInstanceDao struct {
	Orm   orm.Interface
	IDGen idgenerator.IDGenInterface
}

// CreateWithTx 在事务中创建单条 iac_plugin_instance 记录。
func (d *IacPluginInstanceDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx,
	model *tableplugin.IacPluginInstanceTable) (string, error) {

	if model == nil {
		return "", errf.New(errf.InvalidParameter, "iac plugin instance model is nil")
	}

	if err := model.InsertValidate(); err != nil {
		logs.Errorf("insert validate iac_plugin_instance failed, err: %v, rid: %s", err, kt.Rid)
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
		tableplugin.IacPluginInstanceTableColumns.ColumnExpr(),
		tableplugin.IacPluginInstanceTableColumns.ColonNameExpr())

	err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Txn(tx).Insert(kt.Ctx, sql, model)
	if err != nil {
		logs.Errorf("insert %s failed, sql: %s, err: %v, rid: %s", model.TableName(), sql, err, kt.Rid)
		return "", fmt.Errorf("insert %s failed, err: %w", model.TableName(), err)
	}

	return id, nil
}

// UpdateWithTx 在事务中按过滤条件更新 iac_plugin_instance 记录。
func (d *IacPluginInstanceDao) UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *tableplugin.IacPluginInstanceTable) error {

	if filterExpr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is nil")
	}
	if model == nil {
		return errf.New(errf.InvalidParameter, "iac plugin instance update model is nil")
	}

	if err := model.UpdateValidate(); err != nil {
		logs.Errorf("update validate iac_plugin_instance failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	whereExpr, whereValue, err := filterExpr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate update where expr for iac_plugin_instance failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	opts := utils.NewFieldOptions().
		AddIgnoredFields(types.DefaultIgnoredFields...).
		AddIgnoredFields("plugin_name", "tenant_id").
		AddBlankedFields("memo")

	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(model, opts)
	if err != nil {
		logs.Errorf("rearrange sql data for iac_plugin_instance failed, err: %v, rid: %s", err, kt.Rid)
		return fmt.Errorf("prepare parsed sql set filter expr failed, err: %w", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, model.TableName(), setExpr, whereExpr)
	_, err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Txn(tx).Update(
		kt.Ctx, sql, tools.MapMerge(toUpdate, whereValue))
	if err != nil {
		logs.ErrorJson("update iac_plugin_instance failed, filter: %v, err: %v, rid: %s", filterExpr, err, kt.Rid)
		return err
	}

	return nil
}

// List 查询 iac_plugin_instance 记录。
func (d *IacPluginInstanceDao) List(kt *kit.Kit, opt *types.ListOption) (*types.ListIacPluginInstanceDetails, error) {
	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list iac plugin instance option is nil")
	}

	columnTypes := tableplugin.IacPluginInstanceTableColumns.ColumnTypes()
	columnTypes["extension.*"] = enumor.Json
	columnTypes["attrs.*"] = enumor.Json
	exprOpt := filter.NewExprOption(filter.RuleFields(columnTypes))
	if err := opt.Validate(exprOpt, core.NewDefaultPageOption()); err != nil {
		logs.Errorf("validate list iac_plugin_instance option failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate where expr for iac_plugin_instance failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if opt.Page.Count {
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.IacPluginInstanceTable, whereExpr)
		count, err := d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count iac_plugin_instance failed, err: %v, filter: %v, rid: %s",
				err, opt.Filter, kt.Rid)
			return nil, err
		}
		return &types.ListIacPluginInstanceDetails{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		logs.Errorf("generate page sql expr for iac_plugin_instance failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`,
		tableplugin.IacPluginInstanceTableColumns.FieldsNamedExpr(opt.Fields),
		table.IacPluginInstanceTable, whereExpr, pageExpr)

	details := make([]tableplugin.IacPluginInstanceTable, 0)
	err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Do().Select(kt.Ctx, &details, sql, whereValue)
	if err != nil {
		logs.ErrorJson("select iac_plugin_instance failed, sql: %s, err: %v, filter: %v, rid: %s",
			sql, err, opt.Filter, kt.Rid)
		return nil, err
	}

	return &types.ListIacPluginInstanceDetails{Details: details}, nil
}

// DeleteWithTx 在事务中按过滤条件删除 iac_plugin_instance 记录。
func (d *IacPluginInstanceDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("generate delete where expr for iac_plugin_instance failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.IacPluginInstanceTable, whereExpr)
	if _, err = d.Orm.ModifySQLOpts(orm.NewInjectTenantIDOpt(kt.TenantID)).Txn(tx).Delete(
		kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete iac_plugin_instance failed, err: %v, filter: %v, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}
