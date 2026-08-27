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

package billprepaid

import (
	"testing"

	asbill "hcm/pkg/api/account-server/bill"
	billcore "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"

	"github.com/stretchr/testify/assert"
)

func TestCheckSettledPrepaidItem(t *testing.T) {
	testCases := []struct {
		name    string
		items   []*billcore.PrepaidItem
		wantErr bool
	}{
		{
			name: "all unsettled allows delete",
			items: []*billcore.PrepaidItem{
				{ID: "p1", SettleState: enumor.BillSettleStateUnsettled},
				{ID: "p2", SettleState: enumor.BillSettleStateUnsettled},
			},
		},
		{
			name: "any settled rejects whole batch",
			items: []*billcore.PrepaidItem{
				{ID: "p1", SettleState: enumor.BillSettleStateUnsettled},
				{ID: "p2", SettleState: enumor.BillSettleStateSettled},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSettledPrepaidItem(tc.items)
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, errf.Aborted, errf.Error(err).Code)
			assert.Contains(t, err.Error(), "p2")
		})
	}
}

func TestCheckPushingPrepaidAdjustment(t *testing.T) {
	testCases := []struct {
		name    string
		items   []*billcore.AdjustmentItem
		wantErr bool
		wantID  string
	}{
		{
			name: "pushed adjustments allow delete",
			items: []*billcore.AdjustmentItem{
				{ID: "a1", SourceID: "p1", PushStatus: enumor.BillAdjustmentPushStatusPushed},
				{ID: "a2", SourceID: "p1", PushStatus: enumor.BillAdjustmentPushStatusUnpushed},
			},
		},
		{
			name: "any pushing rejects whole batch",
			items: []*billcore.AdjustmentItem{
				{ID: "a1", SourceID: "p1", PushStatus: enumor.BillAdjustmentPushStatusPushed},
				{ID: "a2", SourceID: "p2", PushStatus: enumor.BillAdjustmentPushStatusPushing},
			},
			wantErr: true,
			wantID:  "p2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkPushingPrepaidAdjustment(tc.items)
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, errf.Aborted, errf.Error(err).Code)
			assert.Contains(t, err.Error(), tc.wantID)
		})
	}
}

func TestResolvePrepaidDeleteAuth(t *testing.T) {
	testCases := []struct {
		name        string
		isAny       bool
		ids         []string
		authorized  bool
		wantAuthFlt bool
	}{
		{name: "is any skips account filter", isAny: true, authorized: true},
		{name: "no authorized instance returns empty", authorized: false},
		{name: "specific accounts keep filter", ids: []string{"m1", "m2"}, authorized: true, wantAuthFlt: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authFlt, authorized := resolvePrepaidDeleteAuth(tc.isAny, tc.ids)
			assert.Equal(t, tc.authorized, authorized)
			if tc.wantAuthFlt {
				assert.NotNil(t, authFlt)
				return
			}
			assert.Nil(t, authFlt)
		})
	}
}

func TestBuildDeleteReqFilter(t *testing.T) {
	yearMonth := buildDeleteReqFilter(&asbill.PrepaidItemDeleteReq{OrderYear: 2026, OrderMonth: 7})
	assert.Len(t, yearMonth.Rules, 2)

	withCloudIDs := buildDeleteReqFilter(&asbill.PrepaidItemDeleteReq{
		OrderYear: 2026, OrderMonth: 7, MainAccountCloudIDs: []string{"123456789012", "210987654321"},
	})
	assert.Len(t, withCloudIDs.Rules, 3)
}

func TestCollectPrepaidIDs(t *testing.T) {
	ids := collectPrepaidIDs([]*billcore.PrepaidItem{{ID: "p1"}, {ID: "p2"}})
	assert.Equal(t, []string{"p1", "p2"}, ids)
}
