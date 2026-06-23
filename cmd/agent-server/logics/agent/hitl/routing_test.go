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

package hitl

import (
	"context"
	"testing"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"

	"trpc.group/trpc-go/trpc-agent-go/graph"
)

func TestMakeRoutingFunc(t *testing.T) {
	route := MakeRoutingFunc()
	ctx := context.Background()

	tests := []struct {
		name  string
		state graph.State
		want  string
	}{
		{
			name:  "proceed routes to tool",
			state: graph.State{constant.StateKeyHITLRoute: enumor.CvmApplyNodeTool},
			want:  string(enumor.CvmApplyNodeTool),
		},
		{
			name:  "llm route",
			state: graph.State{constant.StateKeyHITLRoute: enumor.CvmApplyNodeLLM},
			want:  string(enumor.CvmApplyNodeLLM),
		},
		{name: "missing defaults to llm", state: graph.State{}, want: string(enumor.CvmApplyNodeLLM)},
		{name: "unknown defaults to llm", state: graph.State{constant.StateKeyHITLRoute: "x"},
			want: string(enumor.CvmApplyNodeLLM)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := route(ctx, tc.state)
			if err != nil {
				t.Fatalf("route() err = %v", err)
			}
			if got != tc.want {
				t.Errorf("route() = %q, want %q", got, tc.want)
			}
		})
	}
}
