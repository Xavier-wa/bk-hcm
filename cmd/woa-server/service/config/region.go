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

// Package config region config
package config

import (
	types "hcm/cmd/woa-server/types/config"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// GetQcloudRegion gets qcloud region config list
func (s *service) GetQcloudRegion(cts *rest.Contexts) (interface{}, error) {
	rst, err := s.logics.Region().GetRegion(cts.Kit)
	if err != nil {
		logs.Errorf("failed to get region list, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return rst, nil
}

// GetIdcRegion gets idc region config list
func (s *service) GetIdcRegion(cts *rest.Contexts) (interface{}, error) {
	rst, err := s.logics.Region().GetIdcRegion(cts.Kit)
	if err != nil {
		logs.Errorf("failed to get idc region list, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return rst, nil
}

// UpsertRegionRecommend 新增或编辑地域推荐配置
func (s *service) UpsertRegionRecommend(cts *rest.Contexts) (interface{}, error) {
	req := new(types.UpsertRegionRecommendReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to decode upsert region recommend request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate upsert region recommend request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{Basic: &meta.Basic{
		Type: meta.GlobalConfig, Action: meta.Create}}); err != nil {
		logs.Errorf("upsert region recommend global config auth failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	if err := s.logics.Region().UpsertRecommendConfig(cts.Kit, req); err != nil {
		logs.Errorf("failed to upsert region recommend config, req: %+v, err: %v, rid: %s", req, err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
