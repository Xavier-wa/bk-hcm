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

	"hcm/cmd/woa-server/types/config"
	"hcm/pkg/api/core"
	cgconf "hcm/pkg/api/core/global-config"
	datagconf "hcm/pkg/api/data-service/global_config"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	tablegconf "hcm/pkg/dal/table/global-config"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// LoadTestSubnetIf provides management interface for load test subnet config
type LoadTestSubnetIf interface {
	// UpsertConfig upsert load test subnet config (全量替换)
	UpsertConfig(kt *kit.Kit, req config.UpsertLoadTestSubnetReq) error
	// ListConfig list complete load test subnet config
	ListConfig(kt *kit.Kit) (config.LoadTestSubnetConfig, error)
}

// NewLoadTestSubnetOp creates a load test subnet interface
func NewLoadTestSubnetOp(cli *client.ClientSet) LoadTestSubnetIf {
	return &loadTestSubnet{
		client: cli,
	}
}

type loadTestSubnet struct {
	client *client.ClientSet
}

// ListConfig list complete load test subnet config, 配置不存在时返回空 map
func (l *loadTestSubnet) ListConfig(kt *kit.Kit) (config.LoadTestSubnetConfig, error) {
	detail, exist, err := l.getConfig(kt)
	if err != nil {
		logs.Errorf("failed to get load test subnet config, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	if !exist {
		return config.LoadTestSubnetConfig{}, nil
	}

	result := config.LoadTestSubnetConfig{}
	if err = json.Unmarshal([]byte(detail.ConfigValue), &result); err != nil {
		logs.Errorf("failed to unmarshal load test subnet config value, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return result, nil
}

// UpsertConfig upsert load test subnet config, 全量替换 config_value
func (l *loadTestSubnet) UpsertConfig(kt *kit.Kit, req config.UpsertLoadTestSubnetReq) error {
	detail, exist, err := l.getConfig(kt)
	if err != nil {
		logs.Errorf("failed to get load test subnet config, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	configValue := config.LoadTestSubnetConfig(req)
	if exist {
		updateReq := &datagconf.BatchUpdateReq{
			Configs: []cgconf.GlobalConfig{{ID: detail.ID, ConfigValue: configValue}},
		}
		if err = l.client.DataService().Global.GlobalConfig.BatchUpdate(kt, updateReq); err != nil {
			logs.Errorf("failed to update load test subnet config, err: %v, rid: %s", err, kt.Rid)
			return err
		}
		return nil
	}

	createReq := &datagconf.BatchCreateReq{
		Configs: []cgconf.GlobalConfig{
			{
				ConfigType:  string(enumor.GlobalConfigTypeCvmApply),
				ConfigKey:   string(enumor.GlobalConfigKeyCvmApplyLoadTestSubnet),
				ConfigValue: configValue,
			},
		},
	}
	if _, err = l.client.DataService().Global.GlobalConfig.BatchCreate(kt, createReq); err != nil {
		logs.Errorf("failed to create load test subnet config, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// getConfig get load test subnet config record by fixed config_type and config_key
func (l *loadTestSubnet) getConfig(kt *kit.Kit) (*tablegconf.GlobalConfigTable, bool, error) {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("config_type", string(enumor.GlobalConfigTypeCvmApply)),
		tools.RuleEqual("config_key", string(enumor.GlobalConfigKeyCvmApplyLoadTestSubnet)),
	)

	dataReq := &core.ListReq{
		Filter: filter,
		Page:   core.NewDefaultBasePage(),
	}
	dataResp, err := l.client.DataService().Global.GlobalConfig.List(kt, dataReq)
	if err != nil {
		logs.Errorf("failed to list load test subnet config, err: %v, rid: %s", err, kt.Rid)
		return nil, false, err
	}

	if len(dataResp.Details) == 0 {
		return nil, false, nil
	}

	return &dataResp.Details[0], true, nil
}
