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

// Package plugin defines the core types of an IaC plugin record and runtime models.
package plugin

import (
	"encoding/json"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	cvt "hcm/pkg/tools/converter"

	tfjson "github.com/hashicorp/terraform-json"
)

// IaCPlugin IaC 插件注册记录，每个插件的每个版本一条。平台级，不区分租户。
type IaCPlugin struct {
	ID             string                   `json:"id"`
	PluginName     string                   `json:"plugin_name"`
	PluginVersion  string                   `json:"plugin_version"`
	Protocol       enumor.IaCPluginProtocol `json:"protocol"`
	State          enumor.IaCPluginState    `json:"state"`
	Message        string                   `json:"message"`
	Extension      json.RawMessage          `json:"extension"`
	Memo           *string                  `json:"memo"`
	*core.Revision `json:",inline"`
}

const (
	// MinListLimit is the minimum allowed limit for POST /list.
	MinListLimit = 1
	// MaxListLimit is the maximum allowed limit for POST /list.
	MaxListLimit = 500
)

// PathStep represents a single step in a Diagnostic attribute path.
// Exactly one of Attribute, Key, or Index must be set.
type PathStep struct {
	// Attribute selects an object attribute or nested block by schema field name.
	Attribute *string `json:"attribute,omitempty"`
	// Key selects an element of a map by key string.
	Key *string `json:"key,omitempty"`
	// Index selects an element of a list by 0-based index.
	Index *int `json:"index,omitempty"`
}

// NewAttributeStep creates a PathStep for an attribute or nested block name.
func NewAttributeStep(attr string) PathStep {
	return PathStep{Attribute: cvt.ValToPtr(attr)}
}

// NewKeyStep creates a PathStep for a map key.
func NewKeyStep(key string) PathStep {
	return PathStep{Key: cvt.ValToPtr(key)}
}

// NewIndexStep creates a PathStep for a 0-based list index.
func NewIndexStep(index int) PathStep {
	return PathStep{Index: cvt.ValToPtr(index)}
}

// Diagnostic describes a single validation issue in user configuration.
// It embeds tfjson.Diagnostic to stay consistent with Terraform diagnostic JSON format,
// and supplements protocol fields Path and Message.
type Diagnostic struct {
	tfjson.Diagnostic

	// Path is the ordered path from the root of config; empty array means not tied to a single field.
	Path []PathStep `json:"path"`
	// Message is the human-readable issue description without repeating the field name.
	Message string `json:"message"`
}

// EffectiveSeverity returns the diagnostic severity, defaulting to "error" when empty.
func (d Diagnostic) EffectiveSeverity() tfjson.DiagnosticSeverity {
	if d.Severity == "" {
		return tfjson.DiagnosticSeverityError
	}
	return d.Severity
}

// HealthData is the response data of GET /health.
type HealthData struct {
	// PluginVersion is the current version of the plugin.
	PluginVersion string `json:"plugin_version"`
}

// Progress represents the optional execution progress in Execute/Apply, GetResult, and Stop responses.
type Progress struct {
	// Total is the optional total item/stage count.
	Total *int `json:"total,omitempty"`
	// Succeeded is the optional succeeded item/stage count.
	Succeeded *int `json:"succeeded,omitempty"`
	// Failed is the optional failed item/stage count.
	Failed *int `json:"failed,omitempty"`
	// Description is the optional display message for progress.
	Description string `json:"description,omitempty"`
}

// ListItem represents a single resource item returned by POST /list.
type ListItem struct {
	// ImportID is the external object ID that can be passed to Import.
	ImportID string `json:"import_id"`
	// State is the complete state of the resource object.
	State map[string]any `json:"state"`
}

// ListData is the response data of POST /list.
type ListData struct {
	// Items contains the resource objects on the current page.
	Items []ListItem `json:"items"`
	// NextPageToken is the cursor for the next page; empty string indicates the last page.
	NextPageToken string `json:"next_page_token"`
}
