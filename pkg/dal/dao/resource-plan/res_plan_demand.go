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
	"errors"
	"fmt"
	"slices"

	"hcm/pkg/api/core"
	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/audit"
	idgenerator "hcm/pkg/dal/dao/id-generator"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	"hcm/pkg/dal/table"
	rpd "hcm/pkg/dal/table/resource-plan/res-plan-demand"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"

	"github.com/jmoiron/sqlx"
)

// ResPlanDemandInterface only used for resource plan demand interface.
type ResPlanDemandInterface interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []rpd.ResPlanDemandTable) ([]string, error)
	UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression, model *rpd.ResPlanDemandTable) error
	List(kt *kit.Kit, opt *types.ListOption) (*rpproto.ResPlanDemandListResult, error)
	UpdateDemandCreator(kt *kit.Kit, demandIDs []string, creator, reviser string) (int64, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
	ExamineAndLockAllRPDemand(kt *kit.Kit, lockedItems []rpproto.ResPlanDemandLockOpItem) error
	UnlockAllResPlanDemand(kt *kit.Kit, demandIDs []string) error
}

var _ ResPlanDemandInterface = new(ResPlanDemandDao)

// ResPlanDemandDao resource plan demand ResPlanDemandDao.
type ResPlanDemandDao struct {
	Orm   orm.Interface
	IDGen idgenerator.IDGenInterface
	Audit audit.Interface
}

// CreateWithTx create resource plan demand with tx.
func (d ResPlanDemandDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []rpd.ResPlanDemandTable) (
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
		rpd.ResPlanDemandColumns.ColumnExpr(), rpd.ResPlanDemandColumns.ColonNameExpr())

	if err = d.Orm.Txn(tx).BulkInsert(kt.Ctx, sql, models); err != nil {
		logs.Errorf("insert %s failed, err: %v, rid: %s", models[0].TableName(), err, kt.Rid)
		return nil, fmt.Errorf("insert %s failed, err: %v", models[0].TableName(), err)
	}

	return ids, nil
}

// UpdateWithTx update resource plan demand.
func (d ResPlanDemandDao) UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *rpd.ResPlanDemandTable) error {
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
		logs.Errorf("update resource plan demand failed, filter: %v, err: %v, rid: %s", filterExpr, err, kt.Rid)
		return err
	}

	if effected == 0 {
		logs.Errorf("update resource plan demand, but record not found, filter: %v, rid: %s", filterExpr, kt.Rid)
	}

	return nil
}

// UpdateDemandCreator batch update creator for multiple demands.
func (d ResPlanDemandDao) UpdateDemandCreator(kt *kit.Kit, demandIDs []string, creator, reviser string) (int64, error) {
	if len(demandIDs) == 0 {
		return 0, errf.New(errf.InvalidParameter, "demand ids cannot be empty")
	}

	if creator == "" {
		return 0, errf.New(errf.InvalidParameter, "creator cannot be empty")
	}

	idExpr := tools.ContainersExpression("id", demandIDs)
	backendExpr := tools.EqualExpression("creator", constant.BackendOperationUserKey)

	// Combine conditions with AND
	filterExpr := &filter.Expression{
		Op: filter.And,
		Rules: []filter.RuleFactory{
			idExpr,
			backendExpr,
		},
	}

	whereExpr, whereValue, err := filterExpr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("build filter expression failed, err: %v, rid: %s", err, kt.Rid)
		return 0, err
	}

	params := map[string]interface{}{
		"creator": creator,
		"reviser": reviser,
	}

	// Merge where conditions with params
	allParams := tools.MapMerge(whereValue, params)

	sql := fmt.Sprintf(`UPDATE %s SET creator = :creator, reviser = :reviser, updated_at = NOW() %s`,
		table.ResPlanDemandTable, whereExpr)

	affected, err := d.Orm.Do().Update(kt.Ctx, sql, allParams)
	if err != nil {
		logs.Errorf("batch update budget operator demand creator failed, err: %v, demand_ids: %v, rid: %s",
			err, demandIDs, kt.Rid)
		return 0, err
	}

	return affected, nil
}

// List get resource plan demand list.
func (d ResPlanDemandDao) List(kt *kit.Kit, opt *types.ListOption) (*rpproto.ResPlanDemandListResult, error) {
	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list res plan demand options is nil")
	}

	if err := opt.Validate(filter.NewExprOption(filter.RuleFields(rpd.ResPlanDemandColumns.ColumnTypes())),
		core.NewDefaultPageOption()); err != nil {
		return nil, err
	}

	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	if opt.Page.Count {
		// this is a count request, then do count operation only.
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.ResPlanDemandTable, whereExpr)

		count, err := d.Orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.Errorf("count res plan demand failed, err: %v, filter: %v, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &rpproto.ResPlanDemandListResult{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`, rpd.ResPlanDemandColumns.FieldsNamedExpr(opt.Fields),
		table.ResPlanDemandTable, whereExpr, pageExpr)

	details := make([]rpd.ResPlanDemandTable, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		return nil, err
	}

	return &rpproto.ResPlanDemandListResult{Count: 0, Details: details}, nil
}

// DeleteWithTx delete resource plan demand with tx.
func (d ResPlanDemandDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.ResPlanDemandTable, whereExpr)

	if _, err = d.Orm.Txn(tx).Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.Errorf("delete resource plan demand failed, err: %v, filter: %v, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}

// ExamineAndLockAllRPDemand examine and lock all resource plan demand.
func (d ResPlanDemandDao) ExamineAndLockAllRPDemand(kt *kit.Kit, lockedItems []rpproto.ResPlanDemandLockOpItem) error {
	if len(lockedItems) == 0 {
		return errf.New(errf.InvalidParameter, "locked items can not be empty")
	}

	ids := slice.Map(lockedItems, func(item rpproto.ResPlanDemandLockOpItem) string {
		return item.ID
	})

	opt := tools.ContainersExpression("id", ids)
	whereExpr, whereValue, err := opt.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	_, err = d.Orm.AutoTxn(kt, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		details := make([]rpd.ResPlanDemandTable, 0)
		selectSql := fmt.Sprintf(`SELECT id, locked FROM %s %s`, table.ResPlanDemandTable, whereExpr)
		if err = d.Orm.Txn(txn).Select(kt.Ctx, &details, selectSql, whereValue); err != nil {
			return nil, err
		}

		haveLocked := slices.ContainsFunc(details, func(ele rpd.ResPlanDemandTable) bool {
			return ele.Locked != nil && cvt.PtrToVal(ele.Locked) == enumor.CrpDemandLocked
		})
		if haveLocked {
			return nil, errors.New("some resource plan demand has been locked")
		}

		for _, item := range lockedItems {
			updateSql := fmt.Sprintf(`UPDATE %s SET locked=%d, locked_cpu_core=%d, ticket_id='%s' where id = :id`,
				table.ResPlanDemandTable, enumor.CrpDemandLocked, item.LockedCPUCore, item.TicketID)

			whereValue["id"] = item.ID
			_, err = d.Orm.Txn(txn).Update(kt.Ctx, updateSql, whereValue)
			if err != nil {
				logs.Errorf("update resource plan demand failed, err: %v, demand_id: %s, rid: %s",
					err, item.ID, kt.Rid)
				return nil, err
			}
		}
		return nil, nil
	})

	if err != nil {
		logs.Errorf("examine and lock all resource plan demand failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// UnlockAllResPlanDemand unlock all resource plan demand.
func (d ResPlanDemandDao) UnlockAllResPlanDemand(kt *kit.Kit, demandIDs []string) error {
	if len(demandIDs) == 0 {
		return errf.New(errf.InvalidParameter, "demand ids can not be empty")
	}

	opt := tools.ContainersExpression("id", demandIDs)
	whereExpr, whereValue, err := opt.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	updateSql := fmt.Sprintf(`UPDATE %s SET locked=%d, locked_cpu_core=%d, ticket_id="" %s`, table.ResPlanDemandTable,
		enumor.CrpDemandUnLocked, 0, whereExpr)

	if _, err = d.Orm.Do().Update(kt.Ctx, updateSql, whereValue); err != nil {
		logs.Errorf("unlock all resource plan demand failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}
