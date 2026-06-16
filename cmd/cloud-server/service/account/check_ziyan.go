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

package account

import (
	"encoding/json"

	"hcm/cmd/cloud-server/service/common"
	proto "hcm/pkg/api/cloud-server/account"
	hcproto "hcm/pkg/api/hc-service/account"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/rest"
)

// ParseAndCheckTCloudZiyanExtension  联通性校验，并检查字段是否匹配
func ParseAndCheckTCloudZiyanExtension(cts *rest.Contexts, client *client.ClientSet, accountType enumor.AccountType,
	reqExtension json.RawMessage) (*proto.TCloudZiyanAccountExtensionCreateReq, error) {
	// 解析Extension
	extension := new(proto.TCloudZiyanAccountExtensionCreateReq)
	if err := common.DecodeExtension(cts.Kit, reqExtension, extension); err != nil {
		return nil, err
	}
	// 校验Extension
	if err := extension.Validate(accountType); err != nil {
		return nil, err
	}

	// 检查联通性，账号是否正确
	if accountType != enumor.RegistrationAccount || extension.IsFull() {
		err := client.HCService().TCloudZiyan.Account.Check(cts.Kit,
			&hcproto.TCloudAccountCheckReq{
				CloudMainAccountID: extension.CloudMainAccountID,
				CloudSubAccountID:  extension.CloudSubAccountID,
				CloudSecretID:      extension.CloudSecretID,
				CloudSecretKey:     extension.CloudSecretKey,
			},
		)
		if err != nil {
			return nil, err
		}
	}

	return extension, nil
}
