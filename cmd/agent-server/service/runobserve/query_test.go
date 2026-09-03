/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may obtain a copy of the License at http://opensource.org/licenses/MIT
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

package runobserve

import (
	"testing"

	"hcm/pkg/criteria/constant"
)

func TestQueryFromAGUIReq(t *testing.T) {
	cases := []struct {
		name string
		req  map[string]interface{}
		want string
	}{
		{name: "nil req", req: nil, want: ""},
		{
			name: "last user wins",
			req: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{"role": "user", "content": "第一轮"},
					map[string]interface{}{"role": "assistant", "content": "好的"},
					map[string]interface{}{"role": "user", "content": "只要广州的"},
				},
			},
			want: "只要广州的",
		},
		{
			name: "content parts",
			req: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role": "user",
						"content": []interface{}{
							map[string]interface{}{"type": "text", "text": "查 CVM"},
						},
					},
				},
			},
			want: "查 CVM",
		},
		{
			name: "resume fallback",
			req: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{"role": "assistant", "content": "选一台"},
				},
				"forwardedProps": map[string]interface{}{
					constant.ForwardedPropResumeValue: "第一台",
				},
			},
			want: "第一台",
		},
		{
			name: "empty messages",
			req:  map[string]interface{}{"messages": []interface{}{}},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QueryFromAGUIReq(tc.req); got != tc.want {
				t.Fatalf("QueryFromAGUIReq() = %q, want %q", got, tc.want)
			}
		})
	}
}
