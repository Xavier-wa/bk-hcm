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

// Package plan ...
package plan

import (
	"errors"
	"fmt"

	"hcm/cmd/woa-server/types/task"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/thirdparty/cvmapi"
)

// VerifyResPlanDemandReq is verify resource plan demand request.
type VerifyResPlanDemandReq struct {
	BkBizID     int64              `json:"bk_biz_id" validate:"required"`
	RequireType enumor.RequireType `json:"require_type" validate:"required"`
	Suborders   []task.Suborder    `json:"suborders" validate:"required"`
}

// Validate whether VerifyResPlanDemandReq is valid.
func (req *VerifyResPlanDemandReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if req.BkBizID <= 0 {
		return errors.New("bk biz id should be > 0")
	}

	if err := req.RequireType.Validate(); err != nil {
		return err
	}

	suborderLimit := 100
	if len(req.Suborders) > suborderLimit {
		return fmt.Errorf("suborders exceed max suborders %d", suborderLimit)
	}

	for _, suborder := range req.Suborders {
		if err := suborder.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// VerifyResPlanDemandResp is verify resource plan demand response.
type VerifyResPlanDemandResp struct {
	Verifications []VerifyResPlanDemandElem `json:"verifications"`
}

// VerifyResPlanDemandElem is verify resource plan demand element.
type VerifyResPlanDemandElem struct {
	VerifyResult enumor.VerifyResPlanRst `json:"verify_result"`
	Reason       string                  `json:"reason"`
	NeedCPUCore  int64
	ResPlanCore  int64
}

// GetCvmChargeTypeDeviceTypeReq is get cvm charge type and device type request.
type GetCvmChargeTypeDeviceTypeReq struct {
	BkBizID     int64              `json:"bk_biz_id" validate:"required"`
	RequireType enumor.RequireType `json:"require_type" validate:"required"`
	Region      string             `json:"region" validate:"required"`
	Zone        string             `json:"zone" validate:"omitempty"`
	// SuborderID 子订单ID，用于预测校验时跳过当前子订单，避免重复计算本单据的预测占用
	SuborderID string `json:"suborder_id" validate:"omitempty"`
}

// Validate whether GetCvmChargeTypeDeviceTypeReq is valid.
func (req *GetCvmChargeTypeDeviceTypeReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if req.BkBizID <= 0 {
		return errors.New("bk biz id should be > 0")
	}

	if err := req.RequireType.Validate(); err != nil {
		return err
	}

	return nil
}

// GetCvmChargeTypeDeviceTypeRst is get cvm charge type device type result.
type GetCvmChargeTypeDeviceTypeRst struct {
	Count int64                            `json:"count"`
	Info  []GetCvmChargeTypeDeviceTypeElem `json:"info"`
}

// GetCvmChargeTypeDeviceTypeElem is get cvm charge type device type element.
type GetCvmChargeTypeDeviceTypeElem struct {
	ChargeType  cvmapi.ChargeType     `json:"charge_type"`
	Available   bool                  `json:"available"`
	DeviceTypes []DeviceTypeAvailable `json:"device_types"`
}

// DeviceTypeAvailable is device type available struct.
type DeviceTypeAvailable struct {
	DeviceType string `json:"device_type"`
	Available  bool   `json:"available"`
	RemainCore int64  `json:"remain_core"`
}
