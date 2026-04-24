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

// Package cvmapply ziyan cvm generate record dao
package cvmapply

import (
	"fmt"

	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/audit"
	idgenerator "hcm/pkg/dal/dao/id-generator"
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

// ZiyanCvmGenerateRecordInterface only used for ziyan cvm generate record interface.
type ZiyanCvmGenerateRecordInterface interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []cvmapplytable.ZiyanCvmGenerateRecord) ([]string, error)
	Update(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression, model *cvmapplytable.ZiyanCvmGenerateRecord) error
	List(kt *kit.Kit, opt *types.ListOption) (*cvmapplyproto.ZiyanCvmGenerateRecordListResult, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
}

var _ ZiyanCvmGenerateRecordInterface = new(ZiyanCvmGenerateRecordDao)

// ZiyanCvmGenerateRecordDao dao.
type ZiyanCvmGenerateRecordDao struct {
	Orm   orm.Interface
	IDGen idgenerator.IDGenInterface
	Audit audit.Interface
}

// CreateWithTx create ziyan cvm generate record with tx.
func (d ZiyanCvmGenerateRecordDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx,
	models []cvmapplytable.ZiyanCvmGenerateRecord) ([]string, error) {

	if len(models) == 0 {
		return nil, errf.New(errf.InvalidParameter, "models to create cannot be empty")
	}

	ids, err := d.IDGen.Batch(kt, models[0].TableName(), len(models))
	if err != nil {
		return nil, err
	}

	// 验证数据并设置ID
	for index := range models {
		if len(models[index].GenerateID) == 0 {
			models[index].GenerateID = ids[index]
		}

		if err = models[index].InsertValidate(); err != nil {
			return nil, err
		}
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s)	VALUES(%s)`, models[0].TableName(),
		cvmapplytable.ZiyanCvmGenerateRecordColumns.ColumnExpr(),
		cvmapplytable.ZiyanCvmGenerateRecordColumns.ColonNameExpr())

	err = d.Orm.Txn(tx).BulkInsert(kt.Ctx, sql, models)
	if err != nil {
		logs.Errorf("insert table %s failed, err: %v, rid: %s", models[0].TableName(), err, kt.Rid)
		return nil, fmt.Errorf("insert table %s failed, err: %v", models[0].TableName(), err)
	}

	return ids, nil
}

// Update update ziyan cvm generate record.
func (d ZiyanCvmGenerateRecordDao) Update(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *cvmapplytable.ZiyanCvmGenerateRecord) error {

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
		logs.ErrorJson("update ziyan cvm generate record failed, sql: %s, err: %v, updateValue: %+v, rid: %s",
			sql, err, updateValue, kt.Rid)
		return err
	}

	if effected == 0 {
		logs.ErrorJson("update ziyan cvm generate record, but record not found, sql: %s, updateValue: %+v, rid: %s",
			sql, updateValue, kt.Rid)
	}
	return nil
}

// List get ziyan cvm generate record list.
func (d ZiyanCvmGenerateRecordDao) List(kt *kit.Kit, opt *types.ListOption) (
	*cvmapplyproto.ZiyanCvmGenerateRecordListResult, error) {

	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list ziyan cvm generate record options is nil")
	}

	if err := opt.Validate(filter.NewExprOption(filter.RuleFields(
		cvmapplytable.ZiyanCvmGenerateRecordColumns.ColumnTypes())),
		core.NewDefaultPageOption()); err != nil {
		return nil, err
	}

	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	if opt.Page.Count {
		// this is a count request, then do count operation only.
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.ZiyanCvmGenerateRecordTable, whereExpr)

		count, err := d.Orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count ziyan cvm generate record failed, err: %v, filter: %v, rid: %s",
				err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &cvmapplyproto.ZiyanCvmGenerateRecordListResult{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page,
		&types.PageSQLOption{Sort: types.SortOption{Sort: "generate_id", IfNotPresent: true}})
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`,
		cvmapplytable.ZiyanCvmGenerateRecordColumns.FieldsNamedExpr(opt.Fields),
		table.ZiyanCvmGenerateRecordTable, whereExpr, pageExpr)

	details := make([]*cvmapplytable.ZiyanCvmGenerateRecord, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmGenerateRecordListResult{Count: 0, Details: details}, nil
}

// DeleteWithTx delete ziyan cvm generate record with tx.
func (d ZiyanCvmGenerateRecordDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.ZiyanCvmGenerateRecordTable, whereExpr)

	if _, err = d.Orm.Txn(tx).Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete ziyan cvm generate record failed, err: %v, whereValue: %+v, rid: %s",
			err, whereValue, kt.Rid)
		return err
	}

	return nil
}
