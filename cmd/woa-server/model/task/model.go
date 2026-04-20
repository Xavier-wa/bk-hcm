/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package model

import (
	"context"
	"sync"

	daltypes "hcm/cmd/woa-server/storage/dal/types"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/client"
	"hcm/pkg/kit"
	"hcm/pkg/runtime/filter"
)

// model all model operation interface
type model struct {
	applyTicket    ApplyTicket
	applyOrder     ApplyOrder
	applyStep      ApplyStep
	generateRecord GenerateRecord
	initRecord     InitRecord
	deliverRecord  DeliverRecord
	deviceInfo     DeviceInfo
}

// ApplyTicket get apply ticket operation interface
func (m *model) ApplyTicket() ApplyTicket {
	return m.applyTicket
}

// ApplyOrder get apply order operation interface
func (m *model) ApplyOrder() ApplyOrder {
	return m.applyOrder
}

// ApplyStep get apply step operation interface
func (m *model) ApplyStep() ApplyStep {
	return m.applyStep
}

// GenerateRecord get apply generate record operation interface
func (m *model) GenerateRecord() GenerateRecord {
	return m.generateRecord
}

// InitRecord get apply init record operation interface
func (m *model) InitRecord() InitRecord {
	return m.initRecord
}

// DeliverRecord get apply deliver record operation interface
func (m *model) DeliverRecord() DeliverRecord {
	return m.deliverRecord
}

// DeviceInfo get device info operation interface
func (m *model) DeviceInfo() DeviceInfo {
	return m.deviceInfo
}

var (
	operation     *model
	operationOnce sync.Once
)

func newOperation(apiClientSet *client.ClientSet) *model {
	return &model{
		applyTicket:    &applyTicket{apiClientSet: apiClientSet},
		applyOrder:     &applyOrder{apiClientSet: apiClientSet},
		applyStep:      &applyStep{apiClientSet: apiClientSet},
		generateRecord: &generateRecord{apiClientSet: apiClientSet},
		initRecord:     &initRecord{apiClientSet: apiClientSet},
		deliverRecord:  &deliverRecord{apiClientSet: apiClientSet},
		deviceInfo:     &deviceInfo{apiClientSet: apiClientSet},
	}
}

// InitOperation initializes task model operation singleton.
// It only initializes once; subsequent calls are no-ops.
func InitOperation(apiClientSet *client.ClientSet) {
	if apiClientSet == nil {
		return
	}

	operationOnce.Do(func() {
		operation = newOperation(apiClientSet)
	})
}

// Operation returns task model operation singleton.
// It supports both Operation() and Operation(apiClientSet).
// If not initialized yet, it returns a temporary empty model instead of nil to avoid panic.
func Operation(apiClientSet ...*client.ClientSet) *model {
	if len(apiClientSet) > 0 {
		InitOperation(apiClientSet[0])
	}

	if operation != nil {
		return operation
	}

	// Not initialized yet, return a temporary empty model to avoid nil pointer panic.
	// The caller should call InitOperation before using Operation for full functionality.
	return newOperation(nil)
}

// Model provides storage interface for operations of models
type Model interface {
	ApplyTicket() ApplyTicket
	ApplyOrder() ApplyOrder
	ApplyStep() ApplyStep
	GenerateRecord() GenerateRecord
	InitRecord() InitRecord
	DeliverRecord() DeliverRecord
	DeviceInfo() DeviceInfo
}

// ApplyTicket apply ticket operation interface
type ApplyTicket interface {
	// CreateApplyTicket creates apply ticket in db
	CreateApplyTicket(kt *kit.Kit, inst *types.ApplyTicket) (uint64, error)
	// GetApplyTicket gets apply ticket by filter from db
	GetApplyTicket(kt *kit.Kit, filterExpr *filter.Expression) (
		*types.ApplyTicket, error)
	// CountApplyTicket gets apply ticket count by filter from db
	CountApplyTicket(kt *kit.Kit, filter *filter.Expression) (uint64, error)
	// FindManyApplyTicket gets apply ticket list by filter from db
	FindManyApplyTicket(kt *kit.Kit, filterExpr *filter.Expression,
		page *core.BasePage) ([]*types.ApplyTicket, error)
	// UpdateApplyTicket updates apply ticket by filter and doc in db
	UpdateApplyTicket(kt *kit.Kit, filterExpr *filter.Expression,
		updateData *cvmapplyproto.ZiyanCvmApplyOrderUpdateReq) error
	// AggregateAll apply ticket aggregate all operation
	AggregateAll(ctx context.Context, pipeline interface{}, result interface{}, opts ...*daltypes.AggregateOpts) error
}

// ApplyOrder apply order operation interface
type ApplyOrder interface {
	// CreateApplyOrder creates apply order in db
	CreateApplyOrder(kt *kit.Kit, inst *types.ApplyOrder) error
	// GetApplyOrder gets apply order by filter from db
	GetApplyOrder(kt *kit.Kit, filterExpr *filter.Expression) (*types.ApplyOrder, error)
	// CountApplyOrder gets apply order count by filter from db
	CountApplyOrder(kt *kit.Kit, filterExpr *filter.Expression) (uint64, error)
	// FindManyApplyOrder gets apply order list by filter from db
	FindManyApplyOrder(kt *kit.Kit, filterExpr *filter.Expression,
		page *core.BasePage) ([]*types.ApplyOrder, error)
	// UpdateApplyOrder updates apply order by filter and doc in db
	UpdateApplyOrder(kt *kit.Kit, filterExpr *filter.Expression,
		updateData *cvmapplyproto.ZiyanCvmApplySuborderUpdateReq) error
	// AggregateAll apply order aggregate all operation
	AggregateAll(ctx context.Context, pipeline interface{}, result interface{}, opts ...*daltypes.AggregateOpts) error
}

// ApplyStep apply step operation interface
type ApplyStep interface {
	// CreateApplyStep creates apply step in db
	CreateApplyStep(kt *kit.Kit, inst *types.ApplyStep) error
	// GetApplyStep gets apply order by filter from db
	GetApplyStep(kt *kit.Kit, filterExpr *filter.Expression) (*types.ApplyStep, error)
	// CountApplyStep gets apply step count by filter from db
	CountApplyStep(kt *kit.Kit, filterExpr *filter.Expression) (uint64, error)
	// FindManyApplyStep gets apply order list by filter from db
	FindManyApplyStep(kt *kit.Kit, filterExpr *filter.Expression) (
		[]*types.ApplyStep, error)
	// UpdateApplyStep updates apply order by filter and doc in db
	UpdateApplyStep(kt *kit.Kit, filterExpr *filter.Expression,
		updateData *cvmapplyproto.ZiyanCvmApplyStepUpdateReq) error
}

// GenerateRecord apply generate record operation interface
type GenerateRecord interface {
	// CreateGenerateRecord creates apply order generate record in db
	CreateGenerateRecord(kt *kit.Kit, inst *types.GenerateRecord) (string, error)
	// GetGenerateRecord gets apply order generate record by filter from db
	GetGenerateRecord(kt *kit.Kit, filterExpr *filter.Expression) (
		*types.GenerateRecord, error)
	// CountGenerateRecord gets apply order generate record count by filter from db
	CountGenerateRecord(kt *kit.Kit, filterExpr *filter.Expression) (uint64, error)
	// FindManyGenerateRecord gets generate record list by filter from db
	FindManyGenerateRecord(kt *kit.Kit, filterExpr *filter.Expression,
		page *core.BasePage) ([]*types.GenerateRecord, error)
	// UpdateGenerateRecord updates apply order generate record by filter and doc in db
	UpdateGenerateRecord(kt *kit.Kit, filterExpr *filter.Expression,
		updateData *cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq) error
	// AggregateAll generate record aggregate all operation
	AggregateAll(ctx context.Context, pipeline interface{}, result interface{}, opts ...*daltypes.AggregateOpts) error
}

// InitRecord apply init record operation interface
type InitRecord interface {
	// CreateInitRecord creates apply order init record in db
	CreateInitRecord(kt *kit.Kit, inst *types.InitRecord) error
	// GetInitRecord gets apply order init record by filter from db
	GetInitRecord(kt *kit.Kit, filterExpr *filter.Expression) (*types.InitRecord, error)
	// CountInitRecord gets apply order init record count by filter from db
	CountInitRecord(kt *kit.Kit, filterExpr *filter.Expression) (uint64, error)
	// FindManyInitRecord gets init record list by filter from db
	FindManyInitRecord(kt *kit.Kit, filterExpr *filter.Expression,
		page *core.BasePage) ([]*types.InitRecord, error)
	// UpdateInitRecord updates apply order init record by filter and doc in db
	UpdateInitRecord(kt *kit.Kit, filterExpr *filter.Expression,
		updateData *cvmapplyproto.ZiyanCvmApplyInitTaskUpdateReq) error
}

// DeliverRecord apply deliver record operation interface
type DeliverRecord interface {
	// CreateDeliverRecord creates apply order deliver record in db
	CreateDeliverRecord(kt *kit.Kit, inst *types.DeliverRecord) error
	// GetDeliverRecord gets apply order deliver record by filter from db
	GetDeliverRecord(kt *kit.Kit, filterExpr *filter.Expression) (
		*types.DeliverRecord, error)
	// CountDeliverRecord gets apply order deliver record count by filter from db
	CountDeliverRecord(kt *kit.Kit, filterExpr *filter.Expression) (uint64, error)
	// FindManyDeliverRecord gets deliver record list by filter from db
	FindManyDeliverRecord(kt *kit.Kit, filterExpr *filter.Expression,
		page *core.BasePage) ([]*types.DeliverRecord, error)
	// UpdateDeliverRecord updates apply order deliver record by filter and doc in db
	UpdateDeliverRecord(kt *kit.Kit, filterExpr *filter.Expression,
		updateData *cvmapplyproto.ZiyanCvmDeliverRecordUpdateReq) error
}

// DeviceInfo device info operation interface
type DeviceInfo interface {
	// CreateDeviceInfos creates devices info in db
	CreateDeviceInfos(kt *kit.Kit, insts []*types.DeviceInfo) error
	// GetDeviceInfo gets device info by filter from db
	GetDeviceInfo(kt *kit.Kit, filter *filter.Expression) ([]*types.DeviceInfo, error)
	// CountDeviceInfo gets device info count by filter from db
	CountDeviceInfo(kt *kit.Kit, filter *filter.Expression) (uint64, error)
	// FindManyDeviceInfo gets device info list by filter from db
	FindManyDeviceInfo(kt *kit.Kit, filter *filter.Expression,
		page *core.BasePage) ([]*types.DeviceInfo, error)
	// UpdateDeviceInfo updates device info by filter and doc in db
	UpdateDeviceInfo(kt *kit.Kit, filterExpr *filter.Expression,
		updateData *cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq) error
	// AggregateAll device info aggregate all operation
	AggregateAll(ctx context.Context, pipeline interface{}, result interface{}, opts ...*daltypes.AggregateOpts) error
	// Distinct gets device info distinct result from db
	Distinct(ctx context.Context, field string, filter map[string]interface{}) ([]interface{}, error)
}
