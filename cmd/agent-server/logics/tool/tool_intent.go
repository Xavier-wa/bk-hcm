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
	"hcm/pkg/criteria/constant"
	"hcm/pkg/tools/converter"

	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// toolIntentDescription 是所有模型可见工具共用的 tool_intent 字段描述。
//
// 文案质量完全由这段描述约束：既不改 system prompt，也不按工具名维护中文映射或兜底模板。
// 描述必须让模型输出「本次」调用在当前场景下的目的，而不是复述工具名——同一个工具在不同轮次、
// 不同目的下应当写出不同文案，因此这里给的是同工具多种写法的示例，而非可套用的固定句式。
const toolIntentDescription = `面向用户的一句话中文说明，概括【本次】调用在当前对话场景下要做什么。
要求：写目的与场景，不要复述工具名或参数字段名；同一工具在不同上下文必须写出不同目的
（例如同是搜索工具，一次写「正在查找能列出云账号的工具」，另一次写「正在查找能查询磁盘的工具」；
同是执行工具，一次写「正在查询该业务下的云账号」，另一次写「正在提交主机申领单」）。
一句、总结性、用户能看懂，建议以「正在」开头。`

// ToolIntentSchemaProperty returns the shared schema of the optional tool_intent argument.
// 每次返回新对象，避免调用方把它写进不同工具的 Properties 后互相串改。
func ToolIntentSchemaProperty() *tool.Schema {
	return &tool.Schema{
		Type:        "string",
		Description: toolIntentDescription,
	}
}

// DeclWithToolIntent 在 decl 的入参 schema 中加入可选字段 tool_intent 并返回。
// 不改 Required：tool_intent 缺失不能让工具调用失败。
// decl 为 nil 或没有 object 入参 schema 时原样返回。
func DeclWithToolIntent(decl *tool.Declaration) *tool.Declaration {
	if decl == nil || decl.InputSchema == nil {
		return decl
	}

	// 浅拷贝 Schema 与 Properties，防止改到内层工具可能复用的共享 map。
	schema := converter.PtrToVal(decl.InputSchema)
	props := make(map[string]*tool.Schema, len(schema.Properties)+1)
	for k, v := range schema.Properties {
		props[k] = v
	}
	props[constant.ToolIntentArgKey] = ToolIntentSchemaProperty()
	schema.Properties = props
	decl.InputSchema = &schema

	return decl
}
