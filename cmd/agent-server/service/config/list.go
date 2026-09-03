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

package config

import (
	"encoding/json"
	"fmt"

	proto "hcm/pkg/api/agent-server/config"
	"hcm/pkg/api/core"
	datagconf "hcm/pkg/api/data-service/global_config"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// listConfigTypeWhitelist 是允许通过 GET /config/list 下发的 config_type 白名单。
// global_config 还存有 auth/access_token 等凭据类配置，MUST NOT 通过本接口读取，
// 扩大白名单前必须先确认新类型不含凭据信息。
var listConfigTypeWhitelist = map[enumor.GlobalConfigType]struct{}{
	enumor.GlobalConfigTypeAgentFeedbackTag: {},
}

// ListBizAgentConfig 按 config_type 白名单查询可下发给前端的 global_config。
//
// GET /api/v1/agent/bizs/{bk_biz_id}/config/list?config_type={config_type}
func (svc *service) ListBizAgentConfig(cts *rest.Contexts) (interface{}, error) {
	bizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bizID <= 0 {
		return nil, errf.New(errf.InvalidParameter, "bk_biz_id must be greater than 0")
	}

	if err := svc.authorizeBizAccess(cts.Kit, bizID); err != nil {
		return nil, err
	}

	configType := enumor.GlobalConfigType(cts.Request.QueryParameter("config_type"))
	if configType == "" {
		return nil, errf.New(errf.InvalidParameter, "config_type is required")
	}
	if _, ok := listConfigTypeWhitelist[configType]; !ok {
		return nil, errf.Newf(errf.InvalidParameter, "config_type %q is not allowed", configType)
	}

	listReq := &datagconf.ListReq{
		Filter: tools.EqualExpression("config_type", string(configType)),
		Page:   core.NewDefaultBasePage(),
	}
	resp, err := svc.cli.DataService().Global.GlobalConfig.List(cts.Kit, listReq)
	if err != nil {
		logs.Errorf("list global config failed, err: %v, config_type: %s, rid: %s", err, configType, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	details := make([]proto.ListConfigItem, 0, len(resp.Details))
	for _, row := range resp.Details {
		value := make(map[string]string)
		if len(row.ConfigValue) > 0 {
			if err := json.Unmarshal([]byte(row.ConfigValue), &value); err != nil {
				logs.Errorf("parse config_value failed, err: %v, config_type: %s, config_key: %s, rid: %s",
					err, configType, row.ConfigKey, cts.Kit.Rid)
				return nil, errf.NewFromErr(errf.Aborted, fmt.Errorf("parse config_value failed: %v", err))
			}
		}
		details = append(details, proto.ListConfigItem{ConfigKey: row.ConfigKey, ConfigValue: value})
	}

	return &proto.ListConfigResp{Details: details}, nil
}

// authorizeBizAccess 校验调用者对指定业务的「业务访问」权限。标签配置本身是全局的，
// 这里只用 bizID 做鉴权，不按业务过滤配置行。
func (svc *service) authorizeBizAccess(kt *kit.Kit, bizID int64) error {
	if err := svc.authorizer.AuthorizeWithPerm(kt, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access},
		BizID: bizID,
	}); err != nil {
		logs.Errorf("authorize biz access failed, err: %v, user: %s, bk_biz_id: %d, rid: %s",
			err, kt.User, bizID, kt.Rid)
		ef := errf.Error(err)
		if ef != nil && ef.Permissions != nil {
			return errf.NewWithPerm(errf.PermissionDenied,
				fmt.Sprintf("permission denied, bk_biz_id: %d", bizID), ef.Permissions)
		}
		return errf.Newf(errf.PermissionDenied, "permission denied, bk_biz_id: %d", bizID)
	}
	return nil
}
