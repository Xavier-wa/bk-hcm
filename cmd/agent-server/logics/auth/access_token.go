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

package auth

import (
	"encoding/json"
	"fmt"
	"strings"

	"hcm/pkg/api/core"
	gccore "hcm/pkg/api/core/global-config"
	datagconf "hcm/pkg/api/data-service/global_config"
	dataservice "hcm/pkg/client/data-service"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	tablegconf "hcm/pkg/dal/table/global-config"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// LoadInitAccessToken reads access_token for the given virtual-user from global_config.
func LoadInitAccessToken(kt *kit.Kit, dataCli *dataservice.Client, virtualUser string) (string, error) {
	virtualUser = strings.TrimSpace(virtualUser)
	if virtualUser == "" {
		return "", fmt.Errorf("initVirtualUser is empty")
	}
	if dataCli == nil {
		return "", fmt.Errorf("data service client is nil")
	}

	_, tokenMap, err := loadAccessTokenConfig(kt, dataCli)
	if err != nil {
		logs.Errorf("load access token config failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}
	if tokenMap == nil {
		logs.Errorf("global_config auth/access_token not found, rid: %s", kt.Rid)
		return "", fmt.Errorf("global_config auth/access_token not found")
	}

	token, ok := tokenMap[virtualUser]
	if !ok || strings.TrimSpace(token) == "" {
		logs.Errorf("virtual-user %q has no access_token in global_config, rid: %s", virtualUser, kt.Rid)
		return "", fmt.Errorf("virtual-user %q has no access_token in global_config", virtualUser)
	}
	return token, nil
}

// UpsertAccessToken upserts access_token for the given virtual-user in global_config auth/access_token.
func UpsertAccessToken(kt *kit.Kit, dataCli *dataservice.Client, virtualUser, accessToken string) error {
	virtualUser = strings.TrimSpace(virtualUser)
	accessToken = strings.TrimSpace(accessToken)
	if virtualUser == "" {
		return fmt.Errorf("virtualUser is empty")
	}
	if accessToken == "" {
		return fmt.Errorf("accessToken is empty")
	}
	if dataCli == nil {
		return fmt.Errorf("data service client is nil")
	}

	existing, tokenMap, err := loadAccessTokenConfig(kt, dataCli)
	if err != nil {
		return err
	}

	if tokenMap == nil {
		tokenMap = make(map[string]string)
	}
	tokenMap[virtualUser] = accessToken

	configValueBytes, err := json.Marshal(tokenMap)
	if err != nil {
		logs.Errorf("marshal access_token config_value failed, err: %v, rid: %s", err, kt.Rid)
		return fmt.Errorf("marshal access_token config_value: %v", err)
	}

	if existing != nil {
		updateReq := &datagconf.BatchUpdateReq{Configs: []gccore.GlobalConfig{
			{
				ID:          existing.ID,
				ConfigValue: json.RawMessage(configValueBytes),
			},
		}}
		if err = dataCli.Global.GlobalConfig.BatchUpdate(kt, updateReq); err != nil {
			logs.Errorf("update access_token global_config failed, err: %v, rid: %s", err, kt.Rid)
			return fmt.Errorf("update global_config auth/access_token: %v", err)
		}
		logs.Infof("upsert access_token success, virtual_user: %s, rid: %s", virtualUser, kt.Rid)
		return nil
	}

	createReq := &datagconf.BatchCreateReq{Configs: []gccore.GlobalConfig{
		{
			ConfigKey:   string(enumor.GlobalConfigKeyAccessToken),
			ConfigType:  string(enumor.GlobalConfigTypeAuth),
			ConfigValue: json.RawMessage(configValueBytes),
		},
	}}
	if _, err = dataCli.Global.GlobalConfig.BatchCreate(kt, createReq); err != nil {
		logs.Errorf("create access_token global_config failed, err: %v, rid: %s", err, kt.Rid)
		return fmt.Errorf("create global_config auth/access_token: %v", err)
	}

	logs.Infof("upsert access_token success, virtual_user: %s, rid: %s", virtualUser, kt.Rid)
	return nil
}

func loadAccessTokenConfig(kt *kit.Kit, dataCli *dataservice.Client) (*tablegconf.GlobalConfigTable, map[string]string,
	error) {

	listReq := &datagconf.ListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("config_type", enumor.GlobalConfigTypeAuth),
			tools.RuleEqual("config_key", enumor.GlobalConfigKeyAccessToken),
		),
		Page: core.NewDefaultBasePage(),
	}
	resp, err := dataCli.Global.GlobalConfig.List(kt, listReq)
	if err != nil {
		logs.Errorf("load access token global_config failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, fmt.Errorf("list global_config auth/access_token: %v", err)
	}
	if len(resp.Details) == 0 {
		return nil, nil, nil
	}

	tokenMap := make(map[string]string)
	if err = json.Unmarshal([]byte(resp.Details[0].ConfigValue), &tokenMap); err != nil {
		logs.Errorf("parse access_token config_value failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, fmt.Errorf("parse access_token config_value: %v", err)
	}

	return &resp.Details[0], tokenMap, nil
}
