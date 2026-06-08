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

package loadbalancer

import (
	cslb "hcm/pkg/api/cloud-server/load-balancer"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/hooks/handler"
	"hcm/pkg/tools/slice"
)

func (svc *lbSvc) listUrlRuleByCondForTCloudZiyan(kt *kit.Kit, req cslb.TargetQueryLine, lblIDs []string) ([]string,
	map[string]urlRuleInfo, error) {
	ruleType := enumor.Layer4RuleType
	if req.Protocol.IsLayer7Protocol() {
		ruleType = enumor.Layer7RuleType
	}
	queryRules := []*filter.AtomRule{
		tools.RuleEqual("rule_type", ruleType),
	}
	if len(req.Domains) > 0 {
		queryRules = append(queryRules, tools.RuleIn("domain", req.Domains))
	}
	if len(req.Urls) > 0 {
		queryRules = append(queryRules, tools.RuleIn("url", req.Urls))
	}
	result := make([]string, 0)
	ruleMap := make(map[string]urlRuleInfo)
	for _, batch := range slice.Split(lblIDs, int(core.DefaultMaxPageLimit)) {
		listReq := &core.ListReq{
			Filter: tools.ExpressionAnd(
				append(queryRules,
					tools.RuleIn("lbl_id", batch),
				)...,
			),
			Page: core.NewDefaultBasePage(),
		}
		for {
			resp, err := svc.client.DataService().TCloudZiyan.LoadBalancer.ListUrlRule(kt, listReq)
			if err != nil {
				return nil, nil, err
			}
			for _, detail := range resp.Details {
				result = append(result, detail.ID)
				ruleMap[detail.ID] = urlRuleInfo{
					domain:     detail.Domain,
					url:        detail.URL,
					lblID:      detail.LblID,
					cloudLblID: detail.CloudLBLID,
					cloudLBID:  detail.CloudLbID,
				}
			}
			if len(resp.Details) < int(core.DefaultMaxPageLimit) {
				break
			}
			listReq.Page.Start += uint32(core.DefaultMaxPageLimit)
		}
	}

	return result, ruleMap, nil
}

func (svc *lbSvc) listUrlRuleMapByIDsForTCloudZiyan(kt *kit.Kit, ids []string) (map[string]urlRuleInfo, error) {
	result := make(map[string]urlRuleInfo, 0)
	for _, batch := range slice.Split(ids, int(core.DefaultMaxPageLimit)) {
		listReq := &core.ListReq{
			Filter: tools.ContainersExpression("id", batch),
			Page:   core.NewDefaultBasePage(),
		}
		resp, err := svc.client.DataService().TCloudZiyan.LoadBalancer.ListUrlRule(kt, listReq)
		if err != nil {
			return nil, err
		}
		for _, detail := range resp.Details {
			result[detail.ID] = urlRuleInfo{
				domain:     detail.Domain,
				url:        detail.URL,
				lblID:      detail.LblID,
				cloudLblID: detail.CloudLBLID,
				cloudLBID:  detail.CloudLbID,
			}
		}
	}
	return result, nil
}

func (svc *lbSvc) tcloudZiyanUrlBindTargetGroup(cts *rest.Contexts, bizID int64, req *cslb.TCloudRuleBindTargetGroup) (
	string, error) {

	listReq := &core.ListReq{
		Filter: tools.ExpressionAnd(tools.RuleEqual("id", req.UrlRuleID)),
		Page:   core.NewDefaultBasePage(),
	}
	resp, err := svc.listZiyanRuleWithCondition(cts.Kit, listReq)
	if err != nil {
		logs.Errorf("fail to list url rule with condition, err: %v, req: %+v, rid: %s", err, listReq, cts.Kit.Rid)
		return "", err
	}
	if len(resp.Details) == 0 {
		logs.Errorf("url rule not found, id: %s, req: %+v, rid: %s", req.UrlRuleID, listReq, cts.Kit.Rid)
		return "", errf.Newf(errf.RecordNotFound, "url rule(%s) not found", req.UrlRuleID)
	}
	rule := resp.Details[0]
	if rule.RuleType != enumor.Layer7RuleType {
		logs.Errorf("url rule is not layer7 rule, id: %s, ruleType: %s, rid: %s", req.UrlRuleID, rule.RuleType, cts.Kit.Rid)
		return "", errf.Newf(errf.InvalidParameter, "url rule(%s) is not layer7 rule", req.UrlRuleID)
	}

	listenerDetail, lblBasicInfo, err := svc.getListenerDetailByIDAndBiz(
		cts.Kit, enumor.TCloudZiyan, bizID, rule.LblID)
	if err != nil {
		logs.Errorf("fail to get listener info, bizID: %d, listenerID: %s, err: %v, rid: %s",
			bizID, rule.LblID, err, cts.Kit.Rid)
		return "", err
	}

	// 业务校验、鉴权
	valOpt := &handler.ValidWithAuthOption{
		Authorizer: svc.authorizer,
		ResType:    meta.UrlRuleAuditResType,
		Action:     meta.Create,
		BasicInfo:  lblBasicInfo,
	}
	if err = handler.BizOperateAuth(cts, valOpt); err != nil {
		return "", err
	}

	// 预检测-是否有执行中的负载均衡
	_, err = svc.checkResFlowRel(cts.Kit, listenerDetail.LbID, enumor.LoadBalancerCloudResType)
	if err != nil {
		return "", err
	}

	taskManagementID, err := svc.applyTargetToRule(cts.Kit, req.TargetGroupID, rule.CloudID, listenerDetail, bizID)
	if err != nil {
		logs.Errorf("fail to create target register flow, err: %v, rid: %s", err, cts.Kit.Rid)
		return "", err
	}

	return taskManagementID, nil
}
