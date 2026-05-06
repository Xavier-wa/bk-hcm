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

// Package memory ...
package memory

import (
	"net/http"

	"hcm/cmd/agent-server/service/capability"
	"hcm/pkg/cc"
	"hcm/pkg/iam/auth"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/memory"
)

// InitService initialize the memory service.
func InitService(cap *capability.Capability) {
	svc := &service{
		authorizer: cap.Authorizer,
		memorySvc:  cap.RunTime.MemorySvc(),
		appName:    cc.AgentServer().AGUI.AppName,
	}

	h := rest.NewHandler()
	h.Add("AddMemory", http.MethodPost, "/memory", svc.AddMemory)
	h.Add("ListMemories", http.MethodGet, "/memory", svc.ListMemories)
	h.Add("DeleteMemory", http.MethodDelete, "/memory/{memory_id}", svc.DeleteMemory)
	h.Add("ClearMemories", http.MethodDelete, "/memory", svc.ClearMemories)

	h.Load(cap.WebService)
}

type service struct {
	authorizer auth.Authorizer

	memorySvc memory.Service
	appName   string
}
