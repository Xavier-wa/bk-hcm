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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestListInheritedHostsReq_Validate verifies that the missing field name is carried by the error message,
// so that the handler can return an errf.InvalidParameter telling which field is wrong.
func TestListInheritedHostsReq_Validate(t *testing.T) {
	testCases := []struct {
		name        string
		req         *ListInheritedHostsReq
		wantErr     bool
		wantErrHint string
	}{
		{
			name: "valid request",
			req: &ListInheritedHostsReq{
				BkBizID:        100148,
				Region:         "ap-guangzhou",
				DeviceFamilies: []string{"标准型", "高IO型"},
			},
			wantErr: false,
		},
		{
			name: "region is empty",
			req: &ListInheritedHostsReq{
				BkBizID:        100148,
				DeviceFamilies: []string{"标准型"},
			},
			wantErr:     true,
			wantErrHint: "region",
		},
		{
			name: "device families is empty",
			req: &ListInheritedHostsReq{
				BkBizID:        100148,
				Region:         "ap-guangzhou",
				DeviceFamilies: []string{},
			},
			wantErr:     true,
			wantErrHint: "device_families",
		},
		{
			name: "device family is blank",
			req: &ListInheritedHostsReq{
				BkBizID:        100148,
				Region:         "ap-guangzhou",
				DeviceFamilies: []string{"  "},
			},
			wantErr:     true,
			wantErrHint: "device_families",
		},
		{
			name: "biz id is not positive",
			req: &ListInheritedHostsReq{
				Region:         "ap-guangzhou",
				DeviceFamilies: []string{"标准型"},
			},
			wantErr:     true,
			wantErrHint: "bk_biz_id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}

			assert.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), tc.wantErrHint),
				"error %q should contain field name %q", err.Error(), tc.wantErrHint)
		})
	}
}

// TestInheritedHostGroup_MarshalEmptyHosts verifies that a group without any candidate is serialized
// with an empty array rather than null, so the frontend can tell "nothing found" from "family not asked".
func TestInheritedHostGroup_MarshalEmptyHosts(t *testing.T) {
	resp := &ListInheritedHostsResp{
		Info: []*InheritedHostGroup{
			{DeviceFamily: "GPU型", Hosts: make([]*InheritedHostCandidate, 0)},
		},
	}

	raw, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.Equal(t, `{"info":[{"device_family":"GPU型","hosts":[]}]}`, string(raw))
}

// TestInheritedHostCandidate_MarshalFlatten verifies that the embedded InheritedHost fields and
// is_recommended are serialized at the same level, i.e. one candidate is a flat object without nesting.
func TestInheritedHostCandidate_MarshalFlatten(t *testing.T) {
	billingStart := time.Date(2024, 11, 20, 10, 15, 30, 0, time.UTC)
	billingExpire := time.Date(2027, 5, 20, 10, 15, 30, 0, time.UTC)
	candidate := &InheritedHostCandidate{
		InheritedHost: InheritedHost{
			AssetID:            "TC241120001357",
			InnerIP:            "10.20.30.41",
			CloudInstID:        "ins-0a1b2c3d",
			DeviceType:         "S5.LARGE8",
			InstanceChargeType: "PREPAID",
			BillingStartTime:   billingStart,
			BillingExpireTime:  billingExpire,
			ChargeMonths:       10,
		},
		IsRecommended: true,
	}

	raw, err := json.Marshal(candidate)
	assert.NoError(t, err)

	flat := make(map[string]json.RawMessage)
	assert.NoError(t, json.Unmarshal(raw, &flat))

	// 嵌入字段与 is_recommended 必须在同一层，且不存在 inherited_host 这类嵌套对象
	for _, field := range []string{"bk_asset_id", "bk_host_innerip", "bk_cloud_inst_id", "device_type",
		"instance_charge_type", "billing_start_time", "billing_expire_time", "charge_months", "is_recommended"} {
		assert.Contains(t, flat, field)
	}
	assert.NotContains(t, flat, "InheritedHost")
	assert.NotContains(t, flat, "inherited_host")
	assert.Len(t, flat, 9)

	assert.Equal(t, `"TC241120001357"`, string(flat["bk_asset_id"]))
	assert.Equal(t, `true`, string(flat["is_recommended"]))
	assert.Equal(t, `10`, string(flat["charge_months"]))
}
