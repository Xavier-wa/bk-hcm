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

// Package config config
package config

import (
	"encoding/json"
	"errors"
	"time"

	model "hcm/cmd/woa-server/types/dissolve"
	"hcm/pkg/api/core"
	cgconf "hcm/pkg/api/core/global-config"
	datagconf "hcm/pkg/api/data-service/global_config"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	globalconf "hcm/pkg/dal/table/global-config"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/converter"
)

// Config provides interface for operations of dissolve config.
type Config interface {
	GetDissolveHostApplyTime(kt *kit.Kit) (*time.Time, error)
	UpsertDissolveHostApplyTime(kt *kit.Kit, time *time.Time) error
	GetApprovalLimit(kt *kit.Kit) (*float64, error)
	UpsertApprovalLimit(kt *kit.Kit, approveLimit *float64) error
	// 配额系数相关
	GetQuotaCoefficient(kt *kit.Kit) (float64, error)
	UpsertQuotaCoefficient(kt *kit.Kit, coefficient float64) error
	// 偏移配置相关
	GetQuotaOffsets(kt *kit.Kit) ([]model.QuotaOffsetItem, error)
	GetQuotaOffsetsMap(kt *kit.Kit) (map[int64]model.QuotaOffsetItem, error)
	UpsertQuotaOffsets(kt *kit.Kit, offsets []model.QuotaOffsetItem) error
	// 单业务偏移修改
	UpdateBizDissolveQuotaOffset(kt *kit.Kit, bizID int64,
		req *model.UpdateDissolveQuotaOffsetReq) (*model.UpdateDissolveQuotaOffsetResp, error)
}

type logics struct {
	cliSet *client.ClientSet
}

// New create dissolve config logics.
func New(client *client.ClientSet) Config {
	return &logics{
		cliSet: client,
	}
}

// GetDissolveHostApplyTime get dissolve host apply time.
func (l *logics) GetDissolveHostApplyTime(kt *kit.Kit) (*time.Time, error) {
	config, exist, err := l.getDissolveConfigByKey(kt, enumor.GlobalConfigDissolveHostApplyTime)
	if err != nil {
		logs.Errorf("failed to get dissolve host apply time config, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	if !exist {
		logs.Errorf("dissolve host apply time config not exist, rid: %s", kt.Rid)
		return nil, errors.New("dissolve host apply time config not exist")
	}

	applyTime := new(time.Time)
	if err = json.Unmarshal([]byte(config.ConfigValue), &applyTime); err != nil {
		logs.Errorf("failed to unmarshal config value, err: %v, value: %s, rid: %s", err, config.ConfigValue, kt.Rid)
		return nil, err
	}

	return applyTime, nil
}

func (l *logics) getDissolveConfigByKey(kt *kit.Kit, key enumor.GlobalConfigResDissolveKey) (
	*globalconf.GlobalConfigTable, bool, error) {

	req := core.ListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("config_type", enumor.GlobalConfigResDissolve),
			tools.RuleJSONEqual("config_key", key),
		),
		Page: core.NewDefaultBasePage(),
	}
	cfgResp, err := l.cliSet.DataService().Global.GlobalConfig.List(kt, &req)
	if err != nil {
		logs.Errorf("failed to list global config, err: %v, req: %+v, rid: %s", err, req, kt.Rid)
		return nil, false, err
	}
	if len(cfgResp.Details) == 0 {
		return nil, false, nil
	}

	return &cfgResp.Details[0], true, nil
}

// UpsertDissolveHostApplyTime upsert dissolve host apply time.
func (l *logics) UpsertDissolveHostApplyTime(kt *kit.Kit, time *time.Time) error {
	return l.upsertDissolveConfig(kt, enumor.GlobalConfigDissolveHostApplyTime, time)
}

func (l *logics) upsertDissolveConfig(kt *kit.Kit, key enumor.GlobalConfigResDissolveKey, value interface{}) error {
	if value == nil {
		logs.Errorf("value is nil, key: %s, rid: %s", key, kt.Rid)
		return errors.New("value is nil")
	}

	oldConf, exist, err := l.getDissolveConfigByKey(kt, key)
	if err != nil {
		logs.Errorf("failed to get dissolve config, err: %v, key: %s, rid: %s", err, key, kt.Rid)
		return err
	}

	conf := cgconf.GlobalConfigT[any]{
		ConfigType:  string(enumor.GlobalConfigResDissolve),
		ConfigKey:   string(key),
		ConfigValue: value,
	}
	if !exist {
		createReq := datagconf.BatchCreateReqT[any]{Configs: []cgconf.GlobalConfigT[any]{conf}}
		if _, err = l.cliSet.DataService().Global.GlobalConfig.BatchCreate(kt, &createReq); err != nil {
			logs.Errorf("failed to create dissolve global config, err: %v, req: %+v, rid: %s", err, createReq, kt.Rid)
			return err
		}
		return nil
	}

	conf.ID = oldConf.ID
	updateReq := datagconf.BatchUpdateReq{Configs: []cgconf.GlobalConfigT[any]{conf}}
	if err = l.cliSet.DataService().Global.GlobalConfig.BatchUpdate(kt, &updateReq); err != nil {
		logs.Errorf("failed to update dissolve global config, err: %v, req: %+v, rid: %s", err, updateReq, kt.Rid)
		return err
	}

	return nil
}

// GetApprovalLimit get approval limit.
func (l *logics) GetApprovalLimit(kt *kit.Kit) (*float64, error) {
	config, exist, err := l.getDissolveConfigByKey(kt, enumor.GlobalConfigDissolveApprovalLimit)
	if err != nil {
		logs.Errorf("failed to get dissolve approval limit config, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	if !exist {
		logs.Errorf("dissolve approval limit config not exist, rid: %s", kt.Rid)
		return nil, errors.New("dissolve approval limit config not exist")
	}

	approvalLimit := new(float64)
	if err = json.Unmarshal([]byte(config.ConfigValue), &approvalLimit); err != nil {
		logs.Errorf("failed to unmarshal config value, err: %v, value: %s, rid: %s", err, config.ConfigValue, kt.Rid)
		return nil, err
	}

	return approvalLimit, nil
}

// UpsertApprovalLimit upsert approval limit.
func (l *logics) UpsertApprovalLimit(kt *kit.Kit, approvalLimit *float64) error {
	return l.upsertDissolveConfig(kt, enumor.GlobalConfigDissolveApprovalLimit, approvalLimit)
}

// GetQuotaCoefficient get quota coefficient, returns default value 65 if not configured.
func (l *logics) GetQuotaCoefficient(kt *kit.Kit) (float64, error) {
	config, exist, err := l.getDissolveConfigByKey(kt, enumor.GlobalConfigDissolveQuotaCoefficient)
	if err != nil {
		logs.Errorf("failed to get dissolve quota coefficient config, err: %v, rid: %s", err, kt.Rid)
		return 0, err
	}
	if !exist {
		// 未配置时返回默认值
		return constant.DissolveDefaultQuotaCoefficient, nil
	}

	var coefficient float64
	if err = json.Unmarshal([]byte(config.ConfigValue), &coefficient); err != nil {
		logs.Errorf("failed to unmarshal quota coefficient, err: %v, value: %s, rid: %s", err, config.ConfigValue, kt.Rid)
		return 0, err
	}

	return coefficient, nil
}

// UpsertQuotaCoefficient upsert quota coefficient.
func (l *logics) UpsertQuotaCoefficient(kt *kit.Kit, coefficient float64) error {
	return l.upsertDissolveConfig(kt, enumor.GlobalConfigDissolveQuotaCoefficient, coefficient)
}

// GetQuotaOffsets get quota offsets as array, returns empty array if not configured.
func (l *logics) GetQuotaOffsets(kt *kit.Kit) ([]model.QuotaOffsetItem, error) {
	offsetsMap, err := l.GetQuotaOffsetsMap(kt)
	if err != nil {
		return nil, err
	}

	// 转换为数组
	offsets := converter.MapToSlice(offsetsMap, func(bizID int64, item model.QuotaOffsetItem) model.QuotaOffsetItem {
		item.BkBizID = bizID
		return item
	})

	return offsets, nil
}

// GetQuotaOffsetsMap get quota offsets as map for internal use, returns empty map if not configured.
func (l *logics) GetQuotaOffsetsMap(kt *kit.Kit) (map[int64]model.QuotaOffsetItem, error) {
	offsets := make(map[int64]model.QuotaOffsetItem)

	config, exist, err := l.getDissolveConfigByKey(kt, enumor.GlobalConfigDissolveQuotaOffsets)
	if err != nil {
		logs.Errorf("failed to get dissolve quota offsets config, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	if !exist {
		// 未配置时返回空map
		return offsets, nil
	}

	if err = json.Unmarshal([]byte(config.ConfigValue), &offsets); err != nil {
		logs.Errorf("failed to unmarshal quota offsets, err: %v, value: %s, rid: %s", err, config.ConfigValue, kt.Rid)
		return nil, err
	}

	return offsets, nil
}

// UpsertQuotaOffsets upsert quota offsets.
func (l *logics) UpsertQuotaOffsets(kt *kit.Kit, offsets []model.QuotaOffsetItem) error {
	// 转换为 map 存储
	offsetsMap := converter.SliceToMap(offsets, func(item model.QuotaOffsetItem) (int64, model.QuotaOffsetItem) {
		return item.BkBizID, model.QuotaOffsetItem{
			Offset: item.Offset,
			Type:   item.Type,
			Memo:   item.Memo,
		}
	})
	return l.upsertDissolveConfig(kt, enumor.GlobalConfigDissolveQuotaOffsets, offsetsMap)
}

// UpdateBizDissolveQuotaOffset update single business quota offset.
func (l *logics) UpdateBizDissolveQuotaOffset(kt *kit.Kit, bizID int64,
	req *model.UpdateDissolveQuotaOffsetReq) (*model.UpdateDissolveQuotaOffsetResp, error) {

	// 获取当前偏移配置（使用 map 方法便于查找和更新）
	offsetsMap, err := l.GetQuotaOffsetsMap(kt)
	if err != nil {
		logs.Errorf("failed to get quota offsets, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 记录修改前的偏移值
	var beforeOffset int64
	if existing, ok := offsetsMap[bizID]; ok {
		beforeOffset = existing.SignedOffset()
	}

	// 更新偏移配置
	offsetsMap[bizID] = model.QuotaOffsetItem{
		Offset: converter.PtrToVal(req.Offset),
		Type:   req.Type,
		Memo:   req.Memo,
	}

	// 转换为数组保存
	offsets := converter.MapToSlice(offsetsMap, func(bizID int64, item model.QuotaOffsetItem) model.QuotaOffsetItem {
		item.BkBizID = bizID
		return item
	})

	// 保存偏移配置
	if err = l.UpsertQuotaOffsets(kt, offsets); err != nil {
		logs.Errorf("failed to upsert quota offsets, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 计算修改后的偏移值（用于响应）
	afterOffset := req.SignedOffset()

	logs.Infof("updated quota offset for biz %d, before: %d, after: %d, operator: %s, rid: %s",
		bizID, beforeOffset, afterOffset, kt.User, kt.Rid)

	return &model.UpdateDissolveQuotaOffsetResp{
		BkBizID:      bizID,
		BeforeOffset: beforeOffset,
		AfterOffset:  afterOffset,
	}, nil
}
