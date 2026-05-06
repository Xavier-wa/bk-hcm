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
	"fmt"
	"time"

	actcli "hcm/cmd/task-server/logics/action/cli"
	"hcm/pkg/api/core"
	accountsetcore "hcm/pkg/api/core/account-set"
	asproto "hcm/pkg/api/data-service/account-set"
	databill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	tableobs "hcm/pkg/dal/table/obs"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/retry"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

func (act SyncAction) getGcpMainAccount(kt *kit.Kit, mainAccountID string) (
	*asproto.MainAccountGetResult[accountsetcore.GcpMainAccountExtension], error) {

	mainAccount, err := actcli.GetDataService().Gcp.MainAccount.Get(kt, mainAccountID)
	if err != nil {
		logs.Warnf("get gcp main account by id %s failed, err %s, rid: %s", mainAccountID, err.Error(), kt.Rid)
		return nil, err
	}
	return mainAccount, nil
}

func (act SyncAction) doBatchSyncGcpBillitem(kt *kit.Kit,
	mainAccount *asproto.MainAccountGetResult[accountsetcore.GcpMainAccountExtension],
	syncOpt *SyncOption) error {
	rty := retry.NewRetryPolicy(uint(defaultRetryTimes), [2]uint{10, 500})
	for start := syncOpt.Start; start < syncOpt.Start+syncOpt.Limit; start = start + uint64(core.DefaultMaxPageLimit) {
		err := rty.BaseExec(kt, func() error {
			return act.doSyncGcpBillItem(kt, mainAccount, syncOpt, start, uint64(core.DefaultMaxPageLimit))
		})
		if err != nil {
			logs.Warnf("do sync aws bill failed,  err: %v, opt: %v, start %d, limit %d, rid %s", err, syncOpt, start,
				uint64(core.DefaultMaxPageLimit), kt.Rid)
			return err
		}
	}
	return nil
}

func (act SyncAction) doSyncGcpBillItem(kt *kit.Kit,
	mainAccount *asproto.MainAccountGetResult[accountsetcore.GcpMainAccountExtension],
	syncOpt *SyncOption, start, limit uint64) error {

	flt := tools.ExpressionAnd(
		tools.RuleEqual("root_account_id", syncOpt.RootAccountID),
		tools.RuleEqual("main_account_id", syncOpt.MainAccountID),
	)
	comOpt := &databill.ItemCommonOpt{
		Vendor: syncOpt.Vendor,
		Year:   syncOpt.BillYear,
		Month:  syncOpt.BillMonth,
	}
	listReq := &databill.BillItemListReq{
		ItemCommonOpt: comOpt,
		ListReq:       &core.ListReq{Filter: flt, Page: &core.BasePage{Start: uint32(start), Limit: uint(limit)}},
	}
	// 获取分账后的bill item
	result, err := actcli.GetDataService().Gcp.Bill.ListBillItem(kt, listReq)
	if err != nil {
		logs.Warnf("list gcp bill item by option %v failed, err %s, rid: %s", syncOpt, err.Error(), kt.Rid)
		return err
	}

	if len(result.Details) == 0 {
		logs.Infof("get no bill item for main_account_id %s %d-%d %d-%d, rid: %s",
			syncOpt.MainAccountID, syncOpt.BillYear, syncOpt.BillMonth, start, limit, kt.Rid)
		return nil
	}

	regionCityMap, err := loadRegionCityMap(kt, syncOpt.Vendor)
	if err != nil {
		logs.Errorf("load region city map failed, err: %v, vendor: %s, rid: %s", err, syncOpt.Vendor, kt.Rid)
		return fmt.Errorf("load region city map failed, err: %v", err)
	}

	// 清理特定的obs数据，此处防止之前有可能插入事务失败导致的脏数据
	setIndex := fmt.Sprintf("%s-%s-%d-%d-%d-%d",
		syncOpt.Vendor, syncOpt.MainAccountID, syncOpt.BillYear, syncOpt.BillMonth, start, limit)
	delExpressions := []*filter.AtomRule{
		tools.RuleEqual("set_index", setIndex),
	}
	deleteFilter := tools.ExpressionAnd(delExpressions...)
	_, err = actcli.GetObsDaoSet().Txn().AutoTxn(kt, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		if err := actcli.GetObsDaoSet().OBSBillItemGcp().DeleteWithTx(
			kt, txn, deleteFilter, uint64(core.DefaultMaxPageLimit)); err != nil {
			logs.Warnf("delete gcp obs bill item by filter %v failed, err %s, rid: %s",
				deleteFilter, err.Error(), kt.Rid)
			return nil, err
		}
		logs.Infof("delete previous obs data for %s successfully, rid: %s", setIndex, kt.Rid)
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("delete obs bill txn failed, err %s", err.Error())
	}

	// 进行插入
	finalItems, err := act.convertGcpBill(kt, syncOpt, result, setIndex, mainAccount, regionCityMap)
	if err != nil {
		logs.Warnf("convert obs gcp bill failed, err %s, rid: %s", err.Error(), kt.Rid)
		return err
	}
	_, err = actcli.GetObsDaoSet().Txn().AutoTxn(kt, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		if _, err := actcli.GetObsDaoSet().OBSBillItemGcp().CreateWithTx(kt, txn, finalItems); err != nil {
			logs.Errorf("create gcp obs bill item failed of set %s, err: %v, rid: %s", setIndex, err, kt.Rid)
			return nil, err
		}
		logs.Infof("create obs gcp bill for %s successfully", setIndex)
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("create obs bill txn failed, err %s", err.Error())
	}

	return nil
}

func (act SyncAction) convertGcpBill(kt *kit.Kit, syncOpt *SyncOption, result *databill.GcpBillItemListResult,
	setIndex string, mainAccount *asproto.MainAccountGetResult[accountsetcore.GcpMainAccountExtension],
	regionCityMap map[string]int32) ([]*tableobs.OBSBillItemGcp, error) {

	yearM := syncOpt.BillYear*100 + syncOpt.BillMonth
	item := result.Details[0]
	currency := item.Currency
	if len(currency) == 0 {
		logs.Warnf("empty currency for item %v, rid: %s", item, kt.Rid)
		return nil, fmt.Errorf("empty currency for item %v, rid: %s", item, kt.Rid)
	}

	// 获取当月平均汇率
	exchangeRate, err := getExchangeRate(kt, currency, enumor.CurrencyRMB, syncOpt.BillYear, syncOpt.BillMonth)
	if err != nil {
		logs.Warnf("failed to get exchange rate, err %s, rid %s", err.Error(), kt.Rid)
		return nil, fmt.Errorf("failed to get exchange rate, err %s, rid %s", err.Error(), kt.Rid)
	}
	floatRate, _ := exchangeRate.Float64()

	isChina := mainAccount.Site == enumor.MainAccountChinaSite

	var retList = make([]*tableobs.OBSBillItemGcp, 0, len(result.Details))

	for _, item := range result.Details {
		record := item.Extension
		fetchTime, err := time.Parse("2006-01-02T15:04:05Z07:00", item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed, to parse time %s, err %s", item.CreatedAt, err.Error())
		}

		var region, skuDescription string
		if record != nil && record.GcpRawBillItem != nil {
			region = converter.PtrToVal(record.Region)
			skuDescription = converter.PtrToVal(record.SkuDescription)
		}

		cityID := lookupCityID(kt, regionCityMap, region, isChina)
		isGPU := isGcpGPU(skuDescription, item.HcProductName)

		newItem := &tableobs.OBSBillItemGcp{
			SetIndex:               setIndex,
			MainAccountID:          syncOpt.MainAccountID,
			BillYear:               int64(syncOpt.BillYear),
			BillMonth:              int64(syncOpt.BillMonth),
			Vendor:                 string(syncOpt.Vendor),
			YearMonth:              int32(yearM),
			Rate:                   floatRate,
			Cost:                   item.Cost.InexactFloat64(),
			ProductId:              int32(mainAccount.OpProductID),
			Currency:               string(currency),
			CurrencyConversionRate: floatRate,
			UsageAmount:            item.ResAmount.InexactFloat64(),
			UsageUnit:              item.ResAmountUnit,
			FetchTime:              fetchTime.Format("2006-01-02 15:04:05"),
			RealCost:               item.Cost.Mul(decimal.NewFromFloat(floatRate)).InexactFloat64(),
			CityId:                 cityID,
			ResClassId:             enumor.GetOBSResClassID(syncOpt.Vendor, isGPU),
		}
		if record != nil && record.GcpRawBillItem != nil {
			newItem.BillingAccountId = record.BillingAccountID
			newItem.ServiceId = converter.PtrToVal(record.ServiceID)
			newItem.ServiceDescription = converter.PtrToVal(record.ServiceDescription)
			newItem.SkuId = converter.PtrToVal(record.SkuID)
			newItem.SkuDescription = skuDescription
			newItem.UsageStartTime = converter.PtrToVal(record.UsageStartTime)
			newItem.UsageEndTime = converter.PtrToVal(record.UsageEndTime)
			newItem.ProjectId = converter.PtrToVal(record.ProjectID)
			newItem.ProjectName = converter.PtrToVal(record.ProjectName)
			newItem.CreditsAmount = converter.PtrToVal(record.CreditsAmount)
			newItem.ExportTime = converter.PtrToVal(record.UsageEndTime)
			newItem.Location = converter.PtrToVal(record.Location)
			newItem.Country = converter.PtrToVal(record.Country)
			newItem.Region = region
			newItem.Zone = converter.PtrToVal(record.Zone)
			newItem.DispatchProjectId = converter.PtrToVal(record.ProjectID)
		} else {
			// 某些账单是HCM生成的（如支持费），没有云上原始账单，也没有生成对应的Extension字段。
			// 这里使用生成的产品名称填充，保证用户有一个可读的账单
			// 在hcm侧会将ai条目的HcProductName字段拼接特定前缀，在将数据创建到obs时，需要去掉。
			newItem.ServiceDescription = constant.TrimBillItemAIPrefix(item.HcProductName)
			newItem.ServiceId = item.HcProductCode
		}
		retList = append(retList, newItem)
	}
	return retList, nil
}
