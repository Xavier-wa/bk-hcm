/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2022 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package plan

import (
	"slices"
	"sort"

	"hcm/cmd/woa-server/logics/plan"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/thirdparty/cvmapi"
	cvt "hcm/pkg/tools/converter"
)

// VerifyResPlanDemandV2 verify resource plan demand.
func (s *service) VerifyResPlanDemandV2(cts *rest.Contexts) (any, error) {
	req := new(ptypes.VerifyResPlanDemandReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to decode verify resource plan demand v2 request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate verify resource plan demand v2 request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.planController.VerifyResPlanDemandV2(cts.Kit, req.BkBizID, req.RequireType, req.Suborders)
	if err != nil {
		logs.Errorf("failed to verify resource plan demand v2, err: %v, req: %+v, rid: %s",
			err, cvt.PtrToVal(req), cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return &ptypes.VerifyResPlanDemandResp{Verifications: result}, nil
}

// getChargeTypeAvlDeviceTypesV2 get charge type available device types v2.
func (s *service) getChargeTypeAvlDeviceTypesV2(kt *kit.Kit, chargeType cvmapi.ChargeType,
	req *ptypes.GetCvmChargeTypeDeviceTypeReq, prodRemainMap map[plan.ResPlanPoolKeyV2]map[string]int64) (
	[]ptypes.DeviceTypeAvailable, error) {

	// if charge type is pre paid, get available device types from in plan.
	if chargeType == cvmapi.ChargeTypePrePaid {
		return s.planController.GetPlanTypeAvlDeviceTypesV2(kt, enumor.PlanTypeCodeInPlan, req, prodRemainMap)
	}

	// 按量计费只消耗预测外的预测
	result, err := s.planController.GetPlanTypeAvlDeviceTypesV2(kt, enumor.PlanTypeCodeOutPlan, req, prodRemainMap)
	if err != nil {
		logs.Errorf("failed to get out plan available device types v2, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// sort result, put available of true to the head.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Available
	})

	return result, nil
}

// GetCvmChargeTypeDeviceTypeV2 get cvm charge type device type v2.
func (s *service) GetCvmChargeTypeDeviceTypeV2(cts *rest.Contexts) (any, error) {
	req := new(ptypes.GetCvmChargeTypeDeviceTypeReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to decode get cvm charge type device type v2 request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate get cvm charge type device type v2 request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// get biz remained resource plan.
	_, prodMaxAvailable, err := s.planController.GetProdResRemainPoolMatch(cts.Kit, req.BkBizID, req.RequireType,
		req.SuborderID)
	if err != nil {
		logs.Errorf("failed to get biz remained resource plan v2, err: %v, req: %+v, rid: %s",
			err, cvt.PtrToVal(req), cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	// 包年包月
	prePaidAvlDeviceTypes, err := s.getChargeTypeAvlDeviceTypesV2(cts.Kit, cvmapi.ChargeTypePrePaid,
		req, prodMaxAvailable)
	if err != nil {
		logs.Errorf("failed to get pre paid available device types v2, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	// 按量计费
	postPaidByHourAvlDeviceTypes, err := s.getChargeTypeAvlDeviceTypesV2(cts.Kit, cvmapi.ChargeTypePostPaidByHour,
		req, prodMaxAvailable)
	if err != nil {
		logs.Errorf("failed to get post paid by hour available device types v2, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	prePaidAvailable := slices.ContainsFunc(prePaidAvlDeviceTypes,
		func(deviceAvailable ptypes.DeviceTypeAvailable) bool {
			return deviceAvailable.Available == true
		})

	postPaidByHourAvailable := slices.ContainsFunc(postPaidByHourAvlDeviceTypes,
		func(deviceAvailable ptypes.DeviceTypeAvailable) bool {
			return deviceAvailable.Available == true
		})

	infos := []ptypes.GetCvmChargeTypeDeviceTypeElem{
		{
			ChargeType:  cvmapi.ChargeTypePrePaid,
			Available:   prePaidAvailable,
			DeviceTypes: prePaidAvlDeviceTypes,
		},
		{
			ChargeType:  cvmapi.ChargeTypePostPaidByHour,
			Available:   postPaidByHourAvailable,
			DeviceTypes: postPaidByHourAvlDeviceTypes,
		},
	}

	return &ptypes.GetCvmChargeTypeDeviceTypeRst{Count: int64(len(infos)), Info: infos}, nil
}
