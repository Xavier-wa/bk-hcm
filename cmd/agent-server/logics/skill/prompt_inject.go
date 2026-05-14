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

package skill

import (
	"context"
	"encoding/json"
	"strings"

	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
)

// MakeSkillInjectWithModelCallback returns a BeforeModelCallbackStructured that
// injects loaded skill bodies and selected docs into the system message before each LLM call.
func MakeSkillInjectWithModelCallback(agentName string, repo skillpkg.Repository) model.BeforeModelCallbackStructured {
	return func(ctx context.Context, args *model.BeforeModelArgs) (*model.BeforeModelResult, error) {
		if args == nil || args.Request == nil {
			logs.Warnf("[skill_prompt] args or Request is nil, skip skill inject")
			return nil, nil
		}

		inv, ok := agent.InvocationFromContext(ctx)
		if !ok || inv == nil || inv.Session == nil {
			logs.Warnf("[skill_prompt] no invocation or session in context (ok=%v inv=%v session=%v), skip",
				ok, inv != nil, inv != nil && inv.Session != nil)
			return nil, nil
		}

		var builder strings.Builder
		// Inject Available skills.
		if content := buildAvailableSkillsContent(repo); content != "" {
			builder.WriteString(content)
		}

		// Inject loaded skill bodies and selected docs.
		loadedSkills := collectLoadedSkills(agentName, inv.Session.State)
		logs.Infof("[skill_prompt] collected %d loaded skills for agent=%s", len(loadedSkills), agentName)
		for _, skillName := range loadedSkills {
			skill, err := repo.Get(skillName)
			if err != nil || skill == nil {
				logs.Warnf("[skill_prompt] failed to get skill %s: %v", skillName, err)
				continue
			}

			if skill.Body != "" {
				builder.WriteString("\n[Loaded] ")
				builder.WriteString(skillName)
				builder.WriteString("\n\n")
				builder.WriteString(skill.Body)
			}

			appendSelectedDocs(&builder, skill, agentName, skillName, inv.Session.State)
		}

		injectContent := builder.String()
		if injectContent == "" {
			return nil, nil
		}

		args.Request.Messages = mergeSkillContentIntoSystem(args.Request.Messages, injectContent)
		logs.Infof("[skill_prompt] injected into system prompt, loaded=%d contentLen=%d", len(loadedSkills),
			len(injectContent))
		return nil, nil
	}
}

func collectLoadedSkills(agentName string, state map[string][]byte) []string {
	loadedPrefix := skillpkg.LoadedPrefix(agentName)
	var loadedSkills []string
	for key := range state {
		if skillName, ok := strings.CutPrefix(key, loadedPrefix); ok && skillName != "" {
			loadedSkills = append(loadedSkills, skillName)
		}
	}
	return loadedSkills
}

func buildAvailableSkillsContent(repo skillpkg.Repository) string {
	summaries := repo.Summaries()
	if len(summaries) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("Available skills:\n")
	for _, s := range summaries {
		builder.WriteString("- ")
		builder.WriteString(s.Name)
		builder.WriteString(": ")
		builder.WriteString(s.Description)
		builder.WriteString("\n")
	}

	builder.WriteString("\n\n")
	builder.WriteString("Tooling and workspace guidance:\n")
	builder.WriteString("- Progressive disclosure: call skill_load with only skill first.\n")
	builder.WriteString("- For docs, prefer skill_list_docs + skill_select_docs to load only what you need.\n")
	builder.WriteString("- Avoid include_all_docs unless you need every doc or the user asks.\n")
	return builder.String()
}

func appendSelectedDocs(builder *strings.Builder, skill *skillpkg.Skill, agentName, skillName string,
	state map[string][]byte) {

	docsKey := skillpkg.DocsKey(agentName, skillName)
	docsVal := string(state[docsKey])
	if docsVal == "" {
		return
	}

	var selectedDocs []string
	if docsVal == "*" {
		for _, d := range skill.Docs {
			selectedDocs = append(selectedDocs, d.Path)
		}
	} else {
		if err := json.Unmarshal([]byte(docsVal), &selectedDocs); err != nil {
			logs.Warnf("[skill_prompt] failed to unmarshal selected docs for skill %s: %v", skillName, err)
			return
		}
	}

	docMap := make(map[string]string)
	for _, d := range skill.Docs {
		docMap[d.Path] = d.Content
	}

	for _, path := range selectedDocs {
		if content, ok := docMap[path]; ok && content != "" {
			builder.WriteString("\n[Doc] ")
			builder.WriteString(path)
			builder.WriteString("\n\n")
			builder.WriteString(content)
		}
	}
}

// mergeSkillContentIntoSystem appends the skill content to the existing system
// message, or inserts a new system message at the head if none exists.
func mergeSkillContentIntoSystem(msgs []model.Message, content string) []model.Message {
	for i := range msgs {
		if msgs[i].Role == model.RoleSystem {
			msgs[i].Content = msgs[i].Content + "\n\n" + content
			return msgs
		}
	}
	// No existing system message; prepend one.
	return append([]model.Message{model.NewSystemMessage(content)}, msgs...)
}
