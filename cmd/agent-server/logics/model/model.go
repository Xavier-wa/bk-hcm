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

// Package model ...
package model

import (
	"fmt"
	"net/http"

	"hcm/cmd/agent-server/logics/auth"
	"hcm/cmd/agent-server/logics/logger"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"

	openaiopt "github.com/openai/openai-go/option"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
)

// BuildAllModels creates one model instance per name in allowedNames.
// Each model is wired to its provider's gateway config via modelProviders mapping.
// An error is returned when a model's mapped provider is missing or empty.
func BuildAllModels(defaultModelName string, allowedNames []string, modelProviders map[string]string) (
	model.Model, map[string]model.Model, error) {

	m := make(map[string]model.Model, len(allowedNames))
	for _, name := range allowedNames {
		gatewayCfg, err := cc.AgentServer().GetProvider(modelProviders[name])
		if err != nil {
			return nil, nil, err
		}
		m[name] = buildModelWithConfig(name, gatewayCfg)
	}

	dftModel, err := resolveDefaultModel(defaultModelName, allowedNames, m, modelProviders)
	if err != nil {
		return nil, nil, err
	}

	return dftModel, m, nil
}

// resolveDefaultModel picks the model to use when no per-request model is specified.
// Priority: --model-name flag (if in modelsMap) → new instance from flag → first allowed model.
func resolveDefaultModel(flagModelName string, allowedNames []string, modelsMap map[string]model.Model,
	modelProviders map[string]string) (model.Model, error) {

	if flagModelName != "" {
		if m, ok := modelsMap[flagModelName]; ok {
			return m, nil
		}
		// Flag names an unlisted model — build it anyway so legacy configs still work.
		gatewayCfg, err := cc.AgentServer().GetProvider(modelProviders[flagModelName])
		if err != nil {
			return nil, err
		}
		mdl := buildModelWithConfig(flagModelName, gatewayCfg)
		modelsMap[flagModelName] = mdl
		logs.Warnf("model %q is not in allowedModels list but set via --model-name; added to map", flagModelName)
		return mdl, nil
	}
	if len(allowedNames) > 0 {
		return modelsMap[allowedNames[0]], nil
	}
	return nil, fmt.Errorf("no default model found")
}

// BuildModelWithConfig creates a single OpenAI-compatible model instance
// using the given gateway config.
//
// Full request URL: {gatewayCfg.BaseURL}/chat/completions
// (path suffix appended by the OpenAI Go SDK automatically)
func buildModelWithConfig(modelName string, gatewayCfg *cc.AgentModelProvider) model.Model {
	opts := buildOpenAIOptions(gatewayCfg)
	opts = append(opts, openai.WithEnableTokenTailoring(true))
	return openai.New(modelName, opts...)
}

// buildOpenAIOptions converts BKAPIGatewayConfig into openai.Option slice.
// When AppCode or AppSecret is configured, a per-request middleware is registered
// that injects the X-Bkapi-Authorization header; bk_username and bk_ticket are
// read from the request context at call time (see WithBKUsername / WithBKTicket).
func buildOpenAIOptions(cfg *cc.AgentModelProvider) []openai.Option {
	var opts []openai.Option

	opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
	if cfg.IsOpenAIProvider() {
		opts = append(opts, openai.WithAPIKey(cfg.APIKey))
	}
	if cfg.IsBKAPIProvider() {
		appCode := cfg.AppCode
		appSecret := cfg.AppSecret
		defaultUser := cfg.User
		defaultTicket := cfg.BkTicket
		opts = append(opts, openai.WithOpenAIOptions(
			openaiopt.WithMiddleware(func(r *http.Request, next openaiopt.MiddlewareNext) (*http.Response, error) {
				// Per-request context values take precedence over static config defaults.
				username := auth.BKUsernameFromContext(r.Context())
				if username == "" {
					username = defaultUser
				}
				ticket := auth.BKTicketFromContext(r.Context())
				if ticket == "" {
					ticket = defaultTicket
				}
				r.Header.Set(constant.BKGWAuthKey, auth.BKApiAuthHeaderValue(appCode, appSecret, username, ticket))
				return next(r)
			}),
		))
	}

	opts = append(opts, openai.WithOpenAIOptions(
		openaiopt.WithMiddleware(logger.LLMRequestLogger),
	))

	return opts
}
