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

// Package iacplugin 注册 hc-service IaC 插件 Schema 内部查询路由。
package iacplugin

import (
	"net/http"

	"hcm/cmd/hc-service/logics/plugin/schema"
	"hcm/cmd/hc-service/service/capability"
	coreplugin "hcm/pkg/api/core/plugin"
	proto "hcm/pkg/api/hc-service/iac-plugin"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

type service struct {
	schemaMgr *schema.SchemaManager
}

// InitService 注册插件 Schema 内部查询路由。
func InitService(cap *capability.Capability) {
	svc := &service{
		schemaMgr: cap.PluginSchemaMgr,
	}

	h := rest.NewHandler()
	h.Add("NorthboundGetSchemaIaCPlugin", http.MethodGet, "/iac_plugins/schema", svc.GetSchema)
	h.Load(cap.WebService)
}

// GetSchema 按插件名与版本号通过 SchemaManager 查询带缓存的插件 Schema。
func (svc *service) GetSchema(cts *rest.Contexts) (any, error) {
	req := &proto.GetSchemaReq{
		Plugin:        cts.Request.QueryParameter("plugin"),
		PluginVersion: cts.Request.QueryParameter("plugin_version"),
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("get plugin schema validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if svc.schemaMgr == nil {
		logs.Errorf("plugin schema manager is not initialized, rid: %s", cts.Kit.Rid)
		return nil, errf.New(errf.Aborted, "plugin schema manager is not initialized")
	}

	pluginSchema, err := svc.schemaMgr.GetSchema(cts.Kit, coreplugin.PluginInfo{
		Name:    req.Plugin,
		Version: req.PluginVersion,
	})
	if err != nil {
		logs.Errorf("get plugin schema failed, plugin: %s, version: %s, err: %v, rid: %s",
			req.Plugin, req.PluginVersion, err, cts.Kit.Rid)
		return nil, err
	}

	return pluginSchema, nil
}
