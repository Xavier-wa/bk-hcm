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

// Package resplan ...
package resplan

import (
	"fmt"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/audit"
	idgenerator "hcm/pkg/dal/dao/id-generator"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	rtypes "hcm/pkg/dal/dao/types/resource-plan"
	"hcm/pkg/dal/table"
	rpts "hcm/pkg/dal/table/resource-plan/res-plan-ticket-status"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"

	"github.com/jmoiron/sqlx"
)

// ResPlanTicketStatusInterface only used for resource plan ticket status interface.
type ResPlanTicketStatusInterface interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []rpts.ResPlanTicketStatusTable) error
	UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression, model *rpts.ResPlanTicketStatusTable) error
	List(kt *kit.Kit, opt *types.ListOption) (*rtypes.ResPlanTicketStatusListResult, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
}

var _ ResPlanTicketStatusInterface = new(ResPlanTicketStatusDao)

// ResPlanTicketStatusDao resource plan ticket status ResPlanTicketStatusDao.
type ResPlanTicketStatusDao struct {
	Orm   orm.Interface
	IDGen idgenerator.IDGenInterface
	Audit audit.Interface
}

// CreateWithTx create resource plan ticket status with tx.
func (d ResPlanTicketStatusDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []rpts.ResPlanTicketStatusTable) error {
	if len(models) == 0 {
		return errf.New(errf.InvalidParameter, "models to create cannot be empty")
	}

	for _, model := range models {
		if err := model.InsertValidate(); err != nil {
			return err
		}
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s)	VALUES(%s)`, models[0].TableName(),
		rpts.ResPlanTicketStatusColumns.ColumnExpr(), rpts.ResPlanTicketStatusColumns.ColonNameExpr())

	if err := d.Orm.Txn(tx).BulkInsert(kt.Ctx, sql, models); err != nil {
		logs.Errorf("insert %s failed, err: %v, rid: %s", models[0].TableName(), err, kt.Rid)
		return fmt.Errorf("insert %s failed, err: %v", models[0].TableName(), err)
	}

	return nil
}

// UpdateWithTx update resource plan ticket status with tx.
func (d ResPlanTicketStatusDao) UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *rpts.ResPlanTicketStatusTable) error {

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
		logs.ErrorJson("update resource plan ticket status with tx failed, filter: %v, err: %v, rid: %v",
			filterExpr, err, kt.Rid)
		return err
	}

	if effected == 0 {
		logs.ErrorJson("update resource plan ticket status, but record not found, filter: %v, rid: %v",
			filterExpr, kt.Rid)
	}

	return nil
}

// List get resource plan ticket status list.
func (d ResPlanTicketStatusDao) List(kt *kit.Kit, opt *types.ListOption) (
	*rtypes.ResPlanTicketStatusListResult, error) {

	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list res plan ticket status options is nil")
	}

	if err := opt.Validate(filter.NewExprOption(filter.RuleFields(rpts.ResPlanTicketStatusColumns.ColumnTypes())),
		core.NewDefaultPageOption()); err != nil {
		return nil, err
	}

	// use ticket_id as sql where option.
	sqlWhereOption := &filter.SQLWhereOption{
		Priority: filter.Priority{"ticket_id"},
	}
	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(sqlWhereOption)
	if err != nil {
		return nil, err
	}

	if opt.Page.Count {
		// this is a count request, then do count operation only.
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.ResPlanTicketStatusTable, whereExpr)

		count, err := d.Orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count res plan ticket status failed, err: %v, filter: %v, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &rtypes.ResPlanTicketStatusListResult{Count: count}, nil
	}

	// use ticket_id as page sql option.
	pageSQLOption := &types.PageSQLOption{Sort: types.SortOption{Sort: "ticket_id", IfNotPresent: true}}
	pageExpr, err := types.PageSQLExpr(opt.Page, pageSQLOption)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`, rpts.ResPlanTicketStatusColumns.FieldsNamedExpr(opt.Fields),
		table.ResPlanTicketStatusTable, whereExpr, pageExpr)

	details := make([]rpts.ResPlanTicketStatusTable, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		return nil, err
	}

	return &rtypes.ResPlanTicketStatusListResult{Count: 0, Details: details}, nil
}

// DeleteWithTx delete resource plan ticket status with tx.
func (d ResPlanTicketStatusDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.ResPlanTicketStatusTable, whereExpr)

	if _, err = d.Orm.Txn(tx).Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete resource plan ticket status failed, err: %v, filter: %v, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}
