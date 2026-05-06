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

	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

// BatchUpdateZiyanCvmApplyInitTask batch update ziyan cvm apply init task
func (svc *service) BatchUpdateZiyanCvmApplyInitTask(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchUpdateZiyanCvmApplyInitTaskReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		for _, updateReq := range req.InitTasks {
			taskReq := buildUpdateInitTask(updateReq, cts.Kit.User)
			if err := svc.dao.ZiyanCvmApplyInitTask().Update(
				cts.Kit, txn, tools.EqualExpression("id", updateReq.ID), taskReq); err != nil {
				return nil, fmt.Errorf("update ziyan cvm apply init task failed, err: %v, id: %s",
					err, updateReq.ID)
			}
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// buildUpdateInitTask 构建更新请求对象，只设置非零值字段
func buildUpdateInitTask(updateReq cvmapplyproto.ZiyanCvmApplyInitTaskUpdateReq,
	reviser string) *cvmapplytable.ZiyanCvmApplyInitTask {
	taskReq := &cvmapplytable.ZiyanCvmApplyInitTask{
		ID:         updateReq.ID,
		SuborderID: updateReq.SuborderID,
		IP:         updateReq.IP,
		TaskID:     updateReq.TaskID,
		TaskLink:   updateReq.TaskLink,
		Status:     updateReq.Status,
		Message:    updateReq.Message,
		StartAt:    updateReq.StartAt,
		EndAt:      updateReq.EndAt,
		Reviser:    reviser,
	}

	return taskReq
}
