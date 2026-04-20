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
	"os"
	"strings"
	"sync"
	"time"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"

	openaiopt "github.com/openai/openai-go/option"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	localexec "trpc.group/trpc-go/trpc-agent-go/codeexecutor/local"
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
	skilltool "trpc.group/trpc-go/trpc-agent-go/tool/skill"
	trpcmcp "trpc.group/trpc-go/trpc-mcp-go"
)

// Runtime wraps app.Runtime and adds an idempotent Close.
type Runtime struct {
	AGUIRunner      runner.Runner
	aguiSessionSvc  session.Service // non-nil when MySQL session backend is configured
	aguiMemorySvc   memory.Service  // non-nil when MySQL memory backend is configured
	aguiMCPToolSets []tool.ToolSet  // non-nil when MCP toolsets are configured
	closeOnce       sync.Once
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

// New initialises the Agent Runtime from the given options.
// It constructs the []string args expected by app.NewRuntime from the structured
// Option fields so that callers never need to deal with raw CLI args.
func New() (*Runtime, error) {
	gatewayCfg := cc.AgentServer().AIDev
	aguiCfg := cc.AgentServer().AGUI
	// Compute the effective allowed model list: config overrides platform defaults.
	// Also update aguiCfg so the service layer (models endpoint, resolver) sees the
	// resolved list without having to repeat the defaulting logic.
	if len(aguiCfg.AllowedModels) == 0 {
		defaults := enumor.DefaultAllowedAIModels
		aguiCfg.AllowedModels = make([]string, len(defaults))
		for i, m := range defaults {
			aguiCfg.AllowedModels[i] = string(m)
		}
	}

	// Build one model instance per allowed model name (all share gateway auth config).
	// The per-request resolver selects among them via agent.WithModelName.
	modelsMap := buildAllModels(aguiCfg.AllowedModels, &gatewayCfg)

	// Pick the default model: --model-name flag takes precedence, then first allowed.
	defaultMdl := resolveDefaultModel(strings.TrimSpace(aguiCfg.DefaultModel), aguiCfg.AllowedModels,
		modelsMap, &gatewayCfg)

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

	// Build skill tools from configuration.
	skillTools, err := buildSkillTools()
	if err != nil {
		closeMCPToolSets(mcpToolSets)
		return nil, fmt.Errorf("build skill tools: %w", err)
	}

	systemPrompt := loadPromptFile(aguiCfg.Prompt.SystemPromptFile)
	instruction := loadPromptFile(aguiCfg.Prompt.InstructionFile)

	// When any MCP toolset requires per-request authentication (e.g. type "bkaidev"),
	// disable eager tool loading at construction time. Tools are fetched lazily on
	// the first agent run, at which point the real request context (with bk_ticket)
	// is available so MCP session initialization can authenticate successfully.
	refreshOnRun := hasBKAIDevToolSet()
	agt := newAgentWithModel(defaultMdl, modelsMap, aguiCfg.Stream, systemPrompt, instruction,
		skillTools, mcpToolSets, refreshOnRun)
	runnerOpts := buildRunnerOpts(sessionSvc, memorySvc)
	agUIRunner := runner.NewRunner(agt.Info().Name, agt, runnerOpts...)

	return &Runtime{
		AGUIRunner:      agUIRunner,
		aguiSessionSvc:  sessionSvc,
		aguiMemorySvc:   memorySvc,
		aguiMCPToolSets: mcpToolSets,
	}, nil
}

// loadPromptFile reads a prompt text file and returns its trimmed content.
// Returns an empty string when path is empty or the file cannot be read.
func loadPromptFile(path string) string {
	if path = strings.TrimSpace(path); path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		logs.Warnf("failed to load prompt file %q: %v", path, err)
		return ""
	}
	return strings.TrimSpace(string(data))
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
	return extractor.NewExtractor(mdl, opts...)
}

// buildSummarizer constructs a SessionSummarizer from the given config.
// Returns nil when summary is disabled, no thresholds are configured, or mdl is nil.
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

// buildModel creates a single OpenAI-compatible model instance.
// Used directly by the session summarizer which always uses the default model.
func buildModel(modelName string, gatewayCfg *cc.AIDevConfig) model.Model {
	return openai.New(modelName, buildOpenAIOptions(gatewayCfg)...)
}

// buildAllModels creates one model instance per name in allowedNames, sharing
// the same gateway auth config. All per-request model switches draw from this map.
func buildAllModels(allowedNames []string, gatewayCfg *cc.AIDevConfig) map[string]model.Model {
	opts := buildOpenAIOptions(gatewayCfg)
	m := make(map[string]model.Model, len(allowedNames))
	for _, name := range allowedNames {
		m[name] = openai.New(name, opts...)
	}
	return m
}

// resolveDefaultModel picks the model to use when no per-request model is specified.
// Priority: --model-name flag (if in modelsMap) → first allowed model → new instance from flag.
func resolveDefaultModel(flagModelName string, allowedNames []string, modelsMap map[string]model.Model,
	gatewayCfg *cc.AIDevConfig) model.Model {

	if flagModelName != "" {
		if m, ok := modelsMap[flagModelName]; ok {
			return m
		}
		// Flag names an unlisted model — build it anyway so legacy configs still work.
		mdl := buildModel(flagModelName, gatewayCfg)
		modelsMap[flagModelName] = mdl
		logs.Warnf("model %q is not in allowedModels list but set via --model-name; added to map", flagModelName)
		return mdl
	}
	if len(allowedNames) > 0 {
		return modelsMap[allowedNames[0]]
	}
	return nil
}

// newAgentWithModel assembles the AGUI llm agent.
// defaultMdl is the fallback model used when no per-request model name is specified.
// modelsMap registers all models that can be selected per-request via agent.WithModelName.
// systemPrompt is the GlobalInstruction content (prepended to every LLM request).
// instruction is the per-request task instruction content (appended to every LLM request).
// tools contains individual tool.Tool instances (e.g. skill tools).
// toolSets contains ToolSet instances (e.g. MCP server toolsets).
// refreshOnRun controls whether toolset tool lists are resolved lazily per-run.
func newAgentWithModel(defaultMdl model.Model, modelsMap map[string]model.Model, isStream bool,
	systemPrompt, instruction string, tools []tool.Tool, toolSets []tool.ToolSet, refreshOnRun bool) agent.Agent {

	generationConfig := model.GenerationConfig{
		MaxTokens:   cvt.ValToPtr(4096),
		Temperature: cvt.ValToPtr(0.7),
		Stream:      isStream,
	}

	opts := []llmagent.Option{
		llmagent.WithGenerationConfig(generationConfig),
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
	if len(tools) > 0 {
		opts = append(opts, llmagent.WithTools(tools))
	}
	if len(toolSets) > 0 {
		opts = append(opts, llmagent.WithToolSets(toolSets))
		if refreshOnRun {
			opts = append(opts, llmagent.WithRefreshToolSetsOnRun(true))
		}
	}

	logs.Infof("AGUI agent: models=%d tools=%d toolSets=%d refreshToolSetsOnRun=%v systemPrompt=%v instruction=%v",
		len(modelsMap), len(tools), len(toolSets), refreshOnRun, systemPrompt != "", instruction != "")
	return llmagent.New(cc.AgentServer().AGUI.AppName, opts...)
}

// hasBKAIDevToolSet reports whether any MCP toolset config has type "bkaidev".
func hasBKAIDevToolSet() bool {
	for _, cfg := range cc.AgentServer().Tools.MCPToolSets {
		if strings.EqualFold(strings.TrimSpace(cfg.Type), mcpTypeBKAIDev) {
			return true
		}
	}
	return false
}

// buildMCPToolSets constructs MCP ToolSet instances from the given slice of configs.
// Each config maps 1-to-1 to a mcp.ToolSet. Errors from any entry abort the whole build.
// bkaidevCfg provides BK application credentials for toolsets with Type == "bkaidev".
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
// When cfg.Type is "bkaidev" and bkaidevCfg is non-nil, a per-request
// X-Bkapi-Authorization header is injected using bkaidevCfg credentials and
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

	if strings.EqualFold(strings.TrimSpace(cfg.Type), mcpTypeBKAIDev) {
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
						req.Header.Set(headerBkapiAuthorization,
							bkapiMCPAuthHeaderValue(appCode, appSecret, ticket))
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

	return mcp.NewMCPToolSet(conn, opts...), nil
}

const (
	// mcpTypeBKAIDev is the MCP toolset type that enables automatic
	// X-Bkapi-Authorization header injection for BK AI Dev gateways.
	mcpTypeBKAIDev = "bkaidev"
	// headerBkapiAuthorization is the BK API gateway authentication header name.
	headerBkapiAuthorization = "X-Bkapi-Authorization"
)

// buildSkillTools constructs skill tool instances from the given config.
// Returns nil when cfg is nil or no root directories are configured.
// The returned tools include: load, run, list-docs, and select-docs.
func buildSkillTools() ([]tool.Tool, error) {
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

	exec := localexec.New()
	tools := []tool.Tool{
		skilltool.NewLoadTool(repo),
		skilltool.NewRunTool(repo, exec),
		skilltool.NewListDocsTool(repo),
		skilltool.NewSelectDocsTool(repo),
	}
	logs.Infof("AGUI skill tools registered: root=%q extraDirs=%v skills=%d",
		cfg.Root, cfg.ExtraDirs, len(repo.Summaries()))
	return tools, nil
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
func buildOpenAIOptions(cfg *cc.AIDevConfig) []openai.Option {
	var opts []openai.Option

	if len(cfg.Endpoints) > 0 {
		opts = append(opts, openai.WithBaseURL(cfg.Endpoints[0]))
	}
	if cfg.APIKey != "" {
		opts = append(opts, openai.WithAPIKey(cfg.APIKey))
	}
	if cfg.AppCode != "" || cfg.AppSecret != "" {
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
				r.Header.Set("X-Bkapi-Authorization", bkapiAuthHeaderValue(appCode, appSecret, username, ticket))
				return next(r)
			}),
		))
	}

	opts = append(opts, openai.WithOpenAIOptions(
		openaiopt.WithMiddleware(llmRequestLogger),
	))

	return opts
}

const llmRequestBodyLogLimit = 16 * 1024

func llmRequestLogger(r *http.Request, next openaiopt.MiddlewareNext) (*http.Response, error) {
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			body := string(bodyBytes)
			if len(body) > llmRequestBodyLogLimit {
				body = body[:llmRequestBodyLogLimit] + fmt.Sprintf("... (truncated, total %d bytes)", len(bodyBytes))
			}
			logs.Infof("LLM request: %s %s body=%s", r.Method, r.URL.String(), body)
		} else {
			logs.Warnf("LLM request: failed to read body: %v", err)
		}
	} else {
		logs.Infof("LLM request: %s %s (no body)", r.Method, r.URL.String())
	}
	return next(r)
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
