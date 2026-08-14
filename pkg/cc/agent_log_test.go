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

import (
	"testing"

	"hcm/pkg/criteria/constant"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// TestAgentLogOption_Unmarshal ensures the agent-only option and the inlined common
// log options are both read from the same "log" yaml block.
func TestAgentLogOption_Unmarshal(t *testing.T) {
	const content = `
log:
  logDir: "/data/hcm/logs"
  verbosity: 3
  llmRequestBodyLogLimit: 4096
`
	setting := new(AgentServerSetting)
	assert.NoError(t, yaml.Unmarshal([]byte(content), setting))
	assert.Equal(t, "/data/hcm/logs", setting.Log.LogDir)
	assert.Equal(t, uint(3), setting.Log.Verbosity)
	assert.Equal(t, 4096, setting.Log.LLMRequestBodyLogLimit)
}

func TestAgentLogOption_TrySetDefault(t *testing.T) {
	testCases := []struct {
		name     string
		limit    int
		expected int
	}{
		{
			name:     "not configured falls back to default",
			limit:    0,
			expected: constant.DefaultLLMRequestBodyLogLimit,
		},
		{
			name:     "negative value falls back to default",
			limit:    -1,
			expected: constant.DefaultLLMRequestBodyLogLimit,
		},
		{
			name:     "configured value is kept",
			limit:    4096,
			expected: 4096,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opt := &AgentLogOption{LLMRequestBodyLogLimit: tc.limit}
			opt.trySetDefault()
			assert.Equal(t, tc.expected, opt.LLMRequestBodyLogLimit)

			setting := AgentServerSetting{Log: AgentLogOption{LLMRequestBodyLogLimit: tc.limit}}
			assert.Equal(t, tc.expected, setting.GetLLMRequestBodyLogLimit())
		})
	}
}
