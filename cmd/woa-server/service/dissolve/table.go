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

package dissolve

import (
	"hcm/cmd/woa-server/types/dissolve"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// ListHostDetail list resource dissolve host detail
func (s *service) ListHostDetail(cts *rest.Contexts) (interface{}, error) {
	req := new(dissolve.HostDetailListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 服务请求-机房裁撤-菜单粒度
	err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.ServiceResDissolve, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	result, err := s.logics.Table().ListHostDetail(cts.Kit, req)
	if err != nil {
		logs.Errorf("list host detail failed, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
		return nil, err
	}

	return result, nil
}

// ListExportHostDetail list resource dissolve host detail for export
func (s *service) ListExportHostDetail(cts *rest.Contexts) (interface{}, error) {
	req := new(dissolve.HostDetailExportListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 服务请求-机房裁撤-菜单粒度
	err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.ServiceResDissolve, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	result, err := s.logics.Table().ListExportHostDetail(cts.Kit, req)
	if err != nil {
		logs.Errorf("list export host detail failed, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
		return nil, err
	}

	return result, nil
}

// ListResDissolveTable list resource dissolve table
func (s *service) ListResDissolveTable(cts *rest.Contexts) (interface{}, error) {
	req := new(dissolve.ResDissolveReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	// 服务请求-机房裁撤-菜单粒度
	err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.ServiceResDissolve, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	table, err := s.logics.Table().ListResDissolveTable(cts.Kit, req)
	if err != nil {
		logs.Errorf("list resource dissolve table failed, err: %v, req: %v, rid: %s", err, req, cts.Kit.Rid)
		return nil, err
	}

	return dissolve.ResDissolveTable{Items: table}, nil
}

// ListExpectAbolishTime list resource dissolve expect abolish time
func (s *service) ListExpectAbolishTime(cts *rest.Contexts) (interface{}, error) {
	req := new(dissolve.ExpectAbolishTimeListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 服务请求-机房裁撤-菜单粒度
	err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.ServiceResDissolve, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	times, err := s.logics.Table().ListExpectAbolishTime(cts.Kit, req)
	if err != nil {
		logs.Errorf("list expect abolish time failed, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
		return nil, err
	}

	return dissolve.ExpectAbolishTimeListResult{ExpectAbolishTimes: times}, nil
}
