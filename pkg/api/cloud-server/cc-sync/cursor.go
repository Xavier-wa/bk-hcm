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

// Package apiccsync defines request/response protocols for CC sync cursor admin operations.
package apiccsync

import (
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"

	"hcm/pkg/thirdparty/api-gateway/cmdb"
)

// ResetWatchCursorReq is the request to reset cc watch event consume cursor.
type ResetWatchCursorReq struct {
	// Resource is the resource type to reset, only host / host_relation are supported.
	Resource cmdb.CursorType `json:"resource" validate:"required"`
}

// Validate ResetWatchCursorReq.
func (req *ResetWatchCursorReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	// 仅支持 watch 流当前覆盖的资源类型
	switch req.Resource {
	case cmdb.HostType, cmdb.HostRelation:
	default:
		return errf.Newf(errf.InvalidParameter, "unsupported resource type: %s", req.Resource)
	}

	return nil
}

// ResetWatchCursorResp is the response of resetting cc watch event consume cursor.
type ResetWatchCursorResp struct {
	// PreCursor is the cursor before reset, empty means it was already at latest.
	PreCursor string `json:"pre_cursor"`
}
