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

package enumor

import (
	"fmt"
	"regexp"
	"strings"
)

var aiBillItemRegexp *regexp.Regexp

// gcpGpuCardL1Keywords GCP L1 显式卡型关键词列表。
// card 为短卡型名，直接用作正则命名分组名（须为合法标识符）；pattern 为关键词正则片段（内部不得含捕获组）。
var gcpGpuCardL1Keywords = []struct {
	card    string
	pattern string
}{
	{"H200", `h200`},
	{"H100", `h100`},
	{"A100", `a100`},
	{"RTX6000PRO", `rtx\s*(?:pro\s*)?6000`},
	{"L4", `l4`},
	{"TPU", `tpu7x`},
	{"V100", `v100`},
	{"P100", `p100`},
	{"P4", `p4`},
	{"K80", `k80`},
}

// gcpGpuCardRegexp 合并全部 L1 关键词的单一正则，每个关键词为一个以卡型名命名的分组，命中的分组名即短卡型名，避免逐条正则匹配。
var gcpGpuCardRegexp *regexp.Regexp

func init() {
	aiFlag := getAIBillItemAIFlag()
	aiFlagOrStr := strings.Join(aiFlag, "|")

	// (^|[^a-z]) - 开头或者非字母字符
	// ($|[^a-z]) - 结尾或者非字母字符
	pattern := fmt.Sprintf(`(^|[^a-z])(%s)($|[^a-z])`, aiFlagOrStr)
	aiBillItemRegexp = regexp.MustCompile(pattern)

	// 合并全部 L1 关键词为单个正则，每个关键词包一层以卡型名命名的分组，命中后由分组名直接得到卡型。
	// 卡型名含数字，词边界用 [^a-z0-9] 防止子串误命中（如 A1000 命中 A100、P40 命中 P4）
	fragments := make([]string, 0, len(gcpGpuCardL1Keywords))
	for _, kw := range gcpGpuCardL1Keywords {
		fragments = append(fragments, fmt.Sprintf(`(?P<%s>%s)`, kw.card, kw.pattern))
	}
	gcpGpuCardRegexp = regexp.MustCompile(
		fmt.Sprintf(`(?:^|[^a-z0-9])(?:%s)(?:$|[^a-z0-9])`, strings.Join(fragments, "|")))
}

// BillSyncPeriodType 账单同步周期类型
type BillSyncPeriodType string

// Validate the BillSyncPeriodType is valid or not
func (b BillSyncPeriodType) Validate() error {
	switch b {
	case Daily, Weekly, Monthly:
	default:
		return fmt.Errorf("unsupported bill sync period type: %s", b)
	}
	return nil
}

const (
	// Daily 每天拉取
	Daily BillSyncPeriodType = "daily"
	// Weekly 每周拉取
	Weekly BillSyncPeriodType = "weekly"
	// Monthly 每月拉取
	Monthly BillSyncPeriodType = "monthly"
)

// BillPullMode is bill pull mode
type BillPullMode string

// Validate the BillPullMode is valid or not
func (b BillPullMode) Validate() error {
	switch b {
	case AutoPull, ManualPull:
	default:
		return fmt.Errorf("unsupported bill pull mode: %s", b)
	}
	return nil
}

const (
	// AutoPull 自动拉取
	AutoPull BillPullMode = "auto"
	// ManualPull 手动拉取
	ManualPull BillPullMode = "manual"
)

// BillDayNumber is bill date type
type BillDayNumber int

// Validate the BillDayNumber is valid or not
func (b BillDayNumber) Validate() error {
	if b < 1 || b > 31 {
		return fmt.Errorf("unsupported bill day number %d", b)
	}
	return nil
}

// CurrencyCode 货币代码
type CurrencyCode string

const (
	// CurrencyUSD usd currency
	CurrencyUSD CurrencyCode = "USD"
	// CurrencyCNY rmb currency
	CurrencyCNY CurrencyCode = "CNY"
	// CurrencyRMB rmb currency
	CurrencyRMB = CurrencyCNY
)

// BillAdjustmentType 调账类型
type BillAdjustmentType string

const (
	// BillAdjustmentIncrease 增加
	BillAdjustmentIncrease BillAdjustmentType = "increase"
	// BillAdjustmentDecrease 减少
	BillAdjustmentDecrease BillAdjustmentType = "decrease"
)

// BillAdjustmentResClass 调账资源类别
type BillAdjustmentResClass string

// Validate checks if the BillAdjustmentResClass is valid.
func (b BillAdjustmentResClass) Validate() error {
	switch b {
	case BillAdjustmentResClassCPU, BillAdjustmentResClassGpuCard,
		BillAdjustmentResClassGpuAPI, BillAdjustmentResClassGpuOther:
	default:
		return fmt.Errorf("unsupported bill adjustment res class: %s", b)
	}
	return nil
}

// IsGPU 判断资源类别是否属于 GPU 类支出，CPU 之外的三类均为 GPU 类。
func (b BillAdjustmentResClass) IsGPU() bool {
	return b != BillAdjustmentResClassCPU
}

// IsAPI 判断资源类别是否为大模型 API 调用费。
func (b BillAdjustmentResClass) IsAPI() bool {
	return b == BillAdjustmentResClassGpuAPI
}

// NeedResSubClass 判断该资源类别是否要求填写资源子类，仅 GPU 卡与模型 API 两类需要。
func (b BillAdjustmentResClass) NeedResSubClass() bool {
	return b == BillAdjustmentResClassGpuCard || b == BillAdjustmentResClassGpuAPI
}

const (
	// BillAdjustmentResClassCPU CPU 资源类别
	BillAdjustmentResClassCPU BillAdjustmentResClass = "cpu"
	// BillAdjustmentResClassGpuCard GPU 卡资源类别，资源子类为卡型
	BillAdjustmentResClassGpuCard BillAdjustmentResClass = "gpu_card"
	// BillAdjustmentResClassGpuAPI 大模型 API 调用资源类别，资源子类为模型厂商
	BillAdjustmentResClassGpuAPI BillAdjustmentResClass = "gpu_api"
	// BillAdjustmentResClassGpuOther 识别不出卡型的 GPU 资源类别
	BillAdjustmentResClassGpuOther BillAdjustmentResClass = "gpu_other"
)

// BillAdjustmentState 调账明细状态
type BillAdjustmentState string

const (
	// BillAdjustmentStateConfirmed 已确认
	BillAdjustmentStateConfirmed BillAdjustmentState = "confirmed"
	// BillAdjustmentStateUnconfirmed 未确认
	BillAdjustmentStateUnconfirmed BillAdjustmentState = "unconfirmed"
)

// RootBillSummaryState 一级账号账单汇总状态枚举
type RootBillSummaryState string

const (
	// RootAccountBillSummaryStateAccounting 核算中
	RootAccountBillSummaryStateAccounting RootBillSummaryState = "accounting"
	// RootAccountBillSummaryStateAccounted 已核算
	RootAccountBillSummaryStateAccounted RootBillSummaryState = "accounted"
	// RootAccountBillSummaryStateConfirmed 已确认
	RootAccountBillSummaryStateConfirmed RootBillSummaryState = "confirmed"
	// RootAccountBillSummaryStateSyncing 同步中
	RootAccountBillSummaryStateSyncing RootBillSummaryState = "syncing"
	// RootAccountBillSummaryStateSynced 已同步
	RootAccountBillSummaryStateSynced RootBillSummaryState = "synced"
	// RootAccountBillSummaryStateStop 已停止
	RootAccountBillSummaryStateStop RootBillSummaryState = "stopped"
)

// MainBillSummaryState  二级账号账单汇总状态
type MainBillSummaryState string

const (
	// MainAccountBillSummaryStateAccounting 核算中
	MainAccountBillSummaryStateAccounting MainBillSummaryState = "accounting"

	// MainAccountBillSummaryStateWaitMonthTask 等待月度分账
	MainAccountBillSummaryStateWaitMonthTask MainBillSummaryState = "waiting_month_task"

	// MainAccountBillSummaryStateAccounted 已核算
	MainAccountBillSummaryStateAccounted MainBillSummaryState = "accounted"

	// MainAccountBillSummaryStateSyncing 同步中
	MainAccountBillSummaryStateSyncing MainBillSummaryState = "syncing"

	// MainAccountBillSummaryStateSynced 已同步
	MainAccountBillSummaryStateSynced MainBillSummaryState = "synced"

	// MainAccountBillSummaryStateStop 停止中
	MainAccountBillSummaryStateStop MainBillSummaryState = "stopped"
)

// MainRawBillPullState 二级账号账单拉取状态
type MainRawBillPullState string

const (
	// MainAccountRawBillPullStatePulling 拉取中
	MainAccountRawBillPullStatePulling MainRawBillPullState = "pulling"

	// MainAccountRawBillPullStatePulled 已拉取
	MainAccountRawBillPullStatePulled MainRawBillPullState = "pulled"

	// MainAccountRawBillPullStateSplit 已分账
	MainAccountRawBillPullStateSplit MainRawBillPullState = "split"

	// MainAccountRawBillPullStateAccounted 已核算
	MainAccountRawBillPullStateAccounted MainRawBillPullState = "accounted"

	// MainAccountRawBillPullStateStop 停止中
	MainAccountRawBillPullStateStop MainRawBillPullState = "stopped"
)

// BillSyncState 云账单同步状态
type BillSyncState string

const (

	// BillSyncRecordStateNew 新增同步记录
	BillSyncRecordStateNew BillSyncState = "new"

	// BillSyncRecordStateSyncingBillItem 同步账单中
	BillSyncRecordStateSyncingBillItem BillSyncState = "syncing_bill_item"

	// BillSyncRecordStateSyncingAdjustment 同步调账中
	BillSyncRecordStateSyncingAdjustment BillSyncState = "syncing_adjustment_item"

	// BillSyncRecordStateWaitNotifying 同步等待通知
	BillSyncRecordStateWaitNotifying BillSyncState = "wait_notifying"

	// BillSyncRecordStateSynced 已同步
	BillSyncRecordStateSynced BillSyncState = "synced"

	// BillSyncRecordStateFailed 同步失败
	BillSyncRecordStateFailed BillSyncState = "failed"
)

// BillSyncMode 云账单对外（OBS）同步模式
type BillSyncMode string

const (
	// BillSyncModeFull 全量同步：bill_item 明细全量推送后再同步调账
	BillSyncModeFull BillSyncMode = "full"
	// BillSyncModeAdjustmentOnly 只同步调账：跳过 bill_item 明细推送，仅计数后同步调账
	BillSyncModeAdjustmentOnly BillSyncMode = "adjustment_only"
)

// Validate 校验同步模式取值
func (b BillSyncMode) Validate() error {
	switch b {
	case BillSyncModeFull, BillSyncModeAdjustmentOnly:
	default:
		return fmt.Errorf("unsupported bill sync mode: %s", b)
	}
	return nil
}

// RootAccountMonthBillTaskState 一级账号月度账单（除去每日账单）状态
type RootAccountMonthBillTaskState string

const (
	// RootAccountMonthBillTaskStatePulling 拉取中
	RootAccountMonthBillTaskStatePulling = "pulling"

	// RootAccountMonthBillTaskStatePulled 已拉取
	RootAccountMonthBillTaskStatePulled = "pulled"

	// RootAccountMonthBillTaskStateSplit 已分账
	RootAccountMonthBillTaskStateSplit = "split"

	// RootAccountMonthBillTaskStateAccounted 已核算
	RootAccountMonthBillTaskStateAccounted = "accounted"

	// RootAccountMonthBillTaskStateStop 停止中
	RootAccountMonthBillTaskStateStop = "stopped"
)

// MonthTaskType 月度任务类型
type MonthTaskType string

const (
	// AwsOutsideBillMonthTask usage start date outside current bill month
	AwsOutsideBillMonthTask MonthTaskType = "outside_month_bill"
	// AwsSavingsPlansMonthTask aws savings plans month task
	AwsSavingsPlansMonthTask MonthTaskType = "savings_plans"
	// AwsSupportMonthTask aws support month task
	AwsSupportMonthTask MonthTaskType = "support"
	// DeductMonthTask deduct month task
	DeductMonthTask MonthTaskType = "deduct"
	// AwsAIDeductMonthTask ai bill deduct
	AwsAIDeductMonthTask MonthTaskType = "ai_deduct"

	// GcpCreditsMonthTask gcp credits month task
	GcpCreditsMonthTask MonthTaskType = "credits"
	// GcpSupportMonthTask gcp support month task
	GcpSupportMonthTask MonthTaskType = "support"
	// GcpAIDeductMonthTask ai bill deduct
	GcpAIDeductMonthTask MonthTaskType = "ai_deduct"

	// HuaweiSupportMonthTask 华为support plan
	HuaweiSupportMonthTask MonthTaskType = "support"
	// HuaweiTaxDeductMonthTask tax bill deduct
	HuaweiTaxDeductMonthTask MonthTaskType = "tax_deduct"
)

// MonthTaskStep 月度任务步骤
type MonthTaskStep string

const (
	// MonthTaskStepPull 拉取类型
	MonthTaskStepPull MonthTaskStep = "pull"
	// MonthTaskStepSplit 分账类型
	MonthTaskStepSplit MonthTaskStep = "split"
	// MonthTaskStepSummary 汇总类型
	MonthTaskStepSummary MonthTaskStep = "summary"
)

const (
	// MonthRawBillPathName 拉取原始账单保存路径
	MonthRawBillPathName = "monthbill"
	// MonthRawBillSpecialDatePathName 特殊日期原始账单保存路径
	MonthRawBillSpecialDatePathName = "00"
)

// MonthTaskSpecialBillDay special bill day 0 to represent the whole month
const MonthTaskSpecialBillDay = 0

var (
	// BillAdjustmentStateNameMap is the map of bill adjustment state name
	BillAdjustmentStateNameMap = map[BillAdjustmentState]string{
		BillAdjustmentStateConfirmed:   "已确认",
		BillAdjustmentStateUnconfirmed: "未确认",
	}

	// BillAdjustmentTypeNameMap is the map of bill adjustment type name
	BillAdjustmentTypeNameMap = map[BillAdjustmentType]string{
		BillAdjustmentIncrease: "增加",
		BillAdjustmentDecrease: "减少",
	}

	// BillAdjustmentResClassNameMap is the map of bill adjustment res class name
	BillAdjustmentResClassNameMap = map[BillAdjustmentResClass]string{
		BillAdjustmentResClassCPU:      "CPU",
		BillAdjustmentResClassGpuCard:  "GPU卡",
		BillAdjustmentResClassGpuAPI:   "模型API",
		BillAdjustmentResClassGpuOther: "GPU其他",
	}

	// RootAccountBillSummaryStateMap 一级账号账单汇总状态中文名
	RootAccountBillSummaryStateMap = map[RootBillSummaryState]string{
		RootAccountBillSummaryStateAccounting: "核算中",
		RootAccountBillSummaryStateAccounted:  "已核算",
		RootAccountBillSummaryStateConfirmed:  "已确认",
		RootAccountBillSummaryStateSyncing:    "同步中",
		RootAccountBillSummaryStateSynced:     "已同步",
		RootAccountBillSummaryStateStop:       "停止中",
	}
)

// BillItemAIFlag 账单项AI标识
type BillItemAIFlag string

const (
	// BillItemAIFlagGemini gemini
	BillItemAIFlagGemini BillItemAIFlag = "gemini"
	// BillItemAIFlagClaude claude
	BillItemAIFlagClaude BillItemAIFlag = "claude"
	// BillItemAIFlagKimi kimi
	BillItemAIFlagKimi BillItemAIFlag = "kimi"
	// BillItemAIFlagJina jina
	BillItemAIFlagJina BillItemAIFlag = "jina"
	// BillItemAIFlagVeo veo
	BillItemAIFlagVeo BillItemAIFlag = "veo"
	// BillItemAIFlagImagen imagen
	BillItemAIFlagImagen BillItemAIFlag = "imagen"
	// BillItemAIFlagLyria lyria
	BillItemAIFlagLyria BillItemAIFlag = "lyria"
)

// ListGcpGpuCardL1 返回 GCP L1 显式卡型关键词表中的全部短卡型名，已按定义顺序去重。
// 与 MatchGcpGpuCardByKeyword 共用 gcpGpuCardL1Keywords，往关键词表加卡型时本函数自动跟随。
func ListGcpGpuCardL1() []string {
	cards := make([]string, 0, len(gcpGpuCardL1Keywords))
	seen := make(map[string]struct{}, len(gcpGpuCardL1Keywords))
	for _, kw := range gcpGpuCardL1Keywords {
		if _, ok := seen[kw.card]; ok {
			continue
		}
		seen[kw.card] = struct{}{}
		cards = append(cards, kw.card)
	}
	return cards
}

// ListBillAdjustmentAPIBrands 返回调账可选的模型厂商清单。
// 取账单上报侧 MatchAPIBrandName 归并后的四值，不含 veo/imagen/lyria——
// 这三者在上报侧已归并为 gemini，出现在调账下拉里会造成核算口径分裂，
// 因此本函数不复用返回七值的 getAIBillItemAIFlag。
func ListBillAdjustmentAPIBrands() []string {
	return []string{
		string(BillItemAIFlagClaude), string(BillItemAIFlagGemini),
		string(BillItemAIFlagJina), string(BillItemAIFlagKimi),
	}
}

func getAIBillItemAIFlag() []string {
	return []string{
		string(BillItemAIFlagGemini), string(BillItemAIFlagClaude), string(BillItemAIFlagKimi),
		string(BillItemAIFlagJina), string(BillItemAIFlagVeo), string(BillItemAIFlagImagen),
		string(BillItemAIFlagLyria),
	}
}

// IsAIBillItem 判断账单项是否为AI账单
func IsAIBillItem(str string) bool {
	// 将字符串转换为小写以便忽略大小写
	lowerStr := strings.ToLower(str)
	return aiBillItemRegexp.MatchString(lowerStr)
}

// MatchAPIBrandName 在账单明细文本中匹配 API 厂商品牌，返回命中的品牌（小写）。
// 复用 IsAIBillItem 的忽略大小写、词边界匹配规则；
// veo/imagen/lyria 归并为 gemini， 其余取命中关键词原值；
// 未命中返回空字符串。一条文本命中多个关键词时取位置最靠前的命中词。
func MatchAPIBrandName(str string) string {
	// aiBillItemRegexp 的词边界只含 [a-z]，需先转小写再匹配，否则大写文本无法命中
	lowerStr := strings.ToLower(str)
	// 第 2 个捕获组为命中的品牌关键词
	matches := aiBillItemRegexp.FindStringSubmatch(lowerStr)
	if len(matches) < 3 {
		return ""
	}

	keyword := matches[2]
	switch BillItemAIFlag(keyword) {
	case BillItemAIFlagVeo, BillItemAIFlagImagen, BillItemAIFlagLyria:
		return string(BillItemAIFlagGemini)
	default:
		return keyword
	}
}

// MatchGcpGpuCardByKeyword 按 L1 显式卡型关键词识别 GCP GPU 卡型，返回短卡型名，未命中返回空字符串。
// 复用忽略大小写 + 词边界匹配规则；卡型名含数字，词边界用 [^a-z0-9] 防止子串误命中。
// 使用单一合并正则匹配，命中的命名分组名即为短卡型名，避免逐条正则匹配。
func MatchGcpGpuCardByKeyword(str string) string {
	lowerStr := strings.ToLower(str)
	matches := gcpGpuCardRegexp.FindStringSubmatch(lowerStr)
	if matches == nil {
		return ""
	}

	// 命中的命名分组名即为短卡型名（同一位置仅一个关键词分组非空）
	for i, name := range gcpGpuCardRegexp.SubexpNames() {
		if name != "" && matches[i] != "" {
			return name
		}
	}
	return ""
}

// OBSResClassID OBS 资源分类 ID
type OBSResClassID int32

const (
	// OBSResClassIDAwsCPU AWS CPU 资源分类 ID
	OBSResClassIDAwsCPU OBSResClassID = 451
	// OBSResClassIDAwsGPU AWS GPU 资源分类 ID
	OBSResClassIDAwsGPU OBSResClassID = 6311
	// OBSResClassIDGcpCPU GCP CPU 资源分类 ID
	OBSResClassIDGcpCPU OBSResClassID = 601
	// OBSResClassIDGcpGPU GCP GPU 资源分类 ID
	OBSResClassIDGcpGPU OBSResClassID = 6312
	// OBSResClassIDHuaweiCPU 华为 CPU 资源分类 ID
	OBSResClassIDHuaweiCPU OBSResClassID = 1244
	// OBSResClassIDHuaweiGPU 华为 GPU 资源分类 ID
	OBSResClassIDHuaweiGPU OBSResClassID = 6315
	// OBSResClassIDAwsAPI AWS 模型API厂商资源分类 ID
	OBSResClassIDAwsAPI OBSResClassID = 6799
	// OBSResClassIDGcpAPI GCP 模型API厂商资源分类 ID
	OBSResClassIDGcpAPI OBSResClassID = 6800
)

// GetOBSResClassID returns the OBS resource class ID for the given vendor and GPU flag.
func GetOBSResClassID(vendor Vendor, isGPU bool) int32 {
	switch vendor {
	case Aws:
		if isGPU {
			return int32(OBSResClassIDAwsGPU)
		}
		return int32(OBSResClassIDAwsCPU)
	case Gcp:
		if isGPU {
			return int32(OBSResClassIDGcpGPU)
		}
		return int32(OBSResClassIDGcpCPU)
	case HuaWei:
		if isGPU {
			return int32(OBSResClassIDHuaweiGPU)
		}
		return int32(OBSResClassIDHuaweiCPU)
	default:
		return 0
	}
}

// GetOBSResClassIDByType 按厂商与账单类型返回 OBS 资源分类 ID，优先级 API > GPU > CPU。
// 仅 AWS/GCP 有 API 分类，其余厂商 isAPI 不生效，回退到 GPU/CPU 判定。
func GetOBSResClassIDByType(vendor Vendor, isGPU, isAPI bool) int32 {
	if isAPI {
		switch vendor {
		case Aws:
			return int32(OBSResClassIDAwsAPI)
		case Gcp:
			return int32(OBSResClassIDGcpAPI)
		}
	}
	return GetOBSResClassID(vendor, isGPU)
}
