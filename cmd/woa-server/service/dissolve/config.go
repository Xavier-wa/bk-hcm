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

package dissolve

import (
	model "hcm/cmd/woa-server/types/dissolve"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/converter"
)

// GetDissolveConfig get dissolve config
func (s *service) GetDissolveConfig(cts *rest.Contexts) (interface{}, error) {
	// 自研云资源-机房裁撤管理-菜单粒度
	err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.ZiyanResDissolveManage, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	time, err := s.logics.Config().GetDissolveHostApplyTime(cts.Kit)
	if err != nil {
		logs.Errorf("get dissolve host applyTime config failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	approvalLimit, err := s.logics.Config().GetApprovalLimit(cts.Kit)
	if err != nil {
		logs.Errorf("get dissolve approval limit config failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	// 获取配额系数
	quotaCoefficient, err := s.logics.Config().GetQuotaCoefficient(cts.Kit)
	if err != nil {
		logs.Errorf("get dissolve quota coefficient config failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	// 获取偏移配置
	quotaOffsets, err := s.logics.Config().GetQuotaOffsets(cts.Kit)
	if err != nil {
		logs.Errorf("get dissolve quota offsets config failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	config := &model.Config{
		HostApplyTime:    time,
		ApprovalLimit:    approvalLimit,
		QuotaCoefficient: &quotaCoefficient,
		QuotaOffsets:     quotaOffsets,
	}

	return config, nil
}

// UpsertDissolveConfig upsert dissolve config
func (s *service) UpsertDissolveConfig(cts *rest.Contexts) (interface{}, error) {
	req := new(model.UpsertConfigReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.ZiyanResDissolveManage, Action: meta.Update}})
	if err != nil {
		return nil, err
	}

	if req.HostApplyTime != nil {
		err = s.logics.Config().UpsertDissolveHostApplyTime(cts.Kit, req.HostApplyTime)
		if err != nil {
			logs.Errorf("upsert dissolve host apply time config failed, err: %v, val: %v, rid: %s", err,
				converter.PtrToVal(req.HostApplyTime), cts.Kit.Rid)
			return nil, err
		}
	}
	if req.ApprovalLimit != nil {
		err = s.logics.Config().UpsertApprovalLimit(cts.Kit, req.ApprovalLimit)
		if err != nil {
			logs.Errorf("upsert dissolve approval limit config failed, err: %v, val: %v, rid: %s", err,
				converter.PtrToVal(req.ApprovalLimit), cts.Kit.Rid)
			return nil, err
		}
	}

	// 更新配额系数
	if req.QuotaCoefficient != nil {
		err = s.logics.Config().UpsertQuotaCoefficient(cts.Kit, converter.PtrToVal(req.QuotaCoefficient))
		if err != nil {
			logs.Errorf("upsert dissolve quota coefficient config failed, err: %v, val: %v, rid: %s", err,
				converter.PtrToVal(req.QuotaCoefficient), cts.Kit.Rid)
			return nil, err
		}
	}

	// 更新偏移配置（全量覆盖）
	if req.QuotaOffsets != nil {
		err = s.logics.Config().UpsertQuotaOffsets(cts.Kit, req.QuotaOffsets)
		if err != nil {
			logs.Errorf("upsert dissolve quota offsets config failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		// 记录操作日志
		logs.Infof("upsert dissolve quota offsets config, operator: %s, offsets: %+v, rid: %s",
			cts.Kit.User, req.QuotaOffsets, cts.Kit.Rid)
	}

	return nil, nil
}

// UpdateBizDissolveQuotaOffset update single business quota offset
func (s *service) UpdateBizDissolveQuotaOffset(cts *rest.Contexts) (interface{}, error) {
	bizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.Newf(errf.InvalidParameter, "invalid bk_biz_id: %v", err)
	}
	if bizID <= 0 {
		return nil, errf.New(errf.InvalidParameter, "bk_biz_id must be positive")
	}

	req := new(model.UpdateDissolveQuotaOffsetReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	err = s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.ZiyanResDissolveManage, Action: meta.Update}})
	if err != nil {
		return nil, err
	}

	resp, err := s.logics.Config().UpdateBizDissolveQuotaOffset(cts.Kit, bizID, req)
	if err != nil {
		logs.Errorf("update dissolve quota offset failed, err: %v, bizID: %d, req: %+v, rid: %s", err,
			bizID, req, cts.Kit.Rid)
		return nil, err
	}

	return resp, nil
}
