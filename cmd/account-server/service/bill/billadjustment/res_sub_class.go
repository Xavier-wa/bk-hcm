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

package billadjustment

import (
	"encoding/json"
	"sort"

	"hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	billcore "hcm/pkg/api/core/bill"
	datagconf "hcm/pkg/api/data-service/global_config"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/maps"
)

// resSubClassVendors 支持资源子类的云厂商，其余厂商视为参数非法。
var resSubClassVendors = map[enumor.Vendor]struct{}{
	enumor.Aws:    {},
	enumor.Gcp:    {},
	enumor.HuaWei: {},
}

// resSubClassSource 资源子类清单的运营配置来源，取自 global_config。
type resSubClassSource struct {
	// awsGpuInstanceTypes AWS「实例类型 → 卡型」映射，value 即卡型
	awsGpuInstanceTypes map[string]string
	// gcpGpuInstancePrefixes GCP「实例族前缀 → 短卡型名」映射，value 即卡型
	gcpGpuInstancePrefixes map[string]string
}

// isResSubClassVendorSupported 判断云厂商是否在资源子类的支持范围内。
func isResSubClassVendorSupported(vendor enumor.Vendor) bool {
	_, ok := resSubClassVendors[vendor]
	return ok
}

// loadResSubClassSource 从 global_config 读取卡型清单依赖的两项运营配置。
// 与 cmd/task-server/logics/action/obs/sync/gpu_lookup.go 的 loadAwsGpuInstanceTypes、
// loadGcpGpuInstancePrefixes 消费同一份配置，两侧共用配置键常量与配置值的 JSON 结构约定。
func (b *billAdjustmentSvc) loadResSubClassSource(kt *kit.Kit) (*resSubClassSource, error) {
	listReq := &datagconf.ListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("config_type", enumor.GlobalConfigTypeAccountBill),
			tools.RuleIn("config_key", []enumor.GlobalConfigKeyAccountBill{
				enumor.GlobalConfigKeyAwsGpuInstanceTypes,
				enumor.GlobalConfigKeyGcpGpuInstancePrefixes,
			}),
		),
		Page: core.NewDefaultBasePage(),
	}

	resp, err := b.client.DataService().Global.GlobalConfig.List(kt, listReq)
	if err != nil {
		logs.Errorf("fail to list gpu global config for res sub class, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	source := &resSubClassSource{
		awsGpuInstanceTypes:    make(map[string]string),
		gcpGpuInstancePrefixes: make(map[string]string),
	}
	for _, detail := range resp.Details {
		switch enumor.GlobalConfigKeyAccountBill(detail.ConfigKey) {
		case enumor.GlobalConfigKeyAwsGpuInstanceTypes:
			source.awsGpuInstanceTypes = parseGpuCardConfigValue(kt, detail.ConfigKey, detail.ConfigValue)
		case enumor.GlobalConfigKeyGcpGpuInstancePrefixes:
			source.gcpGpuInstancePrefixes = parseGpuCardConfigValue(kt, detail.ConfigKey, detail.ConfigValue)
		}
	}
	return source, nil
}

// parseGpuCardConfigValue 解析「实例类型/实例族前缀 → 卡型」JSON 对象配置，
// 解析失败记警告并返回空映射，由调用方按忽略该来源处理。
func parseGpuCardConfigValue(kt *kit.Kit, configKey string, configValue types.JsonField) map[string]string {
	cardMap := make(map[string]string)
	if err := json.Unmarshal([]byte(configValue), &cardMap); err != nil {
		logs.Warnf("fail to parse gpu card config %s, ignore this source, err: %v, rid: %s", configKey, err, kt.Rid)
		return make(map[string]string)
	}
	return cardMap
}

// listGpuCards 返回指定云厂商可选的卡型清单，结果已去重并按字典序稳定排序。
// AWS 取 aws_gpu_instance_types 的 value 集合；GCP 取一级卡型关键字表并上
// gcp_gpu_instance_prefixes 的 value 集合；华为云无卡型来源，返回空列表。
func listGpuCards(vendor enumor.Vendor, source *resSubClassSource) []string {
	if source == nil {
		source = &resSubClassSource{}
	}

	cardSet := make(map[string]struct{})
	switch vendor {
	case enumor.Aws:
		cardSet = collectNonEmptyValues(cardSet, source.awsGpuInstanceTypes)
	case enumor.Gcp:
		for _, card := range enumor.ListGcpGpuCardL1() {
			cardSet[card] = struct{}{}
		}
		cardSet = collectNonEmptyValues(cardSet, source.gcpGpuInstancePrefixes)
	}
	return sortedKeys(cardSet)
}

// listAPIBrands 返回指定云厂商可选的模型厂商清单。
// AWS 与 GCP 取账单上报侧归并后的四值；华为云无 OBS API 资源分类，返回空列表。
func listAPIBrands(vendor enumor.Vendor) []string {
	switch vendor {
	case enumor.Aws, enumor.Gcp:
		return enumor.ListBillAdjustmentAPIBrands()
	default:
		return []string{}
	}
}

// listResSubClassOptions 按资源类别返回该云厂商可选的资源子类清单，
// 不需要资源子类的类别返回空列表。下拉查询与写入校验共用本函数，保证两者同源。
func listResSubClassOptions(vendor enumor.Vendor, resClass enumor.BillAdjustmentResClass,
	source *resSubClassSource) []string {

	switch resClass {
	case enumor.BillAdjustmentResClassGpuCard:
		return listGpuCards(vendor, source)
	case enumor.BillAdjustmentResClassGpuAPI:
		return listAPIBrands(vendor)
	default:
		return []string{}
	}
}

// isResSubClassInOptions 判断资源子类取值是否命中清单。
// 严格比对且区分大小写：不做大小写归一，也不把取值改写为清单中的写法。
func isResSubClassInOptions(resSubClass string, options []string) bool {
	for _, option := range options {
		if option == resSubClass {
			return true
		}
	}
	return false
}

// validateResSubClass 按资源类别与云厂商校验资源子类的必填性与取值域。
// gpu_card / gpu_api 下必填且取值须命中该厂商的对应清单，cpu / gpu_other 下必须为空，
// 三类违反一律返回参数非法，不静默忽略也不静默置空。
func validateResSubClass(vendor enumor.Vendor, resClass enumor.BillAdjustmentResClass, resSubClass string,
	source *resSubClassSource) error {

	if !resClass.NeedResSubClass() {
		if len(resSubClass) != 0 {
			return errf.Newf(errf.InvalidParameter, "res_sub_class must be empty when res_class is %s", resClass)
		}
		return nil
	}

	if len(resSubClass) == 0 {
		return errf.Newf(errf.InvalidParameter, "res_sub_class is required when res_class is %s", resClass)
	}

	options := listResSubClassOptions(vendor, resClass, source)
	if !isResSubClassInOptions(resSubClass, options) {
		return errf.Newf(errf.InvalidParameter, "res_sub_class %s is not in the %s option list of vendor %s",
			resSubClass, resClass, vendor)
	}
	return nil
}

// validateCreateResSubClass 校验创建请求中每条明细的资源子类，云厂商取自请求中的 vendor。
// 全部明细都不需要资源子类时跳过配置读取。
func (b *billAdjustmentSvc) validateCreateResSubClass(kt *kit.Kit, req *bill.BatchBillAdjustmentItemCreateReq) error {

	var source *resSubClassSource
	for _, item := range req.Items {
		if !item.ResClass.NeedResSubClass() {
			continue
		}
		loaded, err := b.loadResSubClassSource(kt)
		if err != nil {
			return err
		}
		source = loaded
		break
	}

	for i, item := range req.Items {
		if err := validateResSubClass(req.Vendor, item.ResClass, item.ResSubClass, source); err != nil {
			logs.Errorf("invalid res sub class at item idx %d, err: %v, rid: %s", i, err, kt.Rid)
			return err
		}
	}
	return nil
}

// validateUpdateResSubClass 按记录所属云厂商校验更新后的资源类别与资源子类组合。
// 更新请求的 URL 不含云厂商，厂商与未改动字段的现值都取自已读出的记录；
// req.ResSubClass 为 nil 时沿用记录现值参与校验，由此拦下「把类别改为非 GPU 细分却不显式置空」的请求。
func (b *billAdjustmentSvc) validateUpdateResSubClass(kt *kit.Kit, record *billcore.AdjustmentItem,
	req *bill.BillAdjustmentItemUpdateReq) error {

	resClass, resSubClass := mergeUpdateResSubClass(record, req)

	var source *resSubClassSource
	if resClass.NeedResSubClass() {
		loaded, err := b.loadResSubClassSource(kt)
		if err != nil {
			return err
		}
		source = loaded
	}

	return validateResSubClass(record.Vendor, resClass, resSubClass, source)
}

// mergeUpdateResSubClass 把更新请求叠加到记录现值上，得到本次更新后的资源类别与资源子类。
// 请求未携带的字段沿用记录现值，req.ResSubClass 非 nil 即视为显式赋值（含显式置空）。
func mergeUpdateResSubClass(record *billcore.AdjustmentItem, req *bill.BillAdjustmentItemUpdateReq) (
	enumor.BillAdjustmentResClass, string) {

	resClass := record.ResClass
	if len(req.ResClass) != 0 {
		resClass = req.ResClass
	}

	resSubClass := record.ResSubClass
	if req.ResSubClass != nil {
		resSubClass = cvt.PtrToVal(req.ResSubClass)
	}
	return resClass, resSubClass
}

// collectNonEmptyValues 把映射中的非空 value 收集进集合，用于卡型去重。
func collectNonEmptyValues(dst map[string]struct{}, src map[string]string) map[string]struct{} {
	for _, value := range src {
		if len(value) == 0 {
			continue
		}
		dst[value] = struct{}{}
	}

	return dst
}

// sortedKeys 返回集合中按字典序排序的元素，保证下拉选项在多次请求间顺序稳定。
func sortedKeys(set map[string]struct{}) []string {
	keys := maps.Keys(set)
	sort.Strings(keys)
	return keys
}
