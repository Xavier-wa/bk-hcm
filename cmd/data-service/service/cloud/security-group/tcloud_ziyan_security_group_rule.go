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

package securitygroup

import (
	"fmt"
	"reflect"

	"hcm/cmd/data-service/service/capability"
	"hcm/pkg/api/core"
	corecloud "hcm/pkg/api/core/cloud"
	protocloud "hcm/pkg/api/data-service/cloud"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	tablecloud "hcm/pkg/dal/table/cloud"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/slice"

	"github.com/jmoiron/sqlx"
)

// initTCloudZiyanSGRuleService initial the tcloud security group rule service
func initTCloudZiyanSGRuleService(cap *capability.Capability) {
	svc := &tcloudZiyanSGRuleSvc{
		dao: cap.Dao,
	}

	h := rest.NewHandler()

	h.Add("BatchCreateTCloudZiyanRule", "POST",
		"/vendors/tcloud-ziyan/security_groups/{security_group_id}/rules/batch/create", svc.BatchCreateTCloudZiyanRule)
	h.Add("BatchUpdateTCloudZiyanRule", "PUT",
		"/vendors/tcloud-ziyan/security_groups/{security_group_id}/rules/batch", svc.BatchUpdateTCloudZiyanRule)
	h.Add("ListTCloudZiyanRule", "POST",
		"/vendors/tcloud-ziyan/security_groups/{security_group_id}/rules/list", svc.ListTCloudZiyanRule)
	h.Add("DeleteTCloudZiyanRule", "DELETE",
		"/vendors/tcloud-ziyan/security_groups/{security_group_id}/rules/batch", svc.DeleteTCloudZiyanRule)

	h.Load(cap.WebService)
}

type tcloudZiyanSGRuleSvc struct {
	dao dao.Set
}

// BatchCreateTCloudZiyanRule batch create tcloud rule.
func (svc *tcloudZiyanSGRuleSvc) BatchCreateTCloudZiyanRule(cts *rest.Contexts) (interface{}, error) {
	sgID := cts.PathParameter("security_group_id").String()
	if len(sgID) == 0 {
		return nil, errf.New(errf.InvalidParameter, "security group id is required")
	}

	req := new(protocloud.TCloudSGRuleCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	rules := make([]*tablecloud.TCloudZiyanSecurityGroupRuleTable, 0, len(req.Rules))
	for _, rule := range req.Rules {
		rules = append(rules, &tablecloud.TCloudZiyanSecurityGroupRuleTable{
			Region:                     rule.Region,
			CloudPolicyIndex:           rule.CloudPolicyIndex,
			Version:                    rule.Version,
			Type:                       string(rule.Type),
			CloudSecurityGroupID:       rule.CloudSecurityGroupID,
			SecurityGroupID:            rule.SecurityGroupID,
			AccountID:                  rule.AccountID,
			Action:                     rule.Action,
			Protocol:                   rule.Protocol,
			Port:                       rule.Port,
			CloudServiceID:             rule.CloudServiceID,
			CloudServiceGroupID:        rule.CloudServiceGroupID,
			IPv4Cidr:                   rule.IPv4Cidr,
			IPv6Cidr:                   rule.IPv6Cidr,
			CloudTargetSecurityGroupID: rule.CloudTargetSecurityGroupID,
			CloudAddressID:             rule.CloudAddressID,
			CloudAddressGroupID:        rule.CloudAddressGroupID,
			Memo:                       rule.Memo,
			Creator:                    cts.Kit.User,
			Reviser:                    cts.Kit.User,
		})
	}

	ruleIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		ruleIDs, err := svc.dao.TCloudZiyanSGRule().BatchCreateOrUpdateWithTx(cts.Kit, txn, rules)
		if err != nil {
			return nil, fmt.Errorf("batch create tcloud ziyan security group rule failed, err: %v", err)
		}

		return ruleIDs, nil
	})
	if err != nil {
		return nil, err
	}

	ids, ok := ruleIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create tcloud ziyan security group rule but return id type is not string, id type: %v",
			reflect.TypeOf(ruleIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}

// BatchUpdateTCloudZiyanRule update tcloud ziyan rule.
func (svc *tcloudZiyanSGRuleSvc) BatchUpdateTCloudZiyanRule(cts *rest.Contexts) (interface{}, error) {
	sgID := cts.PathParameter("security_group_id").String()
	if len(sgID) == 0 {
		return nil, errf.New(errf.InvalidParameter, "security group id is required")
	}

	req := new(protocloud.TCloudSGRuleBatchUpdateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		for _, one := range req.Rules {
			rule := &tablecloud.TCloudZiyanSecurityGroupRuleTable{
				Region:                     one.Region,
				CloudPolicyIndex:           one.CloudPolicyIndex,
				Version:                    one.Version,
				Type:                       string(one.Type),
				CloudSecurityGroupID:       one.CloudSecurityGroupID,
				SecurityGroupID:            one.SecurityGroupID,
				AccountID:                  one.AccountID,
				Action:                     one.Action,
				Protocol:                   one.Protocol,
				Port:                       one.Port,
				CloudServiceID:             one.CloudServiceID,
				CloudServiceGroupID:        one.CloudServiceGroupID,
				IPv4Cidr:                   one.IPv4Cidr,
				IPv6Cidr:                   one.IPv6Cidr,
				CloudTargetSecurityGroupID: one.CloudTargetSecurityGroupID,
				CloudAddressID:             one.CloudAddressID,
				CloudAddressGroupID:        one.CloudAddressGroupID,
				Memo:                       one.Memo,
				Reviser:                    cts.Kit.User,
			}

			flt := &filter.Expression{
				Op: filter.And,
				Rules: []filter.RuleFactory{
					&filter.AtomRule{
						Field: "id",
						Op:    filter.Equal.Factory(),
						Value: one.ID,
					},
					&filter.AtomRule{
						Field: "security_group_id",
						Op:    filter.Equal.Factory(),
						Value: sgID,
					},
				},
			}
			if err := svc.dao.TCloudZiyanSGRule().UpdateWithTx(cts.Kit, txn, flt, rule); err != nil {
				logs.Errorf("update tcloud ziyan security group rule failed, err: %v, rid: %s", err, cts.Kit.Rid)
				return nil, err
			}
		}

		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// ListTCloudZiyanRule list tcloud ziyan rule.
func (svc *tcloudZiyanSGRuleSvc) ListTCloudZiyanRule(cts *rest.Contexts) (interface{}, error) {
	sgID := cts.PathParameter("security_group_id").String()
	if len(sgID) == 0 {
		return nil, errf.New(errf.InvalidParameter, "security group id is required")
	}

	req := new(protocloud.TCloudSGRuleListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.SGRuleListOption{
		SecurityGroupID: sgID,
		Fields:          req.Field,
		Filter:          req.Filter,
		Page:            req.Page,
	}
	result, err := svc.dao.TCloudZiyanSGRule().List(cts.Kit, opt)
	if err != nil {
		logs.Errorf("list tcloud ziyan security group rule failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, fmt.Errorf("list tcloud ziyan security group rule failed, err: %v", err)
	}

	if req.Page.Count {
		return &types.ListResult[corecloud.TCloudSecurityGroupRule]{Count: result.Count}, nil
	}

	details := make([]corecloud.TCloudSecurityGroupRule, 0, len(result.Details))
	for _, one := range result.Details {
		details = append(details, corecloud.TCloudSecurityGroupRule{
			ID:                         one.ID,
			Region:                     one.Region,
			CloudPolicyIndex:           one.CloudPolicyIndex,
			Version:                    one.Version,
			Protocol:                   one.Protocol,
			Port:                       one.Port,
			CloudServiceID:             one.CloudServiceID,
			CloudServiceGroupID:        one.CloudServiceGroupID,
			IPv4Cidr:                   one.IPv4Cidr,
			IPv6Cidr:                   one.IPv6Cidr,
			CloudTargetSecurityGroupID: one.CloudTargetSecurityGroupID,
			CloudAddressID:             one.CloudAddressID,
			CloudAddressGroupID:        one.CloudAddressGroupID,
			Action:                     one.Action,
			Memo:                       one.Memo,
			Type:                       enumor.SecurityGroupRuleType(one.Type),
			CloudSecurityGroupID:       one.CloudSecurityGroupID,
			SecurityGroupID:            one.SecurityGroupID,
			AccountID:                  one.AccountID,
			Creator:                    one.Creator,
			Reviser:                    one.Reviser,
			CreatedAt:                  one.CreatedAt.String(),
			UpdatedAt:                  one.UpdatedAt.String(),
		})
	}

	return &types.ListResult[corecloud.TCloudSecurityGroupRule]{Details: details}, nil
}

// DeleteTCloudZiyanRule delete tcloud ziyan rule.
func (svc *tcloudZiyanSGRuleSvc) DeleteTCloudZiyanRule(cts *rest.Contexts) (interface{}, error) {
	sgID := cts.PathParameter("security_group_id").String()
	if len(sgID) == 0 {
		return nil, errf.New(errf.InvalidParameter, "security group id is required")
	}

	req := new(protocloud.TCloudSGRuleBatchDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.SGRuleListOption{
		SecurityGroupID: sgID,
		Fields:          []string{"id"},
		Filter:          req.Filter,
		Page:            core.NewDefaultBasePage(),
	}
	listResp, err := svc.dao.TCloudZiyanSGRule().List(cts.Kit, opt)
	if err != nil {
		logs.Errorf("list tcloud ziyan security group rule failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, fmt.Errorf("list tcloud ziyan security group rule failed, err: %v", err)
	}

	if len(listResp.Details) == 0 {
		return nil, nil
	}

	delIDs := make([]string, len(listResp.Details))
	for index, one := range listResp.Details {
		delIDs[index] = one.ID
	}

	delFilter := tools.ContainersExpression("id", delIDs)
	if err := svc.dao.TCloudZiyanSGRule().Delete(cts.Kit, delFilter); err != nil {
		logs.Errorf("delete tcloud ziyan security group rule failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

func (svc *securityGroupSvc) listTCloudZiyanSecurityGroupRulesCount(kt *kit.Kit, ids []string) (
	map[string]int64, error) {

	result := make(map[string]int64)
	for _, sgIDs := range slice.Split(ids, int(core.DefaultMaxPageLimit)) {
		resp, err := svc.dao.TCloudZiyanSGRule().CountBySecurityGroupIDs(kt,
			tools.ContainersExpression("security_group_id", sgIDs))
		if err != nil {
			logs.Errorf("listTCloudSecurityGroupRulesCount failed, err: %v, ids: %v, rid: %s", err, sgIDs, kt.Rid)
			return nil, err
		}
		for k, v := range resp {
			result[k] = v
		}
	}
	return result, nil
}
