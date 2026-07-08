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

package caiche

import "hcm/pkg/criteria/enumor"

// GetTokenResp get token response
type GetTokenResp struct {
	ID      string          `json:"id"`
	JsonRPC string          `json:"jsonrpc"`
	Result  *GetTokenResult `json:"result"`
	Code    int             `json:"code"`
	Msg     string          `json:"msg"`
}

// GetTokenResult get token result
type GetTokenResult struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// ListDeviceV2Resp list device v2 response
type ListDeviceV2Resp struct {
	JsonRPC  string              `json:"jsonrpc"`
	Result   *DeviceListV2Result `json:"result"`
	XTraceID string              `json:"x_trace_id"`
}

// ListProjectsResp list projects response
type ListProjectsResp struct {
	JsonRPC  string    `json:"jsonrpc"`
	Result   []Project `json:"result"`
	XTraceID string    `json:"x_trace_id"`
}

// Project 裁撤项目
type Project struct {
	// ID 项目ID
	ID int `json:"id"`
	// ProjectName 项目名称
	ProjectName string `json:"projectName"`
	// ProjectType 项目类型
	ProjectType enumor.ProjectType `json:"projectType"`
}

// DeviceListV2Result device list v2 result
type DeviceListV2Result struct {
	Data  []DeviceV2 `json:"data"`
	Total int        `json:"total"`
}

// DeviceV2 device v2
type DeviceV2 struct {
	ID                    int                 `json:"id"`
	AbolishTime           string              `json:"abolishTime"`
	ProjectID             int                 `json:"projectId"`
	ProjectName           string              `json:"projectName"`
	SerAssetID            string              `json:"serAssetId"`
	SvrOperator           string              `json:"svrOperator"`
	SvrBakOperator        string              `json:"svrBakOperator"`
	ExpectAbolishTime     string              `json:"expectAbolishTime"`
	RegionName            string              `json:"regionName"`
	ZoneName              string              `json:"zoneName"`
	SzoneName             string              `json:"szoneName"`
	ModuleBsiType         string              `json:"moduleBsiType"`
	IdcParentName         string              `json:"idcParentName"`
	IdcName               string              `json:"idcName"`
	ServerRack            string              `json:"serverRack"`
	RckID                 int                 `json:"rckId"`
	PosCode               string              `json:"posCode"`
	PosID                 int                 `json:"posId"`
	IdcCity               string              `json:"idcCity"`
	PlanProductName       string              `json:"planProductName"`
	OperationProductName  string              `json:"operationProductName"`
	BsiPath               string              `json:"bsiPath"`
	AbolishOperator       string              `json:"abolishOperator"`
	AbolishPhase          enumor.AbolishPhase `json:"abolishPhase"`
	SvrDeviceClassName    string              `json:"svrDeviceClassName"`
	CPULogicCoreNum       int                 `json:"cpuLogicCoreNum"`
	GPUCardType           string              `json:"gpuCardType"`
	ServerLanIP           string              `json:"serverLanIp"`
	SvrFirstUseTime       string              `json:"svrFirstUseTime"`
	SvrTypeName           string              `json:"svrTypeName"`
	EqsName               string              `json:"eqsName"`
	ModName               string              `json:"modName"`
	SvrOwnerAssetId       string              `json:"svrOwnerAssetId"`
	CmdbDeptName          string              `json:"cmdbDeptName"`
	VirtualDepartmentName string              `json:"virtualDepartmentName"`
	ObsBg                 string              `json:"obsBg"`
	AvailabilityZoneName  string              `json:"availabilityZoneName"`
	CustomMigrateType     string              `json:"customMigrateType"`
	ConfirmTime           string              `json:"confirmTime"`
	TechProduct           string              `json:"techProduct"`
}
