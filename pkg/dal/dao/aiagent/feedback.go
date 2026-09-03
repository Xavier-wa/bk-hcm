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

package aiagent

import (
	"fmt"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	idgenerator "hcm/pkg/dal/dao/id-generator"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	"hcm/pkg/dal/table"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
)

// AiagentRunFeedback defines aiagent run feedback DAO operations.
type AiagentRunFeedback interface {
	Create(kt *kit.Kit, model *tableaiagent.FeedbackTable) (string, error)
	Update(kt *kit.Kit, expr *filter.Expression, model *tableaiagent.FeedbackTable) error
	List(kt *kit.Kit, opt *types.ListOption) (*types.ListResult[tableaiagent.FeedbackTable], error)
	Delete(kt *kit.Kit, expr *filter.Expression) error
}

var _ AiagentRunFeedback = new(AiagentRunFeedbackDao)

// AiagentRunFeedbackDao is the DAO implementation for aiagent run feedback.
type AiagentRunFeedbackDao struct {
	orm   orm.Interface
	idGen idgenerator.IDGenInterface
}

// NewAiagentRunFeedbackDao creates a new AiagentRunFeedbackDao.
func NewAiagentRunFeedbackDao(orm orm.Interface, idGen idgenerator.IDGenInterface) AiagentRunFeedback {
	return &AiagentRunFeedbackDao{
		orm:   orm,
		idGen: idGen,
	}
}

// Create inserts a new feedback row and returns the generated id. Callers MUST
// translate a RecordDuplicated error (uk_run_id conflict) into an Update instead
// of surfacing it, since one run has at most one feedback row.
func (d *AiagentRunFeedbackDao) Create(kt *kit.Kit, model *tableaiagent.FeedbackTable) (string, error) {
	id, err := d.idGen.One(kt, table.AiagentRunFeedbackTable)
	if err != nil {
		return "", err
	}
	model.ID = id
	model.Creator = kt.User
	model.Reviser = kt.User

	if err := model.InsertValidate(); err != nil {
		return "", err
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s) VALUES(%s)`, table.AiagentRunFeedbackTable,
		tableaiagent.FeedbackColumns.ColumnExpr(), tableaiagent.FeedbackColumns.ColonNameExpr())

	if err := d.orm.Do().Insert(kt.Ctx, sql, model); err != nil {
		if em := errf.GetMySQLDuplicated(err); em != nil {
			return "", errf.New(errf.RecordDuplicated, em.Message)
		}
		logs.Errorf("insert %s failed, err: %v, rid: %s", table.AiagentRunFeedbackTable, err, kt.Rid)
		return "", fmt.Errorf("insert %s failed, err: %v", table.AiagentRunFeedbackTable, err)
	}

	return id, nil
}

// Update updates the matched feedback row. Empty tags/comment are written via
// AddBlankedFields because they are meaningful values, not "unset" markers.
func (d *AiagentRunFeedbackDao) Update(kt *kit.Kit, expr *filter.Expression, model *tableaiagent.FeedbackTable) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is nil")
	}

	model.Reviser = kt.User
	if err := model.UpdateValidate(); err != nil {
		return err
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	opts := utils.NewFieldOptions().AddBlankedFields("tags", "comment").
		AddIgnoredFields(types.DefaultIgnoredFields...)
	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(model, opts)
	if err != nil {
		return fmt.Errorf("prepare parsed sql set filter expr failed, err: %v", err)
	}

	sql := fmt.Sprintf(`UPDATE %s %s %s`, model.TableName(), setExpr, whereExpr)

	effected, err := d.orm.Do().Update(kt.Ctx, sql, tools.MapMerge(toUpdate, whereValue))
	if err != nil {
		logs.Errorf("update aiagent run feedback failed, err: %v, filter: %s, rid: %s", err, expr, kt.Rid)
		return err
	}

	if effected == 0 {
		logs.Errorf("update aiagent run feedback, but data not found, filter: %s, rid: %s", expr, kt.Rid)
		return errf.New(errf.RecordNotFound, orm.ErrRecordNotFound.Error())
	}

	return nil
}

// List queries aiagent run feedback rows with filter and pagination.
func (d *AiagentRunFeedbackDao) List(kt *kit.Kit, opt *types.ListOption) (
	*types.ListResult[tableaiagent.FeedbackTable], error) {

	if opt == nil {
		return nil, errf.New(errf.InvalidParameter, "list aiagent run feedback options is nil")
	}

	if err := opt.Validate(filter.NewExprOption(filter.RuleFields(tableaiagent.FeedbackColumns.ColumnTypes())),
		core.NewDefaultPageOption()); err != nil {
		return nil, err
	}

	if opt.Filter == nil {
		opt.Filter = tools.AllExpression()
	}
	whereExpr, whereValue, err := opt.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return nil, err
	}

	if opt.Page.Count {
		sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table.AiagentRunFeedbackTable, whereExpr)

		count, err := d.orm.Do().Count(kt.Ctx, sql, whereValue)
		if err != nil {
			logs.Errorf("count aiagent run feedback failed, err: %v, filter: %s, rid: %s", err, opt.Filter, kt.Rid)
			return nil, err
		}

		return &types.ListResult[tableaiagent.FeedbackTable]{Count: count}, nil
	}

	pageExpr, err := types.PageSQLExpr(opt.Page, types.DefaultPageSQLOption)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`SELECT %s FROM %s %s %s`,
		tableaiagent.FeedbackColumns.FieldsNamedExpr(opt.Fields),
		table.AiagentRunFeedbackTable, whereExpr, pageExpr)

	details := make([]tableaiagent.FeedbackTable, 0)
	if err = d.orm.Do().Select(kt.Ctx, &details, sql, whereValue); err != nil {
		return nil, err
	}

	return &types.ListResult[tableaiagent.FeedbackTable]{Details: details}, nil
}

// Delete removes aiagent run feedback rows matched by the filter.
func (d *AiagentRunFeedbackDao) Delete(kt *kit.Kit, expr *filter.Expression) error {
	if expr == nil {
		return errf.New(errf.InvalidParameter, "filter expr is required")
	}

	whereExpr, whereValue, err := expr.SQLWhereExpr(tools.DefaultSqlWhereOption)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`DELETE FROM %s %s`, table.AiagentRunFeedbackTable, whereExpr)
	if _, err := d.orm.Do().Delete(kt.Ctx, sql, whereValue); err != nil {
		logs.Errorf("delete aiagent run feedback failed, err: %v, filter: %s, rid: %s", err, expr, kt.Rid)
		return err
	}

	return nil
}
