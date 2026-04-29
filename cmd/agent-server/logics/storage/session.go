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

package storage

import (
	"strings"
	"time"

	"hcm/pkg/cc"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session/summary"
)

// buildSummarizer constructs a SessionSummarizer from the given config.
// Returns nil when summary is disabled or mdl is nil.
// When no thresholds are configured, a default event threshold of 20 is used.
func buildSummarizer(cfg cc.AgentSessionSummary, mdl model.Model) summary.SessionSummarizer {
	if !cfg.Enabled || mdl == nil {
		return nil
	}

	checkers := make([]summary.Checker, 0, 3)
	if cfg.EventThreshold > 0 {
		checkers = append(checkers, summary.CheckEventThreshold(cfg.EventThreshold))
	}
	if cfg.TokenThreshold > 0 {
		checkers = append(checkers, summary.CheckTokenThreshold(cfg.TokenThreshold))
	}
	if cfg.IdleThreshold != "" {
		var idle time.Duration
		if raw := strings.TrimSpace(cfg.IdleThreshold); raw != "" {
			if d, err := time.ParseDuration(raw); err == nil {
				idle = d
			}
		}
		checkers = append(checkers, summary.CheckTimeThreshold(idle))
	}
	if len(checkers) == 0 {
		// No condition configured — fall back to a sensible default.
		checkers = append(checkers, summary.CheckEventThreshold(20))
	}

	opts := make([]summary.Option, 0, 3)
	opts = append(opts, summary.WithName(cc.AgentServer().AGUI.AppName))
	if cfg.MaxWords > 0 {
		opts = append(opts, summary.WithMaxSummaryWords(cfg.MaxWords))
	}
	policy := strings.ToLower(strings.TrimSpace(cfg.Policy))
	if policy == "all" {
		opts = append(opts, summary.WithChecksAll(checkers...))
	} else {
		opts = append(opts, summary.WithChecksAny(checkers...))
	}

	return summary.NewSummarizer(mdl, opts...)
}
