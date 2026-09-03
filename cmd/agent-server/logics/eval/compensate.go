/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may obtain a copy of the License at http://opensource.org/licenses/MIT
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

package eval

import (
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// Compensate 启动时捞一次「已终态、无 eval 行」的缺口并 SubmitBatch。
// 回看窗口 CompensateLookback，条数上限 CompensateBatchSize；不做 cron。
func Compensate(kt *kit.Kit, cli *client.ClientSet, disp *Dispatcher) {
	if disp == nil || cli == nil {
		return
	}
	// 启动补偿没有登录用户，kit.New() 缺 User/AppCode，ListGap 会被 data-service 拒掉。
	kt = fillCompensateKit(kt)
	cfg := cc.AgentServer().Eval
	if !cfg.IsEnabled() {
		return
	}
	lookback := cfg.CompensateLookbackDur()
	limit := uint(cfg.CompensateBatchSize)

	countResp, err := cli.DataService().Aiagent.RunEval.ListGap(kt, &dsaiagent.ListAiagentRunEvalGapReq{
		LookbackSec: int64(lookback.Seconds()),
		Count:       true,
		Limit:       limit,
	})
	if err != nil {
		logs.Errorf("compensate count gap failed, err: %v, rid: %s", err, kt.Rid)
		return
	}

	resp, err := cli.DataService().Aiagent.RunEval.ListGap(kt, &dsaiagent.ListAiagentRunEvalGapReq{
		LookbackSec: int64(lookback.Seconds()),
		Count:       false,
		Limit:       limit,
	})
	if err != nil {
		logs.Errorf("compensate list gap failed, err: %v, rid: %s", err, kt.Rid)
		return
	}
	ids := make([]string, 0, len(resp.Details))
	for _, run := range resp.Details {
		ids = append(ids, run.RunID)
	}
	disp.SubmitBatch(kt, ids)
	logs.Infof("compensate submit success, count: %d, queued: %d, rid: %s",
		countResp.Count, len(resp.Details), kt.Rid)
}

// fillCompensateKit fills User/AppCode for a startup kit so DS Validate passes.
func fillCompensateKit(kt *kit.Kit) *kit.Kit {
	if kt == nil {
		kt = kit.New()
	}
	if kt.User == "" {
		kt.User = constant.BackendOperationUserKey
	}
	if kt.AppCode == "" {
		kt.AppCode = constant.BackendOperationAppCodeKey
	}
	if kt.TenantID == "" {
		kt.SetBackendTenantID()
	}
	return kt
}
