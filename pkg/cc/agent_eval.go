/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may obtain a copy of the License at http://opensource.org/licenses/MIT
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

package cc

import (
	"fmt"
	"strings"
	"time"
)

const (
	defaultEvalSubmitWaitTimeout   = "1s"
	defaultEvalConcurrency         = 2
	defaultEvalCompensateLookback  = "24h"
	defaultEvalCompensateBatchSize = 20
	defaultEvalContextRunLimit     = 5
	defaultEvalShutdownDrain       = "30s"
	defaultEvalPassThreshold       = 80
	defaultEvalMaxTokens           = 8192
	defaultEvalTemperature         = 0.2
	defaultEvalTrackTTL            = "168h"
	defaultEvalRubricVersion       = "v1"
	defaultSweepInterval           = "10m"
	defaultSweepOrphanThreshold    = "4h"
)

// DefaultEvalProcessWeights is the process-dimension weight table. YAML can override.
var DefaultEvalProcessWeights = map[string]float64{
	"no_text_plan_listing":  0.8,
	"faithfulness":          0.8,
	"guidance_at_interrupt": 0.5,
	"error_handling":        0.5,
	"param_inference":       0.4,
}

// DefaultEvalOutcomeWeights is the outcome-dimension weight table. YAML can override.
var DefaultEvalOutcomeWeights = map[string]float64{
	"conclusion_first": 1.0,
	"outcome_complete": 0.8,
	"concise_closure":  0.5,
	"intent_match":     0.4,
}

// AgentEvalModelConfig is eval.model: the dedicated scoring LLM, not the chat model.
type AgentEvalModelConfig struct {
	// Name is the model name sent to the OpenAI-compatible gateway.
	Name string `yaml:"name"`
	// Provider is the named entry in providers. Empty is invalid when eval is enabled.
	Provider string `yaml:"provider"`
	// MaxTokens caps the completion. Default 8192.
	MaxTokens int `yaml:"maxTokens"`
	// Temperature is in [0, 1]. Default 0.2.
	Temperature float64 `yaml:"temperature"`
}

func (m *AgentEvalModelConfig) trySetDefault() {
	if m.MaxTokens <= 0 {
		m.MaxTokens = defaultEvalMaxTokens
	}
	if m.Temperature < 0 {
		m.Temperature = defaultEvalTemperature
	}
}

// AgentEvalConfig is the agent-server eval: block (sibling of agui / providers).
type AgentEvalConfig struct {
	// Enabled turns evaluation on. Default false.
	Enabled bool `yaml:"enabled"`
	// Model is the dedicated judge; it is not added to agui.modelsMap.
	Model AgentEvalModelConfig `yaml:"model"`
	// Concurrency is the process-local semaphore weight. Default 2.
	Concurrency int64 `yaml:"concurrency"`
	// SubmitWaitTimeout is a Go duration string, same as prompt.syncInterval /
	// tools.toolProxy.refreshInterval. Empty defaults to 1s.
	// 1s–3s is a wait cap; negative (e.g. -1s) waits until a slot is free; 0 is invalid.
	SubmitWaitTimeout string `yaml:"submitWaitTimeout"`
	// CompensateLookback 启动补偿回看窗口。
	// agent-server 启动且 eval.enabled=true 时，按 aiagent_run.updated_at 捞「已终态但还没有
	// eval 行」的缺口；只看 now-lookback 之内的记录。必须小于 trackTTL，否则轨道表已过期、
	// 补评也拼不出上下文。Default 24h。
	CompensateLookback string `yaml:"compensateLookback"`
	// CompensateBatchSize 启动补偿一次最多入队多少条缺口（SubmitBatch 的上限）。
	// 多了下次进程重启再捞。Default 20。
	CompensateBatchSize int `yaml:"compensateBatchSize"`
	// ContextRunLimit is the max ledger rows in a candidate window. Default 5.
	ContextRunLimit int `yaml:"contextRunLimit"`
	// ShutdownDrain 进程退出时等待在途评估 job 跑完的最长时间。
	// Dispatcher.Shutdown 会先取消仍在等槽的 Acquire，再 drain 已开工的 goroutine；
	// 到期仍未结束只打 warn，不阻塞进程退出。Default 30s。
	ShutdownDrain string `yaml:"shutdownDrain"`
	// PassThreshold 运营查询判定「本轮通过」的 quality 门槛。
	// 通过条件：quality_score >= passThreshold 且 redlines 为空。只在查询时计算，
	// 不落 aiagent_run_eval 列。Default 80。
	PassThreshold int `yaml:"passThreshold"`
	// RubricVersion is stored on each eval row. Default v1.
	RubricVersion string `yaml:"rubricVersion"`
	// TrackTTL is the session track event TTL used to validate lookback. Default 168h.
	TrackTTL string `yaml:"trackTTL"`
	// ProcessWeights 过程维权重，key 与判官 JSON dims 一致。空则用 DefaultEvalProcessWeights。
	ProcessWeights map[string]float64 `yaml:"processWeights"`
	// OutcomeWeights 结果维权重。空则用 DefaultEvalOutcomeWeights。
	OutcomeWeights map[string]float64 `yaml:"outcomeWeights"`
}

// IsEnabled reports whether evaluation is turned on.
func (c AgentEvalConfig) IsEnabled() bool {
	return c.Enabled
}

func (c *AgentEvalConfig) trySetDefault() {
	c.Model.trySetDefault()
	if c.Concurrency <= 0 {
		c.Concurrency = defaultEvalConcurrency
	}
	if strings.TrimSpace(c.SubmitWaitTimeout) == "" {
		c.SubmitWaitTimeout = defaultEvalSubmitWaitTimeout
	}
	if strings.TrimSpace(c.CompensateLookback) == "" {
		c.CompensateLookback = defaultEvalCompensateLookback
	}
	if c.CompensateBatchSize <= 0 {
		c.CompensateBatchSize = defaultEvalCompensateBatchSize
	}
	if c.ContextRunLimit <= 0 {
		c.ContextRunLimit = defaultEvalContextRunLimit
	}
	if strings.TrimSpace(c.ShutdownDrain) == "" {
		c.ShutdownDrain = defaultEvalShutdownDrain
	}
	if c.PassThreshold <= 0 {
		c.PassThreshold = defaultEvalPassThreshold
	}
	if strings.TrimSpace(c.RubricVersion) == "" {
		c.RubricVersion = defaultEvalRubricVersion
	}
	if strings.TrimSpace(c.TrackTTL) == "" {
		c.TrackTTL = defaultEvalTrackTTL
	}
	if len(c.ProcessWeights) == 0 {
		c.ProcessWeights = cloneWeightMap(DefaultEvalProcessWeights)
	}
	if len(c.OutcomeWeights) == 0 {
		c.OutcomeWeights = cloneWeightMap(DefaultEvalOutcomeWeights)
	}
}

func (c AgentEvalConfig) validate(s AgentServerSetting) error {
	if !c.IsEnabled() {
		return nil
	}
	if err := c.validateModel(s); err != nil {
		return err
	}
	if err := c.validateDurations(); err != nil {
		return err
	}
	if err := c.validateLimits(); err != nil {
		return err
	}
	return c.validatePromptAndStorage(s)
}

func (c AgentEvalConfig) validateModel(s AgentServerSetting) error {
	if strings.TrimSpace(c.Model.Name) == "" {
		return fmt.Errorf("eval.model.name is required when eval.enabled=true")
	}
	if strings.TrimSpace(c.Model.Provider) == "" {
		return fmt.Errorf("eval.model.provider is required when eval.enabled=true")
	}
	if _, err := s.GetProvider(c.Model.Provider); err != nil {
		return fmt.Errorf("eval.model.provider: %w", err)
	}
	if c.Model.MaxTokens <= 0 {
		return fmt.Errorf("eval.model.maxTokens must be > 0")
	}
	if c.Model.Temperature < 0 || c.Model.Temperature > 1 {
		return fmt.Errorf("eval.model.temperature must be in [0, 1]")
	}
	return nil
}

func (c AgentEvalConfig) validateDurations() error {
	wait, err := parseEvalDuration(c.SubmitWaitTimeout, "eval.submitWaitTimeout")
	if err != nil {
		return err
	}
	if wait == 0 {
		return fmt.Errorf("eval.submitWaitTimeout must not be 0")
	}
	lookback, err := parseEvalDuration(c.CompensateLookback, "eval.compensateLookback")
	if err != nil {
		return err
	}
	if lookback <= 0 {
		return fmt.Errorf("eval.compensateLookback must be positive")
	}
	ttl, err := parseEvalDuration(c.TrackTTL, "eval.trackTTL")
	if err != nil {
		return err
	}
	if ttl <= 0 {
		return fmt.Errorf("eval.trackTTL must be positive")
	}
	if lookback >= ttl {
		return fmt.Errorf("eval.compensateLookback must be less than eval.trackTTL")
	}
	_, err = parseEvalDuration(c.ShutdownDrain, "eval.shutdownDrain")
	return err
}

func (c AgentEvalConfig) validateLimits() error {
	if c.Concurrency < 1 {
		return fmt.Errorf("eval.concurrency must be >= 1")
	}
	if c.ContextRunLimit < 1 {
		return fmt.Errorf("eval.contextRunLimit must be >= 1")
	}
	return nil
}

func (c AgentEvalConfig) validatePromptAndStorage(s AgentServerSetting) error {
	if strings.TrimSpace(s.Storage.Session.DSN) == "" {
		return fmt.Errorf("storage.session.dsn is required when eval.enabled=true")
	}
	if s.Prompt.BKAIDevSyncEnabled() {
		return nil
	}
	if s.Prompt.EvalScopePrompt == "" {
		return fmt.Errorf("prompt.evalScopePromptFile is required when eval.enabled=true")
	}
	if s.Prompt.EvalRubricPrompt == "" {
		return fmt.Errorf("prompt.evalRubricPromptFile is required when eval.enabled=true")
	}
	return nil
}

// SubmitWait returns the parsed submit wait timeout.
func (c AgentEvalConfig) SubmitWait() time.Duration {
	d, _ := parseEvalDuration(c.SubmitWaitTimeout, "eval.submitWaitTimeout")
	return d
}

// CompensateLookbackDur 返回启动补偿回看窗口（updated_at >= now-lookback 的终态缺口）。
func (c AgentEvalConfig) CompensateLookbackDur() time.Duration {
	d, _ := parseEvalDuration(c.CompensateLookback, "eval.compensateLookback")
	return d
}

// ShutdownDrainDur 返回进程退出时 drain 在途评估 job 的最长等待。
func (c AgentEvalConfig) ShutdownDrainDur() time.Duration {
	d, _ := parseEvalDuration(c.ShutdownDrain, "eval.shutdownDrain")
	return d
}

// AiagentRunSweepConfig is the orphan sweep for aiagent_run.
type AiagentRunSweepConfig struct {
	// Enabled turns the sweep cron on. Default false；yaml 里显式 enabled: true 才注册。
	Enabled bool `yaml:"enabled"`
	// Interval is the cron period. Default 10m.
	Interval string `yaml:"interval"`
	// OrphanThreshold is how long a running row may stay before becoming unknown. Default 4h.
	OrphanThreshold string `yaml:"orphanThreshold"`
}

// IsEnabled reports whether sweep is on.
func (c AiagentRunSweepConfig) IsEnabled() bool {
	return c.Enabled
}

func (c *AiagentRunSweepConfig) trySetDefault() {
	if strings.TrimSpace(c.Interval) == "" {
		c.Interval = defaultSweepInterval
	}
	if strings.TrimSpace(c.OrphanThreshold) == "" {
		c.OrphanThreshold = defaultSweepOrphanThreshold
	}
}

func (c AiagentRunSweepConfig) validate() error {
	if _, err := parseEvalDuration(c.Interval, "aiagentRunSweep.interval"); err != nil {
		return err
	}
	d, err := parseEvalDuration(c.OrphanThreshold, "aiagentRunSweep.orphanThreshold")
	if err != nil {
		return err
	}
	if d <= 0 {
		return fmt.Errorf("aiagentRunSweep.orphanThreshold must be positive")
	}
	return nil
}

// IntervalDur returns the parsed sweep interval.
func (c AiagentRunSweepConfig) IntervalDur() time.Duration {
	d, _ := parseEvalDuration(c.Interval, "aiagentRunSweep.interval")
	return d
}

// OrphanThresholdDur returns the parsed orphan threshold.
func (c AiagentRunSweepConfig) OrphanThresholdDur() time.Duration {
	d, _ := parseEvalDuration(c.OrphanThreshold, "aiagentRunSweep.orphanThreshold")
	return d
}

func parseEvalDuration(raw, field string) (time.Duration, error) {
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", field, raw, err)
	}
	return d, nil
}

func cloneWeightMap(src map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
