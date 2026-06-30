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

// Package bkaidev provides a REST client for the BKAIDev API gateway.
package bkaidev

import (
	"fmt"
	"net/http"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/rest/client"
	apigateway "hcm/pkg/thirdparty/api-gateway"
	"hcm/pkg/tools/ssl"

	"github.com/prometheus/client_golang/prometheus"
)

// Client defines the BKAIDev API operations used by the agent-server skill and prompt syncers.
type Client interface {
	// ListSkills calls list_app_v1_skills.
	ListSkills(kt *kit.Kit, req *ListSkillsReq) ([]SkillListItem, error)
	// RetrieveSkill calls retrieve_app_v1_skills.
	RetrieveSkill(kt *kit.Kit, req *RetrieveSkillReq) (*SkillDetail, error)
	// GetSkillDownloadURL calls retrieve_app_v1_skills_download.
	GetSkillDownloadURL(kt *kit.Kit, req *GetSkillDownloadURLReq) (*SkillDownloadResult, error)
	// ListSkillVersions calls retrieve_app_v1_skills_versions.
	ListSkillVersions(kt *kit.Kit, req *ListSkillVersionsReq) ([]SkillVersionItem, error)
	// ListPrompts calls list_app_v1_prompts.
	ListPrompts(kt *kit.Kit, req *ListPromptsReq) (*ListPromptsResp, error)
	// RetrievePrompt calls retrieve_app_v1_prompts.
	RetrievePrompt(kt *kit.Kit, req *RetrievePromptReq) (*PromptDetail, error)
}

// NewClient initializes a BKAIDev skill API gateway client.
func NewClient(cfg *cc.ApiGateway, reg prometheus.Registerer) (Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("bkaidev skill client: api gateway config is nil")
	}
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("bkaidev skill client: api gateway endpoints is not set")
	}

	tls := &ssl.TLSConfig{
		InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		CertFile:           cfg.TLS.CertFile,
		KeyFile:            cfg.TLS.KeyFile,
		CAFile:             cfg.TLS.CAFile,
		Password:           cfg.TLS.Password,
	}
	cli, err := client.NewClient(tls)
	if err != nil {
		return nil, fmt.Errorf("bkaidev skill client: create http client: %w", err)
	}

	cap := &client.Capability{
		Client: cli,
		Discover: &apigateway.Discovery{
			Name:    "bkaidevSkillApiGateWay",
			Servers: cfg.Endpoints,
		},
		MetricOpts: client.MetricOption{Register: reg},
	}

	return &bkaidevClient{
		config: cfg,
		client: rest.NewClient(cap, "/"),
	}, nil
}

var _ Client = (*bkaidevClient)(nil)

// bkaidevClient implements Client.
type bkaidevClient struct {
	config *cc.ApiGateway
	client rest.ClientInterface
}

// bkaidevResp is the BKAIDev skill API response envelope.
// Unlike the standard ApiGatewayResp, code is a string (e.g. "success") rather than int.
type bkaidevResp[T any] struct {
	Result    bool    `json:"result"`
	Code      string  `json:"code"`
	Message   *string `json:"message"`
	RequestID string  `json:"request_id"`
	TraceID   string  `json:"trace_id"`
	Data      T       `json:"data"`
}

func (r *bkaidevResp[T]) successful() bool {
	return r.Result && r.Code == "success"
}

func (r *bkaidevResp[T]) errorMessage() string {
	if r.Message != nil {
		return *r.Message
	}
	return ""
}

// BKAIDevApiGatewayGet performs a GET request against BKAIDev skill APIs with the dedicated response envelope.
func BKAIDevApiGatewayGet[T any](c *bkaidevClient, kt *kit.Kit, params map[string]string, url string,
	urlParams ...any) (T, error) {

	var zero T

	resp := new(bkaidevResp[T])
	err := c.client.Get().
		SubResourcef(url, urlParams...).
		WithContext(kt.Ctx).
		WithHeaders(c.authHeader(kt)).
		WithParams(params).
		Do().Into(resp)
	if err != nil {
		logs.Errorf("call bkaidev api failed, err: %v, url: %s, rid: %s", err, url, kt.Rid)
		return zero, err
	}

	if !resp.successful() {
		apiErr := fmt.Errorf("bkaidev api error: code=%s msg=%s request_id=%s trace_id=%s",
			resp.Code, resp.errorMessage(), resp.RequestID, resp.TraceID)
		logs.Errorf("bkaidev api returns error, url: %s, err: %v, rid: %s", url, apiErr, kt.Rid)
		return zero, apiErr
	}

	return resp.Data, nil
}

func (c *bkaidevClient) authHeader(kt *kit.Kit) http.Header {
	user := kt.User
	if len(c.config.User) > 0 {
		user = c.config.User
	}
	auth := fmt.Sprintf(`{"bk_app_code":"%s","bk_app_secret":"%s","bk_username":"%s"}`,
		c.config.AppCode, c.config.AppSecret, user)
	h := make(http.Header)
	h.Set(constant.BKGWAuthKey, auth)
	h.Set(constant.RidKey, kt.Rid)
	h.Set(constant.ContentTypeKey, "application/json")
	return h
}

// ListSkills calls list_app_v1_skills and returns the skill list.
func (c *bkaidevClient) ListSkills(kt *kit.Kit, req *ListSkillsReq) ([]SkillListItem, error) {
	if err := req.Validate(); err != nil {
		logs.Errorf("validate list skills request failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	page, err := BKAIDevApiGatewayGet[BKAIDevListSkillsResp](c, kt, req.QueryParams(), "/app/v1/skills/")
	if err != nil {
		logs.Errorf("list skills from bkaidev failed, err: %v, rid: %s", err, kt.Rid)
		return nil, fmt.Errorf("list skills: %w", err)
	}
	if page.Results == nil {
		return []SkillListItem{}, nil
	}
	return page.Results, nil
}

// RetrieveSkill calls retrieve_app_v1_skills and returns the full skill detail.
func (c *bkaidevClient) RetrieveSkill(kt *kit.Kit, req *RetrieveSkillReq) (*SkillDetail, error) {
	if err := req.Validate(); err != nil {
		logs.Errorf("validate retrieve skills request failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	data, err := BKAIDevApiGatewayGet[SkillDetail](c, kt, req.QueryParams(), "/app/v1/skills/%d/", req.SkillID)
	if err != nil {
		logs.Errorf("retrieve skills from bkaidev failed, err: %v, rid: %s", err, kt.Rid)
		return nil, fmt.Errorf("retrieve skill %d: %w", req.SkillID, err)
	}
	return &data, nil
}

// GetSkillDownloadURL calls retrieve_app_v1_skills_download.
func (c *bkaidevClient) GetSkillDownloadURL(kt *kit.Kit, req *GetSkillDownloadURLReq) (*SkillDownloadResult, error) {
	if err := req.Validate(); err != nil {
		logs.Errorf("get skill download url request failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	data, err := BKAIDevApiGatewayGet[SkillDownloadResult](c, kt, nil, "/app/v1/skills/%d/download/", req.SkillID)
	if err != nil {
		logs.Errorf("get skill download url from bkaidev %d failed, err: %v, rid: %s", req.SkillID, err, kt.Rid)
		return nil, err
	}
	return &data, nil
}

// ListSkillVersions calls retrieve_app_v1_skills_versions.
func (c *bkaidevClient) ListSkillVersions(kt *kit.Kit, req *ListSkillVersionsReq) ([]SkillVersionItem, error) {
	if err := req.Validate(); err != nil {
		logs.Errorf("validate get skill download url request failed, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.Newf(errf.InvalidParameter, "validate get skill download url request failed: %v", err)
	}

	data, err := BKAIDevApiGatewayGet[[]SkillVersionItem](c, kt, nil, "/app/v1/skills/%d/versions/", req.SkillID)
	if err != nil {
		logs.Errorf("list skill versions %d failed, err: %v, rid: %s", req.SkillID, err, kt.Rid)
		return nil, err
	}
	if data == nil {
		return []SkillVersionItem{}, nil
	}
	return data, nil
}
