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

package meta

import (
	"net/http"

	"hcm/cmd/woa-server/logics/meta"
	"hcm/cmd/woa-server/service/capability"
	"hcm/pkg/client"
	"hcm/pkg/dal/dao"
	"hcm/pkg/iam/auth"
	"hcm/pkg/rest"
)

// InitService initial the meta service.
func InitService(c *capability.Capability) {
	s := &service{
		dao:        c.Dao,
		client:     c.Client,
		authorizer: c.Authorizer,
		logics:     meta.New(c.CmdbCli, c.Authorizer, c.Dao),
	}
	h := rest.NewHandler()

	s.initMetaService(h)

	h.Load(c.WebService)
}

type service struct {
	dao        dao.Set
	client     *client.ClientSet
	authorizer auth.Authorizer
	logics     meta.Logics
}

func (s *service) initMetaService(h *rest.Handler) {
	// TODO: 跟前端确认，是否统一进行url规范化调整
	h.Add("ListDiskType", http.MethodGet, "/meta/disk_type/list", s.ListDiskType)
	h.Add("ListObsProject", http.MethodGet, "/meta/obs_project/list", s.ListObsProject)
	h.Add("ListRegion", http.MethodGet, "/meta/region/list", s.ListRegion)
	h.Add("ListZone", http.MethodPost, "/meta/zone/list", s.ListZone)
	h.Add("ListDeviceClass", http.MethodGet, "/meta/device_class/list", s.ListDeviceClass)
	h.Add("ListDeviceFamily", http.MethodGet, "/meta/device_family/list", s.ListDeviceFamily)
	h.Add("ListDeviceType", http.MethodPost, "/meta/device_type/list", s.ListDeviceType)
	h.Add("ListPlanType", http.MethodPost, "/metas/plan_types/list", s.ListPlanType)
	h.Add("ListTicketType", http.MethodPost, "/metas/ticket_types/list", s.ListTicketType)
	h.Add("ListBizsByOpProduct", http.MethodPost, "/metas/bizs/by/op_product/list", s.ListBizsByOpProduct)
	h.Add("ListOpProducts", http.MethodPost, "/metas/op_products/list", s.ListOpProducts)
	h.Add("ListPlanProducts", http.MethodPost, "/metas/plan_products/list", s.ListPlanProducts)
	h.Add("ListOrgTopos", http.MethodPost, "/metas/org_topos/list", s.ListOrgTopos)
	h.Add("ListRequireObsProject", http.MethodPost, "/meta/requirement/obs_project/list", s.ListRequireObsProject)

	// 资源池
	h.Add("CreateResourcePoolBiz", http.MethodPost, "/metas/respool_bizs/batch/create", s.CreateResourcePoolBiz)
	h.Add("ListResourcePoolBiz", http.MethodPost, "/metas/respool_bizs/list", s.ListResourcePoolBiz)
	h.Add("DeleteResourcePoolBiz", http.MethodDelete, "/metas/respool_biz/{id}", s.DeleteResourcePoolBiz)
}
