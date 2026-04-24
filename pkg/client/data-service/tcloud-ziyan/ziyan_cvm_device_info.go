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

package ziyan

import (
	"context"
	"net/http"

	"hcm/pkg/api/core"
	dataproto "hcm/pkg/api/data-service"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/rest"
)

// ZiyanCvmDeviceInfoClient is data service ziyan cvm device info api client.
type ZiyanCvmDeviceInfoClient struct {
	client rest.ClientInterface
}

// NewZiyanCvmDeviceInfoClient create a new ziyan cvm device info api client.
func NewZiyanCvmDeviceInfoClient(client rest.ClientInterface) *ZiyanCvmDeviceInfoClient {
	return &ZiyanCvmDeviceInfoClient{
		client: client,
	}
}

// BatchCreate batch create ziyan cvm device info.
func (c *ZiyanCvmDeviceInfoClient) BatchCreate(ctx context.Context, h http.Header,
	req *cvmapplyproto.BatchCreateZiyanCvmDeviceInfoReq) (*core.BatchCreateResult, error) {

	resp := new(core.BatchCreateResp)

	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/devices/batch/create").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// List list ziyan cvm device info.
func (c *ZiyanCvmDeviceInfoClient) List(ctx context.Context, h http.Header,
	req *cvmapplyproto.ZiyanCvmDeviceInfoListReq) (*cvmapplyproto.ZiyanCvmDeviceInfoListResult, error) {

	resp := new(cvmapplyproto.ZiyanCvmDeviceInfoListResp)

	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/devices/list").
		WithHeaders(h).
		Do().
		Into(resp)

	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// BatchUpdate batch update ziyan cvm device info.
func (c *ZiyanCvmDeviceInfoClient) BatchUpdate(ctx context.Context, h http.Header,
	req *cvmapplyproto.BatchUpdateZiyanCvmDeviceInfoReq) error {

	resp := new(rest.BaseResp)

	err := c.client.Patch().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/devices/batch").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return err
	}

	if resp.Code != errf.OK {
		return errf.New(resp.Code, resp.Message)
	}

	return nil
}

// BatchDelete batch delete ziyan cvm device info.
func (c *ZiyanCvmDeviceInfoClient) BatchDelete(ctx context.Context, h http.Header,
	req *dataproto.BatchDeleteReq) error {

	resp := new(rest.BaseResp)

	err := c.client.Delete().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/devices/batch").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return err
	}

	if resp.Code != errf.OK {
		return errf.New(resp.Code, resp.Message)
	}

	return nil
}
