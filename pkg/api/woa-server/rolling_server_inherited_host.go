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

package woaserver

import (
	"fmt"
	"strings"
	"time"

	"hcm/pkg/criteria/validator"
)

// ListInheritedHostsReq is the request for listing inheritable hosts of rolling server projects.
type ListInheritedHostsReq struct {
	BkBizID        int64    `json:"bk_biz_id" validate:"required,gt=0"`
	Region         string   `json:"region" validate:"required,max=128"`
	DeviceFamilies []string `json:"device_families" validate:"required,min=1,max=100,dive,required"`
}

// Validate ListInheritedHostsReq.
func (r *ListInheritedHostsReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	for idx, family := range r.DeviceFamilies {
		if len(strings.TrimSpace(family)) == 0 {
			return fmt.Errorf("device_families[%d] should not be blank", idx)
		}
	}
	return nil
}

// ListInheritedHostsResp is the response of listing inheritable hosts, grouped by device family.
type ListInheritedHostsResp struct {
	Info []*InheritedHostGroup `json:"info"`
}

// InheritedHostGroup is the candidate group of one single device family.
type InheritedHostGroup struct {
	DeviceFamily string                    `json:"device_family"`
	Hosts        []*InheritedHostCandidate `json:"hosts"`
}

// InheritedHostCandidate is one candidate element in the recommend response.
type InheritedHostCandidate struct {
	InheritedHost
	IsRecommended bool `json:"is_recommended"`
}

// InheritedHost is the data part of an inheritable host candidate, shared by the recommend
type InheritedHost struct {
	AssetID            string    `json:"bk_asset_id"`
	InnerIP            string    `json:"bk_host_innerip"`
	CloudInstID        string    `json:"bk_cloud_inst_id"`
	DeviceType         string    `json:"device_type"`
	InstanceChargeType string    `json:"instance_charge_type"`
	BillingStartTime   time.Time `json:"billing_start_time"`
	BillingExpireTime  time.Time `json:"billing_expire_time"`
	ChargeMonths       int       `json:"charge_months"`
}
