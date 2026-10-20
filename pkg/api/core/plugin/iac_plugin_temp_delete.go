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

package plugin

import (
	"hcm/pkg/api/core"
	"hcm/pkg/dal/table/types"
)

// TODO(temp_delete): 临时 iac_plugin_temp_delete 核心模型，合并插件注册分支后删除本文件。
// 对齐《插件注册与升级》设计方案 2.1 / 2.2 / 2.3。

const (
	// PluginStateInitTempDelete 注册或升级中，Schema 尚未同步完成。
	PluginStateInitTempDelete = "init"
	// PluginStateEnabledTempDelete 可用，接受新请求。
	PluginStateEnabledTempDelete = "enabled"
	// PluginStateDisabledTempDelete 管理员停用。
	PluginStateDisabledTempDelete = "disabled"
	// PluginStateExceptionTempDelete 异常，Message 写明原因。
	PluginStateExceptionTempDelete = "exception"
)

const (
	// CryptoTypeRSATempDelete http 插件 launch_config 当前唯一支持的加密类型。
	CryptoTypeRSATempDelete = "RSA"
)

// IacPluginTempDelete iac_plugin_temp_delete 临时注册表记录模型（合并后删除）。
type IacPluginTempDelete struct {
	ID             string          `json:"id"`
	PluginName     string          `json:"plugin_name"`
	PluginVersion  string          `json:"plugin_version"`
	Protocol       string          `json:"protocol"`
	State          string          `json:"state"`
	Message        string          `json:"message"`
	Extension      types.JsonField `json:"extension"`
	Memo           *string         `json:"memo"`
	*core.Revision `json:",inline"`
}

// TFProviderArchitectureDigestTempDelete tf_provider 架构文件摘要（临时测试）。
type TFProviderArchitectureDigestTempDelete struct {
	SHA256 string `json:"sha256"`
}

// TFProviderExtensionTempDelete tf_provider extension（方案 2.2，临时测试）。
// architectures 的键为 {os}_{arch}，与架构目录同名。
type TFProviderExtensionTempDelete struct {
	Architectures map[string]TFProviderArchitectureDigestTempDelete `json:"architectures"`
}

// HTTPCryptoConfigRSATempDelete RSA crypto_config（方案 2.3，临时测试）。
type HTTPCryptoConfigRSATempDelete struct {
	// PublicKey PEM 编码的 SPKI 格式 RSA 公钥，长度不低于 3072 位。
	PublicKey string `json:"public_key"`
}

// HTTPPluginExtensionTempDelete http extension（方案 2.3，临时测试）。
type HTTPPluginExtensionTempDelete struct {
	// BaseURL 必填，插件网关地址后缀；探活通过后才写入。
	BaseURL string `json:"base_url"`
	// CryptoType launch_config 加密类型，当前仅支持 RSA。
	CryptoType string `json:"crypto_type"`
	// CryptoConfig 对应加密类型所需参数（RSA 下为 public_key）。
	CryptoConfig HTTPCryptoConfigRSATempDelete `json:"crypto_config"`
}
