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

package tool

import (
	"testing"

	"hcm/pkg/criteria/constant"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func TestToolIntentSchemaProperty(t *testing.T) {
	prop := ToolIntentSchemaProperty()

	assert.Equal(t, "string", prop.Type)
	// 描述必须要求「本次」调用的场景化目的，并明示同一工具在不同上下文要写出不同文案。
	assert.Contains(t, prop.Description, "本次")
	assert.Contains(t, prop.Description, "不同上下文")
	assert.Contains(t, prop.Description, "不要复述工具名")
	// 不得退化成可套用的占位符模板（如「正在搜索「{query}」」），否则同工具不同目的会失真。
	assert.NotContains(t, prop.Description, "{")

	// 每次返回独立对象，调用方写进不同工具的 Properties 后不会互相串改。
	assert.NotSame(t, prop, ToolIntentSchemaProperty())
}

func TestDeclWithToolIntent(t *testing.T) {
	t.Run("adds optional field and keeps required untouched", func(t *testing.T) {
		decl := &tool.Declaration{
			Name: "skill_load",
			InputSchema: &tool.Schema{
				Type:     "object",
				Required: []string{"skill"},
				Properties: map[string]*tool.Schema{
					"skill": {Type: "string"},
				},
			},
		}

		got := DeclWithToolIntent(decl)

		assert.Contains(t, got.InputSchema.Properties, constant.ToolIntentArgKey)
		assert.Contains(t, got.InputSchema.Properties, "skill")
		assert.Equal(t, []string{"skill"}, got.InputSchema.Required)
		assert.NotContains(t, got.InputSchema.Required, constant.ToolIntentArgKey)
	})

	t.Run("does not mutate the inner properties map", func(t *testing.T) {
		inner := map[string]*tool.Schema{"skill": {Type: "string"}}
		decl := &tool.Declaration{
			Name:        "skill_load",
			InputSchema: &tool.Schema{Type: "object", Properties: inner},
		}

		DeclWithToolIntent(decl)

		assert.NotContains(t, inner, constant.ToolIntentArgKey)
		assert.Len(t, inner, 1)
	})

	t.Run("tolerates declarations without an input schema", func(t *testing.T) {
		decl := &tool.Declaration{Name: "human_confirm"}

		assert.Equal(t, decl, DeclWithToolIntent(decl))
		assert.Nil(t, DeclWithToolIntent(nil))
	})
}
