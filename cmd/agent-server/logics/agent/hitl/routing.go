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
	"fmt"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/graph"
)

// MakeRoutingFunc 返回 hitl 节点的条件边路由函数。
// 它读取节点写入的路由决策：HITLRouteTool → tool，否则 → llm。
func MakeRoutingFunc() func(ctx context.Context, state graph.State) (string, error) {
	return func(ctx context.Context, state graph.State) (string, error) {
		rid := rest.RidFromContext(ctx)
		v, _ := state[constant.StateKeyHITLRoute].(enumor.CvmApplyNode)
		switch v {
		case enumor.CvmApplyNodeTool:
			return string(enumor.CvmApplyNodeTool), nil
		case enumor.CvmApplyNodeLLM:
			return string(enumor.CvmApplyNodeLLM), nil
		default:
			logs.Errorf("hitl routing: unsupported hitl route: %s, rid: %s", string(v), rid)
			return "", fmt.Errorf("unsupported hitl route: %s", string(v))
		}
	}
}
