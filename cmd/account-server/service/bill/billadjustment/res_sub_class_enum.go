/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package billadjustment

import (
	"hcm/pkg/api/account-server/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// ListAdjustmentGpuCard 查询指定云厂商可选的调账卡型枚举，供前端渲染资源子类下拉。
func (b *billAdjustmentSvc) ListAdjustmentGpuCard(cts *rest.Contexts) (any, error) {
	vendor, err := parseResSubClassVendor(cts)
	if err != nil {
		return nil, err
	}

	err = b.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AccountBill, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	source, err := b.loadResSubClassSource(cts.Kit)
	if err != nil {
		logs.Errorf("fail to load res sub class source for gpu card enum, vendor: %s, err: %v, rid: %s",
			vendor, err, cts.Kit.Rid)
		return nil, err
	}

	return &bill.AdjustmentGpuCardListResult{Details: listGpuCards(vendor, source)}, nil
}

// ListAdjustmentAPIBrand 查询指定云厂商可选的调账模型厂商枚举，供前端渲染资源子类下拉。
func (b *billAdjustmentSvc) ListAdjustmentAPIBrand(cts *rest.Contexts) (any, error) {
	vendor, err := parseResSubClassVendor(cts)
	if err != nil {
		return nil, err
	}

	err = b.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AccountBill, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	return &bill.AdjustmentAPIBrandListResult{Details: listAPIBrands(vendor)}, nil
}

// parseResSubClassVendor 取出路径参数中的云厂商并校验是否在支持范围内。
func parseResSubClassVendor(cts *rest.Contexts) (enumor.Vendor, error) {
	vendor := enumor.Vendor(cts.PathParameter("vendor").String())
	if !isResSubClassVendorSupported(vendor) {
		return "", errf.Newf(errf.InvalidParameter, "unsupported vendor for res sub class: %s", vendor)
	}
	return vendor, nil
}
