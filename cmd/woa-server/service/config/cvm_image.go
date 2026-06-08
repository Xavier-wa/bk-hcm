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

// Package config cvm image config
package config

import (
	types "hcm/cmd/woa-server/types/config"
	dataproto "hcm/pkg/api/data-service/cloud/image"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// GetCvmImage gets cvm image config list
func (s *service) GetCvmImage(cts *rest.Contexts) (interface{}, error) {
	input := new(types.GetCvmImageParam)
	if err := cts.DecodeInto(input); err != nil {
		logs.Errorf("failed to get cvm image list, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	rst, err := s.logics.CvmImage().GetCvmImage(cts.Kit, input)
	if err != nil {
		logs.Errorf("failed to get cvm image list, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return rst, nil
}

// GetBizCvmImage 业务维度镜像查询，返回公共镜像 + 该业务的私有镜像
func (s *service) GetBizCvmImage(cts *rest.Contexts) (interface{}, error) {
	bizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		logs.Errorf("failed to get bk_biz_id from path, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	// 鉴权：需要业务访问权限
	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bizID,
	}); err != nil {
		logs.Errorf("get biz cvm image auth failed, bizID: %d, err: %v, rid: %s", bizID, err, cts.Kit.Rid)
		return nil, err
	}

	input := new(types.GetCvmImageParam)
	if err := cts.DecodeInto(input); err != nil {
		logs.Errorf("failed to decode GetCvmImageParam, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	rst, err := s.logics.CvmImage().GetBizCvmImage(cts.Kit, bizID, input)
	if err != nil {
		logs.Errorf("failed to get biz cvm image list, bizID: %d, err: %v, rid: %s", bizID, err, cts.Kit.Rid)
		return nil, err
	}

	return rst, nil
}

// BatchEnableImageToApplyCVM 批量允许镜像用于申领CVM
func (s *service) BatchEnableImageToApplyCVM(cts *rest.Contexts) (interface{}, error) {
	return s.batchOpImageToApplyCVM(cts, s.logics.CvmImage().BatchEnableImageCvm)
}

// BatchDisableImageToApplyCVM 批量禁止镜像用于申领CVM
func (s *service) BatchDisableImageToApplyCVM(cts *rest.Contexts) (interface{}, error) {
	return s.batchOpImageToApplyCVM(cts, s.logics.CvmImage().BatchDisableImageCvm)
}

// batchOpImageToApplyCVM 批量操作镜像用于申领CVM的通用处理函数
func (s *service) batchOpImageToApplyCVM(cts *rest.Contexts, opFunc func(*kit.Kit, []string) error) (
	interface{}, error) {

	req := new(types.BatchOpImageToApplyCVMReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to decode batch operate image cvm request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate batch operate image cvm request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	if err := opFunc(cts.Kit, req.ImageIDs); err != nil {
		logs.Errorf("failed to batch operate image cvm, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// UpdateImageBizTag 更新镜像业务标签
func (s *service) UpdateImageBizTag(cts *rest.Contexts) (interface{}, error) {
	imageID := cts.PathParameter("image_id").String()
	if imageID == "" {
		return nil, errf.New(errf.InvalidParameter, "image_id is required")
	}

	req := new(dataproto.UpdateImageBizTagReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to decode update image biz tag request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate update image biz tag request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 鉴权：需要IaaS资源-资源操作权限
	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{Basic: &meta.Basic{
		Type: meta.Image, Action: meta.Update}}); err != nil {
		logs.Errorf("update image biz tag auth failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	// 调用 data-service 更新镜像业务标签
	if err := s.client.DataService().Global.UpdateImageBizTag(cts.Kit, imageID, req); err != nil {
		logs.Errorf("failed to update image biz tag, imageID: %s, req: %+v, err: %v, rid: %s",
			imageID, req, err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
