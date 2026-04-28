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

	actcli "hcm/cmd/task-server/logics/action/cli"
	"hcm/pkg/api/core"
	accountsetcore "hcm/pkg/api/core/account-set"
	asproto "hcm/pkg/api/data-service/account-set"
	databill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	tableobs "hcm/pkg/dal/table/obs"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/retry"

	"github.com/jmoiron/sqlx"
)

func (act SyncAction) getAwsMainAccount(kt *kit.Kit, mainAccountID string) (
	*asproto.MainAccountGetResult[accountsetcore.AwsMainAccountExtension], error) {

	mainAccount, err := actcli.GetDataService().Aws.MainAccount.Get(kt, mainAccountID)
	if err != nil {
		logs.Errorf("get aws main account by id %s failed, err: %s, rid: %s", mainAccountID, err.Error(), kt.Rid)
		return nil, err
	}
	return mainAccount, nil
}
func (act SyncAction) doBatchSyncAwsBillitem(kt *kit.Kit,
	mainAccount *asproto.MainAccountGetResult[accountsetcore.AwsMainAccountExtension],
	syncOpt *SyncOption) error {
	rty := retry.NewRetryPolicy(uint(defaultRetryTimes), [2]uint{10, 500})
	for start := syncOpt.Start; start < syncOpt.Start+syncOpt.Limit; start = start + uint64(core.DefaultMaxPageLimit) {
		err := rty.BaseExec(kt, func() error {
			return act.doSyncAwsBillItem(kt, mainAccount, syncOpt, start, uint64(core.DefaultMaxPageLimit))
		})
		if err != nil {
			logs.Errorf("do sync aws bill failed,  err: %v, opt: %v, start %d, limit %d, rid %s",
				err, syncOpt, start, uint64(core.DefaultMaxPageLimit), kt.Rid)
			return err
		}
	}
	return nil
}

func (act SyncAction) doSyncAwsBillItem(kt *kit.Kit,
	mainAccount *asproto.MainAccountGetResult[accountsetcore.AwsMainAccountExtension],
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
	result, err := actcli.GetDataService().Aws.Bill.ListBillItem(kt, listReq)
	if err != nil {
		logs.Errorf("list aws bill item by option %v failed, err: %s, rid: %s", syncOpt, err.Error(), kt.Rid)
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

	awsGpuSet, err := loadAwsGpuInstanceTypes(kt)
	if err != nil {
		logs.Errorf("load aws gpu instance types failed, err: %v, vendor: %s, rid: %s", err, syncOpt.Vendor, kt.Rid)
		return fmt.Errorf("load aws gpu instance types failed, err: %v", err)
	}

	// 清理特定的obs数据，此处防止之前有可能插入事务失败导致的脏数据
	setIndex := fmt.Sprintf("%s-%s-%d-%d-%d-%d",
		syncOpt.Vendor, syncOpt.MainAccountID, syncOpt.BillYear, syncOpt.BillMonth, start, limit)
	delExpressions := []*filter.AtomRule{
		tools.RuleEqual("set_index", setIndex),
	}
	deleteFilter := tools.ExpressionAnd(delExpressions...)
	_, err = actcli.GetObsDaoSet().Txn().AutoTxn(kt, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		if err := actcli.GetObsDaoSet().OBSBillItemAws().DeleteWithTx(
			kt, txn, deleteFilter, uint64(core.DefaultMaxPageLimit)); err != nil {
			logs.Errorf("delete aws obs bill item by filter %v failed, err: %s, setIndex:%s, rid: %s",
				deleteFilter, err.Error(), setIndex, kt.Rid)
			return nil, err
		}
		logs.Infof("delete previous obs data for %s successfully, rid: %s", setIndex, kt.Rid)
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("delete obs bill txn failed, err %s", err.Error())
	}

	// 进行插入
	finalItems, err := act.convertAwsBill(kt, syncOpt, result, setIndex, mainAccount, regionCityMap, awsGpuSet)
	if err != nil {
		logs.Warnf("convert obs aws bill failed, err %s, rid: %s", err.Error(), kt.Rid)
		return err
	}
	_, err = actcli.GetObsDaoSet().Txn().AutoTxn(kt, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		if _, err := actcli.GetObsDaoSet().OBSBillItemAws().CreateWithTx(kt, txn, finalItems); err != nil {
			logs.Errorf("create aws obs bill item failed of set %s, err: %s, rid: %s", setIndex, err, kt.Rid)
			return nil, err
		}
		logs.Infof("create obs aws bill for %s successfully, rid: %s", setIndex, kt.Rid)
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("create obs bill txn failed, err: %s", err.Error())
	}

	return nil
}

func (act SyncAction) convertAwsBill(kt *kit.Kit, syncOpt *SyncOption, result *databill.AwsBillItemListResult,
	setIndex string, mainAccount *asproto.MainAccountGetResult[accountsetcore.AwsMainAccountExtension],
	regionCityMap map[string]int32, awsGpuSet map[string]struct{}) ([]*tableobs.OBSBillItemAws, error) {

	yearM := syncOpt.BillYear*100 + syncOpt.BillMonth
	item := result.Details[0]
	currency := item.Currency
	if len(currency) == 0 {
		logs.Errorf("empty currency for item %v, rid: %s", item, kt.Rid)
		return nil, fmt.Errorf("empty currency for item %v", item)
	}
	// 获取当月平均汇率
	exchangeRate, err := getExchangeRate(kt, currency, enumor.CurrencyRMB, syncOpt.BillYear, syncOpt.BillMonth)
	if err != nil {
		logs.Warnf("failed to get exchange rate, err: %v, rid: %s", err, kt.Rid)
		return nil, fmt.Errorf("failed to get exchange rate, err: %v", err)
	}
	floatRate, _ := exchangeRate.Float64()

	// OBS 要求数据格式 1 国内 2 国际
	var regionCode = int32(2)
	isChina := mainAccount.Site == enumor.MainAccountChinaSite
	if isChina {
		regionCode = 1
	}

	var retList = make([]*tableobs.OBSBillItemAws, 0, len(result.Details))
	for _, item := range result.Details {
		record := item.Extension

		cityID := lookupCityID(kt, regionCityMap, record.ProductRegion, isChina)
		isGPU := isAwsGPU(record.LineItemProductCode, record.ProductInstanceType, item.HcProductName, awsGpuSet)

		newItem := &tableobs.OBSBillItemAws{
			SetIndex:      setIndex,
			MainAccountID: syncOpt.MainAccountID,
			BillYear:      int64(syncOpt.BillYear),
			BillMonth:     int64(syncOpt.BillMonth),
			Vendor:        string(syncOpt.Vendor),
			YearMonth:     int32(yearM),
			Rate:          floatRate,
			// OBS要求，aws账单独立配置，Cost保存原始数据即可
			Cost:              &types.Decimal{Decimal: item.Cost},
			ProductID:         int32(mainAccount.OpProductID),
			LinkedAccountName: mainAccount.Extension.CloudMainAccountName,
			Region:            regionCode,
			// OBS要求，存入标识
			Memo: "ieg上报",
			// OBS 要求，OBS外币金额写入line_item_unblended_cost字段中
			LineItemUnblendedCost: item.Cost.String(),
			CityId:                cityID,
			ResClassId:            enumor.GetOBSResClassID(syncOpt.Vendor, isGPU),

			BillPayerAccountID:                     record.BillPayerAccountId,
			LineItemUsageAccountID:                 record.LineItemUsageAccountId,
			BillInvoiceID:                          record.BillInvoiceId,
			BillBillingEntity:                      record.BillBillingEntity,
			LineItemProductCode:                    record.LineItemProductCode,
			ProductProductFamily:                   record.ProductProductFamily,
			ProductProductName:                     record.ProductProductName,
			LineItemUsageType:                      record.LineItemUsageType,
			ProductInstanceType:                    record.ProductInstanceType,
			ProductRegion:                          record.ProductRegion,
			ProductLocation:                        record.ProductLocation,
			LineItemResourceID:                     record.LineItemResourceId,
			PricingTerm:                            record.PricingTerm,
			LineItemLineItemType:                   record.LineItemLineItemType,
			LineItemLineItemDescription:            record.LineItemLineItemDescription,
			LineItemUsageStartDate:                 record.LineItemUsageStartDate,
			LineItemUsageEndDate:                   record.LineItemUsageEndDate,
			LineItemUsageAmount:                    record.LineItemUsageAmount,
			PricingUnit:                            record.PricingUnit,
			PricingPublicOnDemandRate:              record.PricingPublicOnDemandRate,
			LineItemUnblendedRate:                  record.LineItemUnblendedRate,
			LineItemNetUnblendedRate:               record.LineItemNetUnblendedRate,
			SavingsPlanSavingsPlanRate:             record.SavingsPlanSavingsPlanRate,
			PricingPublicOnDemandCost:              record.PricingPublicOnDemandCost,
			LineItemNetUnblendedCost:               record.LineItemNetUnblendedCost,
			SavingsPlanSavingsPlanEffectiveCost:    record.SavingsPlanNetSavingsPlanEffectiveCost,
			SavingsPlanSavingsPlanNetEffectiveCost: record.SavingsPlanSavingsPlanEffectiveCost,
			ReservationEffectiveCost:               record.ReservationNetEffectiveCost,
			ReservationNetEffectiveCost:            record.ReservationEffectiveCost,
			LineItemCurrencyCode:                   record.LineItemCurrencyCode,
			LineItemOperation:                      record.LineItemOperation,
			DiscountPrivateRateDiscount:            record.DiscountPrivateRateDiscount,
			DiscountEDPDiscount:                    record.DiscountEdpDiscount,
		}
		retList = append(retList, newItem)
	}
	return retList, nil
}
