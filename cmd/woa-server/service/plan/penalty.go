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
	"time"

	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// CalcPenaltyBase 计算罚金分摊基数
func (s *service) CalcPenaltyBase(cts *rest.Contexts) (interface{}, error) {
	req := new(ptypes.CalcPenaltyBaseReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to calculate penalty base, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate calculate penalty base parameter, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 权限校验
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.ZiYanResPlan, Action: meta.Find}}
	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	if err := s.calcPenaltyBase(cts.Kit, req); err != nil {
		logs.Errorf("failed to calculate penalty base, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

func (s *service) calcPenaltyBase(kt *kit.Kit, req *ptypes.CalcPenaltyBaseReq) error {
	baseDay, err := time.Parse(constant.DateLayout, req.PenaltyBaseDay)
	if err != nil {
		logs.Errorf("failed to parse penalty base day, err: %v, req_base_day: %s, rid: %s", err,
			req.PenaltyBaseDay, kt.Rid)
		return err
	}

	return s.planController.CalcPenaltyBase(kt, baseDay, req.BkBizIDs, req.SourceMode)
}

// CalcAndPushPenaltyRatio 计算并推送罚金分摊比例
func (s *service) CalcAndPushPenaltyRatio(cts *rest.Contexts) (interface{}, error) {
	req := new(ptypes.CalcAndPushPenaltyRatioReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to calculate and push penalty ratio, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate calculate and push penalty ratio parameter, err: %v, rid: %s", err,
			cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 权限校验
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.ZiYanResPlan, Action: meta.Find}}
	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	calcTime, err := time.Parse(constant.DateLayout, req.PenaltyTime)
	if err != nil {
		logs.Errorf("failed to parse year month, err: %v, penalty_time: %s, rid: %s", err, req.PenaltyTime,
			cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := s.planController.CalcPenaltyRatioAndPush(cts.Kit, calcTime); err != nil {
		logs.Errorf("failed to calculate and push penalty ratio, err: %v, calc_time: %s, rid: %s", err,
			calcTime.Format(constant.DateLayout), cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// PushExpireNotification 推送预测到期提醒
func (s *service) PushExpireNotification(cts *rest.Contexts) (interface{}, error) {
	req := new(ptypes.PushExpireNoticeReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to push expire notification, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate push expire notification parameter, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 权限校验
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.ZiYanResPlan, Action: meta.Find}}
	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	if err := s.planController.PushExpireNotifications(cts.Kit, req.BkBizIDs, req.Receivers); err != nil {
		logs.Errorf("failed to push expire notification, err: %v, bk_biz_ids: %v, rid: %s", err, req.BkBizIDs,
			cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
