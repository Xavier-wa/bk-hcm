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
	rpst "hcm/pkg/dal/table/return-plan/return-plan-sub-ticket"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"

	"github.com/jmoiron/sqlx"
)

// ReturnPlanSubTicketInterface only used for return plan sub_ticket interface.
type ReturnPlanSubTicketInterface interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []rpst.ReturnPlanSubTicketTable) ([]string, error)
	UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression, model *rpst.ReturnPlanSubTicketTable) (int64, error)
	List(kt *kit.Kit, opt *types.ListOption) (*rtypes.ReturnPlanSubTicketListResult, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
}

var _ ReturnPlanSubTicketInterface = new(ReturnPlanSubTicketDao)

// ReturnPlanSubTicketDao return plan sub_ticket dao.
type ReturnPlanSubTicketDao struct {
	Orm   orm.Interface
	IDGen idgenerator.IDGenInterface
	Audit audit.Interface
}

// CreateWithTx create return plan sub_ticket with tx.
func (d ReturnPlanSubTicketDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []rpst.ReturnPlanSubTicketTable) (
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
		rpst.ReturnPlanSubTicketColumns.ColumnExpr(), rpst.ReturnPlanSubTicketColumns.ColonNameExpr())

	if err = d.Orm.Txn(tx).BulkInsert(kt.Ctx, sql, models); err != nil {
		logs.Errorf("insert %s failed, err: %v, rid: %s", models[0].TableName(), err, kt.Rid)
		return nil, fmt.Errorf("insert %s failed, err: %v", models[0].TableName(), err)
	}

	return ids, nil
}

// UpdateWithTx update return plan sub_ticket with tx.
func (d ReturnPlanSubTicketDao) UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *rpst.ReturnPlanSubTicketTable) (int64, error) {

	if filterExpr == nil {
		return 0, errf.New(errf.InvalidParameter, "filter expr is nil")
	}

	if err := model.UpdateValidate(); err != nil {
		return 0, err
	}

	whereExpr, whereValue, err := filterExpr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return 0, err
	}

	opts := utils.NewFieldOptions().AddIgnoredFields(types.DefaultIgnoredFields...)
	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(model, opts)
	if err != nil {
		return 0, fmt.Errorf("prepare parsed sql set filter expr failed, err: %v", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, model.TableName(), setExpr, whereExpr)

	effected, err := d.Orm.Txn(tx).Update(kt.Ctx, sql, tools.MapMerge(toUpdate, whereValue))
	if err != nil {
		logs.ErrorJson("update return plan sub ticket failed, filter: %v, err: %v, rid: %v",
			filterExpr, err, kt.Rid)
		return 0, err
	}

	if effected == 0 {
		logs.ErrorJson("update return plan sub ticket, but record not found, filter: %v, rid: %v",
			filterExpr, kt.Rid)
		return 0, err
	}

	return effected, nil
}

// List get return plan sub_ticket list.
func (d ReturnPlanSubTicketDao) List(kt *kit.Kit, opt *types.ListOption) (*rtypes.ReturnPlanSubTicketListResult,
	error) {

	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list return plan sub ticket options is nil")
	}

	exprOpt := filter.NewExprOption(
		filter.RuleFields(rpst.ReturnPlanSubTicketColumns.ColumnTypes()),
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
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.ReturnPlanSubTicketTable, whereExpr)

		count, err := d.Orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count return plan sub ticket failed, err: %v, filter: %v, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &rtypes.ReturnPlanSubTicketListResult{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`, rpst.ReturnPlanSubTicketColumns.FieldsNamedExpr(opt.Fields),
		table.ReturnPlanSubTicketTable, whereExpr, pageExpr)

	details := make([]rpst.ReturnPlanSubTicketTable, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		return nil, err
	}

	return &rtypes.ReturnPlanSubTicketListResult{Count: 0, Details: details}, nil
}

// DeleteWithTx delete return plan sub_ticket with tx.
func (d ReturnPlanSubTicketDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.ReturnPlanSubTicketTable, whereExpr)

	if _, err = d.Orm.Txn(tx).Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete return plan sub ticket failed, err: %v, filter: %v, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}
