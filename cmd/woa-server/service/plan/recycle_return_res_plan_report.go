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

package plan

import (
	crontask "hcm/cmd/woa-server/task"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// PushRecycleReturnResPlanReport 手动触发销毁返还预测周报
func (s *service) PushRecycleReturnResPlanReport(cts *rest.Contexts) (interface{}, error) {
	req := new(ptypes.PushRecycleReturnResPlanReportReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 权限校验
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.ZiYanResPlan, Action: meta.Update}}
	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		logs.Errorf("authorize failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	task, ok := s.tasks[enumor.CronTaskRecycleReturnResPlanReport]
	if !ok {
		logs.Errorf("failed to find recycle return res plan report task, rid: %s", cts.Kit.Rid)
		return nil, errf.Newf(errf.InvalidParameter, "recycle return res plan report task not found")
	}

	reportTask, ok := task.(*crontask.RecycleReturnResPlanReportTask)
	if !ok {
		logs.Errorf("failed to cast task to RecycleReturnResPlanReportTask, rid: %s", cts.Kit.Rid)
		return nil, errf.Newf(errf.InvalidParameter, "invalid task type")
	}

	resp, err := reportTask.DoWithRange(cts.Kit, req.Start, req.End)
	if err != nil {
		logs.Errorf("failed to push recycle return res plan report, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return resp, nil
}
