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

// Package returnplan ...
package returnplan

import (
	"fmt"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/audit"
	idgenerator "hcm/pkg/dal/dao/id-generator"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	rtypes "hcm/pkg/dal/dao/types/return-plan"
	"hcm/pkg/dal/table"
	rpt "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"

	"github.com/jmoiron/sqlx"
)

// ReturnPlanTicketInterface only used for return plan ticket interface.
type ReturnPlanTicketInterface interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []rpt.ReturnPlanTicketTable) ([]string, error)
	UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression, model *rpt.ReturnPlanTicketTable) error
	List(kt *kit.Kit, opt *types.ListOption) (*rtypes.ReturnPlanTicketListResult, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
}

var _ ReturnPlanTicketInterface = new(ReturnPlanTicketDao)

// ReturnPlanTicketDao return plan ticket dao.
type ReturnPlanTicketDao struct {
	Orm   orm.Interface
	IDGen idgenerator.IDGenInterface
	Audit audit.Interface
}

// CreateWithTx create return plan ticket with tx.
func (d ReturnPlanTicketDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []rpt.ReturnPlanTicketTable) (
	[]string, error) {

	if len(models) == 0 {
		return nil, errf.New(errf.InvalidParameter, "models to create cannot be empty")
	}

	ids, err := d.IDGen.Batch(kt, models[0].TableName(), len(models))
	if err != nil {
		return nil, err
	}

	for index := range models {
		models[index].ID = ids[index]

		if err = models[index].InsertValidate(); err != nil {
			return nil, err
		}
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s)	VALUES(%s)`, models[0].TableName(),
		rpt.ReturnPlanTicketColumns.ColumnExpr(), rpt.ReturnPlanTicketColumns.ColonNameExpr())

	if err = d.Orm.Txn(tx).BulkInsert(kt.Ctx, sql, models); err != nil {
		logs.Errorf("insert %s failed, err: %v, rid: %s", models[0].TableName(), err, kt.Rid)
		return nil, fmt.Errorf("insert %s failed, err: %v", models[0].TableName(), err)
	}

	return ids, nil
}

// UpdateWithTx update return plan ticket with tx.
func (d ReturnPlanTicketDao) UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *rpt.ReturnPlanTicketTable) error {

	if filterExpr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is nil")
	}

	if err := model.UpdateValidate(); err != nil {
		return err
	}

	whereExpr, whereValue, err := filterExpr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	opts := utils.NewFieldOptions().AddIgnoredFields(types.DefaultIgnoredFields...)
	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(model, opts)
	if err != nil {
		return fmt.Errorf("prepare parsed sql set filter expr failed, err: %v", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, model.TableName(), setExpr, whereExpr)

	effected, err := d.Orm.Txn(tx).Update(kt.Ctx, sql, tools.MapMerge(toUpdate, whereValue))
	if err != nil {
		logs.ErrorJson("update return plan ticket in txn failed, filter: %v, err: %v, rid: %v",
			filterExpr, err, kt.Rid)
		return err
	}

	if effected == 0 {
		logs.ErrorJson("update return plan ticket, but record not found, filter: %v, rid: %v",
			filterExpr, kt.Rid)
	}

	return nil
}

// List get return plan ticket list.
func (d ReturnPlanTicketDao) List(kt *kit.Kit, opt *types.ListOption) (*rtypes.ReturnPlanTicketListResult, error) {
	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list return plan ticket options is nil")
	}

	exprOpt := filter.NewExprOption(
		filter.RuleFields(rpt.ReturnPlanTicketColumns.ColumnTypes()),
		filter.MaxInLimit(constant.BkBizIDMaxLimit),
	)

	if err := opt.Validate(exprOpt, core.NewDefaultPageOption()); err != nil {
		return nil, err
	}

	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	if opt.Page.Count {
		// this is a count request, then do count operation only.
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.ReturnPlanTicketTable, whereExpr)

		count, err := d.Orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count return plan ticket failed, err: %v, filter: %v, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &rtypes.ReturnPlanTicketListResult{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`, rpt.ReturnPlanTicketColumns.FieldsNamedExpr(opt.Fields),
		table.ReturnPlanTicketTable, whereExpr, pageExpr)

	details := make([]rpt.ReturnPlanTicketTable, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		return nil, err
	}

	return &rtypes.ReturnPlanTicketListResult{Count: 0, Details: details}, nil
}

// DeleteWithTx delete return plan ticket with tx.
func (d ReturnPlanTicketDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.ReturnPlanTicketTable, whereExpr)

	if _, err = d.Orm.Txn(tx).Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete return plan ticket failed, err: %v, filter: %v, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}
