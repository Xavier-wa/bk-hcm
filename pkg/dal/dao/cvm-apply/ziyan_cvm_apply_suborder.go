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

// Package cvmapply ziyan cvm apply suborder dao
package cvmapply

import (
	"fmt"

	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
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

// ZiyanCvmApplySuborderInterface only used for ziyan cvm apply suborder interface.
type ZiyanCvmApplySuborderInterface interface {
	CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []cvmapplytable.ZiyanCvmApplySuborder) ([]string, error)
	Update(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression, model *cvmapplytable.ZiyanCvmApplySuborder) error
	List(kt *kit.Kit, opt *types.ListOption) (*cvmapplyproto.ZiyanCvmApplySuborderListResult, error)
	DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error
	GetOrderTimeCostOverview(kt *kit.Kit, expr *filter.Expression) ([]*cvmapplyproto.OrderTimeCostItem, error)
	GetOrderTimeCostCompare(kt *kit.Kit, expr *filter.Expression) ([]*cvmapplyproto.OrderTimeCostCompareItem, error)
	GetPercentileTimeConsumptionOverview(kt *kit.Kit, expr *filter.Expression) (
		*cvmapplyproto.ZiyanCvmApplyPercentileTimeOverviewResult, error)
	GetPercentileTimeConsumptionCompare(kt *kit.Kit, expr *filter.Expression) (
		*cvmapplyproto.ZiyanCvmApplyPercentileTimeCompareResult, error)
	GetProductionStageTimeCostOverview(kt *kit.Kit, expr *filter.Expression) (
		[]*cvmapplyproto.ProductionStageTimeCostItem, error)
	GetProductionStageTimeCostCompare(kt *kit.Kit, expr *filter.Expression) (
		[]*cvmapplyproto.ProductionStageTimeCostBizItem, error)
	GetApplyBizHostsStatistics(kt *kit.Kit, expr *filter.Expression) (
		[]*cvmapplyproto.ApplyBizHostsStatisticsItem, error)
	GetApplyBizCpuCoresStatistics(kt *kit.Kit, expr *filter.Expression) (
		[]*cvmapplyproto.ApplyBizCpuCoresStatisticsItem, error)
	GetCompletionRateStatistics(kt *kit.Kit, expr *filter.Expression) (
		[]*cvmapplyproto.ApplyCompletionRateStatisticsItem, error)
	GetCompletionRateDetailStatistics(kt *kit.Kit, expr *filter.Expression) (
		[]*cvmapplyproto.ApplyCompletionRateDetailItem, error)
	GetDeliveryRateStatistics(kt *kit.Kit, expr *filter.Expression) (
		[]*cvmapplyproto.ApplyDeliveryRateStatisticsItem, error)
	GetDeliveryRateDetailStatistics(kt *kit.Kit, expr *filter.Expression) (
		[]*cvmapplyproto.ApplyDeliveryRateDetailItem, error)
}

var _ ZiyanCvmApplySuborderInterface = new(ZiyanCvmApplySuborderDao)

// ZiyanCvmApplySuborderDao dao.
type ZiyanCvmApplySuborderDao struct {
	Orm   orm.Interface
	Audit audit.Interface
}

// CreateWithTx create ziyan cvm apply suborder with tx.
func (d ZiyanCvmApplySuborderDao) CreateWithTx(kt *kit.Kit, tx *sqlx.Tx, models []cvmapplytable.ZiyanCvmApplySuborder) (
	[]string, error) {

	if len(models) == 0 {
		return nil, errf.New(errf.InvalidParameter, "models to create cannot be empty")
	}

	ids := make([]string, 0)
	for index := range models {
		if err := models[index].InsertValidate(); err != nil {
			return nil, err
		}
		ids = append(ids, models[index].SuborderID)
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s)	VALUES(%s)`, models[0].TableName(),
		cvmapplytable.ZiyanCvmApplySuborderColumns.ColumnExpr(),
		cvmapplytable.ZiyanCvmApplySuborderColumns.ColonNameExpr())

	err := d.Orm.Txn(tx).BulkInsert(kt.Ctx, sql, models)
	if err != nil {
		logs.Errorf("insert table %s failed, err: %v, rid: %s", models[0].TableName(), err, kt.Rid)
		return nil, fmt.Errorf("insert table %s failed, err: %v", models[0].TableName(), err)
	}

	return ids, nil
}

// Update update ziyan cvm apply suborder.
func (d ZiyanCvmApplySuborderDao) Update(kt *kit.Kit, tx *sqlx.Tx, filterExpr *filter.Expression,
	model *cvmapplytable.ZiyanCvmApplySuborder) error {

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
		logs.ErrorJson("update ziyan cvm apply suborder failed, sql: %s, err: %v, updateValue: %+v, rid: %s",
			sql, err, updateValue, kt.Rid)
		return err
	}

	if effected == 0 {
		logs.ErrorJson("update ziyan cvm apply suborder, but record not found, sql: %s, updateValue: %+v, rid: %s",
			sql, updateValue, kt.Rid)
	}
	return nil
}

// List get ziyan cvm apply suborder list.
func (d ZiyanCvmApplySuborderDao) List(kt *kit.Kit, opt *types.ListOption) (
	*cvmapplyproto.ZiyanCvmApplySuborderListResult, error) {

	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list ziyan cvm apply suborder options is nil")
	}

	if err := opt.Validate(filter.NewExprOption(filter.RuleFields(
		cvmapplytable.ZiyanCvmApplySuborderColumns.ColumnTypes())),
		core.NewDefaultPageOption()); err != nil {
		return nil, err
	}

	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	if opt.Page.Count {
		// this is a count request, then do count operation only.
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.ZiyanCvmApplySuborderTable, whereExpr)

		count, err := d.Orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.ErrorJson("count ziyan cvm apply suborder failed, err: %v, filter: %v, rid: %s",
				err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &cvmapplyproto.ZiyanCvmApplySuborderListResult{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page,
		&types.PageSQLOption{Sort: types.SortOption{Sort: "suborder_id", IfNotPresent: true}})
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`,
		cvmapplytable.ZiyanCvmApplySuborderColumns.FieldsNamedExpr(opt.Fields),
		table.ZiyanCvmApplySuborderTable, whereExpr, pageExpr)

	details := make([]*cvmapplytable.ZiyanCvmApplySuborder, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplySuborderListResult{Count: 0, Details: details}, nil
}

// DeleteWithTx delete ziyan cvm apply suborder with tx.
func (d ZiyanCvmApplySuborderDao) DeleteWithTx(kt *kit.Kit, tx *sqlx.Tx, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.ZiyanCvmApplySuborderTable, whereExpr)

	if _, err = d.Orm.Txn(tx).Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.ErrorJson("delete ziyan cvm apply suborder failed, err: %v, whereValue: %+v, rid: %s",
			err, whereValue, kt.Rid)
		return err
	}

	return nil
}

// GetOrderTimeCostOverview 按月份统计剔除审批阶段耗时
func (d ZiyanCvmApplySuborderDao) GetOrderTimeCostOverview(kt *kit.Kit, expr *filter.Expression) (
	[]*cvmapplyproto.OrderTimeCostItem, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("get order time cost overview failed, err: %v, whereValue: %+v, rid: %s", err, whereValue,
			kt.Rid)
		return nil, err
	}

	sql := fmt.Sprintf(`
		SELECT
			DATE_FORMAT(created_at, '%%Y-%%m') AS yearmonth,
			ROUND(AVG(TIMESTAMPDIFF(HOUR, created_at, updated_at)), 2) AS avg_duration_hours
		FROM %s %s
		GROUP BY yearmonth
		ORDER BY yearmonth ASC`, table.ZiyanCvmApplySuborderTable, whereExpr)

	details := make([]*cvmapplyproto.OrderTimeCostItem, 0)
	if err := d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.Errorf("get order time cost overview failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return details, nil
}

// GetOrderTimeCostCompare 按业务+月份统计剔除审批阶段耗时详情
func (d ZiyanCvmApplySuborderDao) GetOrderTimeCostCompare(kt *kit.Kit, expr *filter.Expression) (
	[]*cvmapplyproto.OrderTimeCostCompareItem, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("get order time cost compare failed, err: %v, whereValue: %+v, rid: %s", err, whereValue,
			kt.Rid)
		return nil, err
	}

	sql := fmt.Sprintf(`
		SELECT
			bk_biz_id,
			DATE_FORMAT(created_at, '%%Y-%%m') AS yearmonth,
			COUNT(*) AS done_orders,
			ROUND(AVG(TIMESTAMPDIFF(HOUR, created_at, updated_at)), 2) AS avg_duration_hours
		FROM %s %s
		GROUP BY bk_biz_id, yearmonth
		ORDER BY bk_biz_id ASC, yearmonth ASC`, table.ZiyanCvmApplySuborderTable, whereExpr)

	details := make([]*cvmapplyproto.OrderTimeCostCompareItem, 0)
	if err := d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.Errorf("get order time cost compare failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return details, nil
}

// GetPercentileTimeConsumptionOverview get percentile time consumption overview by month.
func (d ZiyanCvmApplySuborderDao) GetPercentileTimeConsumptionOverview(kt *kit.Kit, expr *filter.Expression) (
	*cvmapplyproto.ZiyanCvmApplyPercentileTimeOverviewResult, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.ErrorJson("get percentile time consumption overview failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 按月统计百分位耗时
	sql := fmt.Sprintf(`
		WITH duration_data AS (
			-- 第一步：计算每个子单的耗时
			SELECT 
				DATE_FORMAT(created_at, '%%Y-%%m') as yearmonth,
				TIMESTAMPDIFF(SECOND, created_at, updated_at) / 3600.0 as duration_hours 
			FROM %s %s
			  AND TIMESTAMPDIFF(SECOND, created_at, updated_at) > 0
		), 
		ranked_data AS (
			-- 第二步：为每个月的数据排序并计算百分位排名
			SELECT 
				yearmonth,
				duration_hours, 
				ROW_NUMBER() OVER (PARTITION BY yearmonth ORDER BY duration_hours) as row_num,
				COUNT(*) OVER (PARTITION BY yearmonth) as total_count
			FROM duration_data 
		) 
		-- 第三步：计算 P90、P95、P99
		SELECT 
			yearmonth,
			ROUND(MAX(CASE WHEN row_num = FLOOR((total_count - 1) * 0.90) + 1 THEN duration_hours END), 2) as p90_hours, 
			ROUND(MAX(CASE WHEN row_num = FLOOR((total_count - 1) * 0.95) + 1 THEN duration_hours END), 2) as p95_hours, 
			ROUND(MAX(CASE WHEN row_num = FLOOR((total_count - 1) * 0.99) + 1 THEN duration_hours END), 2) as p99_hours 
		FROM ranked_data 
		GROUP BY yearmonth
		ORDER BY yearmonth ASC`,
		table.ZiyanCvmApplySuborderTable, whereExpr)

	details := make([]*cvmapplyproto.PercentileTimeConsumptionItem, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.Errorf("get percentile time consumption overview failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyPercentileTimeOverviewResult{Details: details}, nil
}

// GetPercentileTimeConsumptionCompare get percentile time consumption compare by biz.
func (d ZiyanCvmApplySuborderDao) GetPercentileTimeConsumptionCompare(kt *kit.Kit, expr *filter.Expression) (
	*cvmapplyproto.ZiyanCvmApplyPercentileTimeCompareResult, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "current_expr are required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("get percentile time consumption compare failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	sql := fmt.Sprintf(`
		WITH duration_data AS (
			SELECT 
				bk_biz_id, 
				DATE_FORMAT(created_at, '%%Y-%%m') as yearmonth, 
				TIMESTAMPDIFF(SECOND, created_at, updated_at) / 3600.0 as duration_hours 
			FROM %s %s
			  AND TIMESTAMPDIFF(SECOND, created_at, updated_at) > 0
		), 
		ranked_data AS (
			SELECT 
				bk_biz_id, 
				yearmonth, 
				duration_hours, 
				ROW_NUMBER() OVER (PARTITION BY bk_biz_id, yearmonth ORDER BY duration_hours) as row_num, 
				COUNT(*) OVER (PARTITION BY bk_biz_id, yearmonth) as total_count 
			FROM duration_data 
		) 
		SELECT 
			bk_biz_id, 
			yearmonth, 
			MAX(total_count) as done_orders, 
			ROUND(MAX(CASE WHEN row_num = FLOOR((total_count - 1) * 0.90) + 1 THEN duration_hours END), 2) as p90_hours, 
			ROUND(MAX(CASE WHEN row_num = FLOOR((total_count - 1) * 0.95) + 1 THEN duration_hours END), 2) as p95_hours, 
			ROUND(MAX(CASE WHEN row_num = FLOOR((total_count - 1) * 0.99) + 1 THEN duration_hours END), 2) as p99_hours 
		FROM ranked_data 
		GROUP BY bk_biz_id, yearmonth 
		ORDER BY bk_biz_id ASC, yearmonth ASC`,
		table.ZiyanCvmApplySuborderTable, whereExpr)

	items := make([]*cvmapplyproto.PercentileTimeConsumptionCompareItem, 0)
	if err := d.Orm.Do().Select(kt.Ctx, &items, sql, whereValue); err != nil {
		logs.Errorf("get percentile time consumption compare failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyPercentileTimeCompareResult{Current: items, Compare: nil}, nil
}

// GetProductionStageTimeCostOverview get production stage time cost overview.
func (d ZiyanCvmApplySuborderDao) GetProductionStageTimeCostOverview(kt *kit.Kit, expr *filter.Expression) (
	[]*cvmapplyproto.ProductionStageTimeCostItem, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	// Validate filter expression fields
	exprOpt := filter.NewExprOption(
		filter.RuleFields(cvmapplytable.ZiyanCvmApplySuborderColumns.ColumnTypes()),
	)
	if err := expr.Validate(exprOpt); err != nil {
		logs.Errorf("invalid production stage time cost overview request, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("get order time cost overview failed, err: %v, whereValue: %+v, rid: %s",
			err, whereValue, kt.Rid)
		return nil, err
	}

	if whereValue == nil {
		whereValue = make(map[string]interface{})
	}

	sql := fmt.Sprintf(`
		SELECT
			DATE_FORMAT(g.created_at, '%%Y-%%m') AS yearmonth,
			ROUND(AVG(TIMESTAMPDIFF(HOUR, g.start_at, g.end_at)), 2) AS avg_duration_hours
		FROM %s g
		INNER JOIN %s s ON g.suborder_id = s.suborder_id
		%s
		  AND g.start_at IS NOT NULL
		  AND g.end_at IS NOT NULL
		  AND g.end_at > :min_valid_time
		  AND TIMESTAMPDIFF(HOUR, g.start_at, g.end_at) > :min_duration_hours
		  AND TIMESTAMPDIFF(HOUR, g.start_at, g.end_at) < :max_duration_hours
		GROUP BY DATE_FORMAT(g.created_at, '%%Y-%%m')
		ORDER BY yearmonth ASC`,
		table.ZiyanCvmGenerateRecordTable, table.ZiyanCvmApplySuborderTable, whereExpr)

	queryValue := tools.MapMerge(whereValue, map[string]interface{}{
		"min_valid_time":     constant.ProductionStageMinValidTime,
		"min_duration_hours": constant.ProductionStageMinDurationHours,
		"max_duration_hours": constant.ProductionStageOverviewMaxDurationHours,
	})

	details := make([]*cvmapplyproto.ProductionStageTimeCostItem, 0)
	if err := d.Orm.Do().Select(kt.Ctx, &details, sql, queryValue); err != nil {
		logs.Errorf("get order time cost overview failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return details, nil
}

// GetProductionStageTimeCostCompare get production stage time cost compare.
func (d ZiyanCvmApplySuborderDao) GetProductionStageTimeCostCompare(kt *kit.Kit, expr *filter.Expression) (
	[]*cvmapplyproto.ProductionStageTimeCostBizItem, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	// Validate filter expression fields
	exprOpt := filter.NewExprOption(
		filter.RuleFields(cvmapplytable.ZiyanCvmApplySuborderColumns.ColumnTypes()),
	)
	if err := expr.Validate(exprOpt); err != nil {
		logs.Errorf("invalid production stage time cost compare request, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		logs.Errorf("get order time cost compare failed, err: %v, whereValue: %+v, rid: %s",
			err, whereValue, kt.Rid)
		return nil, err
	}
	if whereValue == nil {
		whereValue = make(map[string]interface{})
	}

	sql := fmt.Sprintf(`
		SELECT 
			s.bk_biz_id,
			DATE_FORMAT(g.created_at, '%%Y-%%m') AS yearmonth,
			COUNT(*) AS done_orders,
			ROUND(AVG(TIMESTAMPDIFF(HOUR, g.start_at, g.end_at)), 2) AS avg_duration_hours
		FROM %s g
		INNER JOIN %s s ON g.suborder_id = s.suborder_id
		%s
		  AND g.start_at IS NOT NULL
		  AND g.end_at IS NOT NULL
		  AND g.end_at > :min_valid_time
		  AND TIMESTAMPDIFF(HOUR, g.start_at, g.end_at) > :min_duration_hours
		  AND TIMESTAMPDIFF(HOUR, g.start_at, g.end_at) < :max_duration_hours
		GROUP BY s.bk_biz_id, DATE_FORMAT(g.created_at, '%%Y-%%m')
		ORDER BY s.bk_biz_id ASC, yearmonth ASC`,
		table.ZiyanCvmGenerateRecordTable, table.ZiyanCvmApplySuborderTable, whereExpr)

	queryValue := tools.MapMerge(whereValue, map[string]interface{}{
		"min_valid_time":     constant.ProductionStageMinValidTime,
		"min_duration_hours": constant.ProductionStageMinDurationHours,
		"max_duration_hours": constant.ProductionStageCompareMaxDurationHours,
	})

	details := make([]*cvmapplyproto.ProductionStageTimeCostBizItem, 0)
	if err := d.Orm.Do().Select(kt.Ctx, &details, sql, queryValue); err != nil {
		logs.Errorf("get order time cost compare failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return details, nil
}

// GetApplyBizHostsStatistics 按业务统计申请主机数
func (d ZiyanCvmApplySuborderDao) GetApplyBizHostsStatistics(kt *kit.Kit, expr *filter.Expression) (
	[]*cvmapplyproto.ApplyBizHostsStatisticsItem, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	// 按 bk_biz_id 分组，统计订单数和主机数，按主机数降序排序
	sql := fmt.Sprintf(`
		SELECT 
			bk_biz_id,
			COUNT(*) AS order_count,
			IFNULL(SUM(success_num), 0) AS host_count
		FROM %s %s
		GROUP BY bk_biz_id
		ORDER BY host_count DESC`, table.ZiyanCvmApplySuborderTable, whereExpr)

	details := make([]*cvmapplyproto.ApplyBizHostsStatisticsItem, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.Errorf("get apply biz hosts statistics failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return details, nil
}

// GetApplyBizCpuCoresStatistics 按业务统计申请CPU核心数
func (d ZiyanCvmApplySuborderDao) GetApplyBizCpuCoresStatistics(kt *kit.Kit, expr *filter.Expression) (
	[]*cvmapplyproto.ApplyBizCpuCoresStatisticsItem, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	// 按 bk_biz_id 分组，统计订单数和交付核心数，按核心数降序排序
	sql := fmt.Sprintf(`
		SELECT 
			bk_biz_id,
			COUNT(*) AS order_count,
			IFNULL(SUM(delivered_core), 0) AS delivered_core_count
		FROM %s %s
		GROUP BY bk_biz_id
		ORDER BY delivered_core_count DESC`, table.ZiyanCvmApplySuborderTable, whereExpr)

	details := make([]*cvmapplyproto.ApplyBizCpuCoresStatisticsItem, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.Errorf("get apply biz cpu cores statistics failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return details, nil
}

func ensureWhereValue(whereValue map[string]interface{}) map[string]interface{} {
	if whereValue == nil {
		return make(map[string]interface{})
	}

	return whereValue
}

func injectCompletionRateParams(whereValue map[string]interface{}) map[string]interface{} {
	whereValue = ensureWhereValue(whereValue)
	whereValue["_inner_stage_done"] = enumor.TicketStageDone
	whereValue["_inner_stage_terminate"] = enumor.TicketStageTerminate
	whereValue["_inner_status_done"] = enumor.ApplyStatusDone
	whereValue["_inner_status_terminate"] = enumor.ApplyStatusTerminate

	return whereValue
}

func injectStageDoneParam(whereValue map[string]interface{}) map[string]interface{} {
	whereValue = ensureWhereValue(whereValue)
	whereValue["_inner_stage_done"] = enumor.TicketStageDone

	return whereValue
}

// GetCompletionRateStatistics 按月份统计结单率
func (d ZiyanCvmApplySuborderDao) GetCompletionRateStatistics(kt *kit.Kit, expr *filter.Expression) (
	[]*cvmapplyproto.ApplyCompletionRateStatisticsItem, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	whereValue = injectCompletionRateParams(whereValue)
	sql := fmt.Sprintf(`
		SELECT
			DATE_FORMAT(created_at, '%%Y-%%m') AS yearmonth,
			COALESCE(ROUND((
				SUM(CASE WHEN stage IN (:_inner_stage_done, :_inner_stage_terminate)
					AND status IN (:_inner_status_done, :_inner_status_terminate) THEN 1 ELSE 0 END)
				/ NULLIF(COUNT(*), 0)
			) * 100, 2), 0) AS completion_rate
		FROM %s %s
		GROUP BY yearmonth
		ORDER BY yearmonth ASC`, table.ZiyanCvmApplySuborderTable, whereExpr)

	details := make([]*cvmapplyproto.ApplyCompletionRateStatisticsItem, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.Errorf("get completion rate statistics failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return details, nil
}

// GetCompletionRateDetailStatistics 按业务+月份统计结单率详情
func (d ZiyanCvmApplySuborderDao) GetCompletionRateDetailStatistics(kt *kit.Kit, expr *filter.Expression) (
	[]*cvmapplyproto.ApplyCompletionRateDetailItem, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	whereValue = injectCompletionRateParams(whereValue)
	sql := fmt.Sprintf(`
		SELECT
			bk_biz_id,
			DATE_FORMAT(created_at, '%%Y-%%m') AS yearmonth,
			COUNT(*) AS total_orders,
			SUM(CASE WHEN stage IN (:_inner_stage_done, :_inner_stage_terminate)
				AND status IN (:_inner_status_done, :_inner_status_terminate) THEN 1 ELSE 0 END) AS done_orders,
			COALESCE(ROUND((
				SUM(CASE WHEN stage IN (:_inner_stage_done, :_inner_stage_terminate)
					AND status IN (:_inner_status_done, :_inner_status_terminate) THEN 1 ELSE 0 END)
				/ NULLIF(COUNT(*), 0)
			) * 100, 2), 0) AS completion_rate
		FROM %s %s
		GROUP BY bk_biz_id, yearmonth
		ORDER BY completion_rate DESC, bk_biz_id ASC, yearmonth ASC`,
		table.ZiyanCvmApplySuborderTable, whereExpr)

	details := make([]*cvmapplyproto.ApplyCompletionRateDetailItem, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.Errorf("get completion rate detail statistics failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return details, nil
}

// GetDeliveryRateStatistics 按月份统计主机交付率
func (d ZiyanCvmApplySuborderDao) GetDeliveryRateStatistics(kt *kit.Kit, expr *filter.Expression) (
	[]*cvmapplyproto.ApplyDeliveryRateStatisticsItem, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`
		SELECT
			DATE_FORMAT(created_at, '%%Y-%%m') AS yearmonth,
			COALESCE(ROUND((
				IFNULL(SUM(success_num), 0) / NULLIF(IFNULL(SUM(total_num), 0), 0)
			) * 100, 2), 0) AS delivery_rate
		FROM %s %s
		GROUP BY yearmonth
		ORDER BY yearmonth ASC`, table.ZiyanCvmApplySuborderTable, whereExpr)

	details := make([]*cvmapplyproto.ApplyDeliveryRateStatisticsItem, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.Errorf("get delivery rate statistics failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return details, nil
}

// GetDeliveryRateDetailStatistics 按业务+月份统计主机交付率详情
func (d ZiyanCvmApplySuborderDao) GetDeliveryRateDetailStatistics(kt *kit.Kit, expr *filter.Expression) (
	[]*cvmapplyproto.ApplyDeliveryRateDetailItem, error) {

	if expr == nil {
		return nil, errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	whereValue = injectStageDoneParam(whereValue)
	sql := fmt.Sprintf(`
		SELECT
			bk_biz_id,
			DATE_FORMAT(created_at, '%%Y-%%m') AS yearmonth,
			COUNT(*) AS total_orders,
			SUM(CASE WHEN stage = :_inner_stage_done THEN 1 ELSE 0 END) AS done_orders,
			IFNULL(SUM(total_num), 0) AS total_num_sum,
			IFNULL(SUM(success_num), 0) AS success_num_sum,
			COALESCE(ROUND((
				IFNULL(SUM(success_num), 0) / NULLIF(IFNULL(SUM(total_num), 0), 0)
			) * 100, 2), 0) AS host_delivery_rate
		FROM %s %s
		GROUP BY bk_biz_id, yearmonth
		ORDER BY host_delivery_rate DESC, bk_biz_id ASC, yearmonth ASC`,
		table.ZiyanCvmApplySuborderTable, whereExpr)

	details := make([]*cvmapplyproto.ApplyDeliveryRateDetailItem, 0)
	if err = d.Orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		logs.Errorf("get delivery rate detail statistics failed, sql: %s, err: %v, whereValue: %+v, rid: %s",
			sql, err, whereValue, kt.Rid)
		return nil, err
	}

	return details, nil
}
