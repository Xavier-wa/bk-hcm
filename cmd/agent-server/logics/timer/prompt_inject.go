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

// Package timer ...
package timer

import (
	"context"
	"fmt"
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

// MakeTimeInjectCallback returns a BeforeModelCallbackStructured that
// injects the current time into the system message before each LLM call.
func MakeTimeInjectCallback() model.BeforeModelCallbackStructured {
	return func(ctx context.Context, args *model.BeforeModelArgs) (*model.BeforeModelResult, error) {
		rid := rest.RidFromContext(ctx)
		if args == nil || args.Request == nil {
			logs.Warnf("[timer] args or Request is nil, skip time inject, rid: %s", rid)
			return nil, nil
		}

		currentTime := time.Now().Format(constant.DateTimeZoneLayout)
		timeContent := fmt.Sprintf("The current time is: %s", currentTime)

		args.Request.Messages = mergeTimeContentIntoSystem(args.Request.Messages, timeContent)
		logs.Infof("[timer] injected current time into system prompt: %s, rid: %s", currentTime, rid)
		return nil, nil
	}
}

// mergeTimeContentIntoSystem appends the time content to the existing system
// message, or inserts a new system message at the head if none exists.
func mergeTimeContentIntoSystem(msgs []model.Message, content string) []model.Message {
	for i := range msgs {
		if msgs[i].Role == model.RoleSystem {
			if msgs[i].Content == "" {
				msgs[i].Content = content
			} else {
				msgs[i].Content = msgs[i].Content + "\n\n" + content
			}
			return msgs
		}
	}
	// No existing system message; prepend one.
	return append([]model.Message{model.NewSystemMessage(content)}, msgs...)
}
