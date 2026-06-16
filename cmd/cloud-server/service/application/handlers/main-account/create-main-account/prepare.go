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

package mainaccount

import (
	"strings"

	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/itsm"
)

// PrepareReq 预处理申请单数据
func (a *ApplicationOfCreateMainAccount) PrepareReq() error {
	// 二级账号申请不包含敏感信息，无需处理
	return nil
}

// GenerateApplicationContent 生成存储到DB的申请单content的内容，Interface格式，便于统一处理
func (a *ApplicationOfCreateMainAccount) GenerateApplicationContent() interface{} {
	return a.req
}

// PrepareReqFromContent 申请单内容从DB里获取后可以进行预处理，便于资源交付时资源请求
func (a *ApplicationOfCreateMainAccount) PrepareReqFromContent() error {
	return nil
}

// GetItsmApprover 获取itsm审批人信息
func (a *ApplicationOfCreateMainAccount) GetItsmApprover(kt *kit.Kit, managers []string) (
	[]itsm.VariableApprover, error) {

	approvers := []itsm.VariableApprover{
		{
			Variable:  "platform_manager",
			Approvers: managers,
		},
	}

	// 无运营产品负责人时，在itsm侧控制是否必须运营产品负责人审批
	opManager, err := a.GetOperationProductManager(a.req.OpProductID)
	if err != nil {
		logs.Errorf("get operation product manager failed, err: %s, rid: %s", err, a.Cts.Kit.Rid)
		// NOTE：此处 error 返回为后添加，为避免影响代码原有行为，此处先保持原逻辑静默，不返回error
		return approvers, nil
	}

	opManagers := strings.Split(opManager, ";")

	if len(opManagers) != 0 {
		approvers = append(approvers, itsm.VariableApprover{
			Variable:  "op_product_manager",
			Approvers: opManagers,
		})
	}

	return approvers, nil
}

// GetBkBizIDs 获取当前的业务IDs
func (a *ApplicationOfCreateMainAccount) GetBkBizIDs() []int64 {
	return []int64{a.req.BkBizID}
}
