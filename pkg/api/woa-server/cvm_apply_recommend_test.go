/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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
	"encoding/json"
	"testing"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/thirdparty/cvmapi"

	"github.com/stretchr/testify/assert"
)

// rollingServerSuborderKeys 五个滚服字段的 JSON 键，仅滚服项目（require_type=6）填充。
var rollingServerSuborderKeys = []string{
	"charge_months",
	"bk_asset_id",
	"inherit_instance_id",
	"billing_start_time",
	"billing_expire_time",
}

// TestApplyRecommendSuborder_MarshalOmitRollingServerFields verifies that the five rolling server
// fields are omitted for non rolling server require types.
func TestApplyRecommendSuborder_MarshalOmitRollingServerFields(t *testing.T) {
	suborder := &ApplyRecommendSuborder{
		RequireType: enumor.RequireTypeRegular,
		Region:      "ap-guangzhou",
		Zone:        "all",
		DeviceType:  "S5.LARGE8",
		ImageID:     "img-xxxxxxxx",
		Replicas:    10,
		ChargeType:  cvmapi.ChargeTypePrePaid,
		SystemDisk:  enumor.DiskSpec{DiskType: enumor.DiskPremium, DiskSize: 100, DiskNum: 1},
	}

	raw, err := json.Marshal(suborder)
	assert.NoError(t, err)

	keys := make(map[string]json.RawMessage)
	assert.NoError(t, json.Unmarshal(raw, &keys))

	for _, key := range rollingServerSuborderKeys {
		_, exists := keys[key]
		assert.False(t, exists, "key %s should be omitted for non rolling server suborder", key)
	}
	// 非滚服路径的其余键不受影响。
	for _, key := range []string{"require_type", "region", "zone", "device_type", "image_id", "replicas",
		"charge_type", "system_disk", "data_disk"} {

		_, exists := keys[key]
		assert.True(t, exists, "key %s should be kept", key)
	}
}

// TestApplyRecommendSplitSubOrderReq_MarshalOmitRollingServerFields verifies the same omitempty
// behaviour on the split suborder request, whose five fields must be typed exactly as the suborder ones.
func TestApplyRecommendSplitSubOrderReq_MarshalOmitRollingServerFields(t *testing.T) {
	req := &ApplyRecommendSplitSubOrderReq{
		RequireType: enumor.RequireTypeRegular,
		Region:      "ap-guangzhou",
		Zone:        "all",
		DeviceType:  "S5.LARGE8",
		ImageID:     "img-xxxxxxxx",
		ResAssign:   enumor.ResPriorityResAssign,
		Replicas:    10,
		SystemDisk:  enumor.DiskSpec{DiskType: enumor.DiskPremium, DiskSize: 100, DiskNum: 1},
	}

	raw, err := json.Marshal(req)
	assert.NoError(t, err)

	keys := make(map[string]json.RawMessage)
	assert.NoError(t, json.Unmarshal(raw, &keys))

	for _, key := range rollingServerSuborderKeys {
		_, exists := keys[key]
		assert.False(t, exists, "key %s should be omitted for non rolling server split request", key)
	}
}

// newSplitSubOrderReq builds a minimal valid split request of the given require type.
func newSplitSubOrderReq(requireType enumor.RequireType) *ApplyRecommendSplitSubOrderReq {
	return &ApplyRecommendSplitSubOrderReq{
		RequireType: requireType,
		Region:      "ap-guangzhou",
		Zone:        "all",
		DeviceType:  "S5.LARGE8",
		ImageID:     "img-xxxxxxxx",
		ResAssign:   enumor.ResPriorityResAssign,
		Replicas:    10,
		SystemDisk:  enumor.DiskSpec{DiskType: enumor.DiskPremium, DiskSize: 100, DiskNum: 1},
	}
}

// TestApplyRecommendSplitSubOrderReq_ValidateRollServer verifies that the split request accepts the
// rolling server require type and rejects it only when inherit_instance_id is missing.
func TestApplyRecommendSplitSubOrderReq_ValidateRollServer(t *testing.T) {
	req := newSplitSubOrderReq(enumor.RequireTypeRollServer)
	req.InheritInstanceID = "ins-xxxxxxxx"
	req.AssetID = "TC000000000001"
	req.ChargeMonths = 9
	req.ChargeType = cvmapi.ChargeTypePostPaidByHour
	assert.NoError(t, req.Validate())

	missing := newSplitSubOrderReq(enumor.RequireTypeRollServer)
	err := missing.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inherit_instance_id")

	badChargeType := newSplitSubOrderReq(enumor.RequireTypeRollServer)
	badChargeType.InheritInstanceID = "ins-xxxxxxxx"
	badChargeType.ChargeType = "UNKNOWN"
	assert.Error(t, badChargeType.Validate())

	// 非滚服需求类型不受滚服校验影响。
	assert.NoError(t, newSplitSubOrderReq(enumor.RequireTypeRegular).Validate())
}

// TestApplyRecommendSplitSubOrderReq_ValidatePrepaidChargeMonths 守住滚服拆单的计费入参校验：
// 拆单短路直接透传入参计费信息，滚服场景 charge_type 必填，包年包月还必须带购买时长，
// 否则会在提单落库的 ResourceSpec.Validate 处被拒，必须在拆单入参阶段就拦下。
func TestApplyRecommendSplitSubOrderReq_ValidatePrepaidChargeMonths(t *testing.T) {
	tests := []struct {
		name        string
		requireType enumor.RequireType
		chargeType  cvmapi.ChargeType
		months      uint
		wantErrKey  string
	}{
		{name: "滚服省略计费模式", requireType: enumor.RequireTypeRollServer, wantErrKey: "charge_type"},
		{name: "包年包月无购买时长", requireType: enumor.RequireTypeRollServer,
			chargeType: cvmapi.ChargeTypePrePaid, wantErrKey: "charge_months"},
		{name: "包年包月带购买时长", requireType: enumor.RequireTypeRollServer,
			chargeType: cvmapi.ChargeTypePrePaid, months: 1},
		{name: "按量计费无购买时长不受约束", requireType: enumor.RequireTypeRollServer,
			chargeType: cvmapi.ChargeTypePostPaidByHour},
		{name: "非滚服无购买时长不受约束", requireType: enumor.RequireTypeRegular},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newSplitSubOrderReq(tt.requireType)
			if tt.requireType == enumor.RequireTypeRollServer {
				req.InheritInstanceID = "ins-xxxxxxxx"
			}
			req.ChargeType = tt.chargeType
			req.ChargeMonths = tt.months

			err := req.Validate()
			if tt.wantErrKey == "" {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrKey)
		})
	}
}

// TestApplyRecommendByStaticReq_ValidateAcceptRollServer verifies that by_static no longer rejects
// the rolling server require type: the inherited host is completed server side, so callers need not
// pass any asset id.
func TestApplyRecommendByStaticReq_ValidateAcceptRollServer(t *testing.T) {
	requireType := enumor.RequireTypeRollServer
	req := &ApplyRecommendByStaticReq{BkUsername: "tester", Limit: 10, RequireType: &requireType}
	assert.NoError(t, req.Validate())

	// 非法需求类型仍然被拒绝。
	invalid := enumor.RequireType(0)
	bad := &ApplyRecommendByStaticReq{BkUsername: "tester", Limit: 10, RequireType: &invalid}
	assert.Error(t, bad.Validate())
}

// TestApplyRecommendByPlanReq_ValidateAcceptRollServer verifies that by_plan no longer treats the
// rolling server require type as a parameter error: handler 侧统一返回空方案列表，参数校验放行。
func TestApplyRecommendByPlanReq_ValidateAcceptRollServer(t *testing.T) {
	requireType := enumor.RequireTypeRollServer
	req := &ApplyRecommendByPlanReq{Limit: 10, RequireType: &requireType}
	assert.NoError(t, req.Validate())

	// 非法需求类型仍然被拒绝。
	invalid := enumor.RequireType(0)
	bad := &ApplyRecommendByPlanReq{Limit: 10, RequireType: &invalid}
	assert.Error(t, bad.Validate())
}
