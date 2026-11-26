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

import "testing"

func TestIacSpecStatus_Validate(t *testing.T) {
	valid := []IacSpecStatus{IacSpecStatusUploading, IacSpecStatusUploaded, IacSpecStatusUploadFailed}
	for _, s := range valid {
		if err := s.Validate(); err != nil {
			t.Fatalf("expected %q to be valid, got err: %v", s, err)
		}
	}

	if err := IacSpecStatus("invalid").Validate(); err == nil {
		t.Fatal("expected invalid status to fail validation")
	}
}

func TestIacSpecNodeType_Validate(t *testing.T) {
	valid := []IacSpecNodeType{IacSpecNodeTypeDir, IacSpecNodeTypeFile}
	for _, nt := range valid {
		if err := nt.Validate(); err != nil {
			t.Fatalf("expected %q to be valid, got err: %v", nt, err)
		}
	}

	if err := IacSpecNodeType("symlink").Validate(); err == nil {
		t.Fatal("expected invalid node type to fail validation")
	}
}

func TestIacStateKind_Validate(t *testing.T) {
	valid := []IacStateKind{IacStateKindResource, IacStateKindData}
	for _, k := range valid {
		if err := k.Validate(); err != nil {
			t.Fatalf("expected %q to be valid, got err: %v", k, err)
		}
	}

	if err := IacStateKind("import").Validate(); err == nil {
		t.Fatal("expected invalid state kind to fail validation")
	}
}
