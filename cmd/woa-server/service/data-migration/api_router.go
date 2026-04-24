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

// Package datamigration provides data migration service
package datamigration

import (
	"fmt"
	"strings"
	"time"

	"hcm/cmd/woa-server/service/data-migration/converters"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/client/data-service/tcloud-ziyan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/table"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
)

// ClientAPIRouter API客户端路由器（支持自动分批）
type ClientAPIRouter struct {
	client       *ziyan.Client
	maxBatchSize int           // data-service单次限制，默认100
	orm          orm.Interface // MySQL ORM（用于数据迁移时更新时间字段等特殊操作）
}

// NewClientAPIRouter 创建API路由器
func NewClientAPIRouter(client *ziyan.Client, orm orm.Interface) *ClientAPIRouter {
	return &ClientAPIRouter{
		client:       client,
		maxBatchSize: 100,
		orm:          orm,
	}
}

// BatchCallCreateAPI 批量调用创建API（自动分批）
func (r *ClientAPIRouter) BatchCallCreateAPI(kt *kit.Kit, tableName string, dataList []interface{}) error {
	if len(dataList) == 0 {
		return nil
	}

	// 按maxBatchSize分批
	batches := r.splitIntoBatches(dataList)

	for i, batch := range batches {
		logs.Infof("[BatchCallCreateAPI] table: %s, batch: %d/%d, size: %d, rid: %s",
			tableName, i+1, len(batches), len(batch), kt.Rid)

		if err := r.callCreateAPIOneBatch(kt, tableName, batch); err != nil {
			return fmt.Errorf("[BatchCallCreateAPI] failed, table: %s, batch %d/%d failed, err: %w",
				tableName, i+1, len(batches), err)
		}
	}

	return nil
}

// callCreateAPIOneBatch 调用创建API（单批）
func (r *ClientAPIRouter) callCreateAPIOneBatch(kt *kit.Kit, tableName string, batch []interface{}) error {
	// 检查是否包含多表创建请求
	if len(batch) > 0 {
		if _, ok := batch[0].(*converters.MultiTableCreateRequest); ok {
			return r.handleMultiTableCreate(kt, batch)
		}
	}

	switch tableName {
	case table.ZiyanCvmApplyOrderTable:
		return r.createApplyOrder(kt, batch)
	case table.ZiyanCvmApplySuborderTable:
		return r.createApplySuborder(kt, batch)
	case table.ZiyanCvmApplyStepTable:
		return r.createApplyStep(kt, batch)
	case table.ZiyanCvmGenerateRecordTable:
		return r.createGenerateRecord(kt, batch)
	case table.ZiyanCvmApplyInitTaskTable:
		return r.createApplyInitTask(kt, batch)
	case table.ZiyanCvmDeviceInfoTable:
		return r.createDeviceInfo(kt, batch)
	case table.ZiyanCvmDeliverRecordTable:
		return r.createDeliverRecord(kt, batch)
	case table.ZiyanCvmModifyRecordTable:
		return r.createModifyRecord(kt, batch)
	default:
		return fmt.Errorf("unsupported table: %s", tableName)
	}
}

// BatchCallUpdateAPI 批量调用更新API（自动分批）
func (r *ClientAPIRouter) BatchCallUpdateAPI(kt *kit.Kit, tableName string, dataList []interface{}) error {
	if len(dataList) == 0 {
		return nil
	}

	batches := r.splitIntoBatches(dataList)

	for i, batch := range batches {
		logs.Infof("[BatchCallUpdateAPI] table: %s, batch: %d/%d, size: %d, rid: %s",
			tableName, i+1, len(batches), len(batch), kt.Rid)

		if err := r.callUpdateAPIOneBatch(kt, tableName, batch); err != nil {
			return fmt.Errorf("batch %d/%d failed: %w", i+1, len(batches), err)
		}
	}

	return nil
}

// callUpdateAPIOneBatch 调用更新API（单批）
func (r *ClientAPIRouter) callUpdateAPIOneBatch(kt *kit.Kit, tableName string, batch []interface{}) error {
	// 检查是否包含多表更新请求
	if len(batch) > 0 {
		if _, ok := batch[0].(*converters.MultiTableUpdateRequest); ok {
			return r.handleMultiTableUpdate(kt, batch)
		}
	}

	switch tableName {
	case table.ZiyanCvmApplyOrderTable:
		return r.updateApplyOrder(kt, batch)
	case table.ZiyanCvmApplySuborderTable:
		return r.updateApplySuborder(kt, batch)
	case table.ZiyanCvmApplyStepTable:
		return r.updateApplyStep(kt, batch)
	case table.ZiyanCvmGenerateRecordTable:
		return r.updateGenerateRecord(kt, batch)
	case table.ZiyanCvmApplyInitTaskTable:
		return r.updateApplyInitTask(kt, batch)
	case table.ZiyanCvmDeviceInfoTable:
		return r.updateDeviceInfo(kt, batch)
	case table.ZiyanCvmDeliverRecordTable:
		return r.updateDeliverRecord(kt, batch)
	case table.ZiyanCvmModifyRecordTable:
		return r.updateModifyRecord(kt, batch)
	default:
		return fmt.Errorf("unsupported table: %s", tableName)
	}
}

// BatchCallListAPI 批量调用查询API（分批查询主键，支持单字段和复合主键）
func (r *ClientAPIRouter) BatchCallListAPI(kt *kit.Kit, tableName string, pkFields []string, pkValues []interface{}) (
	[]interface{}, error) {

	if len(pkValues) == 0 {
		return []interface{}{}, nil
	}

	results := make([]interface{}, 0)

	// 按批次大小分批查询
	// 注意：复合主键查询时，每条记录会生成一个 OR rule，data-service 限制最多 10 个 rules
	var batches [][]interface{}
	if len(pkFields) > 1 {
		// 复合主键：每批最多 10 条记录（避免超出 data-service 的 10 rules 限制）
		batches = r.splitIntoBatchesWithLimit(pkValues, 10)
	} else {
		// 单字段主键：使用默认批次大小
		batches = r.splitIntoBatches(pkValues)
	}

	for i, batch := range batches {
		logs.Infof("[BatchCallListAPI] table: %s, batch: %d/%d, size: %d, composite_key: %v, rid: %s",
			tableName, i+1, len(batches), len(batch), len(pkFields) > 1, kt.Rid)

		batchResults, err := r.callListAPIOneBatch(kt, tableName, pkFields, batch)
		if err != nil {
			return nil, fmt.Errorf("[BatchCallListAPI] table: %s, batch %d/%d failed, err: %w",
				tableName, i+1, len(batches), err)
		}

		results = append(results, batchResults...)
	}

	return results, nil
}

// BatchCallListByFilterAPI 按过滤条件查询目标表数据（用于反向同步）
func (r *ClientAPIRouter) BatchCallListByFilterAPI(kt *kit.Kit, tableName string, filterExpr *filter.Expression) (
	[]interface{}, error) {

	return r.callListAPIByFilter(kt, tableName, filterExpr)
}

// callListAPIOneBatch 调用查询API（单批，支持单字段和复合主键）
func (r *ClientAPIRouter) callListAPIOneBatch(kt *kit.Kit, tableName string, pkFields []string, pkValues []interface{}) (
	[]interface{}, error) {

	// 构建查询条件（支持单字段主键和复合主键）
	var filterExpr *filter.Expression
	if len(pkFields) == 1 {
		// 单字段主键，使用 IN 查询
		filterExpr = r.buildInFilter(pkFields[0], pkValues)
	} else {
		// 复合主键，使用 OR 组合的多个 AND 条件
		filterExpr = r.buildCompositeKeyFilter(pkFields, pkValues)
	}

	return r.callListAPIByFilter(kt, tableName, filterExpr)
}

// callListAPIByFilter 按过滤条件调用查询API
func (r *ClientAPIRouter) callListAPIByFilter(kt *kit.Kit, tableName string, filterExpr *filter.Expression) (
	[]interface{}, error) {

	switch tableName {
	case table.ZiyanCvmApplyOrderTable:
		return r.listApplyOrder(kt, filterExpr)
	case table.ZiyanCvmApplySuborderTable:
		return r.listApplySuborder(kt, filterExpr)
	case table.ZiyanCvmApplyStepTable:
		return r.listApplyStep(kt, filterExpr)
	case table.ZiyanCvmGenerateRecordTable:
		return r.listGenerateRecord(kt, filterExpr)
	case table.ZiyanCvmApplyInitTaskTable:
		return r.listApplyInitTask(kt, filterExpr)
	case table.ZiyanCvmDeviceInfoTable:
		return r.listDeviceInfo(kt, filterExpr)
	case table.ZiyanCvmDeliverRecordTable:
		return r.listDeliverRecord(kt, filterExpr)
	case table.ZiyanCvmModifyRecordTable:
		return r.listModifyRecord(kt, filterExpr)
	default:
		return nil, fmt.Errorf("unsupported table: %s", tableName)
	}
}

// ========================================
// Create 方法（各表）
// ========================================

// createApplyOrder 创建申请单主单（支持数据迁移时间保留）
func (r *ClientAPIRouter) createApplyOrder(kt *kit.Kit, batch []interface{}) error {
	orders := make([]cvmapplyproto.ZiyanCvmApplyOrderCreateReq, 0, len(batch))
	hasCreatedAt := false

	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmApplyOrderCreateReq); ok {
			orders = append(orders, *req)
			// 检查是否有需要保留的创建时间
			if len(req.CreatedAt) > 0 {
				hasCreatedAt = true
			}
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmApplyOrderTable)
		}
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmApplyOrderReq{ApplyOrders: orders}
	result, err := r.client.ZiyanCvmApplyOrder.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("[MigrationEngine:Migrate] createApplyOrder:BatchCreate, err: %v, hasCreatedAt: %v, rid: %s",
			err, hasCreatedAt, kt.Rid)
		return err
	}

	// 如果有需要保留的创建时间，创建后更新created_at和updated_at字段
	if hasCreatedAt && result != nil && len(result.IDs) > 0 {
		// 构建更新数据：将orders和IDs配对
		updates := make([]timestampUpdateItem, 0, len(orders))
		for _, order := range orders {
			if len(order.CreatedAt) > 0 {
				createdAtStr := convertTimeToMySQLFormat(string(order.CreatedAt))
				updatedAtStr := convertTimeToMySQLFormat(string(order.UpdatedAt))
				updates = append(updates, timestampUpdateItem{
					primaryKey: fmt.Sprintf("%d", order.OrderID),
					createdAt:  createdAtStr,
					updatedAt:  updatedAtStr,
				})
			}
		}
		if len(updates) > 0 {
			if err = r.updateTimestampsByPrimaryKey(
				kt, table.ZiyanCvmApplyOrderTable, "order_id", updates); err != nil {
				logs.Warnf("[MigrationEngine:Migrate] createApplyOrder update timestamps failed, err: %v, rid: %s",
					err, kt.Rid)
				// 不返回错误，因为数据已经创建成功，只是时间未更新
			}
		}
	}

	return nil
}

// createApplySuborder 创建申请单子单（支持数据迁移时间保留）
func (r *ClientAPIRouter) createApplySuborder(kt *kit.Kit, batch []interface{}) error {
	suborders := make([]cvmapplyproto.ZiyanCvmApplySuborderCreateReq, 0, len(batch))
	hasCreatedAt := false

	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmApplySuborderCreateReq); ok {
			if len(req.Source) == 0 {
				req.Source = enumor.ApplyTicketSrcBusiness
			}
			if len(req.ProductType) == 0 {
				req.ProductType = enumor.ProductTypeBusiness
			}
			suborders = append(suborders, *req)
			// 检查是否有需要保留的创建时间
			if len(req.CreatedAt) > 0 {
				hasCreatedAt = true
			}
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmApplySuborderTable)
		}
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmApplySuborderReq{ApplySuborders: suborders}
	_, err := r.client.ZiyanCvmApplySuborder.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("[MigrationEngine:Migrate] createApplySuborder:BatchCreate, err: %v, hasCreatedAt: %v, rid: %s",
			err, hasCreatedAt, kt.Rid)
		return err
	}

	// 如果有需要保留的创建时间，创建后更新created_at和updated_at字段
	// suborder表使用suborder_id作为主键（string类型），suborder_id在CreateReq中指定
	if hasCreatedAt {
		// 构建更新数据：直接使用CreateReq中的suborder_id
		updates := make([]timestampUpdateItem, 0, len(suborders))
		for _, suborder := range suborders {
			if len(suborder.CreatedAt) > 0 {
				createdAtStr := convertTimeToMySQLFormat(string(suborder.CreatedAt))
				updatedAtStr := convertTimeToMySQLFormat(string(suborder.UpdatedAt))
				updates = append(updates, timestampUpdateItem{
					primaryKey: suborder.SuborderID,
					createdAt:  createdAtStr,
					updatedAt:  updatedAtStr,
				})
			}
		}
		if len(updates) > 0 {
			if err = r.updateTimestampsByPrimaryKey(
				kt, table.ZiyanCvmApplySuborderTable, "suborder_id", updates); err != nil {
				logs.Warnf("[MigrationEngine:Migrate] createApplySuborder update timestamps failed, err: %v, rid: %s",
					err, kt.Rid)
				// 不返回错误，因为数据已经创建成功，只是时间未更新
			}
		}
	}

	return nil
}

// createApplyStep 创建申请单执行步骤（支持数据迁移时间保留）
func (r *ClientAPIRouter) createApplyStep(kt *kit.Kit, batch []interface{}) error {
	steps := make([]cvmapplyproto.ZiyanCvmApplyStepCreateReq, 0, len(batch))
	hasCreatedAt := false

	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmApplyStepCreateReq); ok {
			steps = append(steps, *req)
			// 检查是否有需要保留的创建时间
			if len(req.CreatedAt) > 0 {
				hasCreatedAt = true
			}
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmApplyStepTable)
		}
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmApplyStepReq{ApplySteps: steps}
	result, err := r.client.ZiyanCvmApplyStep.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("[MigrationEngine:Migrate] createApplyStep:BatchCreate, err: %v, hasCreatedAt: %v, rid: %s",
			err, hasCreatedAt, kt.Rid)
		return err
	}

	// 如果有需要保留的创建时间，创建后更新created_at和updated_at字段
	// step表使用id作为主键（string类型），但BatchCreate返回的是自增ID
	if hasCreatedAt && result != nil && len(result.IDs) > 0 {
		updates := make([]timestampUpdateItem, 0, len(steps))
		for i, id := range result.IDs {
			if len(steps[i].CreatedAt) > 0 {
				createdAtStr := convertTimeToMySQLFormat(string(steps[i].CreatedAt))
				updatedAtStr := convertTimeToMySQLFormat(string(steps[i].UpdatedAt))
				updates = append(updates, timestampUpdateItem{
					primaryKey: fmt.Sprintf("%s", id),
					createdAt:  createdAtStr,
					updatedAt:  updatedAtStr,
				})
			}
		}
		if len(updates) > 0 {
			if err = r.updateTimestampsByPrimaryKey(kt, table.ZiyanCvmApplyStepTable, "id", updates); err != nil {
				logs.Warnf("[MigrationEngine:Migrate] createApplyStep update timestamps failed, err: %v, rid: %s",
					err, kt.Rid)
				// 不返回错误，因为数据已经创建成功，只是时间未更新
			}
		}
	}

	return nil
}

// createGenerateRecord 创建生产任务记录（支持数据迁移时间保留）
func (r *ClientAPIRouter) createGenerateRecord(kt *kit.Kit, batch []interface{}) error {
	records := make([]cvmapplyproto.ZiyanCvmGenerateRecordCreateReq, 0, len(batch))
	hasCreatedAt := false

	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmGenerateRecordCreateReq); ok {
			records = append(records, *req)
			// 检查是否有需要保留的创建时间
			if len(req.CreatedAt) > 0 {
				hasCreatedAt = true
			}
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmGenerateRecordTable)
		}
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmGenerateRecordReq{GenerateRecords: records}
	_, err := r.client.ZiyanCvmGenerateRecord.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("[MigrationEngine:Migrate] createGenerateRecord:BatchCreate, err: %v, hasCreatedAt: %v, rid: %s",
			err, hasCreatedAt, kt.Rid)
		return err
	}

	// 如果有需要保留的创建时间，创建后更新created_at和updated_at字段
	// generate_record表使用(suborder_id, generate_id)作为联合主键
	if hasCreatedAt {
		// 构建更新数据：使用联合主键
		updates := make([]compositeKeyTimestampUpdateItem, 0, len(records))
		for _, record := range records {
			if len(record.CreatedAt) > 0 {
				createdAtStr := convertTimeToMySQLFormat(string(record.CreatedAt))
				updatedAtStr := convertTimeToMySQLFormat(string(record.UpdatedAt))
				updates = append(updates, compositeKeyTimestampUpdateItem{
					primaryKeys: map[string]string{
						"suborder_id": record.SuborderID,
						"generate_id": record.GenerateID,
					},
					createdAt: createdAtStr,
					updatedAt: updatedAtStr,
				})
			}
		}
		if len(updates) > 0 {
			if err = r.updateTimestampsByCompositeKey(
				kt, table.ZiyanCvmGenerateRecordTable, []string{"suborder_id", "generate_id"}, updates); err != nil {
				logs.Warnf("[MigrationEngine:Migrate] createGenerateRecord update timestamps failed, err: %v, rid: %s",
					err, kt.Rid)
				// 不返回错误，因为数据已经创建成功，只是时间未更新
			}
		}
	}

	return nil
}

// createApplyInitTask 创建初始化任务记录（支持数据迁移时间保留）
func (r *ClientAPIRouter) createApplyInitTask(kt *kit.Kit, batch []interface{}) error {
	tasks := make([]cvmapplyproto.ZiyanCvmApplyInitTaskCreateReq, 0, len(batch))
	hasCreatedAt := false

	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmApplyInitTaskCreateReq); ok {
			tasks = append(tasks, *req)
			// 检查是否有需要保留的创建时间
			if len(req.CreatedAt) > 0 {
				hasCreatedAt = true
			}
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmApplyInitTaskTable)
		}
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmApplyInitTaskReq{InitTasks: tasks}
	result, err := r.client.ZiyanCvmApplyInitTask.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("[MigrationEngine:Migrate] createApplyInitTask:BatchCreate, err: %v, hasCreatedAt: %v, rid: %s",
			err, hasCreatedAt, kt.Rid)
		return err
	}

	// 如果有需要保留的创建时间，创建后更新created_at和updated_at字段
	if hasCreatedAt && result != nil && len(result.IDs) > 0 {
		updates := make([]timestampUpdateItem, 0, len(tasks))
		for i, id := range result.IDs {
			if len(tasks[i].CreatedAt) > 0 {
				createdAtStr := convertTimeToMySQLFormat(string(tasks[i].CreatedAt))
				updatedAtStr := convertTimeToMySQLFormat(string(tasks[i].UpdatedAt))
				updates = append(updates, timestampUpdateItem{
					primaryKey: fmt.Sprintf("%s", id),
					createdAt:  createdAtStr,
					updatedAt:  updatedAtStr,
				})
			}
		}
		if len(updates) > 0 {
			if err = r.updateTimestampsByPrimaryKey(kt, table.ZiyanCvmApplyInitTaskTable, "id", updates); err != nil {
				logs.Warnf("[MigrationEngine:Migrate] createApplyInitTask update timestamps failed, err: %v, rid: %s",
					err, kt.Rid)
				// 不返回错误，因为数据已经创建成功，只是时间未更新
			}
		}
	}

	return nil
}

// createDeviceInfo 创建设备交付记录（支持数据迁移时间保留）
func (r *ClientAPIRouter) createDeviceInfo(kt *kit.Kit, batch []interface{}) error {
	devices := make([]cvmapplyproto.ZiyanCvmDeviceInfoCreateReq, 0, len(batch))
	hasCreatedAt := false

	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmDeviceInfoCreateReq); ok {
			devices = append(devices, *req)
			// 检查是否有需要保留的创建时间
			if len(req.CreatedAt) > 0 {
				hasCreatedAt = true
			}
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmDeviceInfoTable)
		}
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmDeviceInfoReq{Devices: devices}
	result, err := r.client.ZiyanCvmDeviceInfo.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("[MigrationEngine:Migrate] createDeviceInfo:BatchCreate, err: %v, hasCreatedAt: %v, rid: %s",
			err, hasCreatedAt, kt.Rid)
		return err
	}

	// 如果有需要保留的创建时间，创建后更新created_at和updated_at字段
	if hasCreatedAt && result != nil && len(result.IDs) > 0 {
		updates := make([]timestampUpdateItem, 0, len(devices))
		for i, id := range result.IDs {
			if len(devices[i].CreatedAt) > 0 {
				createdAtStr := convertTimeToMySQLFormat(string(devices[i].CreatedAt))
				updatedAtStr := convertTimeToMySQLFormat(string(devices[i].UpdatedAt))
				updates = append(updates, timestampUpdateItem{
					primaryKey: fmt.Sprintf("%s", id),
					createdAt:  createdAtStr,
					updatedAt:  updatedAtStr,
				})
			}
		}
		if len(updates) > 0 {
			if err = r.updateTimestampsByPrimaryKey(kt, table.ZiyanCvmDeviceInfoTable, "id", updates); err != nil {
				logs.Warnf("[MigrationEngine:Migrate] createDeviceInfo update timestamps failed, err: %v, rid: %s",
					err, kt.Rid)
				// 不返回错误，因为数据已经创建成功，只是时间未更新
			}
		}
	}

	return nil
}

// createDeliverRecord 创建交付任务记录（支持数据迁移时间保留）
func (r *ClientAPIRouter) createDeliverRecord(kt *kit.Kit, batch []interface{}) error {
	records := make([]cvmapplyproto.ZiyanCvmDeliverRecordCreateReq, 0, len(batch))
	hasCreatedAt := false

	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmDeliverRecordCreateReq); ok {
			records = append(records, *req)
			// 检查是否有需要保留的创建时间
			if len(req.CreatedAt) > 0 {
				hasCreatedAt = true
			}
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmDeliverRecordTable)
		}
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmDeliverRecordReq{Records: records}
	result, err := r.client.ZiyanCvmDeliverRecord.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("[MigrationEngine:Migrate] createDeliverRecord:BatchCreate, err: %v, hasCreatedAt: %v, rid: %s",
			err, hasCreatedAt, kt.Rid)
		return err
	}

	// 如果有需要保留的创建时间，创建后更新created_at和updated_at字段
	if hasCreatedAt && result != nil && len(result.IDs) > 0 {
		updates := make([]timestampUpdateItem, 0, len(records))
		for i, id := range result.IDs {
			if len(records[i].CreatedAt) > 0 {
				createdAtStr := convertTimeToMySQLFormat(string(records[i].CreatedAt))
				updatedAtStr := convertTimeToMySQLFormat(string(records[i].UpdatedAt))
				updates = append(updates, timestampUpdateItem{
					primaryKey: fmt.Sprintf("%s", id),
					createdAt:  createdAtStr,
					updatedAt:  updatedAtStr,
				})
			}
		}
		if len(updates) > 0 {
			if err = r.updateTimestampsByPrimaryKey(kt, table.ZiyanCvmDeliverRecordTable, "id", updates); err != nil {
				logs.Warnf("[MigrationEngine:Migrate] createDeliverRecord update timestamps failed, err: %v, rid: %s",
					err, kt.Rid)
				// 不返回错误，因为数据已经创建成功，只是时间未更新
			}
		}
	}

	return nil
}

// createModifyRecord 创建变更记录（支持数据迁移时间保留）
func (r *ClientAPIRouter) createModifyRecord(kt *kit.Kit, batch []interface{}) error {
	records := make([]cvmapplyproto.ZiyanCvmModifyRecordCreateReq, 0, len(batch))
	hasCreatedAt := false

	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmModifyRecordCreateReq); ok {
			records = append(records, *req)
			// 检查是否有需要保留的创建时间
			if len(req.CreatedAt) > 0 {
				hasCreatedAt = true
			}
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmModifyRecordTable)
		}
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmModifyRecordReq{Records: records}
	result, err := r.client.ZiyanCvmModifyRecord.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("[MigrationEngine:Migrate] createModifyRecord:BatchCreate, err: %v, hasCreatedAt: %v, rid: %s",
			err, hasCreatedAt, kt.Rid)
		return err
	}

	// 如果有需要保留的创建时间，创建后更新created_at和updated_at字段
	if hasCreatedAt && result != nil && len(result.IDs) > 0 {
		updates := make([]timestampUpdateItem, 0, len(records))
		for i, id := range result.IDs {
			if len(records[i].CreatedAt) > 0 {
				createdAtStr := convertTimeToMySQLFormat(string(records[i].CreatedAt))
				updatedAtStr := convertTimeToMySQLFormat(string(records[i].UpdatedAt))
				updates = append(updates, timestampUpdateItem{
					primaryKey: fmt.Sprintf("%s", id),
					createdAt:  createdAtStr,
					updatedAt:  updatedAtStr,
				})
			}
		}
		if len(updates) > 0 {
			if err = r.updateTimestampsByPrimaryKey(kt, table.ZiyanCvmModifyRecordTable, "id", updates); err != nil {
				logs.Warnf("[MigrationEngine:Migrate] createModifyRecord update timestamps failed, err: %v, rid: %s",
					err, kt.Rid)
				// 不返回错误，因为数据已经创建成功，只是时间未更新
			}
		}
	}

	return nil
}

// ========================================
// Update 方法（各表）
// ========================================

// updateApplyOrder 更新申请单主单
func (r *ClientAPIRouter) updateApplyOrder(kt *kit.Kit, batch []interface{}) error {
	orders := make([]cvmapplyproto.ZiyanCvmApplyOrderUpdateReq, 0, len(batch))
	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmApplyOrderUpdateReq); ok {
			orders = append(orders, *req)
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmApplyOrderTable)
		}
	}

	req := &cvmapplyproto.BatchUpdateZiyanCvmApplyOrderReq{ApplyOrders: orders}
	return r.client.ZiyanCvmApplyOrder.BatchUpdate(kt.Ctx, kt.Header(), req)
}

// updateApplySuborder 更新申请单子单
func (r *ClientAPIRouter) updateApplySuborder(kt *kit.Kit, batch []interface{}) error {
	suborders := make([]cvmapplyproto.ZiyanCvmApplySuborderUpdateReq, 0, len(batch))
	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmApplySuborderUpdateReq); ok {
			suborders = append(suborders, *req)
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmApplySuborderTable)
		}
	}

	req := &cvmapplyproto.BatchUpdateZiyanCvmApplySuborderReq{ApplySuborders: suborders}
	return r.client.ZiyanCvmApplySuborder.BatchUpdate(kt.Ctx, kt.Header(), req)
}

// updateApplyStep 更新申请单执行步骤
func (r *ClientAPIRouter) updateApplyStep(kt *kit.Kit, batch []interface{}) error {
	steps := make([]cvmapplyproto.ZiyanCvmApplyStepUpdateReq, 0, len(batch))
	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmApplyStepUpdateReq); ok {
			steps = append(steps, *req)
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmApplyStepTable)
		}
	}

	req := &cvmapplyproto.BatchUpdateZiyanCvmApplyStepReq{ApplySteps: steps}
	return r.client.ZiyanCvmApplyStep.BatchUpdate(kt.Ctx, kt.Header(), req)
}

// updateGenerateRecord 更新生产任务记录
func (r *ClientAPIRouter) updateGenerateRecord(kt *kit.Kit, batch []interface{}) error {
	records := make([]cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq, 0, len(batch))
	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq); ok {
			records = append(records, *req)
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmGenerateRecordTable)
		}
	}

	req := &cvmapplyproto.BatchUpdateZiyanCvmGenerateRecordReq{GenerateRecords: records}
	return r.client.ZiyanCvmGenerateRecord.BatchUpdate(kt.Ctx, kt.Header(), req)
}

// updateApplyInitTask 更新初始化任务记录
func (r *ClientAPIRouter) updateApplyInitTask(kt *kit.Kit, batch []interface{}) error {
	tasks := make([]cvmapplyproto.ZiyanCvmApplyInitTaskUpdateReq, 0, len(batch))
	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmApplyInitTaskUpdateReq); ok {
			tasks = append(tasks, *req)
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmApplyInitTaskTable)
		}
	}

	req := &cvmapplyproto.BatchUpdateZiyanCvmApplyInitTaskReq{InitTasks: tasks}
	return r.client.ZiyanCvmApplyInitTask.BatchUpdate(kt.Ctx, kt.Header(), req)
}

// updateDeviceInfo 更新设备交付记录
func (r *ClientAPIRouter) updateDeviceInfo(kt *kit.Kit, batch []interface{}) error {
	devices := make([]cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq, 0, len(batch))
	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq); ok {
			devices = append(devices, *req)
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmDeviceInfoTable)
		}
	}

	req := &cvmapplyproto.BatchUpdateZiyanCvmDeviceInfoReq{Devices: devices}
	return r.client.ZiyanCvmDeviceInfo.BatchUpdate(kt.Ctx, kt.Header(), req)
}

// updateDeliverRecord 更新交付任务记录
func (r *ClientAPIRouter) updateDeliverRecord(kt *kit.Kit, batch []interface{}) error {
	records := make([]cvmapplyproto.ZiyanCvmDeliverRecordUpdateReq, 0, len(batch))
	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmDeliverRecordUpdateReq); ok {
			records = append(records, *req)
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmDeliverRecordTable)
		}
	}

	req := &cvmapplyproto.BatchUpdateZiyanCvmDeliverRecordReq{Records: records}
	return r.client.ZiyanCvmDeliverRecord.BatchUpdate(kt.Ctx, kt.Header(), req)
}

// updateModifyRecord 更新变更记录
func (r *ClientAPIRouter) updateModifyRecord(kt *kit.Kit, batch []interface{}) error {
	records := make([]cvmapplyproto.ZiyanCvmModifyRecordUpdateReq, 0, len(batch))
	for _, item := range batch {
		if req, ok := item.(*cvmapplyproto.ZiyanCvmModifyRecordUpdateReq); ok {
			records = append(records, *req)
		} else {
			return fmt.Errorf("invalid item type for %s", table.ZiyanCvmModifyRecordTable)
		}
	}

	req := &cvmapplyproto.BatchUpdateZiyanCvmModifyRecordReq{Records: records}
	return r.client.ZiyanCvmModifyRecord.BatchUpdate(kt.Ctx, kt.Header(), req)
}

// ========================================
// List 方法（各表）
// ========================================

// listApplyOrder 查询申请单主单
func (r *ClientAPIRouter) listApplyOrder(kt *kit.Kit, filterExpr *filter.Expression) ([]interface{}, error) {
	results := make([]interface{}, 0)
	req := &cvmapplyproto.ZiyanCvmApplyOrderListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}
	for {
		resp, err := r.client.ZiyanCvmApplyOrder.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, detail := range resp.Details {
			results = append(results, detail)
		}
		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}
	return results, nil
}

// listApplySuborder 查询申请单子单
func (r *ClientAPIRouter) listApplySuborder(kt *kit.Kit, filterExpr *filter.Expression) ([]interface{}, error) {
	results := make([]interface{}, 0)
	req := &cvmapplyproto.ZiyanCvmApplySuborderListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}
	for {
		resp, err := r.client.ZiyanCvmApplySuborder.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, detail := range resp.Details {
			results = append(results, detail)
		}
		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}
	return results, nil
}

// listApplyStep 查询申请单执行步骤
func (r *ClientAPIRouter) listApplyStep(kt *kit.Kit, filterExpr *filter.Expression) ([]interface{}, error) {
	results := make([]interface{}, 0)
	req := &cvmapplyproto.ZiyanCvmApplyStepListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}
	for {
		resp, err := r.client.ZiyanCvmApplyStep.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, detail := range resp.Details {
			results = append(results, detail)
		}
		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}
	return results, nil
}

// listGenerateRecord 查询生产任务记录
func (r *ClientAPIRouter) listGenerateRecord(kt *kit.Kit, filterExpr *filter.Expression) ([]interface{}, error) {
	results := make([]interface{}, 0)
	req := &cvmapplyproto.ZiyanCvmGenerateRecordListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}
	for {
		resp, err := r.client.ZiyanCvmGenerateRecord.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, detail := range resp.Details {
			results = append(results, detail)
		}
		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}
	return results, nil
}

// listApplyInitTask 查询初始化任务记录
func (r *ClientAPIRouter) listApplyInitTask(kt *kit.Kit, filterExpr *filter.Expression) ([]interface{}, error) {
	results := make([]interface{}, 0)
	req := &cvmapplyproto.ZiyanCvmApplyInitTaskListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}
	for {
		resp, err := r.client.ZiyanCvmApplyInitTask.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, detail := range resp.Details {
			results = append(results, detail)
		}
		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}
	return results, nil
}

// listDeviceInfo 查询设备交付记录
func (r *ClientAPIRouter) listDeviceInfo(kt *kit.Kit, filterExpr *filter.Expression) ([]interface{}, error) {
	results := make([]interface{}, 0)
	req := &cvmapplyproto.ZiyanCvmDeviceInfoListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}
	for {
		resp, err := r.client.ZiyanCvmDeviceInfo.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, detail := range resp.Details {
			results = append(results, detail)
		}
		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}
	return results, nil
}

// listDeliverRecord 查询交付任务记录
func (r *ClientAPIRouter) listDeliverRecord(kt *kit.Kit, filterExpr *filter.Expression) ([]interface{}, error) {
	results := make([]interface{}, 0)
	req := &cvmapplyproto.ZiyanCvmDeliverRecordListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}
	for {
		resp, err := r.client.ZiyanCvmDeliverRecord.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, detail := range resp.Details {
			results = append(results, detail)
		}
		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}
	return results, nil
}

// listModifyRecord 查询变更记录
func (r *ClientAPIRouter) listModifyRecord(kt *kit.Kit, filterExpr *filter.Expression) ([]interface{}, error) {
	results := make([]interface{}, 0)
	req := &cvmapplyproto.ZiyanCvmModifyRecordListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}
	for {
		resp, err := r.client.ZiyanCvmModifyRecord.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, detail := range resp.Details {
			results = append(results, detail)
		}
		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}
	return results, nil
}

// ========================================
// 辅助方法
// ========================================

// splitIntoBatches 分批
func (r *ClientAPIRouter) splitIntoBatches(items []interface{}) [][]interface{} {
	return r.splitIntoBatchesWithLimit(items, r.maxBatchSize)
}

// splitIntoBatchesWithLimit 按指定大小分批
func (r *ClientAPIRouter) splitIntoBatchesWithLimit(items []interface{}, batchSize int) [][]interface{} {
	if len(items) == 0 {
		return [][]interface{}{}
	}

	batches := make([][]interface{}, 0)

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}

// timestampUpdateItem 时间戳更新项（支持不同类型的主键）
type timestampUpdateItem struct {
	primaryKey string // 主键值（字符串形式，支持uint64和string类型）
	createdAt  string // 创建时间（MySQL DATETIME格式：YYYY-MM-DD HH:MM:SS）
	updatedAt  string // 更新时间（MySQL DATETIME格式：YYYY-MM-DD HH:MM:SS）
}

// compositeKeyTimestampUpdateItem 联合主键时间戳更新项
type compositeKeyTimestampUpdateItem struct {
	primaryKeys map[string]string // 联合主键的字段名和值的映射，如 {"suborder_id": "xxx", "generate_id": "yyy"}
	createdAt   string            // 创建时间（MySQL DATETIME格式：YYYY-MM-DD HH:MM:SS）
	updatedAt   string            // 更新时间（MySQL DATETIME格式：YYYY-MM-DD HH:MM:SS）
}

// convertTimeToMySQLFormat 将types.Time（ISO 8601格式）转换为MySQL DATETIME格式
func convertTimeToMySQLFormat(timeStr string) string {
	if len(timeStr) == 0 {
		return ""
	}

	// types.Time是ISO 8601格式，如：2024-01-15T10:30:00+08:00
	// 需要转换为MySQL格式：2024-01-15 10:30:00
	// 尝试解析ISO 8601格式
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		// 如果解析失败，尝试其他格式
		t, err = time.Parse(constant.TimeStdFormat, timeStr)
		if err != nil {
			// 如果还是失败，尝试直接使用（可能已经是MySQL格式）
			if strings.Contains(timeStr, "T") {
				// 包含T，可能是ISO格式，尝试简单替换
				timeStr = strings.ReplaceAll(timeStr, "T", " ")
				// 移除时区信息
				if idx := strings.Index(timeStr, "+"); idx > 0 {
					timeStr = timeStr[:idx]
				}
				if idx := strings.Index(timeStr, "-"); idx > 10 { // 避免误删日期中的-
					timeStr = timeStr[:idx]
				}
				return strings.TrimSpace(timeStr)
			}
			// 如果都不行，直接返回（可能是MySQL格式）
			return timeStr
		}
	}

	// 转换为MySQL格式
	return t.Format(constant.DateTimeLayout)
}

// updateTimestampsByPrimaryKey 通用的批量更新created_at和updated_at方法（支持不同表的主键）
// tableName: 表名
// primaryKeyField: 主键字段名（如"order_id", "suborder_id"等）
// updates: 更新项列表
func (r *ClientAPIRouter) updateTimestampsByPrimaryKey(kt *kit.Kit, tableName table.Name,
	primaryKeyField string, updates []timestampUpdateItem) error {

	if len(updates) == 0 {
		return nil
	}

	if r.orm == nil {
		logs.Warnf("[updateTimestampsByPrimaryKey] orm is nil, skip update timestamps, table: %s, rid: %s",
			tableName, kt.Rid)
		return nil
	}

	// 分批更新，每批100条
	batchSize := 100
	if len(updates) > batchSize {
		for i := 0; i < len(updates); i += batchSize {
			end := i + batchSize
			if end > len(updates) {
				end = len(updates)
			}
			if err := r.executeBatchUpdateTimestamps(kt, tableName, primaryKeyField, updates[i:end]); err != nil {
				return fmt.Errorf("batch update timestamps failed (batch %d-%d): %w", i, end, err)
			}
		}
	} else {
		if err := r.executeBatchUpdateTimestamps(kt, tableName, primaryKeyField, updates); err != nil {
			return fmt.Errorf("update timestamps failed: %w", err)
		}
	}

	logs.Infof("[updateTimestampsByPrimaryKey] successfully updated timestamps for %d records, "+
		"table: %s, primaryKey: %s, rid: %s", len(updates), tableName, primaryKeyField, kt.Rid)
	return nil
}

// executeBatchUpdateTimestamps 执行批量更新created_at和updated_at（使用参数化查询逐条更新）
func (r *ClientAPIRouter) executeBatchUpdateTimestamps(kt *kit.Kit, tableName table.Name,
	primaryKeyField string, updates []timestampUpdateItem) error {

	if len(updates) == 0 {
		return nil
	}

	// 使用参数化查询逐条更新，避免SQL注入风险
	successCount := 0
	for _, item := range updates {
		// 构建参数化SQL: UPDATE table_name SET created_at = :created_at, updated_at = :updated_at WHERE primary_key_field = :pk
		sql := fmt.Sprintf("UPDATE %s SET created_at = :created_at, updated_at = :updated_at WHERE %s = :pk",
			tableName, primaryKeyField)

		// 构建参数
		args := map[string]interface{}{
			"created_at": item.createdAt,
			"updated_at": item.updatedAt,
			"pk":         item.primaryKey,
		}

		// 执行参数化更新
		affected, err := r.orm.Do().Update(kt.Ctx, sql, args)
		if err != nil {
			logs.Errorf("[MigrationEngine:Migrate] executeBatchUpdateTimestamps SQL execution failed, "+
				"table: %s, pk: %s, err: %v, rid: %s", tableName, item.primaryKey, err, kt.Rid)
			return fmt.Errorf("execute SQL failed for pk=%s, err: %w", item.primaryKey, err)
		}

		if affected > 0 {
			successCount++
		}
	}

	logs.Infof("[MigrationEngine:Migrate] executeBatchUpdateTimestamps successfully updated %d/%d records, "+
		"table: %s, primaryKey: %s, rid: %s", successCount, len(updates), tableName, primaryKeyField, kt.Rid)
	return nil
}

// updateTimestampsByCompositeKey 批量更新联合主键表的created_at和updated_at字段
func (r *ClientAPIRouter) updateTimestampsByCompositeKey(kt *kit.Kit, tableName table.Name,
	primaryKeyFields []string, updates []compositeKeyTimestampUpdateItem) error {

	if len(updates) == 0 {
		return nil
	}

	if r.orm == nil {
		logs.Warnf("[updateTimestampsByCompositeKey] orm is nil, skip update timestamps, table: %s, rid: %s",
			tableName, kt.Rid)
		return nil
	}

	// 分批更新，每批100条
	batchSize := 100
	if len(updates) > batchSize {
		for i := 0; i < len(updates); i += batchSize {
			end := i + batchSize
			if end > len(updates) {
				end = len(updates)
			}
			if err := r.executeBatchUpdateTimestampsByCompositeKey(
				kt, tableName, primaryKeyFields, updates[i:end]); err != nil {
				return fmt.Errorf("batch update timestamps failed (batch %d-%d): %w", i, end, err)
			}
		}
	} else {
		if err := r.executeBatchUpdateTimestampsByCompositeKey(kt, tableName, primaryKeyFields, updates); err != nil {
			return fmt.Errorf("update timestamps failed: %w", err)
		}
	}

	logs.Infof("[updateTimestampsByCompositeKey] successfully updated timestamps for %d records, "+
		"table: %s, primaryKeys: %v, rid: %s", len(updates), tableName, primaryKeyFields, kt.Rid)
	return nil
}

// executeBatchUpdateTimestampsByCompositeKey 执行批量更新联合主键表的created_at和updated_at（使用参数化查询逐条更新）
func (r *ClientAPIRouter) executeBatchUpdateTimestampsByCompositeKey(kt *kit.Kit, tableName table.Name,
	primaryKeyFields []string, updates []compositeKeyTimestampUpdateItem) error {

	if len(updates) == 0 {
		return nil
	}

	// 使用参数化查询逐条更新，避免SQL注入风险
	successCount := 0
	for _, item := range updates {
		// 构建参数化SQL: UPDATE table_name SET created_at = :created_at, updated_at = :updated_at
		// WHERE field1 = :field1 AND field2 = :field2 ...
		var whereConditions []string
		for _, field := range primaryKeyFields {
			whereConditions = append(whereConditions, fmt.Sprintf("%s = :%s", field, field))
		}
		sql := fmt.Sprintf("UPDATE %s SET created_at = :created_at, updated_at = :updated_at WHERE %s",
			tableName, strings.Join(whereConditions, " AND "))

		// 构建参数
		args := map[string]interface{}{
			"created_at": item.createdAt,
			"updated_at": item.updatedAt,
		}
		// 添加联合主键字段的值
		for _, field := range primaryKeyFields {
			args[field] = item.primaryKeys[field]
		}

		// 执行参数化更新
		affected, err := r.orm.Do().Update(kt.Ctx, sql, args)
		if err != nil {
			logs.Errorf("[MigrationEngine:Migrate] executeBatchUpdateTimestampsByCompositeKey SQL execution failed, "+
				"table: %s, keys: %v, err: %v, rid: %s", tableName, item.primaryKeys, err, kt.Rid)
			return fmt.Errorf("execute SQL failed for keys=%v, err: %w", item.primaryKeys, err)
		}

		if affected > 0 {
			successCount++
		}
	}

	logs.Infof("[MigrationEngine:Migrate] executeBatchUpdateTimestampsByCompositeKey successfully updated %d/%d records, "+
		"table: %s, primaryKeys: %v, rid: %s", successCount, len(updates), tableName, primaryKeyFields, kt.Rid)
	return nil
}

// buildInFilter 构建 IN 查询条件（单字段主键）
func (r *ClientAPIRouter) buildInFilter(field string, values []interface{}) *filter.Expression {
	// 提取实际的值（处理 map[string]interface{} 格式）
	actualValues := make([]interface{}, 0, len(values))
	for _, v := range values {
		// 如果是 map 类型（如 {"id": "1"}），提取对应字段的值
		if pkMap, ok := v.(map[string]interface{}); ok {
			if actualValue, exists := pkMap[field]; exists {
				actualValues = append(actualValues, actualValue)
			} else {
				logs.Warnf("[buildInFilter] field '%s' not found in pk map: %+v", field, pkMap)
			}
		} else {
			// 否则直接使用该值
			actualValues = append(actualValues, v)
		}
	}

	return &filter.Expression{
		Op: filter.And,
		Rules: []filter.RuleFactory{
			&filter.AtomRule{
				Field: field,
				Op:    filter.OpFactory(filter.In),
				Value: actualValues,
			},
		},
	}
}

// buildCompositeKeyFilter 构建复合主键查询条件
// 例如: (suborder_id = "xxx" AND step_name = "yyy") OR (suborder_id = "aaa" AND step_name = "bbb")
func (r *ClientAPIRouter) buildCompositeKeyFilter(fields []string, pkValues []interface{}) *filter.Expression {
	orRules := make([]filter.RuleFactory, 0, len(pkValues))

	for _, pkValue := range pkValues {
		// 每个主键值应该是一个 map[string]interface{}
		pkMap, ok := pkValue.(map[string]interface{})
		if !ok {
			logs.Errorf("[buildCompositeKeyFilter] invalid pk value type: %T, expected map[string]interface{}", pkValue)
			continue
		}

		// 构建 AND 条件：field1 = value1 AND field2 = value2 AND ...
		andRules := make([]filter.RuleFactory, 0, len(fields))
		for _, field := range fields {
			if value, exists := pkMap[field]; exists {
				andRules = append(andRules, &filter.AtomRule{
					Field: field,
					Op:    filter.OpFactory(filter.Equal),
					Value: value,
				})
			}
		}

		if len(andRules) > 0 {
			orRules = append(orRules, &filter.Expression{
				Op:    filter.And,
				Rules: andRules,
			})
		}
	}

	// 用 OR 连接所有的 AND 条件
	return &filter.Expression{
		Op:    filter.Or,
		Rules: orRules,
	}
}

// ========================================
// 多表写入辅助方法
// ========================================

// handleMultiTableCreate 处理多表创建请求
func (r *ClientAPIRouter) handleMultiTableCreate(kt *kit.Kit, batch []interface{}) error {
	// 分离两个表的请求
	suborderBatch := make([]interface{}, 0, len(batch))
	generateRecordBatch := make([]interface{}, 0, len(batch))

	for _, item := range batch {
		multiReq, ok := item.(*converters.MultiTableCreateRequest)
		if !ok {
			return fmt.Errorf("invalid multi-table create request type: %T", item)
		}

		if multiReq.SuborderRequest != nil {
			suborderBatch = append(suborderBatch, multiReq.SuborderRequest)
		}
		if multiReq.GenerateRecordRequest != nil {
			generateRecordBatch = append(generateRecordBatch, multiReq.GenerateRecordRequest)
		}
	}

	// 先创建子单记录
	if len(suborderBatch) > 0 {
		if err := r.createApplySuborder(kt, suborderBatch); err != nil {
			logs.Errorf("[handleMultiTableCreate:createApplySuborder] create cvm apply suborder failed, err: %v, "+
				"suborderBatch: %+v, rid: %s", err, suborderBatch, kt.Rid)
			return fmt.Errorf("create cvm apply suborder failed, err: %w, suborderBatch: %+v", err, suborderBatch)
		}
	}

	// 再创建生产记录
	if len(generateRecordBatch) > 0 {
		if err := r.createGenerateRecord(kt, generateRecordBatch); err != nil {
			logs.Errorf("[handleMultiTableCreate:createGenerateRecord] create cvm generate_record failed, err: %v, "+
				"rid: %s", err, kt.Rid)
			return fmt.Errorf("create cvm generate_record failed, err: %w", err)
		}
	}

	return nil
}

// handleMultiTableUpdate 处理多表更新请求
func (r *ClientAPIRouter) handleMultiTableUpdate(kt *kit.Kit, batch []interface{}) error {
	// 分离两个表的请求
	suborderBatch, generateRecordTemplates, suborderIDs, err := r.separateMultiTableUpdateRequests(batch)
	if err != nil {
		return err
	}

	// 先更新子单记录
	if err = r.updateSuborderBatch(kt, suborderBatch); err != nil {
		return err
	}

	// 更新生产记录
	if err = r.updateGenerateRecordBatch(kt, generateRecordTemplates, suborderIDs); err != nil {
		return err
	}

	return nil
}

// separateMultiTableUpdateRequests 分离多表更新请求
func (r *ClientAPIRouter) separateMultiTableUpdateRequests(batch []interface{}) (
	[]interface{}, []*cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq, []string, error) {

	suborderBatch := make([]interface{}, 0, len(batch))
	generateRecordTemplates := make([]*cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq, 0, len(batch))
	suborderIDs := make([]string, 0, len(batch))

	for _, item := range batch {
		multiReq, ok := item.(*converters.MultiTableUpdateRequest)
		if !ok {
			return nil, nil, nil, fmt.Errorf("invalid multi-table update request type: %T", item)
		}

		if multiReq.SuborderRequest != nil {
			suborderBatch = append(suborderBatch, multiReq.SuborderRequest)
			suborderIDs = append(suborderIDs, multiReq.SuborderRequest.SuborderID)
		}
		if multiReq.GenerateRecordRequest != nil {
			generateRecordTemplates = append(generateRecordTemplates, multiReq.GenerateRecordRequest)
		}
	}

	return suborderBatch, generateRecordTemplates, suborderIDs, nil
}

// updateSuborderBatch 更新子单批次
func (r *ClientAPIRouter) updateSuborderBatch(kt *kit.Kit, suborderBatch []interface{}) error {
	if len(suborderBatch) == 0 {
		return nil
	}

	if err := r.updateApplySuborder(kt, suborderBatch); err != nil {
		return fmt.Errorf("update suborder failed: %w", err)
	}
	return nil
}

// updateGenerateRecordBatch 更新生产记录批次
func (r *ClientAPIRouter) updateGenerateRecordBatch(kt *kit.Kit,
	generateRecordTemplates []*cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq, suborderIDs []string) error {

	if len(generateRecordTemplates) == 0 {
		return nil
	}

	// 查询现有的 generate_record
	existingRecordsMap, err := r.queryGenerateRecordsBySuborderIDs(kt, suborderIDs)
	if err != nil {
		logs.Errorf("[updateGenerateRecordBatch] query generate_record failed: %v, rid: %s", err, kt.Rid)
		return fmt.Errorf("query generate_record failed: %w", err)
	}

	// 构建更新请求
	validBatch := r.buildGenerateRecordUpdateBatch(kt, generateRecordTemplates, suborderIDs, existingRecordsMap)

	// 执行更新
	if len(validBatch) > 0 {
		logs.Infof("[updateGenerateRecordBatch] updating %d generate_record(s), rid: %s", len(validBatch), kt.Rid)
		if err := r.updateGenerateRecord(kt, validBatch); err != nil {
			return fmt.Errorf("update generate_record failed: %w", err)
		}
	}

	return nil
}

// buildGenerateRecordUpdateBatch 构建生产记录更新批次
func (r *ClientAPIRouter) buildGenerateRecordUpdateBatch(kt *kit.Kit,
	templates []*cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq,
	suborderIDs []string,
	existingRecordsMap map[string][]*cvmapplytable.ZiyanCvmGenerateRecord) []interface{} {

	validBatch := make([]interface{}, 0, len(templates)*2)

	for i, template := range templates {
		if i >= len(suborderIDs) {
			break
		}

		suborderID := suborderIDs[i]
		records, found := existingRecordsMap[suborderID]
		if !found || len(records) == 0 {
			logs.Warnf("[buildGenerateRecordUpdateBatch] generate_record not found for suborder_id: %s, rid: %s",
				suborderID, kt.Rid)
			continue
		}

		// 为该 suborder_id 的每个 generate_record 创建更新请求
		for _, record := range records {
			updateReq := &cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq{
				GenerateID: record.GenerateID,
				SuborderID: template.SuborderID,
				TaskID:     template.TaskID,
				TaskLink:   template.TaskLink,
				Status:     template.Status,
				Message:    template.Message,
				TotalNum:   template.TotalNum,
				SuccessNum: template.SuccessNum,
			}
			validBatch = append(validBatch, updateReq)
		}

		logs.Infof("[buildGenerateRecordUpdateBatch] found %d generate_record(s) for suborder_id: %s, rid: %s",
			len(records), suborderID, kt.Rid)
	}

	return validBatch
}

// queryGenerateRecordsBySuborderIDs 根据 suborder_id 批量查询 generate_record（一对多）
func (r *ClientAPIRouter) queryGenerateRecordsBySuborderIDs(kt *kit.Kit, suborderIDs []string) (
	map[string][]*cvmapplytable.ZiyanCvmGenerateRecord, error) {

	if len(suborderIDs) == 0 {
		return make(map[string][]*cvmapplytable.ZiyanCvmGenerateRecord), nil
	}

	// 构建查询条件
	filterExpr := &filter.Expression{
		Op: filter.And,
		Rules: []filter.RuleFactory{
			&filter.AtomRule{
				Field: "suborder_id",
				Op:    filter.OpFactory(filter.In),
				Value: suborderIDs,
			},
		},
	}

	req := &cvmapplyproto.ZiyanCvmGenerateRecordListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}

	resp, err := r.client.ZiyanCvmGenerateRecord.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return nil, err
	}

	// 构建 suborder_id -> []generate_record 的映射（一对多）
	result := make(map[string][]*cvmapplytable.ZiyanCvmGenerateRecord, len(suborderIDs))
	for _, record := range resp.Details {
		result[record.SuborderID] = append(result[record.SuborderID], record)
	}

	return result, nil
}
