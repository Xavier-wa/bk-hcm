/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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
	ziyanlogic "hcm/cmd/cloud-server/logics/ziyan"
	hclb "hcm/pkg/api/hc-service/load-balancer"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/thirdparty/api-gateway/itsm"
)

// PrepareReq 预处理请求参数
func (a *ApplicationOfCreateZiyanLB) PrepareReq() error {
	// 补充业务tag
	tags, err := ziyanlogic.GenTagsForBizs(a.Cts.Kit, a.Client.DataService(), cmdb.CmdbClient(), a.req.BkBizID)
	if err != nil {
		logs.Errorf("fail to generate tags for load balancer application: err: %v,req: %+v, rid: %s",
			err, a.req, a.Cts.Kit.Rid)
		return err
	}
	a.req.Tags = append(a.req.Tags, tags...)
	return nil
}

// GenerateApplicationContent 获取预处理过的数据，以interface格式
func (a *ApplicationOfCreateZiyanLB) GenerateApplicationContent() interface{} {
	// 需要将Vendor也存储进去
	return &struct {
		*hclb.TCloudZiyanLoadBalancerCreateReq `json:",inline"`
		Vendor                                 enumor.Vendor `json:"vendor"`
	}{
		TCloudZiyanLoadBalancerCreateReq: a.req,
		Vendor:                           a.Vendor(),
	}
}

// PrepareReqFromContent 预处理请求参数，对于申请内容来着DB，其实入库前是加密了的
func (a *ApplicationOfCreateZiyanLB) PrepareReqFromContent() error {
	return nil
}

// GetItsmApprover 获取itsm审批人
func (a *ApplicationOfCreateZiyanLB) GetItsmApprover(kt *kit.Kit, managers []string) ([]itsm.VariableApprover, error) {
	return a.GetItsmPlatformAndAccountApprover(kt, managers, a.req.AccountID)
}

// GetBkBizIDs return biz ids
func (a *ApplicationOfCreateZiyanLB) GetBkBizIDs() []int64 {
	return []int64{a.req.BkBizID}
}
