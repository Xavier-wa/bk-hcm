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

// Package plugin 提供 hc-service 侧插件 Schema 的临时联调接口。
package plugin

import (
	"net/http"

	"hcm/cmd/hc-service/logics/plugin/schema"
	"hcm/cmd/hc-service/service/capability"
	coreplugin "hcm/pkg/api/core/plugin"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// TODO(temp_delete): 临时插件 Schema 联调接口，接入正式插件注册与北向协议后删除本文件。

// InitService 初始化插件 Schema 临时联调路由。
func InitService(cap *capability.Capability) {
	svc := &service{
		schemaMgr: schema.NewSchemaManager(cap.ClientSet.DataService(), schema.DefaultSchemaCacheTTL),
	}

	h := rest.NewHandler()

	h.Add("SyncPluginSchemaTempDelete", http.MethodPost,
		"/plugins_temp_delete/schemas/sync", svc.SyncPluginSchemaTempDelete)
	h.Add("GetPluginSchemaTempDelete", http.MethodPost,
		"/plugins_temp_delete/schemas/query", svc.GetPluginSchemaTempDelete)

	h.Load(cap.WebService)
}

type service struct {
	schemaMgr schema.SchemaManager
}

// SyncPluginSchemaTempDeleteReq 临时触发 SchemaManager.Sync 的请求。
// Schema 为空时默认使用内置的腾讯云 1.81.5 静态 Schema。
type SyncPluginSchemaTempDeleteReq struct {
	Schema          *coreplugin.PluginSchema `json:"schema" validate:"omitempty"`
	AutoComputeHash bool                     `json:"auto_compute_hash" validate:"omitempty"`
}

// Validate 校验 SyncPluginSchemaTempDeleteReq。
func (r *SyncPluginSchemaTempDeleteReq) Validate() error {
	return validator.Validate.Struct(r)
}

// GetPluginSchemaTempDeleteReq 临时触发 SchemaManager.GetSchema 的请求。
type GetPluginSchemaTempDeleteReq struct {
	PluginName    string `json:"plugin_name" validate:"required"`
	PluginVersion string `json:"plugin_version" validate:"required"`
}

// Validate 校验 GetPluginSchemaTempDeleteReq。
func (r *GetPluginSchemaTempDeleteReq) Validate() error {
	return validator.Validate.Struct(r)
}

// SyncPluginSchemaTempDelete 使用静态 SchemaGetter 触发 SchemaManager.Sync。
func (svc *service) SyncPluginSchemaTempDelete(cts *rest.Contexts) (interface{}, error) {
	req := new(SyncPluginSchemaTempDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("sync plugin schema temp decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("sync plugin schema temp validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	var getter schema.SchemaGetter
	if req.Schema == nil {
		getter = schema.NewDefaultStaticSchemaGetterTempDelete(req.AutoComputeHash)
	} else {
		getter = schema.NewStaticSchemaGetterTempDelete(req.Schema, req.AutoComputeHash)
	}
	return svc.schemaMgr.Sync(cts.Kit, getter)
}

// GetPluginSchemaTempDelete 触发 SchemaManager.GetSchema 查询插件 Schema。
func (svc *service) GetPluginSchemaTempDelete(cts *rest.Contexts) (interface{}, error) {
	req := new(GetPluginSchemaTempDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("get plugin schema temp decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("get plugin schema temp validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	return svc.schemaMgr.GetSchema(cts.Kit, req.PluginName, req.PluginVersion)
}
