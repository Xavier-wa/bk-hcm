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

// Registry 维护工具名到其 HITL Handler 的映射。
// 它是「哪些工具调用必须触发人工中断」的唯一权威来源。
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry 创建一个空的 Registry。
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register 以工具名为键注册 Handler，后注册的会覆盖先注册的。
// 注册 nil handler 时为空操作。
func (r *Registry) Register(h Handler) {
	if h == nil {
		return
	}
	r.handlers[h.ToolName()] = h
}

// Lookup 返回给定工具名对应的 Handler。
func (r *Registry) Lookup(tool string) (Handler, bool) {
	h, ok := r.handlers[tool]
	return h, ok
}

// Has 判断该工具是否需要人工中断。
func (r *Registry) Has(tool string) bool {
	_, ok := r.handlers[tool]
	return ok
}
