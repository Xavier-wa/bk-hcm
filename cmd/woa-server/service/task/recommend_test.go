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

package task

import (
	"testing"
	"time"

	woaserver "hcm/pkg/api/woa-server"
	"hcm/pkg/thirdparty/cvmapi"
	cvt "hcm/pkg/tools/converter"

	"github.com/stretchr/testify/assert"
)

// TestFillStaticRollServerFields 覆盖滚服子单的五个滚服字段写入与零值不下发行为。
// 其中负数月数不写 charge_months 是防 uint 溢出的第二道防线：候选有效性已在 logic 层收敛，
// 正常链路上包年包月候选的剩余月数恒 >= 1。
func TestFillStaticRollServerFields(t *testing.T) {
	startTime := time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC)
	expireTime := time.Date(2027, 6, 10, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		name                  string
		host                  *woaserver.InheritedHost
		wantChargeType        cvmapi.ChargeType
		wantChargeMonths      uint
		wantAssetID           string
		wantInheritInstanceID string
		wantBillingStartTime  *time.Time
		wantBillingExpireTime *time.Time
	}{
		{
			// 非滚服候选取不到固资，子单保持组装时的默认值
			name:           "nil host keeps suborder untouched",
			host:           nil,
			wantChargeType: cvmapi.ChargeTypePrePaid,
		},
		{
			// 按量计费固资无套餐到期时间，剩余月数算不出正值，零值字段一律不下发
			name: "postpaid host with zero expire time",
			host: &woaserver.InheritedHost{
				AssetID:            "TC260320003234",
				CloudInstID:        "ins-postpaid",
				InstanceChargeType: string(cvmapi.ChargeTypePostPaidByHour),
				BillingStartTime:   startTime,
				ChargeMonths:       -12,
			},
			wantChargeType:        cvmapi.ChargeTypePostPaidByHour,
			wantChargeMonths:      0,
			wantAssetID:           "TC260320003234",
			wantInheritInstanceID: "ins-postpaid",
			wantBillingStartTime:  cvt.ValToPtr(startTime),
		},
		{
			name: "prepaid host with positive charge months",
			host: &woaserver.InheritedHost{
				AssetID:            "TC260320003235",
				CloudInstID:        "ins-prepaid",
				InstanceChargeType: string(cvmapi.ChargeTypePrePaid),
				BillingStartTime:   startTime,
				BillingExpireTime:  expireTime,
				ChargeMonths:       10,
			},
			wantChargeType:        cvmapi.ChargeTypePrePaid,
			wantChargeMonths:      10,
			wantAssetID:           "TC260320003235",
			wantInheritInstanceID: "ins-prepaid",
			wantBillingStartTime:  cvt.ValToPtr(startTime),
			wantBillingExpireTime: cvt.ValToPtr(expireTime),
		},
		{
			name: "prepaid host with negative charge months",
			host: &woaserver.InheritedHost{
				AssetID:            "TC260320003236",
				CloudInstID:        "ins-expired",
				InstanceChargeType: string(cvmapi.ChargeTypePrePaid),
				BillingStartTime:   startTime,
				BillingExpireTime:  expireTime,
				ChargeMonths:       -1,
			},
			wantChargeType:        cvmapi.ChargeTypePrePaid,
			wantChargeMonths:      0,
			wantAssetID:           "TC260320003236",
			wantInheritInstanceID: "ins-expired",
			wantBillingStartTime:  cvt.ValToPtr(startTime),
			wantBillingExpireTime: cvt.ValToPtr(expireTime),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			suborder := &woaserver.ApplyRecommendSuborder{ChargeType: cvmapi.ChargeTypePrePaid}
			fillStaticRollServerFields(suborder, tc.host)

			assert.Equal(t, tc.wantChargeType, suborder.ChargeType)
			assert.Equal(t, tc.wantChargeMonths, suborder.ChargeMonths)
			assert.Equal(t, tc.wantAssetID, suborder.AssetID)
			assert.Equal(t, tc.wantInheritInstanceID, suborder.InheritInstanceID)
			assert.Equal(t, tc.wantBillingStartTime, suborder.BillingStartTime)
			assert.Equal(t, tc.wantBillingExpireTime, suborder.BillingExpireTime)
		})
	}
}
