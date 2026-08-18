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

package permissiontemplate

import (
	"strings"
	"testing"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/types"
	tabletypes "hcm/pkg/dal/table/types"

	"github.com/stretchr/testify/assert"
)

// TestBuildPermTmplJoinInnerSQL_Default verifies the ON clause keeps both account_id/vendor and JSON_CONTAINS.
func TestBuildPermTmplJoinInnerSQL_Default(t *testing.T) {
	whereSQL := "WHERE pt.vendor = :vendor"
	sql := buildPermTmplJoinInnerSQL(whereSQL)

	assert.Contains(t, sql, "LEFT JOIN")
	assert.Contains(t, sql, "sub_account")
	assert.Contains(t, sql, "ON sa.account_id = pt.account_id AND sa.vendor = pt.vendor")
	assert.Contains(t, sql, "JSON_CONTAINS(sa.permission_template_ids, JSON_QUOTE(pt.id))")
	assert.Contains(t, sql, "GROUP BY pt.id")
	assert.Contains(t, sql, "COUNT(sa.id) AS associated_sub_account_count")
	assert.Contains(t, sql, whereSQL)

	// JSON_CONTAINS must be part of the ON clause, not dropped.
	onIdx := strings.Index(sql, "ON sa.account_id")
	jsonIdx := strings.Index(sql, "JSON_CONTAINS")
	groupByIdx := strings.Index(sql, "GROUP BY pt.id")
	assert.True(t, onIdx >= 0 && jsonIdx > onIdx && jsonIdx < groupByIdx,
		"JSON_CONTAINS must be part of the ON clause, before GROUP BY")
}

// TestBuildPermTmplJoinWhere_Basic covers the common filter combinations used by
// ListJoinSubAccount, keeping pt.* prefixed conditions unchanged by the LEFT JOIN refactor.
func TestBuildPermTmplJoinWhere_Basic(t *testing.T) {
	testCases := []struct {
		name       string
		opt        *types.ListPermTmplJoinOption
		wantExprs  []string
		wantArgKey string
	}{
		{
			name:      "vendor only",
			opt:       &types.ListPermTmplJoinOption{Vendor: enumor.TCloud},
			wantExprs: []string{"pt.vendor = :vendor"},
		},
		{
			name: "account ids",
			opt: &types.ListPermTmplJoinOption{
				Vendor: enumor.TCloud, AccountIDs: []string{"acc-1", "acc-2"},
			},
			wantExprs:  []string{"pt.account_id IN (:account_ids)"},
			wantArgKey: "account_ids",
		},
		{
			name: "policy library id is null",
			opt: &types.ListPermTmplJoinOption{
				Vendor: enumor.TCloud, PolicyLibraryIDIsNull: boolPtr(true),
			},
			wantExprs: []string{"pt.policy_library_id IS NULL"},
		},
		{
			name: "policy library id is not null",
			opt: &types.ListPermTmplJoinOption{
				Vendor: enumor.TCloud, PolicyLibraryIDIsNull: boolPtr(false),
			},
			wantExprs: []string{"pt.policy_library_id IS NOT NULL"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			whereSQL, args, err := buildPermTmplJoinWhere(tc.opt)
			assert.NoError(t, err)
			for _, expr := range tc.wantExprs {
				assert.Contains(t, whereSQL, expr)
			}
			if tc.wantArgKey != "" {
				_, ok := args[tc.wantArgKey]
				assert.True(t, ok, "expect arg key %s to be present", tc.wantArgKey)
			}
		})
	}
}

// TestBuildPermTmplJoinWhere_CloudSubAccountIDs verifies the EXISTS filter uses alias sa2.
func TestBuildPermTmplJoinWhere_CloudSubAccountIDs(t *testing.T) {
	ext := tabletypes.JsonField(`{"cloud_sub_account_ids":["100001","100002"]}`)
	opt := &types.ListPermTmplJoinOption{Vendor: enumor.TCloud, Extension: ext}

	whereSQL, args, err := buildPermTmplJoinWhere(opt)
	assert.NoError(t, err)
	assert.Contains(t, whereSQL, "EXISTS (SELECT 1 FROM sub_account AS sa2")
	assert.Contains(t, whereSQL, "sa2.cloud_id IN (:cloud_sub_account_ids)")
	assert.Contains(t, whereSQL, "JSON_CONTAINS(sa2.permission_template_ids, JSON_QUOTE(pt.id))")
	// must not collide with buildPermTmplJoinInnerSQL's outer LEFT JOIN alias `sa`.
	assert.NotContains(t, whereSQL, "FROM sub_account AS sa WHERE")
	_, ok := args["cloud_sub_account_ids"]
	assert.True(t, ok)
}

func boolPtr(b bool) *bool { return &b }
