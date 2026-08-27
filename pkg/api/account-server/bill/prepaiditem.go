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

package bill

import (
	"fmt"
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"

	"github.com/shopspring/decimal"
)

// PrepaidItemSyncReq 批量推送预付费账单的写入请求。
type PrepaidItemSyncReq struct {
	Items []PrepaidItemSyncItem `json:"items" validate:"required,min=1,max=100"`
}

// Validate 逐单执行硬校验，并校验批内业务唯一键不重复。
// 批内重复的 (uuid, 订单年月) 会让后一单覆盖前一单。
func (req *PrepaidItemSyncReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return errf.NewFromErr(errf.InvalidParameter, err)
	}

	uniqueKeys := make(map[string]struct{}, len(req.Items))
	for idx := range req.Items {
		if err := req.Items[idx].Validate(); err != nil {
			return errf.Newf(errf.InvalidParameter, "invalid field items[%d]: %v", idx, err)
		}

		uniqueKey := req.Items[idx].UniqueKey()
		if _, duplicated := uniqueKeys[uniqueKey]; duplicated {
			return errf.Newf(errf.InvalidParameter,
				"invalid field items[%d]: unique key %s duplicated in one batch", idx, uniqueKey)
		}
		uniqueKeys[uniqueKey] = struct{}{}
	}

	return nil
}

// PrepaidItemSyncItem 是单个预付费订单的写入内容，由账单主数据与 N 条已分摊明细构成。
type PrepaidItemSyncItem struct {
	// UUID 外部唯一标识，与订单月份共同构成业务唯一键
	UUID string `json:"uuid" validate:"required,max=255"`
	// OrderYear 订单年份
	OrderYear int `json:"order_year" validate:"required,min=1970"`
	// OrderMonth 订单月份
	OrderMonth int `json:"order_month" validate:"required,min=1,max=12"`
	// Vendor 云厂商
	Vendor enumor.Vendor `json:"vendor" validate:"required"`
	// RootAccountCloudID 一级账号的云上账号 ID
	RootAccountCloudID string `json:"root_account_cloud_id" validate:"required,max=255"`
	// MainAccountCloudID 二级账号的云上账号 ID
	MainAccountCloudID string `json:"main_account_cloud_id" validate:"required,max=255"`
	// ResourceID 资源 ID
	ResourceID string `json:"resource_id" validate:"omitempty,max=255"`
	// InvoiceID 发票 ID
	InvoiceID string `json:"invoice_id" validate:"omitempty,max=255"`
	// GPUType GPU 型号，同时作为派生调账的资源子类
	GPUType string `json:"gpu_type" validate:"required,max=64"`
	// DeviceNum 数量（台），非必填，传入时须大于 0
	DeviceNum int `json:"device_num"`
	// CardNum 数量（卡），非必填，传入时须大于 0
	CardNum int `json:"card_num"`
	// ProductName 产品名称
	ProductName string `json:"product_name" validate:"omitempty,max=255"`
	// ProductSpec 产品规格
	ProductSpec string `json:"product_spec" validate:"omitempty,max=255"`
	// Region 地域
	Region string `json:"region" validate:"omitempty,max=255"`
	// UsageStartAt 使用开始时间，非必填，格式 constant.DateTimeLayout
	UsageStartAt string `json:"usage_start_at" validate:"omitempty"`
	// UsageEndAt 使用结束时间，非必填，格式 constant.DateTimeLayout
	UsageEndAt string `json:"usage_end_at" validate:"omitempty"`
	// OrderAt 订单时间，格式 constant.DateTimeLayout，年份和月份须与 order_year / order_month 一致
	OrderAt string `json:"order_at" validate:"required"`
	// Currency 币种，enumeration values such as: CNY/USD
	Currency enumor.CurrencyCode `json:"currency" validate:"required"`
	// Cost 优惠后总价（不含税），须大于 0；人民币金额由服务端按币种派生
	Cost decimal.Decimal `json:"cost"`
	// SplitItems 逐月分摊明细，至少 1 条，不设条数上限
	SplitItems []PrepaidSplitItem `json:"split_items" validate:"required,min=1,dive"`
}

// PrepaidSplitItem 是预付费账单的单月分摊明细，一条明细对应一条 increase 调账。
type PrepaidSplitItem struct {
	// BillYear 分摊账期年份
	BillYear int `json:"bill_year" validate:"required,min=1970"`
	// BillMonth 分摊账期月份
	BillMonth int `json:"bill_month" validate:"required,min=1,max=12"`
	// Cost 该月分摊额，N 条合计等于主数据 Cost；人民币金额由服务端按币种派生
	Cost decimal.Decimal `json:"cost"`
	// Memo 备注
	Memo *string `json:"memo,omitempty" validate:"omitempty,max=255"`
}

// PrepaidItemSyncResp 是 sync 接口的响应，逐单返回写入结果，顺序与请求 items 一致。
type PrepaidItemSyncResp struct {
	// Results 逐单写入结果
	Results []PrepaidItemSyncResult `json:"results"`
}

// PrepaidItemSyncResult 是单个预付费订单的写入结果。
// 逐单独立事务，故失败只在本条记录原因，不影响其余单据，调用方可按 uuid 单独重推。
type PrepaidItemSyncResult struct {
	// UUID 外部唯一标识，与请求一致
	UUID string `json:"uuid"`
	// OrderYear 订单年份，与请求一致
	OrderYear int `json:"order_year"`
	// OrderMonth 订单月份，与请求一致
	OrderMonth int `json:"order_month"`
	// ID 预付费账单 ID，该单失败时为空串
	ID string `json:"id"`
	// Success 该单是否写入成功
	Success bool `json:"success"`
	// Message 该单的失败原因，成功时为空串
	Message string `json:"message"`
}

// PrepaidItemTimes 保存单个订单中三个时间字段的解析结果，供写入域复用避免二次解析。
// 使用起止时间为非必填，未传入时对应字段为零值时间。
type PrepaidItemTimes struct {
	// UsageStartAt 使用开始时间
	UsageStartAt time.Time
	// UsageEndAt 使用结束时间
	UsageEndAt time.Time
	// OrderAt 订单时间
	OrderAt time.Time
}

// UniqueKey 返回该单的唯一键（uuid + 订单年月），用于批内去重与日志定位。
func (item *PrepaidItemSyncItem) UniqueKey() string {
	return fmt.Sprintf("%s/%d-%02d", item.UUID, item.OrderYear, item.OrderMonth)
}

// Validate 执行单个订单的全部硬校验，任一项不通过返回 errf.InvalidParameter 并指明字段。
func (item *PrepaidItemSyncItem) Validate() error {
	if err := validator.Validate.Struct(item); err != nil {
		return errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := item.Vendor.Validate(); err != nil {
		return errf.Newf(errf.InvalidParameter, "invalid field vendor: %v", err)
	}
	if err := item.Currency.Validate(); err != nil {
		return errf.Newf(errf.InvalidParameter, "invalid field currency: %v", err)
	}

	if _, err := item.ParseTimes(); err != nil {
		return err
	}

	if err := item.validateNumericFields(); err != nil {
		return err
	}

	return item.validateSplitItems()
}

// parseDateTime 按 constant.DateTimeLayout 在本地时区解析时间字符串。
func parseDateTime(value string) (time.Time, error) {
	return time.ParseInLocation(constant.DateTimeLayout, value, time.Local)
}

// ParseTimes 按 constant.DateTimeLayout 解析时间字段。
// 订单时间的年份和月份须与 order_year / order_month 一致。
// 使用起止时间为非必填：两者都传入时校验先后关系，缺任一时不做先后约束，未传入的字段返回零值时间。
func (item *PrepaidItemSyncItem) ParseTimes() (*PrepaidItemTimes, error) {
	times := new(PrepaidItemTimes)

	var err error
	if len(item.UsageStartAt) != 0 {
		if times.UsageStartAt, err = parseDateTime(item.UsageStartAt); err != nil {
			return nil, errf.Newf(errf.InvalidParameter, "invalid field usage_start_at: %v", err)
		}
	}
	if len(item.UsageEndAt) != 0 {
		if times.UsageEndAt, err = parseDateTime(item.UsageEndAt); err != nil {
			return nil, errf.Newf(errf.InvalidParameter, "invalid field usage_end_at: %v", err)
		}
	}
	if times.OrderAt, err = parseDateTime(item.OrderAt); err != nil {
		return nil, errf.Newf(errf.InvalidParameter, "invalid field order_at: %v", err)
	}
	if times.OrderAt.Year() != item.OrderYear || int(times.OrderAt.Month()) != item.OrderMonth {
		return nil, errf.Newf(errf.InvalidParameter,
			"invalid field order_at: must match order_year and order_month, order_at: %s, order_year: %d, order_month: %d",
			item.OrderAt, item.OrderYear, item.OrderMonth)
	}

	if times.UsageStartAt.IsZero() || times.UsageEndAt.IsZero() {
		return times, nil
	}
	if !times.UsageStartAt.Before(times.UsageEndAt) {
		return nil, errf.Newf(errf.InvalidParameter,
			"invalid field usage_start_at: must be earlier than usage_end_at, start: %s, end: %s",
			item.UsageStartAt, item.UsageEndAt)
	}

	return times, nil
}

// validateNumericFields 校验主数据的数量与金额字段。
// 数量为非必填，零值即视为未传入，只拒绝负数；优惠后总价必须严格大于 0。
func (item *PrepaidItemSyncItem) validateNumericFields() error {
	if item.DeviceNum < 0 {
		return errf.Newf(errf.InvalidParameter, "invalid field device_num: must be greater than 0, got: %d",
			item.DeviceNum)
	}
	if item.CardNum < 0 {
		return errf.Newf(errf.InvalidParameter, "invalid field card_num: must be greater than 0, got: %d",
			item.CardNum)
	}
	if item.Cost.LessThanOrEqual(decimal.Zero) {
		return errf.Newf(errf.InvalidParameter, "invalid field cost: must be greater than 0, got: %s",
			item.Cost.String())
	}

	return nil
}

// validateSplitItems 校验分摊明细合计等于优惠后总价。
func (item *PrepaidItemSyncItem) validateSplitItems() error {
	sum := decimal.Zero
	for _, split := range item.SplitItems {
		sum = sum.Add(split.Cost)
	}

	if !sum.Equal(item.Cost) {
		return errf.Newf(errf.InvalidParameter,
			"invalid field split_items: sum of split cost(%s) does not equal cost(%s)", sum.String(),
			item.Cost.String())
	}

	return nil
}

// PrepaidItemDeleteReq 按订单年月批量删除预付费账单的请求。
type PrepaidItemDeleteReq struct {
	// OrderYear 订单年份
	OrderYear int `json:"order_year" validate:"required"`
	// OrderMonth 订单月份
	OrderMonth int `json:"order_month" validate:"required,min=1,max=12"`
	// MainAccountCloudIDs 二级账号的云上账号 ID 列表，非必填；传入时只删除这些账号下的主单
	MainAccountCloudIDs []string `json:"main_account_cloud_ids" validate:"omitempty,dive,required"`
}

// Validate 校验订单年月取值。
func (req *PrepaidItemDeleteReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return errf.NewFromErr(errf.InvalidParameter, err)
	}
	return nil
}

// PrepaidItemDeleteResp 按订单年月批量删除预付费账单的响应。
type PrepaidItemDeleteResp struct {
	// DeletedCount 实际删除的主单条数，无匹配记录时为 0
	DeletedCount int `json:"deleted_count"`
}

// ValidatePrepaidCurrency 比对请求币种与该二级账号所属 summary root 的币种是否一致。
// 比对基准依赖账号映射结果，故由写入域在映射完成后调用。
func ValidatePrepaidCurrency(reqCurrency, summaryRootCurrency enumor.CurrencyCode) error {
	if reqCurrency != summaryRootCurrency {
		return errf.Newf(errf.InvalidParameter,
			"invalid field currency: mismatch with root account summary, want: %s, got: %s",
			summaryRootCurrency, reqCurrency)
	}
	return nil
}

// ValidatePrepaidRootAccountCloudID 比对请求一级账号云上 ID 与映射出的二级账号所属一级账号是否一致。
// 比对基准依赖账号映射结果，故由写入域在映射完成后调用。
func ValidatePrepaidRootAccountCloudID(reqCloudID, mappedCloudID string) error {
	if reqCloudID != mappedCloudID {
		return errf.Newf(errf.InvalidParameter,
			"invalid field root_account_cloud_id: mismatch with main account, want: %s, got: %s",
			mappedCloudID, reqCloudID)
	}
	return nil
}
