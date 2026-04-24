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

package logics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"

	trpcmcp "trpc.group/trpc-go/trpc-mcp-go"
)

// mcpHTTPRespLoggingHandler wraps trpcmcp.HTTPReqHandler to log response bodies for
// HTTP status >= 400. trpc-mcp-go streamable transport returns errors without attaching
// the response body; this wrapper reads the body before the SDK discards it.
type mcpHTTPRespLoggingHandler struct {
	inner       trpcmcp.HTTPReqHandler
	toolsetName string
}

func newMCPHTTPLoggingHandler(inner trpcmcp.HTTPReqHandler, toolsetName string) trpcmcp.HTTPReqHandler {
	if inner == nil {
		inner = trpcmcp.NewDefaultHTTPReqHandler()
	}
	return &mcpHTTPRespLoggingHandler{inner: inner, toolsetName: toolsetName}
}

func (h *mcpHTTPRespLoggingHandler) Handle(ctx context.Context, client *http.Client, req *http.Request) (
	*http.Response, error) {

	resp, err := h.inner.Handle(ctx, client, req)
	if err != nil {
		logs.Errorf("MCP HTTP toolset=%q %s %s request_headers=%s: transport error: %v",
			h.toolsetName, req.Method, safeURLStr(req), safeRequestHeadersStr(req), err)
		return resp, err
	}
	if resp.StatusCode < http.StatusBadRequest {
		return resp, err
	}

	bodyBytes, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		logs.Errorf("MCP HTTP toolset=%q %s %s request_headers=%s: status=%d read response body failed: %v",
			h.toolsetName, req.Method, safeURLStr(req), safeRequestHeadersStr(req), resp.StatusCode, readErr)
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return resp, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	preview := bodyBytes
	if len(preview) > constant.DefaultLLMRequestBodyLogLimit {
		preview = preview[:constant.DefaultLLMRequestBodyLogLimit]
	}
	logs.Infof("MCP HTTP toolset=%q %s %s request_headers=%s: status=%d response_body=%s",
		h.toolsetName, req.Method, safeURLStr(req), safeRequestHeadersStr(req), resp.StatusCode,
		strings.TrimSpace(string(preview)))
	return resp, err
}

func safeURLStr(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	return req.URL.Redacted()
}

// safeRequestHeadersStr formats request headers for logs; sensitive values are redacted.
func safeRequestHeadersStr(req *http.Request) string {
	if req == nil || len(req.Header) == 0 {
		return ""
	}
	sensitive := map[string]bool{
		http.CanonicalHeaderKey("Authorization"):       true,
		http.CanonicalHeaderKey("Cookie"):              true,
		http.CanonicalHeaderKey(constant.BKGWAuthKey):  true,
		http.CanonicalHeaderKey("Proxy-Authorization"): true,
	}
	keys := make([]string, 0, len(req.Header))
	for k := range req.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		canonical := http.CanonicalHeaderKey(k)
		for _, v := range req.Header.Values(k) {
			if sensitive[canonical] {
				v = "[REDACTED]"
			}
			if b.Len() > 0 {
				b.WriteString("; ")
			}
			_, _ = fmt.Fprintf(&b, "%s: %s", canonical, v)
		}
	}
	s := b.String()
	if len(s) > constant.DefaultLLMRequestBodyLogLimit {
		s = s[:constant.DefaultLLMRequestBodyLogLimit] + "...(truncated)"
	}
	return s
}
