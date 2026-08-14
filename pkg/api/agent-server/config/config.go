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

// Package config defines agent-server config API types.
package config

import (
	"hcm/pkg/criteria/validator"
)

// UpsertAccessTokenReq is the request body for upserting a virtual-user access_token.
type UpsertAccessTokenReq struct {
	// VirtualUser is the key in global_config auth/access_token JSON map.
	VirtualUser string `json:"virtual_user" validate:"required"`
	// AccessToken is the BK API gateway access_token for the virtual user.
	AccessToken string `json:"access_token" validate:"required"`
}

// Validate validates the request body.
func (r *UpsertAccessTokenReq) Validate() error {
	return validator.Validate.Struct(r)
}
