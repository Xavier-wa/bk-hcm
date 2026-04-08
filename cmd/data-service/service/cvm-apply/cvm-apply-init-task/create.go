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

// Package cvmapplyinittask ...
package cvmapplyinittask

import (
	"fmt"
	"reflect"

	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	"github.com/jmoiron/sqlx"
)

// BatchCreateZiyanCvmApplyInitTask batch create ziyan cvm apply init task
func (svc *service) BatchCreateZiyanCvmApplyInitTask(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchCreateZiyanCvmApplyInitTaskReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	initTaskIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		tasks := make([]cvmapplytable.ZiyanCvmApplyInitTask, 0, len(req.InitTasks))
		for _, createReq := range req.InitTasks {
			task := cvmapplytable.ZiyanCvmApplyInitTask{
				SuborderID: createReq.SuborderID,
				IP:         createReq.IP,
				TaskID:     createReq.TaskID,
				TaskLink:   createReq.TaskLink,
				Status:     cvt.ValToPtr(createReq.Status),
				Message:    createReq.Message,
				StartAt:    createReq.StartAt,
				EndAt:      createReq.EndAt,
				Creator:    cts.Kit.User,
				Reviser:    cts.Kit.User,
			}
			tasks = append(tasks, task)
		}
		ids, err := svc.dao.ZiyanCvmApplyInitTask().CreateWithTx(cts.Kit, txn, tasks)
		if err != nil {
			return nil, fmt.Errorf("create ziyan cvm apply init task failed, err: %v, tasks: %+v", err, tasks)
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}

	ids, ok := initTaskIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create ziyan cvm apply init task but return id type is not []string, "+
			"id type: %v", reflect.TypeOf(initTaskIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}
