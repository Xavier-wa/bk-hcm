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

// IacSpecNode IaC Spec 目录与文件节点核心模型。
type IacSpecNode struct {
	ID              string                 `json:"id"`
	RepositoryID    string                 `json:"repository_id"`
	SpecID          string                 `json:"spec_id"`
	ParentID        string                 `json:"parent_id"`
	Level           int                    `json:"level"`
	NodeType        enumor.IacSpecNodeType `json:"node_type"`
	NodeName        string                 `json:"node_name"`
	ChildrenNodeIDs *types.JsonField       `json:"children_node_ids"`
	FileSize        int64                  `json:"file_size"`
	Content         *string                `json:"content"`
	TenantID        string                 `json:"tenant_id,omitempty"`
	*core.Revision  `json:",inline"`
}
