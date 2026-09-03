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

package runobserve

import (
	"strings"

	"hcm/pkg/criteria/constant"
)

// QueryFromAGUIReq returns this turn's user text from an /agui request body.
// Prefer the last user message (full history may be present); fall back to resumeValue.
func QueryFromAGUIReq(reqMap map[string]interface{}) string {
	if q := lastUserMessageText(reqMap); q != "" {
		return q
	}
	return resumeValueText(reqMap)
}

func lastUserMessageText(reqMap map[string]interface{}) string {
	if reqMap == nil {
		return ""
	}
	raw, ok := reqMap["messages"]
	if !ok {
		return ""
	}
	msgs, ok := raw.([]interface{})
	if !ok {
		return ""
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		msg, ok := msgs[i].(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if !strings.EqualFold(strings.TrimSpace(role), "user") {
			continue
		}
		if text := messageContentText(msg["content"]); text != "" {
			return text
		}
	}
	return ""
}

func resumeValueText(reqMap map[string]interface{}) string {
	if reqMap == nil {
		return ""
	}
	fp, _ := reqMap["forwardedProps"].(map[string]interface{})
	if fp == nil {
		return ""
	}
	s, _ := fp[constant.ForwardedPropResumeValue].(string)
	return strings.TrimSpace(s)
}

func messageContentText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return strings.TrimSpace(v)
	case []interface{}:
		var b strings.Builder
		for _, part := range v {
			item, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			text, _ := item["text"].(string)
			b.WriteString(text)
		}
		return strings.TrimSpace(b.String())
	default:
		return ""
	}
}
