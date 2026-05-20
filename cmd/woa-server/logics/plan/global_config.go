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

package plan

import (
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/api/core"
	cgconf "hcm/pkg/api/core/global-config"
	datagconf "hcm/pkg/api/data-service/global_config"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/dal/dao/tools"
	tablegconf "hcm/pkg/dal/table/global-config"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

func (c *Controller) getResPlanGlobalConfigByKey(kt *kit.Kit, configKey string) (
	*tablegconf.GlobalConfigTable, bool, error) {

	dataReq := &core.ListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("config_type", constant.GlobalConfigTypeResPlan),
			tools.RuleEqual("config_key", configKey),
		),
		Page: core.NewDefaultBasePage(),
	}
	dataResp, err := c.client.DataService().Global.GlobalConfig.List(kt, dataReq)
	if err != nil {
		logs.Errorf("failed to list res plan global config, err: %v, key: %s, rid: %s", err, configKey, kt.Rid)
		return nil, false, err
	}
	if len(dataResp.Details) == 0 {
		return nil, false, nil
	}

	return &dataResp.Details[0], true, nil
}

// GetNonCurrentYearReportDeadline gets non-current-year demand report deadline config.
func (c *Controller) GetNonCurrentYearReportDeadline(kt *kit.Kit) (
	ptypes.GetResPlanNonCurrentYearReportDeadlineResp, error) {

	config, exist, err := c.getResPlanGlobalConfigByKey(kt, constant.ResPlanNonCurrentYearReportDeadlineConfigKey)
	if err != nil {
		logs.Errorf("failed to get non current year report deadline config, err: %v, rid: %s", err, kt.Rid)
		return ptypes.GetResPlanNonCurrentYearReportDeadlineResp{}, err
	}
	if !exist {
		return ptypes.GetResPlanNonCurrentYearReportDeadlineResp{}, nil
	}

	deadline, err := parseResPlanDeadlineConfigValue(config.ConfigValue, c.location)
	if err != nil {
		logs.Errorf("failed to parse non current year report deadline, err: %v, rid: %s", err, kt.Rid)
		return ptypes.GetResPlanNonCurrentYearReportDeadlineResp{}, err
	}

	return ptypes.GetResPlanNonCurrentYearReportDeadlineResp{
		Deadline: deadline.Format(constant.DateTimeLayout),
	}, nil
}

// UpsertNonCurrentYearReportDeadline upserts non-current-year demand report deadline config.
func (c *Controller) UpsertNonCurrentYearReportDeadline(kt *kit.Kit,
	req *ptypes.UpsertResPlanNonCurrentYearReportDeadlineReq) error {

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate upsert non current year report deadline req, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	existing, exist, err := c.getResPlanGlobalConfigByKey(kt, constant.ResPlanNonCurrentYearReportDeadlineConfigKey)
	if err != nil {
		logs.Errorf("failed to get non current year report deadline config, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	if req.Deadline == "" {
		if !exist {
			return nil
		}

		deleteReq := &datagconf.BatchDeleteReq{
			BatchDeleteReq: core.BatchDeleteReq{
				IDs: []string{existing.ID},
			},
		}
		if err = c.client.DataService().Global.GlobalConfig.BatchDelete(kt, deleteReq); err != nil {
			logs.Errorf("failed to delete non current year report deadline config, err: %v, rid: %s", err, kt.Rid)
			return err
		}
		return nil
	}

	if !exist {
		createReq := &datagconf.BatchCreateReq{
			Configs: []cgconf.GlobalConfig{
				{
					ConfigType:  constant.GlobalConfigTypeResPlan,
					ConfigKey:   constant.ResPlanNonCurrentYearReportDeadlineConfigKey,
					ConfigValue: req.Deadline,
				},
			},
		}
		if _, err = c.client.DataService().Global.GlobalConfig.BatchCreate(kt, createReq); err != nil {
			logs.Errorf("failed to create non current year report deadline config, err: %v, rid: %s", err, kt.Rid)
			return err
		}
		return nil
	}

	updateReq := &datagconf.BatchUpdateReq{
		Configs: []cgconf.GlobalConfig{
			{
				ID:          existing.ID,
				ConfigValue: req.Deadline,
			},
		},
	}
	if err = c.client.DataService().Global.GlobalConfig.BatchUpdate(kt, updateReq); err != nil {
		logs.Errorf("failed to update non current year report deadline config, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}
