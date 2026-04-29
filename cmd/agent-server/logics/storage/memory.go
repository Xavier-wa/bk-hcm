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
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hcm/cmd/agent-server/logics/auth"
	"hcm/cmd/agent-server/logics/logger"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"

	openaiopt "github.com/openai/openai-go/option"
	openaiembed "trpc.group/trpc-go/trpc-agent-go/knowledge/embedder/openai"
	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/memory/extractor"
	memmysql "trpc.group/trpc-go/trpc-agent-go/memory/mysql"
	memsqlitevec "trpc.group/trpc-go/trpc-agent-go/memory/sqlitevec"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// buildMemoryService constructs the appropriate memory.Service based on the resolved backend.
func buildMemoryService(cfg cc.AgentMemoryStorage, mdl model.Model, aidevGW *cc.AgentModelProvider) (
	memory.Service, error) {

	backend := cfg.ResolveMemoryBackend()

	switch backend {
	case "mysql":
		return buildMySQLMemoryService(cfg, mdl)
	case "sqlitevec":
		return buildSQLiteVecMemoryService(cfg, mdl, aidevGW)
	case "":
		logs.Infof("AGUI memory backend: disabled (no backend configured)")
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported memory backend: %q", backend)
	}
}

// buildMySQLMemoryService constructs a MySQL-backed memory service.
func buildMySQLMemoryService(cfg cc.AgentMemoryStorage, mdl model.Model) (memory.Service, error) {
	dsn := strings.TrimSpace(cfg.DSN)
	if dsn == "" {
		return nil, fmt.Errorf("memory backend=mysql requires DSN")
	}
	opts := []memmysql.ServiceOpt{memmysql.WithMySQLClientDSN(dsn)}
	if cfg.SkipDBInit {
		opts = append(opts, memmysql.WithSkipDBInit(true))
	}
	if name := strings.TrimSpace(cfg.TableName); name != "" {
		opts = append(opts, memmysql.WithTableName(name))
	}
	if cfg.Limit > 0 {
		opts = append(opts, memmysql.WithMemoryLimit(cfg.Limit))
	}
	if cfg.MaxSearchResults > 0 {
		opts = append(opts, memmysql.WithMaxResults(cfg.MaxSearchResults))
	}
	if cfg.AutoExtract && mdl != nil {
		opts = append(opts, memmysql.WithExtractor(buildMemoryExtractor(cfg, mdl)))
		logs.Infof("AGUI memory auto-extract: enabled (policy=%q, messages=%d, interval=%s)",
			cfg.AutoExtractPolicy, cfg.AutoExtractMessages, cfg.AutoExtractInterval)
	}
	svc, err := memmysql.NewService(opts...)
	if err != nil {
		return nil, fmt.Errorf("create AGUI memory service (mysql): %w", err)
	}
	logs.Infof("AGUI memory backend: mysql (table=%q)", cfg.TableName)
	return svc, nil
}

// buildSQLiteVecMemoryService constructs a SQLite+sqlite-vec backed memory service.
// The embedding client shares the aidev gateway's base URL and BK auth middleware,
// so only model name and dimensions need to be specified in the memory config.
//
// Full request URL: {gatewayCfg.BaseURL}/embeddings
// (path suffix appended by the OpenAI Go SDK automatically)
func buildSQLiteVecMemoryService(cfg cc.AgentMemoryStorage, mdl model.Model, aidevGW *cc.AgentModelProvider) (
	memory.Service, error) {

	dbPath := strings.TrimSpace(cfg.DBPath)
	if dbPath == "" {
		return nil, fmt.Errorf("memory backend=sqlitevec requires dbPath")
	}
	if strings.TrimSpace(aidevGW.BaseURL) == "" {
		return nil, fmt.Errorf("memory backend=sqlitevec requires aidev baseURL (embedding shares the LLM gateway)")
	}

	embedCfg := cfg.Embedding
	var embedOpts []openaiembed.Option
	embedOpts = append(embedOpts, openaiembed.WithBaseURL(aidevGW.BaseURL))
	if aidevGW.APIKey != "" {
		embedOpts = append(embedOpts, openaiembed.WithAPIKey(aidevGW.APIKey))
	}
	if aidevGW.AppCode != "" || aidevGW.AppSecret != "" {
		appCode := aidevGW.AppCode
		appSecret := aidevGW.AppSecret
		defaultUser := aidevGW.User
		defaultTicket := aidevGW.BkTicket
		embedOpts = append(embedOpts, openaiembed.WithRequestOptions(
			openaiopt.WithMiddleware(func(r *http.Request, next openaiopt.MiddlewareNext) (*http.Response, error) {
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
	if embedCfg.Model != "" {
		embedOpts = append(embedOpts, openaiembed.WithModel(embedCfg.Model))
	}
	if embedCfg.Dimensions > 0 {
		embedOpts = append(embedOpts, openaiembed.WithDimensions(embedCfg.Dimensions))
	}
	emb := openaiembed.New(embedOpts...)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database %q: %w", dbPath, err)
	}

	var svcOpts []memsqlitevec.ServiceOpt
	svcOpts = append(svcOpts, memsqlitevec.WithEmbedder(emb))
	if name := strings.TrimSpace(cfg.TableName); name != "" {
		svcOpts = append(svcOpts, memsqlitevec.WithTableName(name))
	}
	if cfg.SkipDBInit {
		svcOpts = append(svcOpts, memsqlitevec.WithSkipDBInit(true))
	}
	if cfg.Limit > 0 {
		svcOpts = append(svcOpts, memsqlitevec.WithMemoryLimit(cfg.Limit))
	}
	if cfg.MaxSearchResults > 0 {
		svcOpts = append(svcOpts, memsqlitevec.WithMaxResults(cfg.MaxSearchResults))
	}
	if embedCfg.Dimensions > 0 {
		svcOpts = append(svcOpts, memsqlitevec.WithIndexDimension(embedCfg.Dimensions))
	}
	if cfg.AutoExtract && mdl != nil {
		svcOpts = append(svcOpts, memsqlitevec.WithExtractor(buildMemoryExtractor(cfg, mdl)))
		logs.Infof("AGUI memory auto-extract: enabled (policy=%q, messages=%d, interval=%s)",
			cfg.AutoExtractPolicy, cfg.AutoExtractMessages, cfg.AutoExtractInterval)
	}

	svc, err := memsqlitevec.NewService(db, svcOpts...)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create AGUI memory service (sqlitevec): %w", err)
	}
	logs.Infof("AGUI memory backend: sqlitevec (db=%q, table=%q, model=%q, dimension=%d, gateway=%q)",
		dbPath, cfg.TableName, embedCfg.Model, emb.GetDimensions(), aidevGW.BaseURL)
	return svc, nil
}

// buildMemoryExtractor constructs a MemoryExtractor from the given config.
// Checkers are combined according to AutoExtractPolicy ("any" = OR, "all" = AND).
// When no checker is configured the extractor runs after every Run.
func buildMemoryExtractor(cfg cc.AgentMemoryStorage, mdl model.Model) extractor.MemoryExtractor {
	var checkers []extractor.Checker
	if cfg.AutoExtractMessages > 0 {
		checkers = append(checkers, extractor.CheckMessageThreshold(cfg.AutoExtractMessages))
	}
	if cfg.AutoExtractInterval != "" {
		var interval time.Duration
		if raw := strings.TrimSpace(cfg.AutoExtractInterval); raw != "" {
			if d, err := time.ParseDuration(raw); err == nil {
				interval = d
			}
		}
		checkers = append(checkers, extractor.CheckTimeInterval(interval))
	}
	var opts []extractor.Option
	if len(checkers) > 0 {
		if cfg.AutoExtractPolicy == "all" {
			opts = append(opts, extractor.WithChecker(extractor.ChecksAll(checkers...)))
		} else {
			opts = append(opts, extractor.WithCheckersAny(checkers...))
		}
	}
	if cfg.ExtractPrompt != "" {
		opts = append(opts, extractor.WithPrompt(cfg.ExtractPrompt))
		logs.Infof("AGUI memory extract prompt: custom (%d bytes)", len(cfg.ExtractPrompt))
	}

	// Inject model logger into extractor so that AfterModel callbacks fire for
	// LLM calls made by the background memory extraction worker (which bypasses
	// the Agent-level callback pipeline).
	modelCb := logger.ModelLoggerCallback()
	opts = append(opts, extractor.WithModelCallbacks(modelCb))

	return extractor.NewExtractor(mdl, opts...)
}
