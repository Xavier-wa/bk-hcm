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
	proto "hcm/pkg/api/cloud-server"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// ListIaCPlugin 查询 IaC 插件注册表列表（列表与详情共用）。
func (s *svc) ListIaCPlugin(cts *rest.Contexts) (interface{}, error) {
	req := new(proto.ListReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("decode list iac plugin request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// tiyan 联调临时跳过 IAM，验证后恢复。
	logs.Infof("skip list iac plugin auth, rid: %s", cts.Kit.Rid)

	dsReq := &core.ListReq{
		Filter: req.Filter,
		Page:   req.Page,
	}
	resp, err := s.client.DataService().Global.IaCPlugin.List(cts.Kit, dsReq)
	if err != nil {
		logs.Errorf("list iac plugin from data service failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &proto.IaCPluginListResult{
		Count:   resp.Count,
		Details: resp.Details,
	}, nil
}
