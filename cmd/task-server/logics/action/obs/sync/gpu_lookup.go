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

package sync

import (
	"encoding/json"
	"strings"

	actcli "hcm/cmd/task-server/logics/action/cli"
	"hcm/pkg/api/core"
	datagconf "hcm/pkg/api/data-service/global_config"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// loadAwsGpuInstanceTypes loads the AWS GPU instance type set from global_config.
func loadAwsGpuInstanceTypes(kt *kit.Kit) (map[string]struct{}, error) {
	flt := tools.ExpressionAnd(
		tools.RuleEqual("config_type", enumor.GlobalConfigTypeAccountBill),
		tools.RuleEqual("config_key", enumor.GlobalConfigKeyAwsGpuInstanceTypes),
	)
	req := &datagconf.ListReq{Filter: flt, Page: core.NewDefaultBasePage()}

	resp, err := actcli.GetDataService().Global.GlobalConfig.List(kt, req)
	if err != nil {
		logs.Errorf("load aws gpu instance types from global config failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	gpuSet := make(map[string]struct{})
	if len(resp.Details) == 0 {
		return gpuSet, nil
	}

	var types []string
	if err = json.Unmarshal([]byte(resp.Details[0].ConfigValue), &types); err != nil {
		logs.Warnf("fail to parse aws gpu instance types config, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	for _, t := range types {
		gpuSet[t] = struct{}{}
	}

	logs.Infof("loaded %d aws gpu instance types, rid: %s", len(gpuSet), kt.Rid)
	return gpuSet, nil
}

// loadHuaweiGpuPrefixes loads the Huawei GPU instance spec prefix list from global_config.
func loadHuaweiGpuPrefixes(kt *kit.Kit) ([]string, error) {
	flt := tools.ExpressionAnd(
		tools.RuleEqual("config_type", enumor.GlobalConfigTypeAccountBill),
		tools.RuleEqual("config_key", enumor.GlobalConfigKeyHuaweiGpuInstancePrefixes),
	)
	req := &datagconf.ListReq{
		Filter: flt,
		Page:   core.NewDefaultBasePage(),
	}

	resp, err := actcli.GetDataService().Global.GlobalConfig.List(kt, req)
	if err != nil {
		logs.Errorf("load huawei gpu instance prefixes from global config failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if len(resp.Details) == 0 {
		return nil, nil
	}

	var prefixes []string
	if err = json.Unmarshal([]byte(resp.Details[0].ConfigValue), &prefixes); err != nil {
		logs.Warnf("fail to parse huawei gpu instance prefixes config, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	logs.Infof("loaded %d huawei gpu instance prefixes, rid: %s", len(prefixes), kt.Rid)
	return prefixes, nil
}

// isAwsGPU determines whether an AWS bill item represents a GPU resource.
func isAwsGPU(lineItemProductCode, productInstanceType, hcProductName string, awsGpuSet map[string]struct{}) bool {
	if lineItemProductCode == constant.AmazonSageMaker {
		return true
	}

	if _, ok := awsGpuSet[productInstanceType]; ok {
		return true
	}

	if strings.HasPrefix(hcProductName, constant.BillItemAIPrefix) {
		return true
	}

	return false
}

// isGcpGPU determines whether a GCP bill item represents a GPU resource.
func isGcpGPU(skuDescription, hcProductName string) bool {
	if strings.Contains(strings.ToLower(skuDescription), constant.GcpCalendarMode) {
		return true
	}

	if strings.HasPrefix(hcProductName, constant.BillItemAIPrefix) {
		return true
	}

	return false
}

// isHuaweiGPU determines whether a Huawei bill item represents a GPU resource.
func isHuaweiGPU(productSpecDesc string, hwGpuPrefixes []string) bool {
	prefix := extractSpecPrefix(productSpecDesc)
	if len(prefix) == 0 {
		return false
	}

	lowerPrefix := strings.ToLower(prefix)
	for _, p := range hwGpuPrefixes {
		if strings.ToLower(p) == lowerPrefix {
			return true
		}
	}
	return false
}

// extractSpecPrefix extracts the spec family prefix from a Huawei product spec code.
// The spec code is the first segment of productSpecDesc split by "|".
// The prefix is the first dot-separated part of the spec code, e.g. "p2s.2xlarge.8" → "p2s".
func extractSpecPrefix(productSpecDesc string) string {
	if len(productSpecDesc) == 0 {
		return ""
	}

	specCodes := strings.SplitN(productSpecDesc, "|", 2)
	if len(specCodes) == 0 {
		return ""
	}
	specCode := strings.TrimSpace(specCodes[0])

	dotIdx := strings.IndexByte(specCode, '.')
	if dotIdx < 0 {
		return specCode
	}
	return specCode[:dotIdx]
}
