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

package enumor

import "fmt"

// IacSpecStatus IaC Spec 版本上传状态。
type IacSpecStatus string

const (
	// IacSpecStatusUploading 上传写入中。
	IacSpecStatusUploading IacSpecStatus = "uploading"
	// IacSpecStatusUploaded 已上传生效，只读可执行。
	IacSpecStatusUploaded IacSpecStatus = "uploaded"
	// IacSpecStatusUploadFailed 上传失败，不可执行。
	IacSpecStatusUploadFailed IacSpecStatus = "upload_failed"
)

// Validate 校验 Spec 版本状态是否合法。
func (s IacSpecStatus) Validate() error {
	switch s {
	case IacSpecStatusUploading, IacSpecStatusUploaded, IacSpecStatusUploadFailed:
		return nil
	default:
		return fmt.Errorf("unsupported iac spec status: %s", s)
	}
}

// IacSpecNodeType IaC Spec 节点类型。
type IacSpecNodeType string

const (
	// IacSpecNodeTypeDir 目录节点。
	IacSpecNodeTypeDir IacSpecNodeType = "dir"
	// IacSpecNodeTypeFile 文件节点。
	IacSpecNodeTypeFile IacSpecNodeType = "file"
)

// Validate 校验 Spec 节点类型是否合法。
func (t IacSpecNodeType) Validate() error {
	switch t {
	case IacSpecNodeTypeDir, IacSpecNodeTypeFile:
		return nil
	default:
		return fmt.Errorf("unsupported iac spec node type: %s", t)
	}
}

// IacStateKind IaC 受管实例状态类别。
type IacStateKind string

const (
	// IacStateKindResource 受管资源实例。
	IacStateKindResource IacStateKind = "resource"
	// IacStateKindData 数据源实例。
	IacStateKindData IacStateKind = "data"
)

// Validate 校验受管实例状态类别是否合法。
func (k IacStateKind) Validate() error {
	switch k {
	case IacStateKindResource, IacStateKindData:
		return nil
	default:
		return fmt.Errorf("unsupported iac state kind: %s", k)
	}
}
