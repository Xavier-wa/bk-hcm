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

package tableiac

import (
	"testing"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/table/types"
)

func TestIacRepositoryTable_Validate(t *testing.T) {
	valid := IacRepositoryTable{
		BkBizID: 100,
		Name:    "prod-stack",
		Creator: "admin",
		Reviser: "admin",
	}
	if err := valid.InsertValidate(); err != nil {
		t.Fatalf("expected valid repository insert, got err: %v", err)
	}

	invalidBiz := valid
	invalidBiz.BkBizID = 0
	if err := invalidBiz.InsertValidate(); err == nil {
		t.Fatal("expected bk_biz_id=0 to fail insert validate")
	}

	upd := IacRepositoryTable{Name: "new-name", Reviser: "admin"}
	if err := upd.UpdateValidate(); err != nil {
		t.Fatalf("expected valid repository update, got err: %v", err)
	}
}

func TestIacSpecTable_Validate(t *testing.T) {
	valid := IacSpecTable{
		BkBizID:      100,
		RepositoryID: "repo-1",
		Version:      "v1",
		Status:       enumor.IacSpecStatusUploading,
		Creator:      "admin",
		Reviser:      "admin",
	}
	if err := valid.InsertValidate(); err != nil {
		t.Fatalf("expected valid spec insert, got err: %v", err)
	}

	invalidStatus := valid
	invalidStatus.Status = "draft"
	if err := invalidStatus.InsertValidate(); err == nil {
		t.Fatal("expected invalid status to fail insert validate")
	}
}

func TestIacSpecNodeTable_Validate(t *testing.T) {
	children := types.JsonField(`["n1","n2"]`)
	dirNode := IacSpecNodeTable{
		RepositoryID:    "repo-1",
		SpecID:          "spec-1",
		Level:           0,
		NodeType:        enumor.IacSpecNodeTypeDir,
		NodeName:        ".",
		ChildrenNodeIDs: &children,
		Creator:         "admin",
		Reviser:         "admin",
	}
	if err := dirNode.InsertValidate(); err != nil {
		t.Fatalf("expected valid dir node insert, got err: %v", err)
	}

	content := "resource \"tcloud_vpc\" \"v\" {}"
	badDir := dirNode
	badDir.Content = &content
	if err := badDir.InsertValidate(); err == nil {
		t.Fatal("expected dir node with content to fail insert validate")
	}

	fileNode := IacSpecNodeTable{
		RepositoryID: "repo-1",
		SpecID:       "spec-1",
		ParentID:     "n0",
		Level:        1,
		NodeType:     enumor.IacSpecNodeTypeFile,
		NodeName:     "main.tf",
		FileSize:     int64(len(content)),
		Content:      &content,
		Creator:      "admin",
		Reviser:      "admin",
	}
	if err := fileNode.InsertValidate(); err != nil {
		t.Fatalf("expected valid file node insert, got err: %v", err)
	}
}

func TestIacStateTable_ValidateAndJsonFieldIsNull(t *testing.T) {
	if !types.JsonField("null").IsNull() {
		t.Fatal("expected JsonField(\"null\").IsNull() to be true")
	}
	if types.JsonField("{}").IsNull() || types.JsonField("").IsNull() {
		t.Fatal("expected non-null JsonField to return false for IsNull()")
	}

	emptySpecID := ""
	valid := IacStateTable{
		RepositoryID:     "repo-1",
		BlockID:          "tencentcloud_instance.web",
		PluginInstanceID: "inst-1",
		Kind:             enumor.IacStateKindResource,
		Current:          types.JsonField(`{"state":{"id":"ins-1"}}`),
		Deposed:          types.JsonField(`{}`),
		ImportFromSpecID: &emptySpecID,
		Creator:          "admin",
		Reviser:          "admin",
	}
	if err := valid.InsertValidate(); err != nil {
		t.Fatalf("expected valid state insert, got err: %v", err)
	}

	badKind := valid
	badKind.Kind = "import"
	if err := badKind.InsertValidate(); err == nil {
		t.Fatal("expected invalid kind to fail insert validate")
	}

	upd := IacStateTable{
		Current: types.JsonField("null"),
		Reviser: "admin",
	}
	if err := upd.UpdateValidate(); err != nil {
		t.Fatalf("expected valid state update with null current, got err: %v", err)
	}
}
