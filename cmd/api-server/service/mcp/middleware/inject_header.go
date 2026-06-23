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

package middleware

import (
	"net/http"
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"
	"hcm/pkg/tools/uuid"
)

// InjectInternalHeaders 在 outbound 内部调用前向 dest 注入下列 5 个 header：
//
//   - X-Bkapi-User-Name      （来自 kt.User，权威 bk_username）
//   - X-Bkapi-App-Code       （来自 kt.AppCode）
//   - X-Bk-Tenant-Id         （来自 kt.TenantID）
//   - X-Bkapi-Request-Id     （来自 kt.Rid，缺失时自动生成 UUID）
//   - X-Bkhcm-Caller-Source  （固定字面量 callerSource，由调用方传入）
//
// 已存在的 header 会被覆盖（dest.Set），避免上游伪造 header 透传到下游。
//
// callerSource 期望取 cc.APIServerName / cc.AgentServerName 的字符串形式。
func InjectInternalHeaders(kt *kit.Kit, dest http.Header, callerSource string) {
	if dest == nil {
		return
	}

	if kt != nil {
		setHeaderIfNotEmpty(dest, constant.UserKey, kt.User)
		setHeaderIfNotEmpty(dest, constant.AppCodeKey, kt.AppCode)
		setHeaderIfNotEmpty(dest, constant.TenantIDKey, kt.TenantID)
	}

	rid := ""
	if kt != nil {
		rid = kt.Rid
	}
	if strings.TrimSpace(rid) == "" {
		rid = uuid.UUID()
	}
	dest.Set(constant.RidKey, rid)

	if strings.TrimSpace(callerSource) != "" {
		dest.Set(constant.MCPCallerSourceHeader, callerSource)
	}
}

// setHeaderIfNotEmpty 仅在 value 非空时设置 header，避免覆盖出空字符串值。
func setHeaderIfNotEmpty(h http.Header, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	h.Set(key, value)
}
