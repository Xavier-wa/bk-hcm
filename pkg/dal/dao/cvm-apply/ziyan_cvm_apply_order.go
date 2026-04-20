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

// Package cvmapply ziyan cvm apply order dao
package cvmapply

import (
	"fmt"

	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/audit"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	"hcm/pkg/dal/table"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"

	"github.com/jmoiron/sqlx"
)

// ZiyanCvmApplyOrderInterface only used for ziyan cvm apply order interface.
type ZiyanCvmApplyOrderInterface interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []cvmapplytable.ZiyanCvmApplyOrder) ([]uint64, error)
	Update(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression, model *cvmapplytable.ZiyanCvmApplyOrder) error
	List(kt *kit.Kit, opt *types.ListOption) (*cvmapplyproto.ZiyanCvmApplyOrderListResult, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
}

var _ ZiyanCvmApplyOrderInterface = new(ZiyanCvmApplyOrderDao)

// ZiyanCvmApplyOrderDao dao.
type ZiyanCvmApplyOrderDao struct {
	Orm   orm.Interface
	Audit audit.Interface
}

// CreateWithTx create ziyan cvm apply order with tx.
// Note: 表使用AUTO_INCREMENT自增ID，插入后返回数据库自动生成的ID数组
func (d ZiyanCvmApplyOrderDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []cvmapplytable.ZiyanCvmApplyOrder) (
	[]uint64, error) {

	if len(models) == 0 {
		return nil, errf.New(errf.InvalidParameter, "models to create cannot be empty")
	}

	// 验证数据
	for index := range models {
		if err := models[index].InsertValidate(); err != nil {
			return nil, err
		}
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s)	VALUES(%s)`, models[0].TableName(),
		cvmapplytable.ZiyanCvmApplyOrderColumns.ColumnExpr(), cvmapplytable.ZiyanCvmApplyOrderColumns.ColonNameExpr())

	ids, err := d.Orm.Txn(tx).BulkInsertWithIDs(kt.Ctx, sql, models, len(models))
	if err != nil {
		logs.Errorf("insert table %s failed, err: %v, rid: %s", models[0].TableName(), err, kt.Rid)
		return nil, fmt.Errorf("insert table %s failed, err: %v", models[0].TableName(), err)
	}

	return ids, nil
}

// Update update ziyan cvm apply order.
func (d ZiyanCvmApplyOrderDao) Update(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *cvmapplytable.ZiyanCvmApplyOrder) error {

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

	ignoredFields := append(types.DefaultIgnoredFields, "updated_at")
	opts := utils.NewFieldOptions().AddIgnoredFields(ignoredFields...)
	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(model, opts)
	if err != nil {
		return fmt.Errorf("prepare parsed sql set filter expr failed, err: %v", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, model.TableName(), setExpr, whereExpr)
	updateValue := tools.MapMerge(toUpdate, whereValue)
	effected, err := d.Orm.Txn(tx).Update(kt.Ctx, sql, updateValue)
	if err != nil {
		logs.ErrorJson("update ziyan cvm apply order failed, sql: %s, err: %v, updateValue: %+v, rid: %s",
			sql, err, updateValue, kt.Rid)
		return err
	}

	if effected == 0 {
		logs.ErrorJson("update ziyan cvm apply order, but record not found, sql: %s, updateValue: %+v, rid: %s",
			sql, updateValue, kt.Rid)
	}
	return nil
}

// List get ziyan cvm apply order list.
func (d ZiyanCvmApplyOrderDao) List(kt *kit.Kit, opt *types.ListOption) (
	*cvmapplyproto.ZiyanCvmApplyOrderListResult, error) {

	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list ziyan cvm apply order options is nil")
	}

	expr := filter.NewExprOption(
		filter.RuleFields(cvmapplytable.ZiyanCvmApplyOrderColumns.ColumnTypes()),
		filter.MaxInLimit(constant.CvmApplyDeviceQueryInLimit),
	)
	if err := opt.Validate(expr, core.NewDefaultPageOption()); err != nil {
		return nil, err
	}

	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	if opt.Page.Count {
		// this is a count request, then do count operation only.
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.ZiyanCvmApplyOrderTable, whereExpr)

		count, err := d.Orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count ziyan cvm apply order failed, err: %v, filter: %v, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &cvmapplyproto.ZiyanCvmApplyOrderListResult{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page,
		&types.PageSQLOption{Sort: types.SortOption{Sort: "order_id", IfNotPresent: true}})
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`, cvmapplytable.ZiyanCvmApplyOrderColumns.FieldsNamedExpr(opt.Fields),
		table.ZiyanCvmApplyOrderTable, whereExpr, pageExpr)

	details := make([]*cvmapplytable.ZiyanCvmApplyOrder, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyOrderListResult{Count: 0, Details: details}, nil
}

// DeleteWithTx delete ziyan cvm apply order with tx.
func (d ZiyanCvmApplyOrderDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.ZiyanCvmApplyOrderTable, whereExpr)

	if _, err = d.Orm.Txn(tx).Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete ziyan cvm apply order failed, err: %v, whereValue: %+v, rid: %s",
			err, whereValue, kt.Rid)
		return err
	}

	return nil
}
