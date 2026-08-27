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

// Package billprepaid 提供预付费账单的写入与查询服务。
package billprepaid

import (
	"hcm/cmd/account-server/service/capability"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	croncore "hcm/pkg/cron/core"
	"hcm/pkg/iam/auth"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
	"net/http"
)

// billSettleRunner 手动触发定账所需的执行入口。
type billSettleRunner interface {
	croncore.Task
	RunOnce(kt *kit.Kit) error
}

// InitBillPrepaidService 注册预付费账单服务
func InitBillPrepaidService(c *capability.Capability) {
	svc := &billPrepaidSvc{
		client:     c.ApiClient,
		authorizer: c.Authorizer,
	}
	if runner, ok := c.CronTasks[enumor.CronTaskBillSettle].(billSettleRunner); ok {
		svc.settleTask = runner
	}

	h := rest.NewHandler()

	h.Add("SyncBillPrepaidItem", http.MethodPost,
		"/bills/prepaid_items/sync", svc.SyncBillPrepaidItem)
	h.Add("DeleteBillPrepaidItem", http.MethodDelete,
		"/bills/prepaid_items/batch", svc.DeleteBillPrepaidItem)
	h.Add("ListBillPrepaidItem", http.MethodPost,
		"/bills/prepaid_items/list", svc.ListBillPrepaidItem)
	h.Add("ListBillPrepaidSplitItem", http.MethodPost,
		"/bills/prepaid_items/{id}/split_items/list", svc.ListBillPrepaidSplitItem)
	if task, ok := c.CronTasks[enumor.CronTaskBillSettle]; ok {
		h.Add("RunBillSettleTask", http.MethodPost, task.GetURL(), svc.RunBillSettleTask)
	}

	h.Load(c.WebService)
}

// billPrepaidSvc 预付费账单服务
type billPrepaidSvc struct {
	client     *client.ClientSet
	authorizer auth.Authorizer
	// settleTask 定账任务实例
	settleTask billSettleRunner
}
