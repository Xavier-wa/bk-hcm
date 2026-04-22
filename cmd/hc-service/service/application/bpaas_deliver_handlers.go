/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2025 THL A29 Limited,
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

package application

import (
	ressync "hcm/cmd/hc-service/logics/res-sync"
	ziyanSync "hcm/cmd/hc-service/logics/res-sync/ziyan"
	coreziyan "hcm/pkg/api/core/ziyan"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// BPaasDeliverHandler BPaaS审批通过后执行补偿交付的handler接口
type BPaasDeliverHandler interface {
	Deliver(kt *kit.Kit) error
}

// bpaasDeliverHandlerFactory 根据 content 构造对应 handler 的工厂函数类型
type bpaasDeliverHandlerFactory func(content *coreziyan.BPaasApplicationContent,
	syncCli ressync.Interface) BPaasDeliverHandler

// bpaasDeliverHandlerRegistry 按 ApplicationType 注册各资源类型的 deliver handler
// 新资源类型只需在此注册对应的工厂函数，无需修改分发逻辑
var bpaasDeliverHandlerRegistry = map[enumor.ApplicationType]bpaasDeliverHandlerFactory{
	enumor.CreateSecurityGroupRule: newSGRuleDeliverHandler,
	enumor.UpdateSecurityGroupRule: newSGRuleDeliverHandler,
	enumor.DeleteSecurityGroupRule: newSGRuleDeliverHandler,
}

// GetBPaasDeliverHandler 根据 action 从注册表中获取对应的 deliver handler，未注册的 action 返回 nil
func GetBPaasDeliverHandler(action enumor.ApplicationType, content *coreziyan.BPaasApplicationContent,
	syncCli ressync.Interface) BPaasDeliverHandler {

	factory, ok := bpaasDeliverHandlerRegistry[action]
	if !ok {
		return nil
	}
	return factory(content, syncCli)
}

// --- 安全组规则 deliver handler ---

type sgRuleDeliverHandler struct {
	content *coreziyan.BPaasApplicationContent
	syncCli ressync.Interface
}

func newSGRuleDeliverHandler(content *coreziyan.BPaasApplicationContent,
	syncCli ressync.Interface) BPaasDeliverHandler {

	return &sgRuleDeliverHandler{content: content, syncCli: syncCli}
}

// Deliver sgRule deliver to sync the security group rule from cloud to local.
func (h *sgRuleDeliverHandler) Deliver(kt *kit.Kit) error {
	syncClient, err := h.syncCli.TCloudZiyan(kt, h.content.AccountID)
	if err != nil {
		logs.Errorf("init tcloud ziyan sync client failed, account id: %s, err: %v, rid: %s",
			h.content.AccountID, err, kt.Rid)
		return err
	}

	syncParams := &ziyanSync.SyncBaseParams{
		AccountID: h.content.AccountID,
		Region:    h.content.Region,
		CloudIDs:  []string{h.content.SecurityGroupID},
	}
	_, err = syncClient.SecurityGroupRule(kt, syncParams, new(ziyanSync.SyncSGRuleOption))
	if err != nil {
		logs.Errorf("sync security group rule failed, account id: %s, security group id: %s, err: %v, rid: %s",
			h.content.AccountID, h.content.SecurityGroupID, err, kt.Rid)
	}
	return err
}
