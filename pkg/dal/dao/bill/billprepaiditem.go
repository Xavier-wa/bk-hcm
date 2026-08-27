/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package bill

import (
	"fmt"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	idgenerator "hcm/pkg/dal/dao/id-generator"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	typesbill "hcm/pkg/dal/dao/types/bill"
	"hcm/pkg/dal/table"
	tablebill "hcm/pkg/dal/table/bill"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/slice"

	"github.com/jmoiron/sqlx"
)

// AccountBillPrepaidItem holds all the operations of the prepaid bill main table.
type AccountBillPrepaidItem interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, model *tablebill.AccountBillPrepaidItem) (string, error)
	BatchCreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []tablebill.AccountBillPrepaidItem) ([]string, error)
	UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression,
		updateData *tablebill.AccountBillPrepaidItem) error
	List(kt *kit.Kit, opt *types.ListOption) (*typesbill.ListAccountBillPrepaidItemDetails, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
}

// AccountBillPrepaidItemDao account bill prepaid item dao
type AccountBillPrepaidItemDao struct {
	Orm   orm.Interface
	IDGen idgenerator.IDGenInterface
}

// CreateWithTx create one account bill prepaid item with tx, returns the generated id.
func (a AccountBillPrepaidItemDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx,
	model *tablebill.AccountBillPrepaidItem) (string, error) {

	if model == nil {
		return "", errf.New(errf.InvalidParameter, "model to create cannot be nil")
	}

	ids, err := a.BatchCreateWithTx(kt, tx, []tablebill.AccountBillPrepaidItem{*model})
	if err != nil {
		return "", err
	}

	return ids[0], nil
}

// BatchCreateWithTx batch create account bill prepaid item with tx, sharded by constant.BatchOperationMaxLimit.
func (a AccountBillPrepaidItemDao) BatchCreateWithTx(kt *kit.Kit, tx *sqlx.Tx,
	models []tablebill.AccountBillPrepaidItem) ([]string, error) {

	if len(models) == 0 {
		return nil, errf.New(errf.InvalidParameter, "models to create cannot be empty")
	}

	ids, err := a.IDGen.Batch(kt, table.AccountBillPrepaidItemTable, len(models))
	if err != nil {
		logs.Errorf("generate account bill prepaid item id failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	for index := range models {
		models[index].ID = ids[index]
		if err = models[index].InsertValidate(); err != nil {
			return nil, errf.NewFromErr(errf.InvalidParameter, err)
		}
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s)	VALUES(%s)`, table.AccountBillPrepaidItemTable,
		tablebill.AccountBillPrepaidItemColumns.ColumnExpr(),
		tablebill.AccountBillPrepaidItemColumns.ColonNameExpr())

	for _, batch := range slice.Split(models, constant.BatchOperationMaxLimit) {
		if err = a.Orm.Txn(tx).BulkInsert(kt.Ctx, sql, batch); err != nil {
			logs.Errorf("insert %s failed, err: %v, batch_size: %d, rid: %s", table.AccountBillPrepaidItemTable,
				err, len(batch), kt.Rid)
			// 唯一键冲突必须以 RecordDuplicated 返回：并发首推双双判空后双双 INSERT 是设计预期路径，
			// 调用方依赖该错误码重试。用 %v 包装会截断错误链，errf.IsDuplicated 将无法识别。
			if merr := errf.GetMySQLDuplicated(err); merr != nil {
				return nil, errf.Newf(errf.RecordDuplicated, "insert %s duplicated, err: %v",
					table.AccountBillPrepaidItemTable, merr)
			}

			return nil, fmt.Errorf("insert %s failed, err: %w", table.AccountBillPrepaidItemTable, err)
		}
	}

	return ids, nil
}

// UpdateWithTx update account bill prepaid item by the given filter expression.
func (a AccountBillPrepaidItemDao) UpdateWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression,
	updateData *tablebill.AccountBillPrepaidItem) error {

	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}
	if err := updateData.UpdateValidate(); err != nil {
		return errf.NewFromErr(errf.InvalidParameter, err)
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	opts := utils.NewFieldOptions().AddIgnoredFields(types.DefaultIgnoredFields...).AddIgnoredFields("id")
	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(updateData, opts)
	if err != nil {
		return fmt.Errorf("prepare parsed sql set filter expr failed, err: %v", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, table.AccountBillPrepaidItemTable, setExpr, whereExpr)

	for key, value := range whereValue {
		toUpdate[key] = value
	}
	if _, err = a.Orm.Txn(tx).Update(kt.Ctx, sql, toUpdate); err != nil {
		logs.Errorf("update account bill prepaid item failed, err: %v, filter: %+v, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}

// List get account bill prepaid item list.
func (a AccountBillPrepaidItemDao) List(kt *kit.Kit, opt *types.ListOption) (
	*typesbill.ListAccountBillPrepaidItemDetails, error) {

	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list account bill prepaid item options is nil")
	}

	if err := opt.Validate(filter.NewExprOption(
		filter.RuleFields(tablebill.AccountBillPrepaidItemColumns.ColumnTypes())),
		core.NewDefaultPageOption()); err != nil {
		return nil, err
	}

	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	if opt.Page.Count {
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.AccountBillPrepaidItemTable, whereExpr)
		count, err := a.Orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count account bill prepaid item failed, err: %v, filter: %s, rid: %s",
				err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &typesbill.ListAccountBillPrepaidItemDetails{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`,
		tablebill.AccountBillPrepaidItemColumns.FieldsNamedExpr(opt.Fields),
		table.AccountBillPrepaidItemTable, whereExpr, pageExpr)

	details := make([]tablebill.AccountBillPrepaidItem, 0)
	if err = a.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.Errorf("fail to select account bill prepaid item, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return &typesbill.ListAccountBillPrepaidItemDetails{Details: details}, nil
}

// DeleteWithTx delete account bill prepaid item with tx.
func (a AccountBillPrepaidItemDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.AccountBillPrepaidItemTable, whereExpr)

	if _, err = a.Orm.Txn(tx).Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete account bill prepaid item failed, err: %v, filter: %s, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}
