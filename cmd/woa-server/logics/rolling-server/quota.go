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

package rollingserver

import (
	"errors"
	"fmt"
	"math"
	"time"

	rstypes "hcm/cmd/woa-server/types/rolling-server"
	"hcm/pkg/api/core"
	rsproto "hcm/pkg/api/data-service/rolling-server"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	rstablers "hcm/pkg/dal/table/rolling-server"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/maps"
	"hcm/pkg/tools/slice"
)

// createBaseQuotaConfigPeriodically 每个月1号生成当月的所有业务基础额度
// 协程启动时必定会尝试创建一次当月基础额度（幂等），并将下次执行的时间设置为下个月的1号0点
func (l *logics) createBaseQuotaConfigPeriodically(loc *time.Location) {
	rootKit := core.NewBackendKit()

	for {
		// 记录运行前的时间作为计算下次执行时间的基准，防止运行时超过0点错过1号
		now := time.Now()
		kt := rootKit.NewSubKit()
		nextMonthFirstDay := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

		// 只有 master 节点才执行
		if !l.sd.IsMaster() {
			logs.V(5).Infof("current node is not master, skip createBaseQuotaConfig at: %v", now)
		} else {
			quotaMonth := rstypes.QuotaMonth(fmt.Sprintf("%04d-%02d",
				nextMonthFirstDay.Year(), nextMonthFirstDay.Month()))
			if _, err := l.CreateBizQuotaConfigsForAllBiz(kt, quotaMonth); err != nil {
				logs.Errorf("create base quota configs for all biz failed, err: %v, rid: %s", err, kt.Rid)
			}
		}

		// 计算下个月1号0点的时间
		nextMonthFirstDay = nextMonthFirstDay.AddDate(0, 1, 0)
		// 等待直到下个月1号0点
		time.Sleep(time.Until(nextMonthFirstDay))
	}
}

// CreateBizQuotaConfigsForAllBiz for all biz.
func (l *logics) CreateBizQuotaConfigsForAllBiz(kt *kit.Kit, quotaMonth rstypes.QuotaMonth) (
	*rstypes.CreateBizQuotaConfigsResp, error) {

	// 获取所有业务ID
	allBizIDs, err := l.listIEGBizIDs(kt)
	if err != nil {
		logs.Errorf("list ieg biz ids failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 从配置表获取基础额度
	globalCfg, err := l.GetGlobalQuotaConfig(kt)
	if err != nil {
		logs.Errorf("get global quota config failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	req := &rstypes.CreateBizQuotaConfigsReq{
		BkBizIDs:   allBizIDs,
		QuotaMonth: quotaMonth,
		Quota:      cvt.PtrToVal(globalCfg.BizQuota),
	}
	resp, err := l.CreateBizQuotaConfigs(kt, req)
	if err != nil {
		logs.Errorf("create base quota configs failed, err: %v, req: %v, rid: %s", err, *req, kt.Rid)
		return nil, err
	}

	return resp, nil
}

// GetBkBizName get biz name.
func (l *logics) GetBkBizName(kt *kit.Kit, bkBizIDs []int64) (map[int64]string, error) {
	data := make(map[int64]string)

	bkBizIDs = slice.Unique(bkBizIDs)
	if len(bkBizIDs) == 0 {
		return data, nil
	}

	for _, split := range slice.Split(bkBizIDs, int(filter.DefaultMaxInLimit)) {
		rules := []cmdb.Rule{
			&cmdb.AtomRule{
				Field:    "bk_biz_id",
				Operator: cmdb.OperatorIn,
				Value:    split,
			},
		}
		expression := &cmdb.QueryFilter{Rule: &cmdb.CombinedRule{Condition: "AND", Rules: rules}}

		params := &cmdb.SearchBizParams{
			BizPropertyFilter: expression,
			Fields:            []string{"bk_biz_id", "bk_biz_name"},
		}
		resp, err := l.cmdbClient.SearchBusiness(kt, params)
		if err != nil {
			logs.Errorf("call cmdb search business api failed, err: %v, rid: %s", err, kt.Rid)
			return nil, fmt.Errorf("call cmdb search business api failed, err: %v", err)
		}

		for _, biz := range resp.Info {
			data[biz.BizID] = biz.BizName
		}
	}

	return data, nil
}

// GetGlobalQuotaConfig get global quota config
func (l *logics) GetGlobalQuotaConfig(kt *kit.Kit) (*rstablers.RollingGlobalConfigTable,
	error) {

	listRst := new(rsproto.RollingGlobalConfigListResult)
	listReq := &rsproto.RollingGlobalConfigListReq{
		Filter: tools.AllExpression(),
		Page:   core.NewDefaultBasePage(),
	}
	for {
		res, err := l.client.DataService().Global.RollingServer.ListGlobalConfig(kt, listReq)
		if err != nil {
			logs.Errorf("list global config failed, err: %v, req: %+v, rid: %s", err, *listReq, kt.Rid)
			return nil, err
		}

		listRst.Details = append(listRst.Details, res.Details...)

		if len(res.Details) < int(listReq.Page.Limit) {
			break
		}
		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	// 理论上global_config只会有一条，仅返回第一条
	if len(listRst.Details) != 1 {
		logs.Warnf("global quota config expected one, but list %d items, rid: %s", len(listRst.Details), kt.Rid)
		if len(listRst.Details) == 0 {
			return nil, errf.NewFromErr(errf.RecordNotFound, errors.New("global quota config not found"))
		}
	}

	return &listRst.Details[0], nil
}

// BatchCreateQuotaOffsetConfigAudit batch create quota offset config audit
func (l *logics) BatchCreateQuotaOffsetConfigAudit(kt *kit.Kit, effectIDs []string, quotaOffset int64) error {

	createAudit := make([]rsproto.QuotaOffsetAuditCreate, len(effectIDs))
	for idx, id := range effectIDs {
		createAudit[idx] = rsproto.QuotaOffsetAuditCreate{
			OffsetConfigID: id,
			Operator:       kt.User,
			QuotaOffset:    &quotaOffset,
			Rid:            kt.Rid,
			AppCode:        kt.AppCode,
		}
	}
	createReq := &rsproto.QuotaOffsetAuditCreateReq{
		QuotaOffsetsAudit: createAudit,
	}
	_, err := l.client.DataService().Global.RollingServer.BatchCreateQuotaOffsetAudit(kt, createReq)
	if err != nil {
		logs.Errorf("failed to batch create quota offset audit, err: %v, req: %v, rid: %s", err, *createReq, kt.Rid)
		return err
	}

	return nil
}

// AdjustQuotaOffsetConfigs adjust quota offset configs
func (l *logics) AdjustQuotaOffsetConfigs(kt *kit.Kit, bkBizIDs []int64, adjustMonth rstypes.AdjustMonthRange,
	quotaOffset int64) (*rstypes.AdjustQuotaOffsetsResp, error) {

	if len(bkBizIDs) == 0 {
		return nil, errors.New("failed to adjust quota offset configs, bk biz ids cannot be empty")
	}

	// 不管修改哪个月，均以当月的基础额度为基准（未来的基础额度还未生成），本次修改不可超过全局额度上限
	bizExceedQuotaMap, err := l.hasExceededQuotaLimit(kt, bkBizIDs, quotaOffset)
	if err != nil {
		logs.Errorf("failed to adjust quota offset configs, exceed quota limit, err: %v, "+
			"bk_biz_ids: %v, quota_offset: %d, rid: %s", err, bkBizIDs, quotaOffset, kt.Rid)
	}
	if len(bizExceedQuotaMap) > 0 {
		logs.Warnf("failed to adjust quota offset configs, exceed quota limit, exceed bizs: %v, rid: %s",
			bizExceedQuotaMap, kt.Rid)
		return nil, fmt.Errorf("business: %v has exceeded quota limit", maps.Keys(bizExceedQuotaMap))
	}

	startYear, startMonth, err := adjustMonth.Start.GetYearMonth()
	if err != nil {
		logs.Errorf("failed to adjust quota offset configs, cannot parse adjust_month.start, err: %v, "+
			"adjust_month: %v, rid: %s", err, adjustMonth, kt.Rid)
		return nil, err
	}
	endYear, endMonth, err := adjustMonth.End.GetYearMonth()
	if err != nil {
		logs.Errorf("failed to adjust quota offset configs, cannot parse adjust_month.end, err: %v, "+
			"adjust_month: %v, rid: %s", err, adjustMonth, kt.Rid)
		return nil, err
	}

	resp := new(rstypes.AdjustQuotaOffsetsResp)
	failedBizIDs := make(map[string][]int64)

	for year := startYear; year <= endYear; year++ {
		startMon := time.January
		endMon := time.December

		if year == startYear {
			startMon = time.Month(startMonth)
		}
		if year == endYear {
			endMon = time.Month(endMonth)
		}

		for month := startMon; month <= endMon; month++ {
			oneResp, failedIDs, err := l.AdjustQuotaOffsetConfigsForOneMonth(kt, bkBizIDs, year, int64(month),
				quotaOffset)
			if err != nil {
				date := fmt.Sprintf("%04d-%02d", year, month)
				failedBizIDs[date] = failedIDs

				logs.Warnf("failed to adjust quota offset config for one month, err: %v, year_month: %d-%d, rid: %s",
					err, year, month, kt.Rid)
				continue
			}

			resp.IDs = append(resp.IDs, oneResp.IDs...)
		}
	}

	if len(failedBizIDs) > 0 {
		errMsg := "failed to adjust quota offset.\n"
		for date, ids := range failedBizIDs {
			errMsg += fmt.Sprintf(" %s failed biz id: %v\n", date, ids)
		}
		return nil, errors.New(errMsg)
	}

	return resp, nil
}

func (l *logics) hasExceededQuotaLimit(kt *kit.Kit, bkBizIDs []int64, quotaOffset int64) (map[int64]int64, error) {
	// 未来的基础额度可能还没有创建，因此默认用本月的额度上限来做判断
	year := int64(time.Now().Year())
	month := int64(time.Now().Month())

	// 获取基础额度
	bkBizIDsQuota, _, err := l.getQuotaConfigIsExistBizIDs(kt, bkBizIDs, year, month)
	if err != nil {
		logs.Errorf("failed to get quota config where is exist, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 获取全局额度
	globalQuotaTable, err := l.GetGlobalQuotaConfig(kt)
	if err != nil {
		logs.Errorf("failed to get global quota config, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	globalQuota := cvt.PtrToVal(globalQuotaTable.GlobalQuota)

	// 记录超出额度的业务及溢出值
	bizExceedQuotaMap := make(map[int64]int64)
	for bkBizID, baseQuota := range bkBizIDsQuota {
		if baseQuota == constant.PlaceholderQuotaConfig {
			baseQuota = cvt.PtrToVal(globalQuotaTable.BizQuota)
		}

		if baseQuota+quotaOffset > globalQuota {
			bizExceedQuotaMap[bkBizID] = baseQuota + quotaOffset - globalQuota
		}
	}

	return bizExceedQuotaMap, nil
}

// AdjustQuotaOffsetConfigsForOneMonth adjust quota offset configs for one month
func (l *logics) AdjustQuotaOffsetConfigsForOneMonth(kt *kit.Kit, bkBizIDs []int64, year, month int64,
	quotaOffset int64) (*rstypes.AdjustQuotaOffsetsResp, []int64, error) {

	effectIDs := make([]string, 0)
	failedBizIDs := make([]int64, 0)

	// 查询当月有哪些业务已有调整记录
	bizIDExistOffsetIDMap, err := l.getQuotaOffsetIsExistBizIDs(kt, bkBizIDs, year, month)
	if err != nil {
		failedBizIDs = append(failedBizIDs, bkBizIDs...)
		logs.Errorf("failed to get quota offset config where is exist, err: %v, rid: %s", err, kt.Rid)
		return nil, failedBizIDs, err
	}

	if len(bizIDExistOffsetIDMap) > 0 {
		// 更新已有记录
		needUpdateRecordIDs := maps.Values(bizIDExistOffsetIDMap)
		updatedIDs, err := l.updateQuotaOffsetForMonth(kt, needUpdateRecordIDs, quotaOffset)
		if err != nil {
			failedBizIDs = append(failedBizIDs, maps.Keys(bizIDExistOffsetIDMap)...)
			logs.Errorf("failed to update quota offset for month, err: %v, rid: %s", err, kt.Rid)
		} else {
			effectIDs = append(effectIDs, updatedIDs...)
		}
	}

	needCreateBizIDs := slice.NotIn(maps.Keys(bizIDExistOffsetIDMap), bkBizIDs)
	if len(needCreateBizIDs) > 0 {
		// 对于当月无调整记录的业务，创建基础额度记录
		createBaseQuotaReq := &rstypes.CreateBizQuotaConfigsReq{
			BkBizIDs:   needCreateBizIDs,
			QuotaMonth: rstypes.QuotaMonth(fmt.Sprintf("%04d-%02d", year, month)),
			Quota:      constant.PlaceholderQuotaConfig,
		}
		_, err = l.CreateBizQuotaConfigs(kt, createBaseQuotaReq)
		if err != nil {
			failedBizIDs = append(failedBizIDs, needCreateBizIDs...)
			logs.Errorf("failed to create biz quota configs, err: %v, req: %v, rid: %s", err, *createBaseQuotaReq,
				kt.Rid)
			return nil, failedBizIDs, err
		}

		// 对于当月无调整记录的业务，创建新纪录
		createdIDs, err := l.createQuotaOffsetForMonth(kt, needCreateBizIDs, year, month, quotaOffset)
		if err != nil {
			failedBizIDs = append(failedBizIDs, needCreateBizIDs...)
			logs.Errorf("failed to create quota offset for month, err: %v, rid: %s", err, kt.Rid)
			return nil, failedBizIDs, err
		}
		effectIDs = append(effectIDs, createdIDs...)
	}

	if len(failedBizIDs) > 0 {
		return nil, failedBizIDs, errors.New("failed to adjust quota offset for one month")
	}

	return &rstypes.AdjustQuotaOffsetsResp{
		IDs: effectIDs,
	}, nil, nil
}

func (l *logics) updateQuotaOffsetForMonth(kt *kit.Kit, updateRecordIDs []string, quotaOffset int64) (
	[]string, error) {

	effectIDs := make([]string, 0)

	updateQuotaOffsets := make([]rsproto.RollingQuotaOffsetUpdateReq, 0, len(updateRecordIDs))
	for _, id := range updateRecordIDs {
		updateQuotaOffsets = append(updateQuotaOffsets, rsproto.RollingQuotaOffsetUpdateReq{
			ID:          id,
			QuotaOffset: &quotaOffset,
		})
		effectIDs = append(effectIDs, id)
	}
	updateReq := &rsproto.RollingQuotaOffsetBatchUpdateReq{
		QuotaOffsets: updateQuotaOffsets,
	}
	err := l.client.DataService().Global.RollingServer.BatchUpdateQuotaOffset(kt, updateReq)
	if err != nil {
		logs.Errorf("failed to batch update quota offset config, err: %v, req: %v, rid: %s", err, *updateReq, kt.Rid)
		return nil, err
	}

	return effectIDs, nil
}

func (l *logics) createQuotaOffsetForMonth(kt *kit.Kit, createBizIDs []int64, year, month int64, quotaOffset int64) (
	[]string, error) {

	createQuotaOffsets := make([]rsproto.RollingQuotaOffsetCreate, 0, len(createBizIDs))

	bizNamesMap, err := l.GetBkBizName(kt, createBizIDs)
	if err != nil {
		logs.Errorf("failed to get bk biz name, err: %v, ids: %v, rid: %s", err, createBizIDs, kt.Rid)
		return nil, err
	}

	for _, bkBizID := range createBizIDs {
		if _, ok := bizNamesMap[bkBizID]; !ok {
			logs.Errorf("cannot find bk biz name by bk biz id: %d, rid: %s", bkBizID, kt.Rid)
			return nil, fmt.Errorf("cannot find bk_biz_name by bk_biz_id: %d", bkBizID)
		}

		createQuotaOffsets = append(createQuotaOffsets, rsproto.RollingQuotaOffsetCreate{
			BkBizID:     bkBizID,
			BkBizName:   bizNamesMap[bkBizID],
			Year:        year,
			Month:       month,
			QuotaOffset: &quotaOffset,
		})
	}
	createReq := &rsproto.RollingQuotaOffsetCreateReq{
		QuotaOffsets: createQuotaOffsets,
	}
	createRst, err := l.client.DataService().Global.RollingServer.BatchCreateQuotaOffset(kt, createReq)
	if err != nil {
		logs.Errorf("failed to create quota offset configs, err: %v, req: %v, rid: %s", err, *createReq, kt.Rid)
		return nil, err
	}

	return createRst.IDs, nil
}

// getQuotaOffsetIsExistBizIDs return map: bkBizID => recordID
func (l *logics) getQuotaOffsetIsExistBizIDs(kt *kit.Kit, bkBizIDs []int64, year, month int64) (
	map[int64]string, error) {

	bizIDExistOffsetIDMap := make(map[int64]string)
	listReq := &rsproto.RollingQuotaOffsetListReq{
		Filter: &filter.Expression{
			Op: filter.And,
			Rules: []filter.RuleFactory{
				&filter.AtomRule{Field: "year", Op: filter.Equal.Factory(), Value: year},
				&filter.AtomRule{Field: "month", Op: filter.Equal.Factory(), Value: month},
				&filter.AtomRule{Field: "bk_biz_id", Op: filter.In.Factory(), Value: bkBizIDs},
			},
		},
		Fields: []string{"id", "bk_biz_id"},
		Page:   core.NewDefaultBasePage(),
	}

	for {
		result, err := l.client.DataService().Global.RollingServer.ListQuotaOffset(kt, listReq)
		if err != nil {
			logs.Errorf("failed to list rolling quota offset configs, err: %v, req: %v, rid: %s", err, *listReq, kt.Rid)
			return nil, err
		}

		for _, detail := range result.Details {
			bizIDExistOffsetIDMap[detail.BkBizID] = detail.ID
		}

		if len(result.Details) < int(listReq.Page.Limit) {
			break
		}

		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	return bizIDExistOffsetIDMap, nil
}

// CreateBizQuotaConfigs create biz quota configs
func (l *logics) CreateBizQuotaConfigs(kt *kit.Kit, req *rstypes.CreateBizQuotaConfigsReq) (
	*rstypes.CreateBizQuotaConfigsResp, error) {

	if len(req.BkBizIDs) == 0 {
		return nil, errors.New("failed to create biz quota configs, bk biz ids cannot be empty")
	}

	year, month, err := req.QuotaMonth.GetYearMonth()
	if err != nil {
		logs.Errorf("failed to create biz quota configs, cannot parse quota_month, err: %v, createReq: %v, rid: %s",
			err, *req, kt.Rid)
		return nil, err
	}

	existBkBizIDsQuota, needUpdateIDs, err := l.getQuotaConfigIsExistBizIDs(kt, req.BkBizIDs, year, month)
	if err != nil {
		logs.Errorf("failed to get quota config where is exist, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	existBkBizIDs := maps.Keys(existBkBizIDsQuota)

	effectIDs := make([]string, 0)

	needCreateBizIDs := slice.NotIn(existBkBizIDs, req.BkBizIDs)
	logs.Infof("create biz quota configs for all biz,"+
		" all biz number: %d, need create number: %d, need update number: %d, rid: %s",
		len(req.BkBizIDs), len(needCreateBizIDs), len(needUpdateIDs), kt.Rid) // need to create
	if len(needCreateBizIDs) > 0 {
		createIDs, err := l.batchCreateQuotaConfigs(kt, needCreateBizIDs, req.Quota, year, month)
		if err != nil {
			logs.Errorf("failed to batch create quota configs, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		effectIDs = append(effectIDs, createIDs...)
	}

	// need to update
	if len(needUpdateIDs) > 0 {
		updateIDs, err := l.batchUpdateQuotaConfigs(kt, needUpdateIDs, req.Quota)
		if err != nil {
			logs.Errorf("failed to batch update quota configs, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		effectIDs = append(effectIDs, updateIDs...)
	}

	return &rstypes.CreateBizQuotaConfigsResp{
		IDs: effectIDs,
	}, nil
}

func (l *logics) batchCreateQuotaConfigs(kt *kit.Kit, createBizIDs []int64, quota int64, year, month int64) (
	[]string, error) {

	effectIDs := make([]string, 0, len(createBizIDs))
	for _, split := range slice.Split(createBizIDs, constant.BatchOperationMaxLimit) {
		// 根据bk_biz_ids获取业务名称
		bizNameMap, err := l.GetBkBizName(kt, split)
		if err != nil {
			logs.Errorf("failed to get bk biz name, err: %v, ids: %v, rid: %s", err, split, kt.Rid)
			return nil, err
		}

		createQuotaConfigs := make([]rsproto.RollingQuotaConfigCreate, 0, len(split))
		for _, bkBizID := range split {
			if _, ok := bizNameMap[bkBizID]; !ok {
				logs.Errorf("cannot find bk biz name by bk biz id: %d, rid: %s", bkBizID, kt.Rid)
				return nil, fmt.Errorf("cannot find bk_biz_name by bk_biz_id: %d", bkBizID)
			}

			createQuotaConfigs = append(createQuotaConfigs, rsproto.RollingQuotaConfigCreate{
				BkBizID:   bkBizID,
				BkBizName: bizNameMap[bkBizID],
				Year:      year,
				Month:     month,
				Quota:     &quota,
			})
		}
		createReq := &rsproto.RollingQuotaConfigCreateReq{
			QuotaConfigs: createQuotaConfigs,
		}
		dataRst, err := l.client.DataService().Global.RollingServer.BatchCreateQuotaConfig(kt, createReq)
		if err != nil {
			logs.Errorf("failed to create biz quota configs, err: %v, req: %v, rid: %s", err, *createReq, kt.Rid)
			return nil, err
		}

		effectIDs = append(effectIDs, dataRst.IDs...)
	}

	return effectIDs, nil
}

func (l *logics) batchUpdateQuotaConfigs(kt *kit.Kit, updateRecordIDs []string, quota int64) ([]string, error) {
	effectIDs := make([]string, 0, len(updateRecordIDs))
	for _, split := range slice.Split(updateRecordIDs, constant.BatchOperationMaxLimit) {
		updateQuotaConfigs := make([]rsproto.RollingQuotaConfigUpdateReq, len(split))
		for idx, id := range split {
			updateQuotaConfigs[idx] = rsproto.RollingQuotaConfigUpdateReq{
				ID:    id,
				Quota: &quota,
			}
		}
		updateReq := &rsproto.RollingQuotaConfigBatchUpdateReq{
			QuotaConfigs: updateQuotaConfigs,
		}
		err := l.client.DataService().Global.RollingServer.BatchUpdateQuotaConfig(kt, updateReq)
		if err != nil {
			logs.Errorf("failed to update biz quota configs, err: %v, req: %v, rid: %s", err, *updateReq, kt.Rid)
			return nil, err
		}

		effectIDs = append(effectIDs, split...)
	}

	return effectIDs, nil
}

// getQuotaConfigIsExistBizIDs return: existBkBizIDs => quota; IDs where quota is -1; error
func (l *logics) getQuotaConfigIsExistBizIDs(kt *kit.Kit, bkBizIDs []int64, year, month int64) (
	map[int64]int64, []string, error) {

	existBkBizIDsQuota := make(map[int64]int64)
	needUpdateIDs := make([]string, 0, len(bkBizIDs))

	for _, split := range slice.Split(bkBizIDs, int(filter.DefaultMaxInLimit)) {
		listReq := &rsproto.RollingQuotaConfigListReq{
			Filter: &filter.Expression{
				Op: filter.And,
				Rules: []filter.RuleFactory{
					&filter.AtomRule{Field: "year", Op: filter.Equal.Factory(), Value: year},
					&filter.AtomRule{Field: "month", Op: filter.Equal.Factory(), Value: month},
					&filter.AtomRule{Field: "bk_biz_id", Op: filter.In.Factory(), Value: split},
				},
			},
			Fields: []string{"id", "bk_biz_id", "quota"},
			Page:   core.NewDefaultBasePage(),
		}

		result, err := l.client.DataService().Global.RollingServer.ListQuotaConfig(kt, listReq)
		if err != nil {
			logs.Errorf("list rolling quota configs failed, err: %v, req: %+v, rid: %s", err, *listReq, kt.Rid)
			return existBkBizIDsQuota, needUpdateIDs, err
		}

		for _, detail := range result.Details {
			quota := cvt.PtrToVal(detail.Quota)
			existBkBizIDsQuota[detail.BkBizID] = quota
			if quota == constant.PlaceholderQuotaConfig {
				needUpdateIDs = append(needUpdateIDs, detail.ID)
			}
		}
	}

	return existBkBizIDsQuota, needUpdateIDs, nil
}

// ListBizsWithExistQuota list biz with exist quota
func (l *logics) ListBizsWithExistQuota(kt *kit.Kit, req *rstypes.ListBizsWithExistQuotaReq) (
	*rstypes.ListBizsWithExistQuotaResp, error) {

	year, month, err := req.QuotaMonth.GetYearMonth()
	if err != nil {
		logs.Errorf("failed to list bizs with exist quota, cannot parse quota_month, err: %v, createReq: %v, rid: %s",
			err, *req, kt.Rid)
		return nil, err
	}

	listReq := &rsproto.RollingQuotaConfigListReq{
		Filter: &filter.Expression{
			Op: filter.And,
			Rules: []filter.RuleFactory{
				&filter.AtomRule{Field: "year", Op: filter.Equal.Factory(), Value: year},
				&filter.AtomRule{Field: "month", Op: filter.Equal.Factory(), Value: month}},
		},
		Page: core.NewDefaultBasePage(),
	}

	listResp := new(rsproto.RollingQuotaConfigListResult)
	for {
		oneResp, err := l.client.DataService().Global.RollingServer.ListQuotaConfig(kt, listReq)
		if err != nil {
			logs.Errorf("list bizs with exist quota failed, err: %v, req: %+v, rid: %s", err, *listReq, kt.Rid)
			return nil, err
		}

		listResp.Details = append(listResp.Details, oneResp.Details...)

		if len(oneResp.Details) < int(listReq.Page.Limit) {
			break
		}

		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	resp := new(rstypes.ListBizsWithExistQuotaResp)

	for _, one := range listResp.Details {
		// 预创建的记录排除，视为没有基础额度
		quota := cvt.PtrToVal(one.Quota)
		if quota == constant.PlaceholderQuotaConfig {
			continue
		}

		item := &rstypes.ListBizsWithExistQuotaItem{
			ID:        one.ID,
			BkBizID:   one.BkBizID,
			BkBizName: one.BkBizName,
			Quota:     quota,
		}
		resp.Details = append(resp.Details, item)
	}

	return resp, nil
}

// ListBizBizQuotaConfigs get biz's biz quota configs
func (l *logics) ListBizBizQuotaConfigs(kt *kit.Kit, bkBizID int64, req *rstypes.ListBizBizQuotaConfigsReq) (
	*rstypes.ListBizQuotaConfigsResp, error) {

	listReq := &rstypes.ListBizQuotaConfigsReq{
		BkBizIDs:   []int64{bkBizID},
		QuotaMonth: req.QuotaMonth,
		Page:       core.NewDefaultBasePage(),
	}

	listResp, err := l.ListBizQuotaConfigs(kt, listReq)
	if err != nil {
		logs.Errorf("failed to list biz quota configs, err: %v, req: %v, rid: %s", err, *listReq, kt.Rid)
	}

	if len(listResp.Details) == 0 {
		logs.Errorf("failed to list biz quota configs, biz quota config not found, "+
			"biz id: %d, quota month: %s, rid: %s", bkBizID, req.QuotaMonth, kt.Rid)
		return nil, errf.New(errf.InvalidParameter, "biz quota config not found")
	}

	return listResp, nil
}

// ListBizQuotaConfigs list biz quota configs
func (l *logics) ListBizQuotaConfigs(kt *kit.Kit, req *rstypes.ListBizQuotaConfigsReq) (
	*rstypes.ListBizQuotaConfigsResp, error) {

	listReq, err := genRollingQuotaConfigListOption(req)
	if err != nil {
		logs.Errorf("failed to generate rolling quota config list options, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	listRes, err := l.client.DataService().Global.RollingServer.ListQuotaConfigWithOffset(kt, listReq)
	if err != nil {
		logs.Errorf("failed to list rolling quota config with quota offset, err: %v, req: %v, rid: %s", err, *listReq,
			kt.Rid)
		return nil, err
	}

	resp := new(rstypes.ListBizQuotaConfigsResp)
	if req.Page.Count {
		resp.Count = listRes.Count
		return resp, nil
	}

	for _, one := range listRes.Details {
		item := &rstypes.ListBizQuotaConfigsItem{
			ID:        one.ID,
			Year:      one.Year,
			Month:     one.Month,
			BkBizID:   one.BkBizID,
			BkBizName: one.BkBizName,
		}
		// 基础额度为占位符时，给前端返回null
		if cvt.PtrToVal(one.Quota) != constant.PlaceholderQuotaConfig {
			item.Quota = one.Quota
		}

		offsetConfigID := one.OffsetConfigID
		offsetCreator := one.OffsetCreator
		offsetReviser := one.OffsetReviser
		offsetCreatedAt := one.OffsetCreatedAt
		offsetUpdatedAt := one.OffsetUpdatedAt
		// 没有进行过偏移调整的记录，偏移相关字段返回null
		if offsetConfigID == "" {
			resp.Details = append(resp.Details, item)
			continue
		}

		// 数据库中存储的偏移量需要转换为符号+绝对值再返回
		var adjustType enumor.QuotaOffsetAdjustType
		offset := cvt.PtrToVal(one.QuotaOffset)
		if offset < 0 {
			adjustType = enumor.DecreaseOffsetAdjustType
		} else {
			adjustType = enumor.IncreaseOffsetAdjustType
		}
		quotaOffset := uint64(math.Abs(float64(offset)))

		item.AdjustType = &adjustType
		item.QuotaOffset = &quotaOffset
		item.OffsetConfigID = &offsetConfigID
		item.Creator = &offsetCreator
		item.Reviser = &offsetReviser
		item.CreatedAt = &offsetCreatedAt
		item.UpdatedAt = &offsetUpdatedAt

		resp.Details = append(resp.Details, item)
	}

	return resp, nil
}

func genRollingQuotaConfigListOption(req *rstypes.ListBizQuotaConfigsReq) (
	*rsproto.RollingQuotaConfigListWithOffsetReq, error) {

	listOption := new(rsproto.RollingQuotaConfigListWithOffsetReq)
	// 当查询参数包含offset表的字段时，不展示offset为null的数据
	displayNullOffset := true

	if len(req.BkBizIDs) > 0 {
		listOption.BkBizIDs = req.BkBizIDs
	}
	if len(req.Revisers) > 0 {
		listOption.Revisers = req.Revisers
		displayNullOffset = false
	}
	if len(req.QuotaMonth) > 0 {
		year, month, err := req.QuotaMonth.GetYearMonth()
		if err != nil {
			return nil, err
		}
		listOption.Year = year
		listOption.Month = month
	}

	rules := make([]filter.RuleFactory, 0)
	if len(req.AdjustType) > 0 {
		displayNullOffset = false
		typeExp, err := genAdjustTypeExpression(req.AdjustType)
		if err != nil {
			return nil, err
		}
		rules = append(rules, typeExp)
	}

	listOption.ExtraOpt = rsproto.RollingQuotaConfigListReq{
		Filter: &filter.Expression{
			Op:    filter.And,
			Rules: rules,
		},
		Page: req.Page,
	}
	listOption.DisplayNullOffset = &displayNullOffset

	return listOption, nil
}

func genAdjustTypeExpression(adjustTypes []enumor.QuotaOffsetAdjustType) (*filter.Expression, error) {
	typeRules := make([]filter.RuleFactory, 0)
	for _, t := range adjustTypes {
		var rule filter.AtomRule
		switch t {
		case enumor.IncreaseOffsetAdjustType:
			rule = filter.AtomRule{Field: "quota_offset", Op: filter.GreaterThan.Factory(),
				Value: 0}
		case enumor.DecreaseOffsetAdjustType:
			rule = filter.AtomRule{Field: "quota_offset", Op: filter.LessThan.Factory(),
				Value: 0}
		default:
			return nil, fmt.Errorf("unsupported adjust type: %s", t)
		}
		typeRules = append(typeRules, &filter.Expression{
			Op:    filter.And,
			Rules: []filter.RuleFactory{rule},
		})
	}

	return &filter.Expression{
		Op:    filter.Or,
		Rules: typeRules,
	}, nil
}

// ListQuotaOffsetAdjustRecords list quota offset adjust records by offset audit
func (l *logics) ListQuotaOffsetAdjustRecords(kt *kit.Kit, offsetConfigIDs []string, page *core.BasePage) (
	*rstypes.ListQuotaOffsetsAdjustRecordsResp, error) {

	listReq := &rsproto.QuotaOffsetAuditListReq{
		Filter: tools.ContainersExpression("offset_config_id", offsetConfigIDs),
		Page:   page,
	}
	listRst, err := l.client.DataService().Global.RollingServer.ListQuotaOffsetAudit(kt, listReq)
	if err != nil {
		logs.Errorf("list quota offsets audit failed, err: %v, rid: %s", err, kt.Rid)
	}

	if page.Count {
		return &rstypes.ListQuotaOffsetsAdjustRecordsResp{
			Count: listRst.Count,
		}, nil
	}

	result := new(rstypes.ListQuotaOffsetsAdjustRecordsResp)
	for _, listItem := range listRst.Details {
		respItem := rstypes.ListQuotaOffsetsAdjustRecordsItem{
			ID:             listItem.ID,
			OffsetConfigID: listItem.OffsetConfigID,
			Operator:       listItem.Operator,
			CreatedAt:      listItem.CreatedAt,
		}

		// 数据库中存储的偏移量需要转换为符号+绝对值再返回
		offset := cvt.PtrToVal(listItem.QuotaOffset)
		if offset < 0 {
			respItem.AdjustType = enumor.DecreaseOffsetAdjustType
		} else {
			respItem.AdjustType = enumor.IncreaseOffsetAdjustType
		}
		respItem.QuotaOffset = uint64(math.Abs(float64(offset)))

		result.Details = append(result.Details, &respItem)
	}

	return result, nil
}

func (l *logics) isBizCurMonthHavingQuota(kt *kit.Kit, bizID int64, appliedCount uint) (bool, string, error) {
	now := time.Now()
	summaryReq := &rstypes.CpuCoreSummaryReq{
		BkBizIDs: []int64{bizID},
		RollingServerDateRange: rstypes.RollingServerDateRange{
			Start: rstypes.RollingServerDateTimeItem{
				Year: now.Year(), Month: int(now.Month()), Day: constant.FirstDay,
			},
			End: rstypes.RollingServerDateTimeItem{
				Year: now.Year(), Month: int(now.Month()), Day: now.Day(),
			},
		},
	}
	summary, err := l.GetCpuCoreSummary(kt, summaryReq)
	if err != nil {
		logs.Errorf("get cpu core summary failed, err: %v, req: %+v, rid: %s", err, *summaryReq, kt.Rid)
		return false, "", err
	}

	bizQuota, err := l.getBizCurMonthQuota(kt, bizID)
	if err != nil {
		logs.Errorf("get biz current month quota failed, err: %v, bizID: %d, rid: %s", err, bizID, kt.Rid)
		return false, "", err
	}

	if int64(summary.SumDeliveredCore)+int64(appliedCount) > bizQuota {
		reason := fmt.Sprintf("业务(%d)滚服项目当前已交付%d核心，本次申请%d核心，超过本业务限制:%d核心", bizID,
			summary.SumDeliveredCore, appliedCount, bizQuota)
		return false, reason, nil
	}

	return true, "", nil
}

func (l *logics) isSystemCurMonthHavingQuota(kt *kit.Kit, appliedCount uint) (bool, string, error) {
	now := time.Now()
	summaryReq := &rstypes.CpuCoreSummaryReq{
		RollingServerDateRange: rstypes.RollingServerDateRange{
			Start: rstypes.RollingServerDateTimeItem{
				Year: now.Year(), Month: int(now.Month()), Day: constant.FirstDay,
			},
			End: rstypes.RollingServerDateTimeItem{
				Year: now.Year(), Month: int(now.Month()), Day: now.Day(),
			},
		},
	}
	summary, err := l.GetCpuCoreSummary(kt, summaryReq)
	if err != nil {
		logs.Errorf("get cpu core summary failed, err: %v, req: %+v, rid: %s", err, *summaryReq, kt.Rid)
		return false, "", err
	}

	listGlobalConfigReq := &rsproto.RollingGlobalConfigListReq{
		Filter: tools.AllExpression(),
		Fields: []string{"global_quota"},
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}
	config, err := l.client.DataService().Global.RollingServer.ListGlobalConfig(kt, listGlobalConfigReq)
	if err != nil {
		logs.Errorf("list rolling server global config failed, err: %v, rid: %s", err, kt.Rid)
		return false, "", err
	}
	if len(config.Details) == 0 {
		logs.Errorf("can not get rolling server global config, rid: %s", kt.Rid)
		return false, "", errors.New("can not get rolling server global config")
	}

	if int64(summary.SumDeliveredCore)+int64(appliedCount) > *config.Details[0].GlobalQuota {
		reason := fmt.Sprintf("当月滚服项目已交付%d核心，本次申请%d核心，超过本月总额度限制:%d核心",
			summary.SumDeliveredCore,
			appliedCount, *config.Details[0].GlobalQuota)
		return false, reason, nil
	}

	return true, "", nil
}

func (l *logics) getBizCurMonthQuota(kt *kit.Kit, bizID int64) (int64, error) {
	now := time.Now()
	rules := []filter.RuleFactory{
		&filter.AtomRule{Field: "bk_biz_id", Op: filter.Equal.Factory(), Value: bizID},
		&filter.AtomRule{Field: "year", Op: filter.Equal.Factory(), Value: now.Year()},
		&filter.AtomRule{Field: "month", Op: filter.Equal.Factory(), Value: now.Month()},
	}

	listQuotaReq := &rsproto.RollingQuotaConfigListReq{
		Filter: &filter.Expression{Op: filter.And, Rules: rules},
		Fields: []string{"quota"},
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}

	quotaConfig, err := l.client.DataService().Global.RollingServer.ListQuotaConfig(kt, listQuotaReq)
	if err != nil {
		logs.Errorf("list rolling server quota config failed, err: %v, bizID: %d, rid: %s", err, bizID, kt.Rid)
		return 0, err
	}
	if len(quotaConfig.Details) == 0 {
		logs.Errorf("can not get biz rolling server quota config, bizID: %d, rid: %s", bizID, kt.Rid)
		return 0, fmt.Errorf("can not get biz rolling server quota config, bizID: %d", bizID)
	}
	quota := *quotaConfig.Details[0].Quota

	listQuotaOffsetReq := &rsproto.RollingQuotaOffsetListReq{
		Filter: &filter.Expression{Op: filter.And, Rules: rules},
		Fields: []string{"quota_offset"},
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}
	quotaOffsetConfig, err := l.client.DataService().Global.RollingServer.ListQuotaOffset(kt, listQuotaOffsetReq)
	if err != nil {
		logs.Errorf("list rolling server quota offset config failed, err: %v, bizID: %d, rid: %s", err, bizID, kt.Rid)
		return 0, err
	}

	if len(quotaOffsetConfig.Details) != 0 {
		quota += *quotaOffsetConfig.Details[0].QuotaOffset
	}

	return quota, nil
}
