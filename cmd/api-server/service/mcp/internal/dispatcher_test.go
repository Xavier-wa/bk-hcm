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

package internalmcp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildBackendCallShape_BizIDKeepsDecimal(t *testing.T) {
	t.Parallel()

	backend := ToolBackend{
		Method: http.MethodPost,
		Path:   "/api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/by_static_recommend",
	}

	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{
			name: "grouped float64",
			args: map[string]interface{}{
				"path_param": map[string]interface{}{"bk_biz_id": float64(5016972)},
				"body_param": map[string]interface{}{"bk_username": "kevzheng", "limit": 5},
			},
		},
		{
			name: "flat float64",
			args: map[string]interface{}{
				"bk_biz_id":   float64(5016972),
				"bk_username": "kevzheng",
				"limit":       5,
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path, body, err := buildBackendCallShape(backend, tc.args)
			require.NoError(t, err)
			assert.Equal(t, "/api/v1/woa/bizs/5016972/task/apply/recommend/by_static_recommend", path)
			assert.NotContains(t, path, "e+")
			assert.NotContains(t, path, "E+")
			assert.NotEmpty(t, body)
		})
	}
}
