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

	"hcm/pkg/kit"
)

// PromptVariable is a single variable entry in a prompt's variables list.
type PromptVariable struct {
	FieldName  string `json:"field_name"`
	FieldValue string `json:"field_value"`
}

// BasePrompt holds fields shared by list and retrieve prompt responses.
type BasePrompt struct {
	PromptID      int              `json:"prompt_id"`
	PromptName    string           `json:"prompt_name"`
	PromptCode    string           `json:"prompt_code"`
	Content       string           `json:"content"`
	SpaceID       string           `json:"space_id"`
	TenantID      string           `json:"tenant_id"`
	GenerateType  string           `json:"generate_type"`
	IsPublic      bool             `json:"is_public"`
	PromptType    string           `json:"prompt_type"`
	Status        string           `json:"status"`
	Variables     []PromptVariable `json:"variables"`
	Likes         int              `json:"likes"`
	UsedCount     int              `json:"used_count"`
	IsRecommended bool             `json:"is_recommended"`
	TagNames      [][]string       `json:"tag_names"`
	CreatedBy     string           `json:"created_by"`
	UpdatedBy     string           `json:"updated_by"`
	CreatedAt     string           `json:"created_at"`
	UpdatedAt     string           `json:"updated_at"`
}

// PromptListItem is a single element in the list_app_v1_prompts response results array.
type PromptListItem struct {
	BasePrompt   `json:",inline"`
	CollectionID int  `json:"collection_id"`
	Liked        bool `json:"liked"`
}

// PromptDetail is the data object in retrieve_app_v1_prompts response.
type PromptDetail struct {
	BasePrompt   `json:",inline"`
	CollectionID int `json:"collection_id"`
}

// ListPromptsResp is the paginated response for list_app_v1_prompts.
type ListPromptsResp struct {
	Page     int              `json:"page"`
	NumPages int              `json:"num_pages"`
	Count    int              `json:"count"`
	Results  []PromptListItem `json:"results"`
}

// ListPromptsReq is the query parameters for list_app_v1_prompts.
type ListPromptsReq struct {
	// SpaceID is the space identifier.
	SpaceID string `json:"space_id,omitempty"`
	// Page is the page number.
	Page int `json:"page,omitempty"`
	// PageSize is the number of items per page.
	PageSize int `json:"page_size,omitempty"`
	// PromptCode filters by prompt code (fuzzy).
	PromptCode string `json:"prompt_code,omitempty"`
	// PromptName filters by prompt name (fuzzy).
	PromptName string `json:"prompt_name,omitempty"`
	// CreatedBy filters by creator.
	CreatedBy string `json:"created_by,omitempty"`
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
	// TagID is the tag identifier filter.
	TagID string `json:"tag_id,omitempty"`
	// TagName is the tag name filter.
	TagName string `json:"tag_name,omitempty"`
}

// Validate validates ListPromptsReq.
func (r *ListPromptsReq) Validate() error {
	if r == nil {
		return errors.New("list prompts request is nil")
	}
	return nil
}

// QueryParams converts ListPromptsReq to gateway query parameters.
func (r *ListPromptsReq) QueryParams() map[string]string {
	params := make(map[string]string)
	setQueryParam(params, "space_id", r.SpaceID)
	setQueryParam(params, "prompt_code", r.PromptCode)
	setQueryParam(params, "prompt_name", r.PromptName)
	setQueryParam(params, "created_by", r.CreatedBy)
	setQueryParam(params, "fuzzy", r.Fuzzy)
	setQueryParam(params, "generate_type", r.GenerateType)
	setQueryParam(params, "group_type", r.GroupType)
	setQueryParam(params, "order_by", r.OrderBy)
	setQueryParam(params, "order_method", r.OrderMethod)
	setQueryParam(params, "tag_id", r.TagID)
	setQueryParam(params, "tag_name", r.TagName)
	if r.Page > 0 {
		params["page"] = strconv.Itoa(r.Page)
	}
	if r.PageSize > 0 {
		params["page_size"] = strconv.Itoa(r.PageSize)
	}
	return params
}

// RetrievePromptReq is the request for retrieve_app_v1_prompts.
type RetrievePromptReq struct {
	// PromptID is the prompt identifier (path parameter).
	PromptID int `json:"prompt_id"`
	// SpaceID is the optional space identifier (query parameter).
	SpaceID string `json:"space_id"`
}

// Validate validates RetrievePromptReq.
func (r *RetrievePromptReq) Validate() error {
	if r == nil {
		return errors.New("retrieve prompt request is nil")
	}
	if r.PromptID <= 0 {
		return errors.New("prompt_id must be positive")
	}
	return nil
}

// QueryParams converts optional query fields on RetrievePromptReq.
func (r *RetrievePromptReq) QueryParams() map[string]string {
	params := make(map[string]string)
	setQueryParam(params, "space_id", r.SpaceID)
	return params
}

// ListPrompts calls list_app_v1_prompts and returns the paginated response.
func (c *bkaidevClient) ListPrompts(kt *kit.Kit, req *ListPromptsReq) (*ListPromptsResp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	data, err := BKAIDevApiGatewayGet[ListPromptsResp](c, kt, req.QueryParams(), "/app/v1/prompts/")
	if err != nil {
		return nil, fmt.Errorf("list prompts: %v", err)
	}
	if data.Results == nil {
		data.Results = []PromptListItem{}
	}
	return &data, nil
}

// RetrievePrompt calls retrieve_app_v1_prompts and returns the full prompt detail.
func (c *bkaidevClient) RetrievePrompt(kt *kit.Kit, req *RetrievePromptReq) (*PromptDetail, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	data, err := BKAIDevApiGatewayGet[PromptDetail](c, kt, req.QueryParams(), "/app/v1/prompts/%d/", req.PromptID)
	if err != nil {
		return nil, fmt.Errorf("retrieve prompt %d: %w", req.PromptID, err)
	}
	return &data, nil
}
