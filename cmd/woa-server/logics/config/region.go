/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"encoding/json"
	"fmt"

	"hcm/cmd/woa-server/model/config"
	types "hcm/cmd/woa-server/types/config"
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

// RegionIf provides management interface for operations of region config
type RegionIf interface {
	// GetRegion get region type config list
	GetRegion(kt *kit.Kit) (*types.GetRegionResult, error)
	// GetIdcRegion get region type config list
	GetIdcRegion(kt *kit.Kit) (*types.GetIdcRegionRst, error)
	// UpsertRecommendConfig upsert region recommend config
	UpsertRecommendConfig(kt *kit.Kit, req *types.UpsertRegionRecommendReq) error
}

// NewRegionOp creates a region interface
func NewRegionOp(client *client.ClientSet) RegionIf {
	return &region{
		client: client,
	}
}

type region struct {
	client *client.ClientSet
}

// GetRegion get region type config list
func (r *region) GetRegion(kt *kit.Kit) (*types.GetRegionResult, error) {
	// 从 data-service 查询 region 列表
	req := &core.ListReq{
		Filter: tools.EqualExpression("vendor", enumor.TCloudZiyan),
		Page:   core.NewCountPage(),
	}
	countRes, err := r.client.DataService().TCloudZiyan.Region.ListRegion(kt, req)
	if err != nil {
		return nil, fmt.Errorf("list region count failed, err: %v", err)
	}

	req.Page = core.NewDefaultBasePage()
	allRegions := make([]*types.Region, 0)
	
	// 查询 global_config 获取推荐地域列表
	recommendedIDs := r.getRecommendedRegionIDs(kt)
	recommendedSet := make(map[string]struct{}, len(recommendedIDs))
	for _, id := range recommendedIDs {
		recommendedSet[id] = struct{}{}
	}

	for {
		apiRegions, err := r.client.DataService().TCloudZiyan.Region.ListRegion(kt, req)
		if err != nil {
			return nil, fmt.Errorf("list region failed, err: %v", err)
		}

		// 转换数据为 types.Region 类型
		for _, item := range apiRegions.Details {
			regionOne := &types.Region{
				Region:         item.RegionID,
				RegionCn:       item.RegionName,
				CmdbRegionName: item.CityName,
			}
			if _, ok := recommendedSet[item.RegionID]; ok {
				regionOne.IsRecommended = true
			}
			allRegions = append(allRegions, regionOne)
		}

		if uint(len(apiRegions.Details)) < req.Page.Limit {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	rst := &types.GetRegionResult{
		Count: int64(countRes.Count),
		Info:  allRegions,
	}

	return rst, nil
}

// getRecommendedRegionIDs 从 global_config 表查询推荐的地域ID列表
func (r *region) getRecommendedRegionIDs(kt *kit.Kit) []string {
	cfg, exist, err := r.getRecommendConfig(kt)
	if err != nil {
		logs.Errorf("failed to get recommended region config, err: %v, rid: %s", err, kt.Rid)
		return nil
	}
	if !exist {
		return nil
	}

	var regionIDs []string
	if err = json.Unmarshal([]byte(cfg.ConfigValue), &regionIDs); err != nil {
		logs.Errorf("failed to unmarshal recommended region ids, err: %v, value: %s, rid: %s",
			err, string(cfg.ConfigValue), kt.Rid)
		return nil
	}

	return regionIDs
}

// UpsertRecommendConfig upsert region recommend config
func (r *region) UpsertRecommendConfig(kt *kit.Kit, req *types.UpsertRegionRecommendReq) error {
	existingConfig, exist, err := r.getRecommendConfig(kt)
	if err != nil {
		logs.Errorf("failed to get existing region recommend config, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	if exist {
		updateReq := &datagconf.BatchUpdateReq{
			Configs: []cgconf.GlobalConfig{{ID: existingConfig.ID, ConfigValue: req.RegionIDs}},
		}
		if err = r.client.DataService().Global.GlobalConfig.BatchUpdate(kt, updateReq); err != nil {
			logs.Errorf("failed to update region recommend config, req: %+v, err: %v, rid: %s",
				req, err, kt.Rid)
			return err
		}
		return nil
	}

	createReq := &datagconf.BatchCreateReq{
		Configs: []cgconf.GlobalConfig{{
			ConfigType:  string(enumor.GlobalConfigTypeRegionRecommend),
			ConfigKey:   string(enumor.GlobalConfigKeyRegionRecommend),
			ConfigValue: req.RegionIDs,
		}},
	}
	if _, err = r.client.DataService().Global.GlobalConfig.BatchCreate(kt, createReq); err != nil {
		logs.Errorf("failed to create region recommend config, req: %+v, err: %v, rid: %s", req, err, kt.Rid)
		return err
	}

	return nil
}

// getRecommendConfig 获取推荐地域配置
func (r *region) getRecommendConfig(kt *kit.Kit) (*tablegconf.GlobalConfigTable, bool, error) {
	configFilter := tools.ExpressionAnd(
		tools.RuleEqual("config_type", string(enumor.GlobalConfigTypeRegionRecommend)),
		tools.RuleEqual("config_key", string(enumor.GlobalConfigKeyRegionRecommend)),
	)

	dataReq := &core.ListReq{Filter: configFilter, Page: core.NewDefaultBasePage()}
	dataResp, err := r.client.DataService().Global.GlobalConfig.List(kt, dataReq)
	if err != nil {
		return nil, false, err
	}
	if len(dataResp.Details) == 0 {
		return nil, false, nil
	}

	return &dataResp.Details[0], true, nil
}

// GetIdcRegion get idc region list
func (r *region) GetIdcRegion(kt *kit.Kit) (*types.GetIdcRegionRst, error) {
	filter := make(map[string]interface{})

	// TODO 替换mysql，依然使用静态数据
	insts, err := config.Operation().IdcZone().GetRegionList(kt.Ctx, filter)
	if err != nil {
		return nil, err
	}

	rst := &types.GetIdcRegionRst{
		Info: insts,
	}

	return rst, nil
}
