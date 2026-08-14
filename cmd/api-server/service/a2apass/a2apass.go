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

// Package a2apass provides api-server northbound A2A passthrough reverse proxy.
package a2apass

import (
	"errors"
	"net/http"
	"strings"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
)

// Register mounts northbound A2A passthrough routes to the given ServeMux.
func Register(mux *http.ServeMux, cfg cc.A2APassthroughSetting, dis serviced.Discover) error {
	if mux == nil {
		return errors.New("a2apass.Register: mux is nil")
	}
	if !cfg.Enable {
		logs.Infof("a2apass: a2aPassthrough.enable=false, skip mounting A2A passthrough paths")
		return nil
	}
	cfg = normalizeSetting(cfg)
	if err := cfg.Validate(); err != nil {
		return err
	}

	handler := BuildPassthroughHandler(cfg, dis)
	basePath := normalizeBasePath(cfg.BasePath)
	jsonRPCPath := basePath + constant.A2AJSONRPCSubPath
	wellKnownPath := basePath + "/.well-known/"

	mux.Handle(jsonRPCPath, handler)
	mux.Handle(wellKnownPath, handler)

	logs.Infof("a2apass: mounted A2A passthrough at %s and %s", jsonRPCPath, wellKnownPath)
	return nil
}

func normalizeBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "/" {
		return constant.A2ABasePathDefault
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return strings.TrimSuffix(basePath, "/")
}
