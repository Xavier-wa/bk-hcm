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

package iacplugin

import (
	"net/http"

	proto "hcm/pkg/api/hc-service/iac-plugin"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

func (s *svc) initNorthboundRoutes(h *rest.Handler) {
	h.Add("NorthboundHealthIaCPlugin", http.MethodGet, "/iac_plugins/health", s.Health)
	h.Add("NorthboundGetSchemaIaCPlugin", http.MethodGet, "/iac_plugins/schema", s.GetSchema)
	h.Add("NorthboundExecuteIaCPlugin", http.MethodPost, "/iac_plugins/execute", s.Execute)
	h.Add("NorthboundGetResultIaCPlugin", http.MethodPost, "/iac_plugins/result", s.GetResult)
	h.Add("NorthboundStopIaCPlugin", http.MethodPost, "/iac_plugins/stop", s.Stop)
	h.Add("NorthboundPlanIaCPlugin", http.MethodPost, "/iac_plugins/plan", s.Plan)
	h.Add("NorthboundValidateIaCPlugin", http.MethodPost, "/iac_plugins/validate", s.Validate)
	h.Add("NorthboundImportIaCPlugin", http.MethodPost, "/iac_plugins/import", s.Import)
	h.Add("NorthboundRefreshIaCPlugin", http.MethodPost, "/iac_plugins/refresh", s.Refresh)
	h.Add("NorthboundReadDataIaCPlugin", http.MethodPost, "/iac_plugins/read_data", s.ReadData)
	h.Add("NorthboundListIaCPlugin", http.MethodPost, "/iac_plugins/list", s.List)
}

// Health handles GET /api/v1/hc/iac_plugins/health.
func (s *svc) Health(cts *rest.Contexts) (any, error) {
	req := &proto.HealthReq{
		Plugin: cts.Request.QueryParameter("plugin"),
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound health request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.Health(cts.Kit, req)
	if err != nil {
		logs.Errorf("probe plugin %s health failed, err: %v, rid: %s", req.Plugin, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// GetSchema handles GET /api/v1/hc/iac_plugins/schema.
func (s *svc) GetSchema(cts *rest.Contexts) (any, error) {
	req := &proto.GetSchemaReq{
		Plugin:        cts.Request.QueryParameter("plugin"),
		PluginVersion: cts.Request.QueryParameter("plugin_version"),
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound get schema request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.GetSchema(cts.Kit, req)
	if err != nil {
		logs.Errorf("get plugin %s schema failed, err: %v, rid: %s", req.Plugin, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// Execute handles POST /api/v1/hc/iac_plugins/execute.
func (s *svc) Execute(cts *rest.Contexts) (any, error) {
	req := new(proto.ExecuteReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound execute request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.Execute(cts.Kit, req)
	if err != nil {
		logs.Errorf("execute plugin %s instance %s type %s failed, err: %v, rid: %s",
			req.Plugin, req.PluginInstanceID, req.TypeName, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// GetResult handles POST /api/v1/hc/iac_plugins/result.
func (s *svc) GetResult(cts *rest.Contexts) (any, error) {
	req := new(proto.GetResultReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound get result request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.GetResult(cts.Kit, req)
	if err != nil {
		logs.Errorf("get result for plugin %s instance %s type %s failed, err: %v, rid: %s",
			req.Plugin, req.PluginInstanceID, req.TypeName, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// Stop handles POST /api/v1/hc/iac_plugins/stop.
func (s *svc) Stop(cts *rest.Contexts) (any, error) {
	req := new(proto.StopReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound stop request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.Stop(cts.Kit, req)
	if err != nil {
		logs.Errorf("stop plugin %s instance %s type %s failed, err: %v, rid: %s",
			req.Plugin, req.PluginInstanceID, req.TypeName, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// Plan handles POST /api/v1/hc/iac_plugins/plan.
func (s *svc) Plan(cts *rest.Contexts) (any, error) {
	req := new(proto.PlanReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound plan request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.Plan(cts.Kit, req)
	if err != nil {
		logs.Errorf("plan plugin %s instance %s type %s failed, err: %v, rid: %s",
			req.Plugin, req.PluginInstanceID, req.TypeName, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// Validate handles POST /api/v1/hc/iac_plugins/validate.
func (s *svc) Validate(cts *rest.Contexts) (any, error) {
	req := new(proto.ValidateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.Validate(cts.Kit, req)
	if err != nil {
		logs.Errorf("validate config for plugin %s instance %s type %s failed, err: %v, rid: %s",
			req.Plugin, req.PluginInstanceID, req.TypeName, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// Import handles POST /api/v1/hc/iac_plugins/import.
func (s *svc) Import(cts *rest.Contexts) (any, error) {
	req := new(proto.ImportReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound import request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.Import(cts.Kit, req)
	if err != nil {
		logs.Errorf("import resource for plugin %s instance %s type %s failed, err: %v, rid: %s",
			req.Plugin, req.PluginInstanceID, req.TypeName, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// Refresh handles POST /api/v1/hc/iac_plugins/refresh.
func (s *svc) Refresh(cts *rest.Contexts) (any, error) {
	req := new(proto.RefreshReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound refresh request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.Refresh(cts.Kit, req)
	if err != nil {
		logs.Errorf("refresh resource for plugin %s instance %s type %s failed, err: %v, rid: %s",
			req.Plugin, req.PluginInstanceID, req.TypeName, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// ReadData handles POST /api/v1/hc/iac_plugins/read_data.
func (s *svc) ReadData(cts *rest.Contexts) (any, error) {
	req := new(proto.ReadDataReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound read_data request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.ReadData(cts.Kit, req)
	if err != nil {
		logs.Errorf("read_data for plugin %s instance %s type %s failed, err: %v, rid: %s",
			req.Plugin, req.PluginInstanceID, req.TypeName, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// List handles POST /api/v1/hc/iac_plugins/list.
func (s *svc) List(cts *rest.Contexts) (any, error) {
	req := new(proto.ListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("validate northbound list request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := s.router.List(cts.Kit, req)
	if err != nil {
		logs.Errorf("list resource for plugin %s instance %s type %s failed, err: %v, rid: %s",
			req.Plugin, req.PluginInstanceID, req.TypeName, err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}
