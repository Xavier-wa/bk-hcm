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

package application

import (
	"testing"

	proto "hcm/pkg/api/cloud-server/application"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBizID int64 = 213

// checkBizAddAccountReq mirrors CreateBizForAddAccount checks before create().
func checkBizAddAccountReq(pathBizID int64, req *proto.AccountAddReq) error {
	if pathBizID <= 0 {
		return errf.New(errf.InvalidParameter, "biz id is invalid")
	}
	if err := validateBizAddAccountPathBody(pathBizID, req); err != nil {
		return err
	}
	return req.Validate()
}

// checkResourceAddAccountReq mirrors CreateForAddAccount checks before create().
func checkResourceAddAccountReq(req *proto.AccountAddReq) error {
	return req.Validate()
}

func newTCloudAddAccountReq(accountType enumor.AccountType, bkBizID int64, usageBizIDs []int64) *proto.AccountAddReq {
	req := &proto.AccountAddReq{
		Extension: map[string]string{
			"cloud_main_account_id": "1234567890",
			"cloud_sub_account_id":  "1234567890",
		},
	}
	req.Vendor = enumor.TCloud
	req.Name = "hcm-repro-reg"
	req.Managers = []string{"hcm"}
	req.Type = accountType
	req.Site = enumor.ChinaSite
	req.BkBizID = bkBizID
	req.UsageBizIDs = usageBizIDs
	return req
}

func assertAddAccountValidateErr(t *testing.T, err error, wantErr bool, errCode int32, errSubstr string) {
	t.Helper()
	if !wantErr {
		assert.NoError(t, err)
		return
	}

	require.Error(t, err)
	assert.Contains(t, err.Error(), errSubstr)
	if errCode == 0 {
		return
	}

	ef := errf.Error(err)
	require.NotNil(t, ef)
	assert.Equal(t, errCode, ef.Code)
}

func TestCreateBizForAddAccountValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		pathBizID   int64
		accountType enumor.AccountType
		bodyBizID   int64
		usageBizIDs []int64
		wantErr     bool
		errCode     int32
		errSubstr   string
	}{
		{
			name:      "combo A registration path real biz body empty succeeds",
			pathBizID: testBizID, accountType: enumor.RegistrationAccount,
			bodyBizID: 0, usageBizIDs: []int64{testBizID},
		},
		{
			name:      "combo B registration path and body same real biz rejected",
			pathBizID: testBizID, accountType: enumor.RegistrationAccount,
			bodyBizID: testBizID, usageBizIDs: []int64{testBizID},
			wantErr: true, errSubstr: "bk_biz_id must be empty for non-resource account",
		},
		{
			name:      "combo C/D registration path 0 rejected",
			pathBizID: 0, accountType: enumor.RegistrationAccount,
			bodyBizID: 0, usageBizIDs: []int64{testBizID},
			wantErr: true, errCode: errf.InvalidParameter, errSubstr: "biz id is invalid",
		},
		{
			name:      "registration path negative biz id rejected",
			pathBizID: -1, accountType: enumor.RegistrationAccount,
			bodyBizID: 0, usageBizIDs: []int64{testBizID},
			wantErr: true, errCode: errf.InvalidParameter, errSubstr: "biz id is invalid",
		},
		{
			name:      "security_audit path real biz body empty succeeds",
			pathBizID: testBizID, accountType: enumor.SecurityAuditAccount,
			bodyBizID: 0, usageBizIDs: []int64{testBizID},
		},
		{
			name:      "security_audit path and body same real biz rejected",
			pathBizID: testBizID, accountType: enumor.SecurityAuditAccount,
			bodyBizID: testBizID, usageBizIDs: []int64{testBizID},
			wantErr: true, errSubstr: "bk_biz_id must be empty for non-resource account",
		},
		{
			name:      "resource path matches body succeeds",
			pathBizID: testBizID, accountType: enumor.ResourceAccount,
			bodyBizID: testBizID, usageBizIDs: []int64{testBizID},
		},
		{
			name:      "resource path real biz body empty rejected",
			pathBizID: testBizID, accountType: enumor.ResourceAccount,
			bodyBizID: 0, usageBizIDs: []int64{testBizID},
			wantErr: true, errCode: errf.InvalidParameter,
			errSubstr: "path bk_biz_id(213) does not match request body bk_biz_id(0)",
		},
		{
			name:      "resource path and body mismatch rejected",
			pathBizID: testBizID, accountType: enumor.ResourceAccount,
			bodyBizID: 999, usageBizIDs: []int64{999},
			wantErr: true, errCode: errf.InvalidParameter,
			errSubstr: "path bk_biz_id(213) does not match request body bk_biz_id(999)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := newTCloudAddAccountReq(tc.accountType, tc.bodyBizID, tc.usageBizIDs)
			err := checkBizAddAccountReq(tc.pathBizID, req)
			assertAddAccountValidateErr(t, err, tc.wantErr, tc.errCode, tc.errSubstr)
		})
	}
}

func TestCreateForAddAccountValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		accountType enumor.AccountType
		bodyBizID   int64
		usageBizIDs []int64
		wantErr     bool
		errSubstr   string
	}{
		{
			name:        "registration body empty succeeds",
			accountType: enumor.RegistrationAccount,
			bodyBizID:   0,
			usageBizIDs: []int64{testBizID},
		},
		{
			name:        "registration body with manage biz rejected",
			accountType: enumor.RegistrationAccount,
			bodyBizID:   testBizID,
			usageBizIDs: []int64{testBizID},
			wantErr:     true,
			errSubstr:   "bk_biz_id must be empty for non-resource account",
		},
		{
			name:        "security_audit body empty succeeds",
			accountType: enumor.SecurityAuditAccount,
			bodyBizID:   0,
			usageBizIDs: []int64{testBizID},
		},
		{
			name:        "security_audit body with manage biz rejected",
			accountType: enumor.SecurityAuditAccount,
			bodyBizID:   testBizID,
			usageBizIDs: []int64{testBizID},
			wantErr:     true,
			errSubstr:   "bk_biz_id must be empty for non-resource account",
		},
		{
			name:        "resource body real biz succeeds",
			accountType: enumor.ResourceAccount,
			bodyBizID:   testBizID,
			usageBizIDs: []int64{testBizID},
		},
		{
			name:        "resource body empty rejected",
			accountType: enumor.ResourceAccount,
			bodyBizID:   0,
			usageBizIDs: []int64{testBizID},
			wantErr:     true,
			errSubstr:   "bk_biz_id can not be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := newTCloudAddAccountReq(tc.accountType, tc.bodyBizID, tc.usageBizIDs)
			err := checkResourceAddAccountReq(req)
			assertAddAccountValidateErr(t, err, tc.wantErr, 0, tc.errSubstr)
		})
	}
}
