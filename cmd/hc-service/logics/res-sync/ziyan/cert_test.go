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

package ziyan

import (
	"testing"

	typecert "hcm/pkg/adaptor/types/cert"
	"hcm/pkg/api/core"
	corecert "hcm/pkg/api/core/cloud/cert"
	datacli "hcm/pkg/client/data-service"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/tools/converter"
	pkgziyan "hcm/pkg/ziyan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ssl "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ssl/v20191205"
)

func newCloudCert(cloudID string, bs2Value string) typecert.TCloudCert {
	cert := typecert.TCloudCert{
		Certificates: &ssl.Certificates{
			CertificateId: converter.ValToPtr(cloudID),
		},
	}
	if bs2Value != "" {
		key := pkgziyan.TagKeyBs2
		val := bs2Value
		cert.Tags = []*ssl.Tags{
			{TagKey: &key, TagValue: &val},
		}
	}
	return cert
}

func mockGetBkBizIdByBs2(mapping map[int64]int64) func(*kit.Kit, *datacli.Client, cmdb.Client, []int64) ([]int64, error) {
	return func(_ *kit.Kit, _ *datacli.Client, _ cmdb.Client, bs2NameIDs []int64) ([]int64, error) {
		bizIds := make([]int64, len(bs2NameIDs))
		for i, id := range bs2NameIDs {
			if id < 0 {
				bizIds[i] = constant.UnassignedBiz
				continue
			}
			if bk, ok := mapping[id]; ok {
				bizIds[i] = bk
				continue
			}
			bizIds[i] = constant.UnassignedBiz
		}
		return bizIds, nil
	}
}

func TestFillCertBkBizID(t *testing.T) {
	kt := kit.New()
	cli := &client{}
	oldFn := getBkBizIdByBs2Fn
	defer func() { getBkBizIdByBs2Fn = oldFn }()

	testCases := []struct {
		name          string
		certFromCloud []typecert.TCloudCert
		bs2BizMap     map[int64]int64
		wantBkBizIDs  []int64
		wantErr       bool
	}{
		{
			name:          "cloud bs2 maps to biz",
			certFromCloud: []typecert.TCloudCert{newCloudCert("cert-1", "game_123")},
			bs2BizMap:     map[int64]int64{123: 456},
			wantBkBizIDs:  []int64{456},
		},
		{
			name:          "no cloud tag keeps unassigned",
			certFromCloud: []typecert.TCloudCert{newCloudCert("cert-2", "")},
			wantBkBizIDs:  []int64{constant.UnassignedBiz},
		},
		{
			name:          "db assigned cert is overwritten by cloud tag",
			certFromCloud: []typecert.TCloudCert{newCloudCert("cert-3", "game_123")},
			bs2BizMap:     map[int64]int64{123: 456},
			wantBkBizIDs:  []int64{456},
		},
		{
			name:          "unassigned cert filled from cloud tag",
			certFromCloud: []typecert.TCloudCert{newCloudCert("cert-4", "game_123")},
			bs2BizMap:     map[int64]int64{123: 789},
			wantBkBizIDs:  []int64{789},
		},
		{
			name: "batch index alignment",
			certFromCloud: []typecert.TCloudCert{
				newCloudCert("cert-a", "game_123"),
				newCloudCert("cert-b", ""),
				newCloudCert("cert-c", "game_456"),
			},
			bs2BizMap:    map[int64]int64{123: 111, 456: 222},
			wantBkBizIDs: []int64{111, constant.UnassignedBiz, 222},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getBkBizIdByBs2Fn = mockGetBkBizIdByBs2(tc.bs2BizMap)

			got, err := cli.fillCertBkBizID(kt, tc.certFromCloud)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, len(tc.wantBkBizIDs))
			for i, want := range tc.wantBkBizIDs {
				assert.Equal(t, want, got[i].BkBizID, "cert index %d", i)
			}
		})
	}
}

func TestGetTagMapReturnsEmptyMapWhenNoTags(t *testing.T) {
	cloudCert := newCloudCert("cert-no-tag", "")
	tagMap := cloudCert.GetTagMap()
	require.NotNil(t, tagMap)
	assert.Empty(t, tagMap)
}

func TestConvCertCloudToDBUpdateClearsTags(t *testing.T) {
	cloudCert := newCloudCert("cert-clear-tag", "")
	updateReq, err := convCertCloudToDBUpdate("id-1", "account-1", cloudCert)
	require.NoError(t, err)
	require.NotNil(t, updateReq.Tags)
	assert.Empty(t, updateReq.Tags)
}

func TestIsCertChange_tagsClearedOnCloud(t *testing.T) {
	name := "cert-tags"
	status := uint64(1)
	cloudCert := typecert.TCloudCert{
		Certificates: &ssl.Certificates{
			CertificateId: converter.ValToPtr("cert-tags"),
			Alias:         &name,
			Status:        &status,
		},
	}
	dbCert := &corecert.Cert[corecert.TCloudCertExtension]{
		BaseCert: corecert.BaseCert{
			CloudID:    "cert-tags",
			Name:       name,
			CertStatus: "1",
			Tags: core.TagMap{
				pkgziyan.TagKeyBs2: "game_123",
			},
		},
	}
	assert.True(t, isCertChange(cloudCert, dbCert))
}

func TestConvCertCloudToDBUpdateBkBizID(t *testing.T) {
	cloudCert := newCloudCert("cert-update", "")
	cloudCert.BkBizID = 456

	updateReq, err := convCertCloudToDBUpdate("id-1", "account-1", cloudCert)
	require.NoError(t, err)
	assert.Equal(t, int64(456), updateReq.BkBizID)

	cloudCert.BkBizID = constant.UnassignedBiz
	updateReq, err = convCertCloudToDBUpdate("id-2", "account-1", cloudCert)
	require.NoError(t, err)
	assert.Equal(t, int64(constant.UnassignedBiz), updateReq.BkBizID)
}

func TestIsCertChange_assignedCertNoDiff(t *testing.T) {
	name := "same-cert"
	status := uint64(1)
	cloudCert := typecert.TCloudCert{
		Certificates: &ssl.Certificates{
			CertificateId: converter.ValToPtr("cert-same"),
			Alias:         &name,
			Status:        &status,
		},
		BkBizID: 100,
	}
	dbCert := &corecert.Cert[corecert.TCloudCertExtension]{
		BaseCert: corecert.BaseCert{
			CloudID:    "cert-same",
			Name:       name,
			BkBizID:    100,
			CertStatus: "1",
			Tags:       core.TagMap{},
		},
	}
	assert.False(t, isCertChange(cloudCert, dbCert))
}

func TestApplyCreateCertBkBizIDFallback(t *testing.T) {
	input := []typecert.TCloudCert{newCloudCert("cert-create", "")}
	input[0].BkBizID = constant.UnassignedBiz
	got := applyCreateCertBkBizIDFallback(input, &SyncCertOption{BkBizID: 888})
	assert.Equal(t, int64(888), got[0].BkBizID)
	assert.Equal(t, int64(constant.UnassignedBiz), input[0].BkBizID)

	input[0].BkBizID = 456
	got = applyCreateCertBkBizIDFallback(input, &SyncCertOption{BkBizID: 888})
	assert.Equal(t, int64(456), got[0].BkBizID)
}

func TestBuildCertBatchCreateReqBkBizID(t *testing.T) {
	testCases := []struct {
		name      string
		cloudCert typecert.TCloudCert
		wantBkBiz int64
	}{
		{
			name: "sync create uses filled cloud bk biz id",
			cloudCert: func() typecert.TCloudCert {
				cert := newCloudCert("cert-sync", "")
				cert.BkBizID = 456
				return cert
			}(),
			wantBkBiz: 456,
		},
		{
			name: "manual create uses bk biz id after create fallback",
			cloudCert: func() typecert.TCloudCert {
				addSlice := []typecert.TCloudCert{newCloudCert("cert-manual", "")}
				addSlice[0].BkBizID = constant.UnassignedBiz
				addSlice = applyCreateCertBkBizIDFallback(addSlice, &SyncCertOption{BkBizID: 888})
				return addSlice[0]
			}(),
			wantBkBiz: 888,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			createReq, err := buildCertBatchCreateReq("account-1", []typecert.TCloudCert{tc.cloudCert})
			require.NoError(t, err)
			require.Len(t, createReq.Certs, 1)
			assert.Equal(t, tc.wantBkBiz, createReq.Certs[0].BkBizID)
		})
	}
}
