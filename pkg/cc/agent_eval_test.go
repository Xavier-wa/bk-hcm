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

package cc

import "testing"

func TestAgentEvalConfigOmittedIsOff(t *testing.T) {
	cfg := AgentEvalConfig{}
	cfg.trySetDefault()
	if cfg.IsEnabled() {
		t.Fatal("omitted enabled must be off")
	}
	if err := cfg.validate(AgentServerSetting{}); err != nil {
		t.Fatalf("disabled eval must skip strong validate, err: %v", err)
	}
}

func TestAgentEvalConfigEnabledRequiresModelName(t *testing.T) {
	cfg := AgentEvalConfig{Enabled: true}
	cfg.trySetDefault()
	err := cfg.validate(AgentServerSetting{})
	if err == nil {
		t.Fatal("enabled without model.name must fail")
	}
}

func TestAgentEvalConfigZeroSubmitWaitInvalid(t *testing.T) {
	cfg := AgentEvalConfig{
		Enabled:           true,
		SubmitWaitTimeout: "0s",
		Model: AgentEvalModelConfig{
			Name:     "judge",
			Provider: "p1",
		},
	}
	cfg.trySetDefault()
	cfg.SubmitWaitTimeout = "0s"
	setting := AgentServerSetting{
		Providers: []AgentModelProvider{{Name: "p1", BaseURL: "http://x"}},
		Storage:   AgentStorage{Session: AgentSessionStorage{DSN: "user:pass@tcp(h:3306)/db"}},
		Prompt:    AgentPromptConfig{EvalScopePrompt: "scope", EvalRubricPrompt: "rubric"},
	}
	if err := cfg.validate(setting); err == nil {
		t.Fatal("submitWaitTimeout=0 must fail")
	}
}

func TestAiagentRunSweepDefaultDisabled(t *testing.T) {
	cfg := AiagentRunSweepConfig{}
	cfg.trySetDefault()
	if cfg.IsEnabled() {
		t.Fatal("omitted sweep enabled must default off")
	}
	cfg.Enabled = true
	if !cfg.IsEnabled() {
		t.Fatal("enabled=true must turn sweep on")
	}
}

func TestEvalWeightsDefaulted(t *testing.T) {
	cfg := AgentEvalConfig{}
	cfg.trySetDefault()
	if len(cfg.ProcessWeights) != len(DefaultEvalProcessWeights) {
		t.Fatalf("process weights: %+v", cfg.ProcessWeights)
	}
	if cfg.ShutdownDrain != defaultEvalShutdownDrain {
		t.Fatalf("shutdownDrain = %s", cfg.ShutdownDrain)
	}
	if cfg.PassThreshold != defaultEvalPassThreshold {
		t.Fatalf("passThreshold = %d", cfg.PassThreshold)
	}
}
