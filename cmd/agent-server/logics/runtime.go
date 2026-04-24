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

// Package logics holds the business-logic layer for agent-server: Runtime lifecycle,
// Channel management, and version information.
package logics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"

	openaiopt "github.com/openai/openai-go/option"
	"github.com/tidwall/gjson"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	openaiembed "trpc.group/trpc-go/trpc-agent-go/knowledge/embedder/openai"
	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/memory/extractor"
	memmysql "trpc.group/trpc-go/trpc-agent-go/memory/mysql"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session"
	sessionmysql "trpc.group/trpc-go/trpc-agent-go/session/mysql"
	"trpc.group/trpc-go/trpc-agent-go/session/summary"
	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/mcp"
	trpcmcp "trpc.group/trpc-go/trpc-mcp-go"
)

// Runtime wraps app.Runtime and adds an idempotent Close.
type Runtime struct {
	AGUIRunner        runner.Runner
	aguiSessionSvc    session.Service // non-nil when MySQL session backend is configured
	aguiMemorySvc     memory.Service  // non-nil when MySQL memory backend is configured
	aguiMCPToolSets   []tool.ToolSet  // non-nil when MCP toolsets are configured
	dynamicToolFilter tool.FilterFunc // non-nil when dynamic tool loading is enabled
	closeOnce         sync.Once
}

// DynamicToolFilter returns the dynamic tool filter function, or nil if not enabled.
func (rt *Runtime) DynamicToolFilter() tool.FilterFunc {
	return rt.dynamicToolFilter
}

// SessionSvc returns the session service used by the AGUI runner.
// Returns nil when the MySQL session backend is not configured (in-memory mode).
func (rt *Runtime) SessionSvc() session.Service {
	return rt.aguiSessionSvc
}

// MemorySvc returns the memory service used by the AGUI runner.
// Returns nil when the MySQL memory backend is not configured.
func (rt *Runtime) MemorySvc() memory.Service {
	return rt.aguiMemorySvc
}

// New initialises the Agent Runtime from the global configuration (cc.AgentServer).
// It wires up model providers, storage services, MCP toolsets, skills and the AGUI runner.
func New() (*Runtime, error) {
	aguiCfg := cc.AgentServer().AGUI
	providers := cc.AgentServer().GetProviders()
	toolsCfg := cc.AgentServer().Tools

	allowedModels := resolveAllowedModels(aguiCfg)
	// Register operator-supplied context windows for private models so that
	// the framework's token tailoring can resolve them correctly.
	if cw := aguiCfg.ModelContextWindows(); len(cw) > 0 {
		model.RegisterModelContextWindows(cw)
		logs.Infof("registered custom model context windows: %v", cw)
	}

	// Build one model instance per allowed model name.
	// Each model is associated with its provider's gateway config; models without
	// an explicit provider use the default "aidev" provider.
	modelsMap, err := buildAllModels(allowedModels, aguiCfg.ModelProviderMapping(), providers)
	if err != nil {
		return nil, fmt.Errorf("build all models: %w", err)
	}

	// Pick the default model: --model-name flag takes precedence, then first allowed.
	defaultMdl, err := resolveDefaultModel(strings.TrimSpace(aguiCfg.DefaultModel), allowedModels,
		modelsMap, aguiCfg.ModelProviderMapping(), providers)
	if err != nil {
		return nil, fmt.Errorf("resolve default model: %w", err)
	}

	// Share the default model with the session summarizer (both need the same LLM endpoint).
	sessionSvc, memorySvc, err := buildStorageServices(defaultMdl)
	if err != nil {
		return nil, err
	}

	// Build MCP toolsets from configuration.
	mcpToolSets, err := buildMCPToolSets()
	if err != nil {
		return nil, fmt.Errorf("build MCP toolsets: %w", err)
	}

	var dynFilter tool.FilterFunc
	if d := toolsCfg.DynamicToolLoading; d != nil && d.Enabled {
		var err error
		dynFilter, err = buildDynamicToolFilter(d, mcpToolSets, providers)
		if err != nil {
			closeMCPToolSets(mcpToolSets)
			return nil, err
		}
	}

	// Build skill repository from configuration.
	skillRepo, err := buildSkillRepo()
	if err != nil {
		closeMCPToolSets(mcpToolSets)
		return nil, fmt.Errorf("build skill repo: %w", err)
	}

	// When any MCP toolset requires per-request authentication (e.g. type "bkaidev"),
	// disable eager tool loading at construction time. Tools are fetched lazily on
	// the first agent run, at which point the real request context (with bk_ticket)
	// is available so MCP session initialization can authenticate successfully.
	refreshOnRun := hasBKAIDevToolSet()
	agt := newAgentWithModel(defaultMdl, modelsMap, aguiCfg.Stream,
		aguiCfg.Prompt.SystemPrompt, aguiCfg.Prompt.Instruction, skillRepo, mcpToolSets, refreshOnRun)
	runnerOpts := buildRunnerOpts(sessionSvc, memorySvc)
	agUIRunner := runner.NewRunner(agt.Info().Name, agt, runnerOpts...)

	return &Runtime{
		AGUIRunner:        agUIRunner,
		aguiSessionSvc:    sessionSvc,
		aguiMemorySvc:     memorySvc,
		aguiMCPToolSets:   mcpToolSets,
		dynamicToolFilter: dynFilter,
	}, nil
}

// resolveAllowedModels returns the configured allowed model names,
// falling back to platform defaults when none are configured.
func resolveAllowedModels(aguiCfg cc.AgentAGUI) []string {
	allowedModels := aguiCfg.AllowedModelNames()
	if len(allowedModels) == 0 {
		defaults := enumor.DefaultAllowedAIModels
		allowedModels = make([]string, len(defaults))
		for i, m := range defaults {
			allowedModels[i] = string(m)
		}
	}
	return allowedModels
}

// buildDynamicToolFilter builds the dynamic tool filter from the given config.
// It performs per-invocation BM25/keyword retrieval so the LLM only sees relevant MCP tools.
func buildDynamicToolFilter(cfg *cc.AgentDynamicToolLoadingConfig, mcpToolSets []tool.ToolSet,
	providers map[string]*cc.AgentModelProvider) (tool.FilterFunc, error) {

	aidevGW, err := resolveProviderConfig(constant.DefaultProviderName, providers)
	if err != nil {
		return nil, fmt.Errorf("resolve default provider: %w, provider name: %s", err, constant.DefaultProviderName)
	}
	idx := buildToolIndex(cfg, aidevGW)
	lazy := &lazyToolIndex{
		mcpToolSets:        mcpToolSets,
		toolTags:           cfg.ToolTags,
		scoreThreshold:     cfg.ScoreThreshold,
		topN:               cfg.TopN,
		queryContextWindow: cfg.QueryContextWindow,
		index:              idx,
	}
	logs.Infof("dynamic tool loading: enabled (strategy=%s, topN=%d, scoreThreshold=%.2f, tags=%d)",
		cfg.Strategy, cfg.TopN, cfg.ScoreThreshold, len(cfg.ToolTags))
	return makeDynamicToolFilter(lazy), nil
}

// buildStorageServices creates MySQL-backed session and memory services when DSNs are
// configured, returning nil for each service when its DSN is empty.
// mdl is used by the session summarizer when summary is enabled; it may be nil
// (in which case summary is silently disabled even if configured).
func buildStorageServices(mdl model.Model) (session.Service, memory.Service, error) {
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

	var memorySvc memory.Service
	if dsn := strings.TrimSpace(cfg.Memory.DSN); dsn != "" {
		opts := []memmysql.ServiceOpt{memmysql.WithMySQLClientDSN(dsn)}
		if cfg.Memory.SkipDBInit {
			opts = append(opts, memmysql.WithSkipDBInit(true))
		}
		if name := strings.TrimSpace(cfg.Memory.TableName); name != "" {
			opts = append(opts, memmysql.WithTableName(name))
		}
		if cfg.Memory.Limit > 0 {
			opts = append(opts, memmysql.WithMemoryLimit(cfg.Memory.Limit))
		}
		if cfg.Memory.AutoExtract && mdl != nil {
			opts = append(opts, memmysql.WithExtractor(buildMemoryExtractor(cfg.Memory, mdl)))
			logs.Infof("AGUI memory auto-extract: enabled (policy=%q, messages=%d, interval=%s)",
				cfg.Memory.AutoExtractPolicy, cfg.Memory.AutoExtractMessages, cfg.Memory.AutoExtractInterval)
		}
		svc, err := memmysql.NewService(opts...)
		if err != nil {
			if sessionSvc != nil {
				_ = sessionSvc.Close()
			}
			return nil, nil, fmt.Errorf("create AGUI memory service: %w", err)
		}
		memorySvc = svc
		logs.Infof("AGUI memory backend: mysql (table=%q)", cfg.Memory.TableName)
	} else {
		logs.Infof("AGUI memory backend: disabled (no DSN configured)")
	}

	return sessionSvc, memorySvc, nil
}

// buildEmbeddingClient constructs an OpenAI-compatible embedder from gateway and embedding configs.
// Supports both APIKey auth and BK application auth (X-Bkapi-Authorization header injection).
// Used by both memory service and EmbeddingIndex tool retrieval.
func buildEmbeddingClient(provider *cc.AgentModelProvider, embedCfg *cc.AgentEmbeddingConfig) *openaiembed.Embedder {
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
			openaiopt.WithMiddleware(func(r *http.Request, next openaiopt.MiddlewareNext) (*http.Response, error) {
				username = BKUsernameFromContext(r.Context())
				ticket := BKTicketFromContext(r.Context())
				if ticket == "" {
					ticket = defaultTicket
				}
				r.Header.Set(constant.BKGWAuthKey, bkapiAuthHeaderValue(appCode, appSecret, username, ticket))
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
	return extractor.NewExtractor(mdl, opts...)
}

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

// buildRunnerOpts assembles runner.Option slice from the provided services.
// Nil services are omitted so the runner applies its own defaults.
func buildRunnerOpts(sessionSvc session.Service, memorySvc memory.Service) []runner.Option {
	var opts []runner.Option
	if sessionSvc != nil {
		opts = append(opts, runner.WithSessionService(sessionSvc))
	}
	if memorySvc != nil {
		opts = append(opts, runner.WithMemoryService(memorySvc))
	}
	return opts
}

// resolveProviderConfig looks up the gateway config for a named provider.
// Returns an error if the provider name is empty or not found in the map.
func resolveProviderConfig(providerName string, providers map[string]*cc.AgentModelProvider) (
	*cc.AgentModelProvider, error) {

	if providerName == "" {
		return nil, fmt.Errorf("provider name is empty")
	}

	if cfg, ok := providers[providerName]; ok {
		return cfg, nil
	}
	return nil, fmt.Errorf("provider %q not found", providerName)
}

// buildModelWithConfig creates a single OpenAI-compatible model instance
// using the given gateway config.
func buildModelWithConfig(modelName string, gatewayCfg *cc.AgentModelProvider) model.Model {
	opts := buildOpenAIOptions(gatewayCfg)
	opts = append(opts, openai.WithEnableTokenTailoring(true))
	return openai.New(modelName, opts...)
}

// buildAllModels creates one model instance per name in allowedNames.
// Each model is wired to its provider's gateway config via modelProviders mapping.
// An error is returned when a model's mapped provider is missing or empty.
func buildAllModels(allowedNames []string, modelProviders map[string]string,
	providers map[string]*cc.AgentModelProvider) (map[string]model.Model, error) {

	m := make(map[string]model.Model, len(allowedNames))
	for _, name := range allowedNames {
		gatewayCfg, err := resolveProviderConfig(modelProviders[name], providers)
		if err != nil {
			return nil, err
		}
		m[name] = buildModelWithConfig(name, gatewayCfg)
	}
	return m, nil
}

// resolveDefaultModel picks the model to use when no per-request model is specified.
// Priority: --model-name flag (if in modelsMap) → new instance from flag → first allowed model.
func resolveDefaultModel(flagModelName string, allowedNames []string, modelsMap map[string]model.Model,
	modelProviders map[string]string, providers map[string]*cc.AgentModelProvider) (model.Model, error) {

	if flagModelName != "" {
		if m, ok := modelsMap[flagModelName]; ok {
			return m, nil
		}
		// Flag names an unlisted model — build it anyway so legacy configs still work.
		gatewayCfg, err := resolveProviderConfig(modelProviders[flagModelName], providers)
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

// newAgentWithModel assembles the AGUI llm agent.
// defaultMdl is the fallback model used when no per-request model name is specified.
// modelsMap registers all models that can be selected per-request via agent.WithModelName.
// systemPrompt is the GlobalInstruction content (prepended to every LLM request).
// instruction is the per-request task instruction content (appended to every LLM request).
// skillRepo is the optional skill repository for progressive skill loading (may be nil).
// toolSets contains ToolSet instances (e.g. MCP server toolsets).
// refreshOnRun controls whether toolset tool lists are resolved lazily per-run.
func newAgentWithModel(defaultMdl model.Model, modelsMap map[string]model.Model, isStream bool,
	systemPrompt, instruction string, skillRepo skillpkg.Repository, toolSets []tool.ToolSet,
	refreshOnRun bool) agent.Agent {

	generationConfig := model.GenerationConfig{
		MaxTokens:   cvt.ValToPtr(38000),
		Temperature: cvt.ValToPtr(0.7),
		Stream:      isStream,
	}

	opts := []llmagent.Option{
		llmagent.WithGenerationConfig(generationConfig),
		llmagent.WithMaxLLMCalls(constant.DefaultMaxLLMCalls),
		llmagent.WithMaxToolIterations(constant.DefaultMaxToolIterations),
		llmagent.WithAddCurrentTime(true),
		llmagent.WithTimezone("Asia/Shanghai"),
		llmagent.WithAddSessionSummary(true),
		llmagent.WithMaxHistoryRuns(constant.DefaultMaxHistoryRuns),
		llmagent.WithPreloadMemory(constant.DefaultPreloadMemoryLimit),
	}
	if systemPrompt != "" {
		opts = append(opts, llmagent.WithGlobalInstruction(systemPrompt))
	}
	if instruction != "" {
		opts = append(opts, llmagent.WithInstruction(instruction))
	}
	if defaultMdl != nil {
		opts = append(opts, llmagent.WithModel(defaultMdl))
	}
	if len(modelsMap) > 0 {
		opts = append(opts, llmagent.WithModels(modelsMap))
	}
	if skillRepo != nil {
		opts = append(opts, llmagent.WithSkills(skillRepo))
	}
	if len(toolSets) > 0 {
		opts = append(opts, llmagent.WithToolSets(toolSets))
		if refreshOnRun {
			opts = append(opts, llmagent.WithRefreshToolSetsOnRun(true))
		}
	}

	opts = append(opts, llmagent.WithToolCallbacks(makeToolLogger()))

	modelCb := makeModelLogger()
	modelCb.BeforeModel = append(modelCb.BeforeModel, makeHistoricalToolResultFilter())
	opts = append(opts, llmagent.WithModelCallbacks(modelCb))

	logs.Infof("AGUI agent: models=%d skills=%v toolSets=%d refreshToolSetsOnRun=%v systemPrompt=%v instruction=%v",
		len(modelsMap), skillRepo != nil, len(toolSets), refreshOnRun, systemPrompt != "", instruction != "")
	return llmagent.New(cc.AgentServer().AGUI.AppName, opts...)
}

// hasBKAIDevToolSet reports whether any MCP toolset config has type "bkaidev".
func hasBKAIDevToolSet() bool {
	for _, cfg := range cc.AgentServer().Tools.MCPToolSets {
		if strings.EqualFold(strings.TrimSpace(cfg.Type), constant.MCPTypeBKAIDev) {
			return true
		}
	}
	return false
}

// buildMCPToolSets constructs MCP ToolSet instances from the global configuration.
// Each config maps 1-to-1 to a mcp.ToolSet. Errors from any entry abort the whole build.
// BK application credentials for toolsets with Type == "bkaidev" are read from cc.AgentServer().Tools.BKAIDev.
func buildMCPToolSets() ([]tool.ToolSet, error) {
	cfgs := cc.AgentServer().Tools.MCPToolSets

	sets := make([]tool.ToolSet, 0, len(cfgs))
	for _, cfg := range cfgs {
		ts, err := buildOneMCPToolSet(cfg)
		if err != nil {
			closeMCPToolSets(sets)
			return nil, fmt.Errorf("toolset %q: %w", cfg.Name, err)
		}
		sets = append(sets, ts)
		logs.Infof("AGUI MCP toolset registered: name=%q type=%q transport=%q serverUrl=%q",
			cfg.Name, cfg.Type, cfg.Transport, cfg.ServerURL)
	}
	return sets, nil
}

// buildOneMCPToolSet constructs a single MCP ToolSet from the given config.
// When cfg.Type is "bkaidev" and the global BKAIDev config is non-nil, a per-request
// X-Bkapi-Authorization header is injected using BKAIDev credentials and
// the bk_ticket resolved from the request context at call time.
func buildOneMCPToolSet(cfg cc.AgentMCPToolSet) (tool.ToolSet, error) {
	conn := mcp.ConnectionConfig{
		Transport: strings.TrimSpace(cfg.Transport),
		ServerURL: strings.TrimSpace(cfg.ServerURL),
		Headers:   cfg.Headers,
		Command:   strings.TrimSpace(cfg.Command),
		Args:      cfg.Args,
	}

	if raw := strings.TrimSpace(cfg.Timeout); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			conn.Timeout = d
		}
	}

	opts := make([]mcp.ToolSetOption, 0, 4)
	if name := strings.TrimSpace(cfg.Name); name != "" {
		opts = append(opts, mcp.WithName(name))
	}

	if f := cfg.Filter; f != nil && len(f.Names) > 0 {
		var filterFunc tool.FilterFunc
		if strings.EqualFold(strings.TrimSpace(f.Mode), "exclude") {
			filterFunc = tool.NewExcludeToolNamesFilter(f.Names...)
		} else {
			filterFunc = tool.NewIncludeToolNamesFilter(f.Names...)
		}
		opts = append(opts, mcp.WithToolFilterFunc(filterFunc))
	}

	if r := cfg.Reconnect; r != nil && r.Enabled {
		attempts := r.MaxAttempts
		if attempts <= 0 {
			attempts = 3
		}
		opts = append(opts, mcp.WithSessionReconnect(attempts))
	}

	if strings.EqualFold(strings.TrimSpace(cfg.Type), constant.MCPTypeBKAIDev) {
		if cc.AgentServer().Tools.BKAIDev == nil {
			logs.Warnf("AGUI MCP toolset %q: type=bkaidev but tools.bkAIDev config is nil, "+
				"X-Bkapi-Authorization will NOT be injected", cfg.Name)
		} else {
			bkaidevCfg := cc.AgentServer().Tools.BKAIDev
			appCode := bkaidevCfg.AppCode
			appSecret := bkaidevCfg.AppSecret
			logs.Infof("AGUI MCP toolset %q: bkaidev auth hook registered (appCode=%q)",
				cfg.Name, appCode)
			opts = append(opts, mcp.WithMCPOptions(
				trpcmcp.WithHTTPBeforeRequest(func(ctx context.Context, req *http.Request) error {
					ticket := BKTicketFromContext(ctx)
					if ticket != "" {
						req.Header.Set(constant.BKGWAuthKey,
							bkapiMCPAuthHeaderValue(appCode, appSecret, BKUsernameFromContext(ctx), ticket))
						logs.Infof("bkaidev MCP hook: injected auth header for %s %s",
							req.Method, req.URL.Path)
					} else {
						logs.Warnf("bkaidev MCP hook: no bk_ticket in context for %s %s, "+
							"skipping auth header injection", req.Method, req.URL.Path)
					}
					return nil
				}),
			))
		}
	}

	// Log HTTP >=400 response bodies: trpc-mcp-go does not attach body to errors on non-200.
	// See trpcmcp streamable_client.send(). Disable via env AGENT_SERVER_MCP_HTTP_LOG_ERROR_BODY=0.
	opts = append(opts, mcp.WithMCPOptions(
		trpcmcp.WithHTTPReqHandler(newMCPHTTPLoggingHandler(trpcmcp.NewDefaultHTTPReqHandler(), cfg.Name)),
	))

	return mcp.NewMCPToolSet(conn, opts...), nil
}

// buildSkillRepo constructs a filesystem-backed skill repository from the given config.
// Returns nil when cfg is nil or no root directories are configured.
func buildSkillRepo() (skillpkg.Repository, error) {
	cfg := cc.AgentServer().Tools.Skills
	if cfg == nil {
		return nil, nil
	}

	roots := make([]string, 0, 1+len(cfg.ExtraDirs))
	if r := strings.TrimSpace(cfg.Root); r != "" {
		roots = append(roots, r)
	}
	for _, d := range cfg.ExtraDirs {
		if d = strings.TrimSpace(d); d != "" {
			roots = append(roots, d)
		}
	}
	if len(roots) == 0 {
		return nil, nil
	}

	repo, err := skillpkg.NewFSRepository(roots...)
	if err != nil {
		return nil, fmt.Errorf("create skill repository: %w", err)
	}

	logs.Infof("AGUI skill repo loaded: root=%q extraDirs=%v skills=%d",
		cfg.Root, cfg.ExtraDirs, len(repo.Summaries()))
	return repo, nil
}

// closeMCPToolSets closes a slice of ToolSets, logging any errors.
func closeMCPToolSets(sets []tool.ToolSet) {
	for _, ts := range sets {
		if err := ts.Close(); err != nil {
			logs.Warnf("close MCP toolset %q: %v", ts.Name(), err)
		}
	}
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
				username := BKUsernameFromContext(r.Context())
				if username == "" {
					username = defaultUser
				}
				ticket := BKTicketFromContext(r.Context())
				if ticket == "" {
					ticket = defaultTicket
				}
				r.Header.Set(constant.BKGWAuthKey, bkapiAuthHeaderValue(appCode, appSecret, username, ticket))
				return next(r)
			}),
		))
	}

	opts = append(opts, openai.WithOpenAIOptions(
		openaiopt.WithMiddleware(llmRequestLogger),
	))

	return opts
}

func llmRequestLogger(r *http.Request, next openaiopt.MiddlewareNext) (*http.Response, error) {
	logBodyLimit := constant.DefaultLLMRequestBodyLogLimit
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			body := string(bodyBytes)

			logLLMToolsSummary(body)
			logLLMTokenConfig(body)

			// Estimate input tokens: roughly chars/4 for English, chars/2 for Chinese;
			// use chars/4 as a conservative lower-bound heuristic for mixed content.
			var estTokens int
			for _, msg := range gjson.Get(body, "messages").Array() {
				estTokens += len(msg.Get("content").String()) / 4
			}
			estTokens += len(body) / 10 // account for system prompts, tool definitions, etc.

			if len(body) > logBodyLimit {
				body = body[:logBodyLimit] + fmt.Sprintf("... (truncated, total %d bytes)", len(bodyBytes))
			}
			logs.Infof("LLM request: %s %s body=%s est_input_tokens≈%d", r.Method, r.URL.String(), body, estTokens)
		} else {
			logs.Warnf("LLM request: failed to read body: %v", err)
		}
	} else {
		logs.Infof("LLM request: %s %s (no body)", r.Method, r.URL.String())
	}
	return next(r)
}

// logLLMToolsSummary extracts tool names from the OpenAI request JSON and logs
// a compact summary so operators can verify dynamic tool filtering at a glance.
func logLLMToolsSummary(body string) {
	tools := gjson.Get(body, "tools")
	if !tools.Exists() {
		return
	}
	arr := tools.Array()
	var mcpNames, otherNames []string
	for _, t := range arr {
		name := t.Get("function.name").String()
		if name == "" {
			continue
		}
		if strings.HasPrefix(name, "skill_") || strings.HasPrefix(name, "transfer_to_") ||
			name == "knowledge_search" || name == "agentic_knowledge_search" {
			otherNames = append(otherNames, name)
		} else {
			mcpNames = append(mcpNames, name)
		}
	}
	logs.Infof("LLM request tools: total=%d, mcp=%d %v, framework/skill=%d %v",
		len(mcpNames)+len(otherNames), len(mcpNames), mcpNames, len(otherNames), otherNames)
}

// logLLMTokenConfig extracts token budget fields from the OpenAI request JSON
// to help diagnose max_tokens issues with upstream API gateways.
func logLLMTokenConfig(body string) {
	model := gjson.Get(body, "model").String()
	maxTokens := gjson.Get(body, "max_tokens")
	maxCompletionTokens := gjson.Get(body, "max_completion_tokens")
	stream := gjson.Get(body, "stream")

	msgCount := len(gjson.Get(body, "messages").Array())
	var inputChars int
	for _, msg := range gjson.Get(body, "messages").Array() {
		inputChars += len(msg.Get("content").String())
	}

	logs.Infof("LLM request token config: model=%s, messages=%d, input_chars≈%d, stream=%v, "+
		"max_tokens=%v (present=%v), max_completion_tokens=%v (present=%v)",
		model, msgCount, inputChars, stream.String(),
		maxTokens.String(), maxTokens.Exists(),
		maxCompletionTokens.String(), maxCompletionTokens.Exists())
}

// Close releases all resources held by the Runtime. It is safe to call multiple
// times; the underlying close is executed exactly once.
func (rt *Runtime) Close() error {
	var err error
	rt.closeOnce.Do(func() {
		if rt.aguiMemorySvc != nil {
			if cerr := rt.aguiMemorySvc.Close(); cerr != nil {
				logs.Warnf("close AGUI memory service: %v", cerr)
			}
		}
		if rt.aguiSessionSvc != nil {
			if cerr := rt.aguiSessionSvc.Close(); cerr != nil {
				logs.Warnf("close AGUI session service: %v", cerr)
			}
		}
		closeMCPToolSets(rt.aguiMCPToolSets)
	})
	return err
}
