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
	"errors"
	"net/http"
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
)

// ErrCallerSourceMismatch 表示请求 X-Bkhcm-Caller-Source 与期望值不符。
var ErrCallerSourceMismatch = errors.New("caller source mismatch")

// RequireCallerSource 校验请求 header 中的 X-Bkhcm-Caller-Source 是否等于 expected。
// expected 取值通常为 cc.APIServerName / cc.AgentServerName（取 string 后比较）。
//
// 空 expected 表示禁用校验，函数直接返回 nil。
func RequireCallerSource(h http.Header, expected string) error {
	if strings.TrimSpace(expected) == "" {
		return nil
	}
	got := h.Get(constant.MCPCallerSourceHeader)
	if got != expected {
		return ErrCallerSourceMismatch
	}
	return nil
}

// CallerSourceMiddleware 返回一个 http.Handler 包装器，
// 校验请求 header 中的 X-Bkhcm-Caller-Source 等于 expected；
// 不匹配时返回 HTTP 403。
//
// expected 为空时该中间件直通（不做校验），便于联调期临时关闭。
//
// 使用场景：对内部提供服务的 HCM MCP server 入口（仅允许 agent-server 调用）。
func CallerSourceMiddleware(expected string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := RequireCallerSource(r.Header, expected); err != nil {
			got := r.Header.Get(constant.MCPCallerSourceHeader)
			logs.Warnf("caller source middleware rejected request: expected=%q, got=%q, "+
				"remote=%s, path=%s", expected, got, r.RemoteAddr, r.URL.Path)
			writeForbidden(w, "caller source not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}
