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

package task

import (
	"fmt"

	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// CheckBizApplyOrder 提单前只读校验接口（业务视角）。
// 与人工提单 CreateBizApplyOrder 执行同源前置校验，但只校验、不落库、不建 ITSM、无任何副作用。
// 返回结构化结果 CheckApplyOrderResp{Pass, Reason}：业务不通过返回 Pass=false + 详细原因（HTTP 200），
// 供调用方（含大模型）提示用户调整规格或数量；系统异常（DB/CRP/预测服务等调用失败）以 error 返回。
func (s *service) CheckBizApplyOrder(cts *rest.Contexts) (any, error) {
	input := new(types.ApplyReq)
	if err := cts.DecodeInto(input); err != nil {
		logs.Errorf("failed to decode check biz apply order request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, err
	}
	if bkBizID <= 0 {
		return nil, errf.New(errf.InvalidParameter, "biz id is invalid")
	}
	input.BkBizId = bkBizID

	// 结构校验失败属于业务不通过：返回 Pass=false + 详细原因，供调用方提示用户修正参数。
	if err = input.Validate(); err != nil {
		logs.Infof("check biz apply order not passed for invalid input, err: %v, bizID: %d, rid: %s",
			err, bkBizID, cts.Kit.Rid)
		return &types.CheckApplyOrderResp{
			Pass:   false,
			Reason: fmt.Sprintf("申领参数校验未通过：%v，请修正后重试。", err),
		}, nil
	}

	// 业务创建权限校验：无权限直接返回错误（IAM 鉴权失败）。
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.Biz, Action: meta.Create}, BizID: input.BkBizId,
	}); err != nil {
		logs.Errorf("no permission to check biz apply order, bizID: %d, err: %v, rid: %s",
			input.BkBizId, err, cts.Kit.Rid)
		return nil, err
	}

	return s.checkApplyOrder(cts.Kit, input)
}

// checkApplyOrder 执行提单前完整只读校验链并归一化结果。
// 校验顺序与创建路径完全对齐：需求类型/预测余量 → 需求类型特定校验/数据填充 → GPU 计费时长 → 机型信息填充 → 实时容量。
// 前四步与创建路径完全同源：validateApplyOrder + ProcessApplyOrderByRequireType + VerifyCvmGPUChargeMonth
// + FillCVMAppliedCore，确保 check 和 create 在相同的数据状态与校验顺序下执行后续逻辑。
// 业务类失败（参数非法、预测不足、GPU 不通过、容量不足）归一为 {pass:false, reason}；
// 系统异常（下游调用失败等）透出为 error。
func (s *service) checkApplyOrder(kt *kit.Kit, input *types.ApplyReq) (*types.CheckApplyOrderResp, error) {
	// 1. 需求类型校验 + 预测内/外余量校验（与创建路径同源）
	if err := s.validateApplyOrder(kt, input); err != nil {
		if reason, ok := businessRejectReason(err); ok {
			logs.Infof("check apply order not passed on require/resplan verify, reason: %s, bizID: %d, rid: %s",
				reason, input.BkBizId, kt.Rid)
			return &types.CheckApplyOrderResp{Pass: false, Reason: reason}, nil
		}
		logs.Errorf("failed to check apply order on require/resplan verify, err: %v, bizID: %d, rid: %s",
			err, input.BkBizId, kt.Rid)
		return nil, err
	}

	// 2. 需求类型特定校验 + 内存数据填充（与创建路径同源）：
	//    - 滚服：校验继承机型族一致性
	//    - 裁撤：从 BKCC 继承 charge_type，供后续 GPU 计费时长与容量校验使用
	//    - 弹性资源池：校验 charge_type 与配置一致
	if err := s.logics.Scheduler().VerifyCvmApplyTicketByRequireType(kt, input); err != nil {
		if reason, ok := businessRejectReason(err); ok {
			logs.Infof("check apply order not passed on require type processing, reason: %s, bizID: %d, rid: %s",
				reason, input.BkBizId, kt.Rid)
			return &types.CheckApplyOrderResp{Pass: false, Reason: reason}, nil
		}
		logs.Errorf("failed to check apply order on require type processing, err: %v, bizID: %d, rid: %s",
			err, input.BkBizId, kt.Rid)
		return nil, err
	}

	// 3. GPU 计费时长校验（只读，与创建路径同源）
	if err := s.logics.Scheduler().VerifyCvmGPUChargeMonth(kt, input.Suborders); err != nil {
		if reason, ok := businessRejectReason(err); ok {
			logs.Infof("check apply order not passed on gpu charge month verify, reason: %s, bizID: %d, rid: %s",
				reason, input.BkBizId, kt.Rid)
			return &types.CheckApplyOrderResp{Pass: false, Reason: reason}, nil
		}
		logs.Errorf("failed to check apply order on gpu charge month verify, err: %v, bizID: %d, rid: %s",
			err, input.BkBizId, kt.Rid)
		return nil, err
	}

	// 4. 机型信息填充（机型族/核数），顺序与创建路径一致
	filled, err := s.logics.Scheduler().FillCVMAppliedCore(kt, input)
	if err != nil {
		logs.Errorf("failed to fill cvm applied core for check, err: %v, bizID: %d, rid: %s",
			err, input.BkBizId, kt.Rid)
		return nil, err
	}
	input = filled

	// 5. 实时容量校验（与生产路径同源：委托给 scheduler 统一维护）
	pass, reason, err := s.logics.Scheduler().VerifyApplyCapacity(kt, input)
	if err != nil {
		logs.Errorf("failed to check apply order on capacity verify, err: %v, bizID: %d, rid: %s",
			err, input.BkBizId, kt.Rid)
		return nil, err
	}
	if !pass {
		logs.Infof("check apply order not passed on capacity verify, reason: %s, bizID: %d, rid: %s",
			reason, input.BkBizId, kt.Rid)
		return &types.CheckApplyOrderResp{Pass: false, Reason: reason}, nil
	}

	logs.Infof("check apply order passed, bizID: %d, suborders: %d, rid: %s", input.BkBizId,
		len(input.Suborders), kt.Rid)
	return &types.CheckApplyOrderResp{Pass: true}, nil
}

// businessRejectReason 判断错误是否属于"业务不通过"类（应归一为 Pass=false 而非系统异常）。
// 业务类错误码：参数非法、预测余量校验失败、CVM 申请校验失败（含 GPU 计费时长）。
// 返回 (可读原因, 是否业务类)。
func businessRejectReason(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	ef := errf.Error(err)
	switch ef.Code {
	case errf.InvalidParameter, errf.ResPlanVerifyFailed, errf.CvmApplyVerifyFailed:
		return ef.Message, true
	default:
		return "", false
	}
}
