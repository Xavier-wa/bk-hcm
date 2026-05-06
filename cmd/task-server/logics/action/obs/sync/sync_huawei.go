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
	"errors"
	"fmt"
	"time"

	actcli "hcm/cmd/task-server/logics/action/cli"
	"hcm/pkg/api/core"
	accountsetcore "hcm/pkg/api/core/account-set"
	dataas "hcm/pkg/api/data-service/account-set"
	databill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	tableobs "hcm/pkg/dal/table/obs"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/retry"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

func (act SyncAction) doBatchSyncHuaweiBillitem(kt *kit.Kit,
	mainAccount *dataas.MainAccountGetResult[accountsetcore.HuaWeiMainAccountExtension],
	syncOpt *SyncOption) error {

	policy := retry.NewRetryPolicy(uint(defaultRetryTimes), [2]uint{50, 500})
	for start := syncOpt.Start; start < syncOpt.Start+syncOpt.Limit; start = start + uint64(core.DefaultMaxPageLimit) {
		err := policy.BaseExec(kt, func() error {
			return act.doSyncHuaweiBillItem(kt, mainAccount, syncOpt, start, uint64(core.DefaultMaxPageLimit))
		})
		if err != nil {
			logs.Errorf("fail to sync huawei bill %+v, start %d, limit: %d, err: %v, rid: %s",
				syncOpt, start, uint64(core.DefaultMaxPageLimit), err, kt.Rid)
			return err
		}
	}
	return nil
}

func (act SyncAction) doSyncHuaweiBillItem(kt *kit.Kit,
	mainAccount *dataas.MainAccountGetResult[accountsetcore.HuaWeiMainAccountExtension],
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
	result, err := actcli.GetDataService().HuaWei.Bill.ListBillItem(kt, listReq)
	if err != nil {
		logs.Errorf("list huawei bill item by option %+v failed, err: %s, rid: %s", syncOpt, err.Error(), kt.Rid)
		return err
	}

	if len(result.Details) == 0 {
		logs.Infof("get no bill item for main_account_id %s %d-%d %d-%d",
			syncOpt.MainAccountID, syncOpt.BillYear, syncOpt.BillMonth, start, limit)
		return nil
	}

	regionCityMap, err := loadRegionCityMap(kt, syncOpt.Vendor)
	if err != nil {
		logs.Errorf("load region city map failed, err: %v, vendor: %s, rid: %s", err, syncOpt.Vendor, kt.Rid)
		return fmt.Errorf("load region city map failed, err: %v", err)
	}

	hwGpuPrefixes, err := loadHuaweiGpuPrefixes(kt)
	if err != nil {
		logs.Errorf("load huawei gpu prefixes failed, err: %v, vendor: %s, rid: %s", err, syncOpt.Vendor, kt.Rid)
		return fmt.Errorf("load huawei gpu prefixes failed, err: %v", err)
	}

	// 清理特定的obs数据，此处防止之前有可能插入事务失败导致的脏数据
	setIndex := fmt.Sprintf("%s-%s-%d-%d-%d-%d",
		syncOpt.Vendor, syncOpt.MainAccountID, syncOpt.BillYear, syncOpt.BillMonth, start, limit)
	delExpressions := []*filter.AtomRule{
		tools.RuleEqual("set_index", setIndex),
	}
	deleteFilter := tools.ExpressionAnd(delExpressions...)
	_, err = actcli.GetObsDaoSet().Txn().AutoTxn(kt, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		if err := actcli.GetObsDaoSet().OBSBillItemHuawei().DeleteWithTx(
			kt, txn, deleteFilter, uint64(core.DefaultMaxPageLimit)); err != nil {
			logs.Errorf("delete huawei obs bill item of set %s failed, err: %v, filter %+v, rid: %s",
				setIndex, err, deleteFilter, kt.Rid)
			return nil, err
		}
		logs.Infof("delete previous obs data for %s successfully", setIndex)
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("delete obs bill txn failed, err %s", err.Error())
	}

	// 进行插入
	finalItems, err := act.convertHuaweiBill(kt, syncOpt, result, setIndex, mainAccount, regionCityMap, hwGpuPrefixes)
	if err != nil {
		logs.Errorf("convert obs huawei bill failed, err: %s, setIndex: %s, rid: %s", err.Error(), setIndex, kt.Rid)
		return err
	}
	_, err = actcli.GetObsDaoSet().Txn().AutoTxn(kt, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		if _, err := actcli.GetObsDaoSet().OBSBillItemHuawei().CreateWithTx(kt, txn, finalItems); err != nil {
			logs.Errorf("create huawei obs bill item of set %s failed, err: %v, rid: %s", setIndex, err, kt.Rid)
			return nil, err
		}
		logs.Infof("create obs huawei bill for %s successfully", setIndex)
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("create obs bill txn failed, err %s", err.Error())
	}

	return nil
}

func (act SyncAction) getHuaweiMainAccount(kt *kit.Kit, mainAccountID string) (
	*dataas.MainAccountGetResult[accountsetcore.HuaWeiMainAccountExtension], error) {

	mainAccount, err := actcli.GetDataService().HuaWei.MainAccount.Get(kt, mainAccountID)
	if err != nil {
		logs.Errorf("get main account by id %s failed, err: %s, rid: %s", mainAccountID, err.Error(), kt.Rid)
		return nil, err
	}
	return mainAccount, nil
}

func (act SyncAction) convertHuaweiBill(kt *kit.Kit, syncOpt *SyncOption, result *databill.HuaweiBillItemListResult,
	setIndex string, mainAccount *dataas.MainAccountGetResult[accountsetcore.HuaWeiMainAccountExtension],
	regionCityMap map[string]int32, hwGpuPrefixes []string) ([]*tableobs.OBSBillItemHuawei, error) {

	if result == nil || len(result.Details) == 0 {
		return nil, errors.New("nil bill item result or empty bill result details")
	}
	item := result.Details[0]
	currency := item.Currency
	if len(currency) == 0 {
		logs.Errorf("empty currency for item %v, rid: %s", item, kt.Rid)
		return nil, fmt.Errorf("empty currency for item %v", item)
	}
	// 获取当月平均汇率
	exchangeRate, err := getExchangeRate(kt, currency, enumor.CurrencyRMB, syncOpt.BillYear, syncOpt.BillMonth)
	if err != nil {
		logs.Errorf("failed to get exchange rate, err: %s, syncOpt: %+v, rid: %s", err.Error(), syncOpt, kt.Rid)
		return nil, fmt.Errorf("failed to get exchange rate, err: %s", err.Error())
	}
	floatRate, _ := exchangeRate.Float64()
	yearM := syncOpt.BillYear*100 + syncOpt.BillMonth

	// OBS 要求数据，决定汇率
	var accountType = "HW国际区"
	isChina := mainAccount.Site == enumor.MainAccountChinaSite
	if isChina {
		accountType = "国内账单"
	}

	var retList []*tableobs.OBSBillItemHuawei
	for _, item := range result.Details {
		record := item.Extension.ResFeeRecordV2

		fetchTime, err := time.Parse(constant.TimeStdFormat, item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed, to parse time %s, err %s", item.CreatedAt, err.Error())
		}

		region := converter.PtrToVal[string](record.Region)
		productSpecDesc := converter.PtrToVal[string](record.ProductSpecDesc)
		cityID := lookupCityID(kt, regionCityMap, region, isChina)
		isGPU := isHuaweiGPU(productSpecDesc, hwGpuPrefixes)

		newItem := &tableobs.OBSBillItemHuawei{
			SetIndex:                  setIndex,
			Vendor:                    string(syncOpt.Vendor),
			MainAccountID:             syncOpt.MainAccountID,
			BillYear:                  int64(syncOpt.BillYear),
			BillMonth:                 int64(syncOpt.BillMonth),
			EffectiveTime:             converter.PtrToVal[string](record.EffectiveTime),
			ExpireTime:                converter.PtrToVal[string](record.ExpireTime),
			ProductID:                 converter.PtrToVal[string](record.ProductId),
			ProductName:               converter.PtrToVal[string](record.ProductName),
			OrderID:                   converter.PtrToVal[string](record.OrderId),
			Amount:                    fmt.Sprintf("%f", converter.PtrToVal[float64](record.Amount)),
			MeasureID:                 fmt.Sprintf("%d", converter.PtrToVal[int32](record.MeasureId)),
			UsageType:                 converter.PtrToVal[string](record.UsageType),
			Usages:                    fmt.Sprintf("%f", converter.PtrToVal[float64](record.Usage)),
			UsageMeasureID:            fmt.Sprintf("%d", converter.PtrToVal[int32](record.UsageMeasureId)),
			FreeResourceUsage:         fmt.Sprintf("%f", converter.PtrToVal[float64](record.FreeResourceUsage)),
			FreeResourceMeasureID:     fmt.Sprintf("%d", converter.PtrToVal[int32](record.FreeResourceMeasureId)),
			CloudServiceType:          converter.PtrToVal[string](record.CloudServiceType),
			Region:                    region,
			ResourceType:              converter.PtrToVal[string](record.ResourceType),
			ChargeMode:                converter.PtrToVal[string](record.ChargeMode),
			ResourceTag:               converter.PtrToVal[string](record.ResourceTag),
			ResourceName:              converter.PtrToVal[string](record.ResourceName),
			ResourceID:                converter.PtrToVal[string](record.ResourceId),
			BillType:                  fmt.Sprintf("%d", converter.PtrToVal[int32](record.BillType)),
			EnterpriseProjectID:       converter.PtrToVal[string](record.EnterpriseProjectId),
			PeriodType:                converter.PtrToVal[string](record.PeriodType),
			Spot:                      "",
			RiUsage:                   fmt.Sprintf("%f", converter.PtrToVal[float64](record.RiUsage)),
			RiUsageMeasureID:          fmt.Sprintf("%d", converter.PtrToVal[int32](record.RiUsageMeasureId)),
			OfficialAmount:            fmt.Sprintf("%f", converter.PtrToVal[float64](record.OfficialAmount)),
			DiscountAmount:            fmt.Sprintf("%f", converter.PtrToVal[float64](record.DiscountAmount)),
			CashAmount:                fmt.Sprintf("%f", converter.PtrToVal[float64](record.CashAmount)),
			CreditAmount:              fmt.Sprintf("%f", converter.PtrToVal[float64](record.CreditAmount)),
			CouponAmount:              fmt.Sprintf("%f", converter.PtrToVal[float64](record.CouponAmount)),
			FlexipurchaseCouponAmount: fmt.Sprintf("%f", converter.PtrToVal[float64](record.FlexipurchaseCouponAmount)),
			StoredCardAmount:          fmt.Sprintf("%f", converter.PtrToVal[float64](record.StoredCardAmount)),
			BonusAmount:               fmt.Sprintf("%f", converter.PtrToVal[float64](record.BonusAmount)),
			DebtAmount:                fmt.Sprintf("%f", converter.PtrToVal[float64](record.DebtAmount)),
			AdjustmentAmount:          fmt.Sprintf("%f", converter.PtrToVal[float64](record.AdjustmentAmount)),
			SpecSize:                  fmt.Sprintf("%f", converter.PtrToVal[float64](record.SpecSize)),
			SpecSizeMeasureID:         fmt.Sprintf("%d", converter.PtrToVal[int32](record.SpecSizeMeasureId)),
			AccountName:               mainAccount.Extension.CloudMainAccountName,
			AccountType:               accountType,
			ProductId:                 int32(mainAccount.OpProductID),
			YearMonth:                 int32(yearM),
			FetchTime:                 fetchTime.Format(constant.DateTimeLayout),
			TotalCount:                int32(len(result.Details)),
			Rate:                      floatRate,
			RealCost:                  &types.Decimal{Decimal: item.Cost.Mul(decimal.NewFromFloat(floatRate))},
			CityId:                    cityID,
			ResClassId:                enumor.GetOBSResClassID(syncOpt.Vendor, isGPU),
		}
		retList = append(retList, newItem)
	}
	return retList, nil
}
