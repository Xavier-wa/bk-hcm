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

package woaserver

import (
	types "hcm/cmd/woa-server/types/task"
	woaserver "hcm/pkg/api/woa-server"
	"hcm/pkg/client/common"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// TaskClient task related client
type TaskClient struct {
	client rest.ClientInterface
}

// NewTaskClient return task client instance.
func NewTaskClient(client rest.ClientInterface) *TaskClient {
	return &TaskClient{
		client: client,
	}
}

// StartIdleCheck ...
func (c *TaskClient) StartIdleCheck(kt *kit.Kit, request *woaserver.StartIdleCheckReq) (
	*woaserver.StartIdleCheckRsp, error) {
	return common.Request[woaserver.StartIdleCheckReq, woaserver.StartIdleCheckRsp](c.client, rest.POST, kt, request,
		"/task/start/cvms/idle_check")
}

// CheckBizApplyOrder 提单前只读校验（业务视角），与人工提单同源校验但只校验、不落库、不建单。
// 返回 CheckApplyOrderResp{Pass, Reason}：业务不通过 Pass=false 并携带详细原因；系统异常以 error 返回。
func (c *TaskClient) CheckBizApplyOrder(kt *kit.Kit, bizID int64, request *types.ApplyReq) (
	*types.CheckApplyOrderResp, error) {

	return common.Request[types.ApplyReq, types.CheckApplyOrderResp](c.client, rest.POST, kt, request,
		"/bizs/%d/task/check/apply", bizID)
}

// ListDetectTask ...
func (c *TaskClient) ListDetectTask(kt *kit.Kit, request *types.GetRecycleDetectReq) (*types.GetDetectTaskRst, error) {
	return common.Request[types.GetRecycleDetectReq, types.GetDetectTaskRst](c.client, rest.POST, kt, request,
		"/task/list/detect/task")
}

// ListDetectStep ...
func (c *TaskClient) ListDetectStep(kt *kit.Kit, request *types.GetDetectStepReq) (*types.GetDetectStepRst, error) {
	return common.Request[types.GetDetectStepReq, types.GetDetectStepRst](c.client, rest.POST, kt, request,
		"/task/findmany/recycle/detect/step")
}

// ListRecycleOrder ...
func (c *TaskClient) ListRecycleOrder(kt *kit.Kit, request *types.GetRecycleOrderReq) (*types.GetRecycleOrderRst, error) {
	return common.Request[types.GetRecycleOrderReq, types.GetRecycleOrderRst](c.client, rest.POST, kt, request,
		"/task/findmany/recycle/order")
}
