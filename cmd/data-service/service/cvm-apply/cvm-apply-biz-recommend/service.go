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

// Package cvmapplybizrecommend ...
package cvmapplybizrecommend

import (
	"fmt"
	"reflect"

	"hcm/pkg/api/core"
	dataproto "hcm/pkg/api/data-service"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"

	"github.com/jmoiron/sqlx"
)

// BatchCreateZiyanCvmApplyBizRecommend batch creates biz recommend records.
func (svc *service) BatchCreateZiyanCvmApplyBizRecommend(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchCreateZiyanCvmApplyBizRecommendReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	ids, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		models := make([]cvmapplytable.ZiyanCvmApplyBizRecommend, 0, len(req.Items))
		for _, item := range req.Items {
			models = append(models, cvmapplytable.ZiyanCvmApplyBizRecommend{
				BkBizID:     item.BkBizID,
				RequireType: item.RequireType,
				Region:      item.Region,
				DeviceType:  item.DeviceType,
				ImageID:     item.ImageID,
				Count:       item.Count,
				Creator:     cts.Kit.User,
				Reviser:     cts.Kit.User,
			})
		}
		return svc.dao.ZiyanCvmApplyBizRecommend().CreateWithTx(cts.Kit, txn, models)
	})
	if err != nil {
		return nil, err
	}

	createdIDs, ok := ids.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create ziyan cvm apply biz recommend but return id type is not []string, "+
			"id type: %v", reflect.TypeOf(ids).String())
	}

	return &core.BatchCreateResult{IDs: createdIDs}, nil
}

// ListZiyanCvmApplyBizRecommend lists biz recommend records.
func (svc *service) ListZiyanCvmApplyBizRecommend(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.ZiyanCvmApplyBizRecommendListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.ListOption{
		Filter: req.Filter,
		Page:   req.Page,
		Fields: req.Fields,
	}
	result, err := svc.dao.ZiyanCvmApplyBizRecommend().List(cts.Kit, opt)
	if err != nil {
		logs.Errorf("list ziyan cvm apply biz recommend failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return result, nil
}

// BatchUpdateZiyanCvmApplyBizRecommend batch updates biz recommend records.
func (svc *service) BatchUpdateZiyanCvmApplyBizRecommend(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchUpdateZiyanCvmApplyBizRecommendReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		for _, item := range req.Items {
			model := &cvmapplytable.ZiyanCvmApplyBizRecommend{
				RequireType: cvt.PtrToVal(item.RequireType),
				Region:      item.Region,
				DeviceType:  item.DeviceType,
				ImageID:     item.ImageID,
				Count:       item.Count,
				Reviser:     cts.Kit.User,
			}
			filterExpr := tools.EqualExpression("id", item.ID)
			if err := svc.dao.ZiyanCvmApplyBizRecommend().UpdateWithTx(cts.Kit, txn, filterExpr, model); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("batch update ziyan cvm apply biz recommend failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// DeleteZiyanCvmApplyBizRecommend deletes biz recommend records.
func (svc *service) DeleteZiyanCvmApplyBizRecommend(cts *rest.Contexts) (interface{}, error) {
	req := new(dataproto.BatchDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.ListOption{
		Filter: req.Filter,
		Page:   core.NewDefaultBasePage(),
	}
	delIDs := make([]string, 0)
	for {
		listResp, err := svc.dao.ZiyanCvmApplyBizRecommend().List(cts.Kit, opt)
		if err != nil {
			logs.Errorf("list ziyan cvm apply biz recommend failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, fmt.Errorf("list ziyan cvm apply biz recommend failed, err: %v", err)
		}
		for _, one := range listResp.Details {
			delIDs = append(delIDs, one.ID)
		}
		if len(listResp.Details) < int(opt.Page.Limit) {
			break
		}
		opt.Page.Start += uint32(len(listResp.Details))
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		for _, batch := range slice.Split(delIDs, int(filter.DefaultMaxInLimit)) {
			delFilter := tools.ContainersExpression("id", batch)
			if err := svc.dao.ZiyanCvmApplyBizRecommend().DeleteWithTx(cts.Kit, txn, delFilter); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("delete ziyan cvm apply biz recommend failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
