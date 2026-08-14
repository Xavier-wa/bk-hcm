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

// Package storage provides functionality for creating and managing session and memory services.
package storage

import (
	"fmt"
	"strings"

	"hcm/cmd/agent-server/logics/logger"
	"hcm/cmd/agent-server/logics/prompt"
	"hcm/pkg/cc"
	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	sessionmysql "trpc.group/trpc-go/trpc-agent-go/session/mysql"
)

// BuildStorageServices creates session and memory services from the given config,
// returning nil for each service when its backend is not configured.
// mdl is used by the session summarizer and memory extractor; it may be nil
// (in which case summary/extraction is silently disabled even if configured).
// promptStore is optional; when provided, a BeforeModel callback is registered in
// the extractor's model pipeline that reads MemoryExtractKey on every LLM call,
// enabling prompt hot-reload without any extra synchronization.
func BuildStorageServices(mdl model.Model, aidevGW *cc.AgentModelProvider, promptStore *prompt.Store) (
	session.Service, memory.Service, error) {

	cfg := cc.AgentServer().Storage

	var sessionSvc session.Service
	if dsn := strings.TrimSpace(cfg.Session.DSN); dsn != "" {
		opts := []sessionmysql.ServiceOpt{sessionmysql.WithMySQLClientDSN(dsn)}
		if cfg.Session.SkipDBInit {
			opts = append(opts, sessionmysql.WithSkipDBInit(true))
		}
		if pref := strings.TrimSpace(cfg.Session.TablePrefix); pref != "" {
			opts = append(opts, sessionmysql.WithTablePrefix(pref))
		}
		if s := buildSummarizer(cfg.Session.Summary, mdl); s != nil {
			opts = append(opts, sessionmysql.WithSummarizer(s))
		}
		svc, err := sessionmysql.NewService(opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("create AGUI session service: %w", err)
		}
		sessionSvc = svc
		logs.Infof("AGUI session backend: mysql (table_prefix=%q, summary=%v)",
			cfg.Session.TablePrefix, cfg.Session.Summary.Enabled)
	} else {
		logs.Infof("AGUI session backend: inmemory (no DSN configured)")
	}

	memorySvc, err := buildMemoryService(cfg.Memory, mdl, aidevGW, promptStore)
	if err != nil {
		if sessionSvc != nil {
			_ = sessionSvc.Close()
		}
		return nil, nil, err
	}

	// log hook
	if memorySvc != nil {
		memorySvc = logger.NewLoggingMemoryService(memorySvc)
	}

	return sessionSvc, memorySvc, nil
}
