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

package bkaidev

import (
	"errors"
	"fmt"
	"strconv"
)

// ListSkillsReq is the query parameters for list_app_v1_skills.
type ListSkillsReq struct {
	// Fuzzy is the fuzzy search keyword.
	Fuzzy string `json:"fuzzy,omitempty"`
	// GenerateType is the public/generate type filter.
	GenerateType string `json:"generate_type,omitempty"`
	// GroupType is the group type filter.
	GroupType string `json:"group_type,omitempty"`
	// OrderBy is the sort field.
	OrderBy string `json:"order_by,omitempty"`
	// OrderMethod is the sort direction.
	OrderMethod string `json:"order_method,omitempty"`
	// SpaceID is the space identifier (required).
	SpaceID string `json:"space_id"`
	// TagID is the tag identifier filter.
	TagID string `json:"tag_id,omitempty"`
	// TagName is the tag name filter.
	TagName string `json:"tag_name,omitempty"`
}

// Validate validates ListSkillsReq.
func (r *ListSkillsReq) Validate() error {
	if r == nil {
		return errors.New("list skills request is nil")
	}
	if r.SpaceID == "" {
		return errors.New("space_id is required")
	}
	return nil
}

// QueryParams converts ListSkillsReq to gateway query parameters.
func (r *ListSkillsReq) QueryParams() map[string]string {
	params := map[string]string{"space_id": r.SpaceID}
	// TODO: setQueryParam 看看是否有工具函数可以优化
	setQueryParam(params, "fuzzy", r.Fuzzy)
	setQueryParam(params, "generate_type", r.GenerateType)
	setQueryParam(params, "group_type", r.GroupType)
	setQueryParam(params, "order_by", r.OrderBy)
	setQueryParam(params, "order_method", r.OrderMethod)
	setQueryParam(params, "tag_id", r.TagID)
	setQueryParam(params, "tag_name", r.TagName)
	return params
}

// BaseSkill holds fields shared by list_app_v1_skills items and retrieve_app_v1_skills detail.
type BaseSkill struct {
	ID            int64      `json:"id"`
	Version       string     `json:"version"`
	SkillName     string     `json:"skill_name"`
	SkillCode     string     `json:"skill_code"`
	Description   string     `json:"description"`
	URL           string     `json:"url"`
	Icon          string     `json:"icon"`
	GenerateType  string     `json:"generate_type"`
	IsPublic      bool       `json:"is_public"`
	SpaceID       string     `json:"space_id"`
	DownloadCount int64      `json:"download_count"`
	InstallCount  int64      `json:"install_count"`
	FileName      string     `json:"file_name"`
	FileSize      int64      `json:"file_size"`
	FileType      string     `json:"file_type"`
	TagNames      [][]string `json:"tag_names"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     string     `json:"created_at"`
	UpdatedAt     string     `json:"updated_at"`
}

// InstallKey returns the local directory name for this skill under the skill root.
func (s BaseSkill) InstallKey() string {
	if s.SkillCode != "" {
		return s.SkillCode
	}
	if s.SkillName != "" {
		return s.SkillName
	}
	return fmt.Sprintf("skill-%d", s.ID)
}

// SkillListItem is a single element in the list_app_v1_skills response data array.
type SkillListItem struct {
	BaseSkill         `json:",inline"`
	UpdatedBy         string                 `json:"updated_by"`
	Property          map[string]interface{} `json:"property"`
	TenantID          string                 `json:"tenant_id"`
	ImageStatus       string                 `json:"image_status"`
	ImageErrorMessage string                 `json:"image_error_message"`
	Status            string                 `json:"status"`
	RefCount          int64                  `json:"ref_count"`
	Scanner           SkillScanner           `json:"scanner"`
}

// SkillScanner is the scanner object in SkillListItem.
type SkillScanner struct {
	EffectiveStatus   *string `json:"effective_status"`
	EffectiveStatusCN *string `json:"effective_status_cn"`
	LastScanAt        *string `json:"last_scan_at"`
	ReportContent     *string `json:"report_content"`
}

// RetrieveSkillReq is the request for retrieve_app_v1_skills.
type RetrieveSkillReq struct {
	// SkillID is the skill identifier (path parameter).
	SkillID int64 `json:"-"`
	// Version is the target version; empty means latest.
	Version string `json:"-"`
}

// Validate validates RetrieveSkillReq.
func (r *RetrieveSkillReq) Validate() error {
	if r == nil {
		return errors.New("retrieve skill request is nil")
	}
	if r.SkillID <= 0 {
		return errors.New("skill_id must be positive")
	}
	return nil
}

// QueryParams converts optional query fields on RetrieveSkillReq.
func (r *RetrieveSkillReq) QueryParams() map[string]string {
	params := make(map[string]string)
	setQueryParam(params, "version", r.Version)
	return params
}

// SkillDetail is the data object in retrieve_app_v1_skills response.
type SkillDetail struct {
	BaseSkill     `json:",inline"`
	SkillMarkdown string `json:"skill_markdown"`
	FileMD5       string `json:"file_md5"`
	Sandbox       string `json:"sandbox,omitempty"`
}

// GetSkillDownloadURLReq is the request for retrieve_app_v1_skills_download.
type GetSkillDownloadURLReq struct {
	// SkillID is the skill identifier (path parameter).
	SkillID int64 `json:"-"`
}

// Validate validates GetSkillDownloadURLReq.
func (r *GetSkillDownloadURLReq) Validate() error {
	if r == nil {
		return errors.New("get skill download url request is nil")
	}
	if r.SkillID <= 0 {
		return errors.New("skill_id must be positive")
	}
	return nil
}

// SkillDownloadResult is the data object in retrieve_app_v1_skills_download response.
type SkillDownloadResult struct {
	URL           string `json:"url"`
	DownloadCount int64  `json:"download_count"`
}

// ListSkillVersionsReq is the request for retrieve_app_v1_skills_versions.
type ListSkillVersionsReq struct {
	// SkillID is the skill identifier (path parameter).
	SkillID int64 `json:"-"`
}

// Validate validates ListSkillVersionsReq.
func (r *ListSkillVersionsReq) Validate() error {
	if r == nil {
		return errors.New("list skill versions request is nil")
	}
	if r.SkillID <= 0 {
		return errors.New("skill_id must be positive")
	}
	return nil
}

// SkillVersionItem is a single element in the list skill versions response data array.
type SkillVersionItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func setQueryParam(params map[string]string, key, value string) {
	if value != "" {
		params[key] = value
	}
}

// skillIDPath formats skill_id for URL path segments.
func skillIDPath(skillID int64) string {
	return strconv.FormatInt(skillID, 10)
}
