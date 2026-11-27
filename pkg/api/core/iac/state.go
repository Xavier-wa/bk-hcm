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

package coreiac

import (
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/table/types"
)

// IacState IaC 受管实例状态核心模型。
// Current 使用指针以区分数据库 NULL（JSON null）与非空 JSON 对象。
type IacState struct {
	ID               string              `json:"id"`
	RepositoryID     string              `json:"repository_id"`
	BlockID          string              `json:"block_id"`
	Kind             enumor.IacStateKind `json:"kind"`
	Current          *types.JsonField    `json:"current"`
	Deposed          types.JsonField     `json:"deposed"`
	LastFlowID       string              `json:"last_flow_id"`
	ImportFromSpecID string              `json:"import_from_spec_id"`
	TenantID         string              `json:"tenant_id,omitempty"`
	*core.Revision   `json:",inline"`
}
