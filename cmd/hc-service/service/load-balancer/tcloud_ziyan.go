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

package loadbalancer

import (
	"net/http"

	"hcm/cmd/hc-service/service/capability"
	"hcm/pkg/rest"
)

func (svc *clbSvc) initTCloudZiyanClbService(cap *capability.Capability) {
	h := rest.NewHandler()

	h.Add("BatchCreateTCloudZiyanClb", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/batch/create", svc.BatchCreateTCloudZiyanClb)
	h.Add("InquiryPriceTCloudZiyanLB", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/prices/inquiry", svc.InquiryPriceTCloudZiyanLB)
	h.Add("ListTCloudZiyanClb", http.MethodPost, "/vendors/tcloud-ziyan/load_balancers/list", svc.ListTCloudZiyanClb)
	h.Add("TCloudZiyanDescribeResources", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/resources/describe", svc.TCloudZiyanDescribeResources)
	h.Add("TCloudZiyanUpdateCLB", http.MethodPatch,
		"/vendors/tcloud-ziyan/load_balancers/{id}", svc.TCloudZiyanUpdateCLB)
	h.Add("BatchDeleteTCloudZiyanLoadBalancer", http.MethodDelete,
		"/vendors/tcloud-ziyan/load_balancers/batch", svc.BatchDeleteTCloudZiyanLoadBalancer)
	h.Add("ListQuotaTCloudZiyanLB", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/quota", svc.ListTCloudZiyanLBQuota)
	h.Add("TCloudZiyanDescribeSlaCapacity", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/sla/capacity/describe", svc.TCloudZiyanDescribeSlaCapacity)
	// 内部测试端点：查询独占集群资源（含VIP闲置状态），仅供联调/测试直接验证，不对外暴露
	h.Add("TCloudZiyanDescribeClusterIdleVipsRaw", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/exclusive_clusters/idle_vips/query",
		svc.TCloudZiyanDescribeClusterIdleVipsRaw)
	// 业务视角闲置VIP查询接口用：自动翻页取全并去重后返回
	h.Add("TCloudZiyanDescribeClusterIdleVips", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/exclusive_clusters/idle_vips/describe",
		svc.TCloudZiyanDescribeClusterIdleVips)

	h.Add("ZiyanCreateSnatIps", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/snat_ips/create", svc.ZiyanCreateSnatIps)
	h.Add("TCloudDeleteSnatIps", http.MethodDelete,
		"/vendors/tcloud-ziyan/load_balancers/snat_ips", svc.ZiyanDeleteSnatIps)

	h.Add("TCloudZiyanCreateUrlRule", http.MethodPost,
		"/vendors/tcloud-ziyan/listeners/{lbl_id}/rules/batch/create", svc.TCloudZiyanCreateUrlRule)
	h.Add("TCloudZiyanUpdateUrlRule", http.MethodPatch,
		"/vendors/tcloud-ziyan/listeners/{lbl_id}/rules/{rule_id}", svc.TCloudZiyanUpdateUrlRule)
	h.Add("TCloudZiyanBatchDeleteUrlRule", http.MethodDelete,
		"/vendors/tcloud-ziyan/listeners/{lbl_id}/rules/batch", svc.TCloudZiyanBatchDeleteUrlRule)
	h.Add("TCloudZiyanBatchDeleteUrlRuleByDomain", http.MethodDelete,
		"/vendors/tcloud-ziyan/listeners/{lbl_id}/rules/by/domain/batch", svc.TCloudZiyanBatchDeleteUrlRuleByDomain)

	// 监听器
	h.Add("CreateTCloudZiyanListenerWithTargetGroup", http.MethodPost,
		"/vendors/tcloud-ziyan/listeners/create_with_target_group", svc.CreateTCloudZiyanListenerWithTargetGroup)
	h.Add("UpdateTCloudZiyanListener", http.MethodPatch,
		"/vendors/tcloud-ziyan/listeners/{id}", svc.UpdateTCloudZiyanListener)
	h.Add("UpdateTCloudZiyanListenerHealthCheck", http.MethodPatch,
		"/vendors/tcloud-ziyan/listeners/{lbl_id}/health_check", svc.UpdateTCloudZiyanListenerHealthCheck)
	h.Add("DeleteTCloudZiyanListener", http.MethodDelete,
		"/vendors/tcloud-ziyan/listeners/batch", svc.DeleteTCloudZiyanListener)
	// 仅创建监听器
	h.Add("CreateTCloudZiyanListener", http.MethodPost,
		"/vendors/tcloud-ziyan/listeners/create", svc.CreateTCloudZiyanListener)
	// 域名、规则
	h.Add("UpdateTCloudZiyanDomainAttr", http.MethodPatch, "/vendors/tcloud-ziyan/listeners/{lbl_id}/domains",
		svc.UpdateTCloudZiyanDomainAttr)

	// 目标组
	h.Add("BatchCreateTCloudZiyanTargets", http.MethodPost,
		"/vendors/tcloud-ziyan/target_groups/{target_group_id}/targets/create", svc.BatchCreateTCloudZiyanTargets)
	h.Add("BatchRemoveTCloudZiyanTargets", http.MethodDelete,
		"/vendors/tcloud-ziyan/target_groups/{target_group_id}/targets/batch", svc.BatchRemoveTCloudZiyanTargets)
	h.Add("BatchModifyTCloudZiyanTargetsPort", http.MethodPatch,
		"/vendors/tcloud-ziyan/target_groups/{target_group_id}/targets/port", svc.BatchModifyTCloudZiyanTargetsPort)
	h.Add("BatchModifyTCloudZiyanTargetsWeight", http.MethodPatch,
		"/vendors/tcloud-ziyan/target_groups/{target_group_id}/targets/weight", svc.BatchModifyTCloudZiyanTargetsWeight)
	h.Add("ListTCloudZiyanTargetsHealth", http.MethodPost, "/vendors/tcloud-ziyan/load_balancers/targets/health",
		svc.ListTCloudZiyanTargetsHealth)

	h.Add("RegisterZiyanTargetToListenerRule", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/{lb_id}/targets/create", svc.RegisterZiyanTargetToListenerRule)

	h.Add("QueryZiyanListenerTargetsByCloudIDs", http.MethodPost,
		"/vendors/tcloud-ziyan/targets/query_by_cloud_ids", svc.QueryZiyanListenerTargetsByCloudIDs)

	h.Add("BatchModifyZiyanListenerTargetsWeight", http.MethodPatch,
		"/vendors/tcloud-ziyan/load_balancers/{lb_id}/targets/weight", svc.BatchModifyZiyanListenerTargetsWeight)
	h.Add("BatchRemoveZiyanListenerTargets", http.MethodDelete,
		"/vendors/tcloud-ziyan/load_balancers/{lb_id}/targets/batch", svc.BatchRemoveZiyanListenerTargets)

	h.Add("DescribeZiyanExclusiveCluster", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/exclusive_clusters/describe", svc.DescribeZiyanExclusiveCluster)
	h.Add("DescribeZiyanClusterResources", http.MethodPost,
		"/vendors/tcloud-ziyan/load_balancers/cluster_resources/describe", svc.DescribeClusterResources)

	h.Load(cap.WebService)
}
