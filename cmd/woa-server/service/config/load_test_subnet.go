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
	types "hcm/cmd/woa-server/types/config"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// ListLoadTestSubnets list complete load test subnet config, 无需鉴权
func (s *service) ListLoadTestSubnets(cts *rest.Contexts) (interface{}, error) {
	result, err := s.logics.LoadTestSubnet().ListConfig(cts.Kit)
	if err != nil {
		logs.Errorf("failed to list load test subnet config, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return result, nil
}

// UpsertLoadTestSubnets upsert load test subnet config, 全量替换
func (s *service) UpsertLoadTestSubnets(cts *rest.Contexts) (interface{}, error) {
	req := types.UpsertLoadTestSubnetReq{}
	if err := cts.DecodeInto(&req); err != nil {
		logs.Errorf("failed to decode request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{Basic: &meta.Basic{
		Type: meta.GlobalConfig, Action: meta.Create}}); err != nil {
		logs.Errorf("upsert global config auth failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	if err := s.logics.LoadTestSubnet().UpsertConfig(cts.Kit, req); err != nil {
		logs.Errorf("failed to upsert load test subnet config, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
