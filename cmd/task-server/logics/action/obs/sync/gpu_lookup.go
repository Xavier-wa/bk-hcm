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
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/util"
)

// loadAwsGpuInstanceTypes loads the AWS GPU instance type to card category mapping from global_config.
// config_value 为「实例类型 → 卡型」JSON 对象；解析失败返回错误，配置缺失返回空 map（不阻断上报）。
func loadAwsGpuInstanceTypes(kt *kit.Kit) (map[string]string, error) {
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

	if len(resp.Details) == 0 {
		return make(map[string]string), nil
	}

	gpuMap, err := parseAwsGpuInstanceTypes(resp.Details[0].ConfigValue)
	if err != nil {
		logs.Warnf("fail to parse aws gpu instance types config, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	logs.Infof("loaded %d aws gpu instance types, rid: %s", len(gpuMap), kt.Rid)
	return gpuMap, nil
}

// parseAwsGpuInstanceTypes 解析「实例类型 → 卡型」JSON 对象配置，解析失败返回错误。
func parseAwsGpuInstanceTypes(configValue types.JsonField) (map[string]string, error) {
	gpuMap := make(map[string]string)
	if err := json.Unmarshal([]byte(configValue), &gpuMap); err != nil {
		return nil, err
	}
	return gpuMap, nil
}

// loadGcpGpuInstancePrefixes loads the GCP GPU instance family prefix to card category mapping from global_config.
// config_value 为「实例族前缀 → 短卡型名」JSON 对象；解析失败返回错误，配置缺失返回空 map（不阻断上报，仅 L1 生效）。
func loadGcpGpuInstancePrefixes(kt *kit.Kit) (map[string]string, error) {
	flt := tools.ExpressionAnd(
		tools.RuleEqual("config_type", enumor.GlobalConfigTypeAccountBill),
		tools.RuleEqual("config_key", enumor.GlobalConfigKeyGcpGpuInstancePrefixes),
	)
	req := &datagconf.ListReq{Filter: flt, Page: core.NewDefaultBasePage()}

	resp, err := actcli.GetDataService().Global.GlobalConfig.List(kt, req)
	if err != nil {
		logs.Errorf("load gcp gpu instance prefixes from global config failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if len(resp.Details) == 0 {
		return make(map[string]string), nil
	}

	prefixMap := make(map[string]string)
	if err = json.Unmarshal([]byte(resp.Details[0].ConfigValue), &prefixMap); err != nil {
		logs.Warnf("fail to parse gcp gpu instance prefixes config, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	logs.Infof("loaded %d gcp gpu instance prefixes, rid: %s", len(prefixMap), kt.Rid)
	return prefixMap, nil
}

// lookupAwsGpuCardCategory 取 AWS 实例类型对应的 GPU 卡型，未命中返回空字符串。
// SageMaker 场景（productProductName == "Amazon SageMaker"）实例类型带用途后缀，
// 如 ml.p5en.48xlarge-trainingplanunused、ml.t3.medium-Notebook、ml.c5.xlarge-Cluster，
// 需去除最后一段「-后缀」后再匹配机型（保留机型内部可能存在的连字符，如 p6-b200）。
func lookupAwsGpuCardCategory(productProductName, productInstanceType string, awsGpuMap map[string]string) string {
	instanceType := productInstanceType
	if productProductName == constant.AmazonSageMakerProductName {
		if idx := strings.LastIndex(instanceType, "-"); idx >= 0 {
			instanceType = instanceType[:idx]
		}
	}
	return awsGpuMap[instanceType]
}

// lookupGcpGpuCardCategory 识别 GCP 账单的 GPU 卡型，返回短卡型名，未命中返回空字符串。
// 先匹配 L1 显式卡型关键词（硬编码），未命中再匹配 L2 实例族前缀（global_config）。
func lookupGcpGpuCardCategory(skuDescription string, gcpGpuPrefixes map[string]string) string {
	// L1：显式卡型关键词（最高优先级）
	if card := enumor.MatchGcpGpuCardByKeyword(skuDescription); card != "" {
		return card
	}
	// L2：实例族前缀，词边界 + 最长前缀优先
	return matchGcpInstancePrefix(skuDescription, gcpGpuPrefixes)
}

// isAIDeductBillItem reports whether the bill item is an AI deduct entry rewritten by month task.
// AWS/GCP 月任务均将 HcProductCode/Name 覆盖为字面量 AIDeduct。
func isAIDeductBillItem(hcProductCode, hcProductName string) bool {
	return hcProductCode == constant.AwsAIDeductProductCode ||
		hcProductName == constant.AwsAIDeductProductCode
}

// hcProductNameForGPUClass 返回用于 GPU AI 前缀判定的产品名。
// AI 扣减源单必带 _HCM_AI_ 前缀，产品码改写后用前缀占位恢复 isGPU 的 AI 血缘分支。
func hcProductNameForGPUClass(hcProductCode, hcProductName string) string {
	if isAIDeductBillItem(hcProductCode, hcProductName) {
		return constant.BillItemAIPrefix
	}
	return hcProductName
}

// resolveAwsAPIBrandName 解析 AWS 账单 API 厂商。
// AI 扣减条目优先从 extension 产品名/行描述匹配（与原始账单同口径），禁止仅因 AIDeduct 一律 API。
func resolveAwsAPIBrandName(hcProductCode, hcProductName, productProductName, lineItemDescription string) string {
	if isAIDeductBillItem(hcProductCode, hcProductName) {
		for _, text := range []string{productProductName, lineItemDescription, hcProductName} {
			if brand := enumor.MatchAPIBrandName(text); brand != "" {
				return brand
			}
		}
		return ""
	}
	return enumor.MatchAPIBrandName(hcProductName)
}

// resolveGcpAPIBrandName 解析 GCP 账单 API 厂商：优先匹配 hcProductName，命中为空时兜底匹配 skuDescription，
// 与 isGcpGPU 的双路径判定保持一致，避免「被判为 AI/GPU 但品牌为空」的口径不一致。
// AI 扣减条目的 hcProductName 为 AIDeduct，无品牌语义，直接走 skuDescription 兜底。
func resolveGcpAPIBrandName(hcProductName, skuDescription string) string {
	if isAIDeductBillItem("", hcProductName) {
		if brand := enumor.MatchAPIBrandName(skuDescription); brand != "" {
			return brand
		}
		return ""
	}
	if brand := enumor.MatchAPIBrandName(hcProductName); brand != "" {
		return brand
	}
	return enumor.MatchAPIBrandName(skuDescription)
}

// matchGcpInstancePrefix 按实例族前缀映射识别卡型，命中多个前缀时取前缀字符串最长者对应的卡型。
// 采用忽略大小写 + 词边界匹配，消解 A3 与 A3Ultra/A3 Ultra 的歧义。
func matchGcpInstancePrefix(skuDescription string, gcpGpuPrefixes map[string]string) string {
	if len(gcpGpuPrefixes) == 0 {
		return ""
	}

	lowerDesc := strings.ToLower(skuDescription)
	var matchedCard string
	var matchedLen int
	for prefix, card := range gcpGpuPrefixes {
		lowerPrefix := strings.ToLower(prefix)
		if len(lowerPrefix) == 0 || !util.ContainsWord(lowerDesc, lowerPrefix) {
			continue
		}
		if len(lowerPrefix) > matchedLen {
			matchedLen = len(lowerPrefix)
			matchedCard = card
		}
	}
	return matchedCard
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
func isAwsGPU(lineItemProductCode, productInstanceType, hcProductName string, awsGpuMap map[string]string) bool {
	if lineItemProductCode == constant.AmazonSageMaker {
		return true
	}

	if _, ok := awsGpuMap[productInstanceType]; ok {
		return true
	}

	if strings.HasPrefix(hcProductName, constant.BillItemAIPrefix) {
		return true
	}

	return false
}

// isGcpGPU determines whether a GCP bill item represents a GPU resource.
// 判定链：① 识别到GPU卡型；
// ② HcProductName 含 AI 前缀；
// ③ 兜底对 SkuDescription 做 AI 关键词匹配（解决 Credit 条目 HcProductName 不含 AI 前缀的问题）。
func isGcpGPU(gpuCardCategory, skuDescription, hcProductName string) bool {
	if gpuCardCategory != "" {
		return true
	}

	if strings.HasPrefix(hcProductName, constant.BillItemAIPrefix) {
		return true
	}

	// 兜底：直接对 SkuDescription 做 AI 关键词匹配
	// 解决 Credit 条目 HcProductName 不含 AI 前缀的问题
	if enumor.IsAIBillItem(skuDescription) {
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
