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

// Package embedding provides functionality for building embedding clients.
package embedding

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"hcm/cmd/agent-server/logics/auth"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"

	openaiopt "github.com/openai/openai-go/option"
	openaiembed "trpc.group/trpc-go/trpc-agent-go/knowledge/embedder/openai"
)

// BuildEmbeddingClient constructs an OpenAI-compatible embedder from gateway and embedding configs.
// Supports both APIKey auth and BK application auth (X-Bkapi-Authorization header injection).
// Used by both memory service and EmbeddingIndex tool retrieval.
func BuildEmbeddingClient(provider *cc.AgentModelProvider, embedCfg *cc.AgentEmbeddingConfig) *openaiembed.Embedder {
	var embedOpts []openaiembed.Option
	embedOpts = append(embedOpts, openaiembed.WithBaseURL(provider.BaseURL))

	// use openai api key auth
	if provider.IsOpenAIProvider() {
		embedOpts = append(embedOpts, openaiembed.WithAPIKey(provider.APIKey))
	}

	// use bk api gateway auth
	username := provider.User
	if provider.IsBKAPIProvider() {
		appCode := provider.AppCode
		appSecret := provider.AppSecret
		defaultTicket := provider.BkTicket

		embedOpts = append(embedOpts, openaiembed.WithRequestOptions(
			// 注入 bkapigw auth header
			openaiopt.WithMiddleware(func(r *http.Request, next openaiopt.MiddlewareNext) (*http.Response, error) {
				username = auth.BKUsernameFromContext(r.Context())
				ticket := auth.BKTicketFromContext(r.Context())
				if ticket == "" {
					ticket = defaultTicket
				}
				r.Header.Set(constant.BKGWAuthKey, auth.BKApiAuthHeaderValue(appCode, appSecret, username, ticket))
				return next(r)
			}),
		))
	}

	// logging middleware: records transport errors and HTTP error responses (body snippet)
	// to diagnose gateway 403/401 etc.
	embedOpts = append(embedOpts, openaiembed.WithRequestOptions(
		openaiopt.WithMiddleware(func(r *http.Request, next openaiopt.MiddlewareNext) (*http.Response, error) {
			resp, err := next(r)
			if err != nil {
				logs.Warnf("embedding API: transport error %s %s: %v (api_key_set=%v bk_user=%q model=%q dims=%d)",
					r.Method, r.URL.String(), err, provider.APIKey, username, embedCfg.Model, embedCfg.Dimensions)
				return resp, err
			}
			if resp == nil {
				return resp, err
			}
			if resp.StatusCode >= 400 {
				bodyStr := ""
				if resp.Body != nil {
					slurp, readErr := io.ReadAll(io.LimitReader(resp.Body, constant.DefaultLLMRequestBodyLogLimit))
					_ = resp.Body.Close()
					resp.Body = io.NopCloser(bytes.NewReader(slurp))
					if readErr != nil {
						bodyStr = fmt.Sprintf("<read body: %v>", readErr)
					} else {
						bodyStr = string(slurp)
						if len(bodyStr) > constant.DefaultLLMRequestBodyLogLimit {
							bodyStr = bodyStr[:constant.DefaultLLMRequestBodyLogLimit] +
								fmt.Sprintf("... (truncated, total %d bytes)", len(slurp))
						}
					}
				}
				logs.Errorf("embedding API: HTTP %s %s %s (api_key_set=%v bk_user=%q model=%q dims=%d) response_body=%q",
					resp.Status, r.Method, r.URL.String(), provider.APIKey, username, embedCfg.Model,
					embedCfg.Dimensions, bodyStr)
			}
			return resp, err
		}),
	))

	if embedCfg.Model != "" {
		embedOpts = append(embedOpts, openaiembed.WithModel(embedCfg.Model))
	}
	if embedCfg.Dimensions > 0 {
		embedOpts = append(embedOpts, openaiembed.WithDimensions(embedCfg.Dimensions))
	}
	return openaiembed.New(embedOpts...)
}
