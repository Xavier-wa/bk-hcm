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

package table

import (
	"fmt"
	"strconv"

	"hcm/cmd/woa-server/logics/config"
	dissolveconfig "hcm/cmd/woa-server/logics/dissolve/config"
	logicshost "hcm/cmd/woa-server/logics/dissolve/host"
	model "hcm/cmd/woa-server/model/task"
	"hcm/cmd/woa-server/types/dissolve"
	taskTypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	dsproto "hcm/pkg/api/data-service/dissolve"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	hostdefine "hcm/pkg/dal/table/dissolve/host"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/thirdparty/es"
	"hcm/pkg/tools/concurrence"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/maps"
	"hcm/pkg/tools/slice"
)

// Table provides interface for operations of dissolve table.
type Table interface {
	ListResDissolveTable(kt *kit.Kit, req *dissolve.ResDissolveReq) ([]dissolve.BizDetail, error)
	ListHostDetail(kt *kit.Kit, req *dissolve.HostDetailListReq) (*dissolve.HostDetailListResult, error)
	ListExportHostDetail(kt *kit.Kit, req *dissolve.HostDetailExportListReq) (*dissolve.HostDetailListResult, error)
	ListBizCpuCoreSummary(kt *kit.Kit, bizIDs []int64) (map[int64]dissolve.CpuCoreSummary, error)
	ListExpectAbolishTime(kt *kit.Kit, req *dissolve.ExpectAbolishTimeListReq) ([]string, error)
}

type logics struct {
	recycledHost       logicshost.RecycledHost
	dissolveConfig     dissolveconfig.Config
	configLogics       config.Logics
	esCli              *es.EsCli
	originDate         string
	excludedProjectIDs []int
	cliSet             *client.ClientSet
}

// New create resource dissolve table logics.
func New(recycledHost logicshost.RecycledHost, dissolveConfig dissolveconfig.Config, configLogics config.Logics,
	esCli *es.EsCli, originDate string, excludedProjectIDs []int, cliSet *client.ClientSet) Table {

	return &logics{
		recycledHost:       recycledHost,
		dissolveConfig:     dissolveConfig,
		configLogics:       configLogics,
		esCli:              esCli,
		originDate:         originDate,
		excludedProjectIDs: excludedProjectIDs,
		cliSet:             cliSet,
	}
}

// baseRules 查询本地裁撤主机表的恒定条件：未忽略 + 排除指定项目
func (l *logics) baseRules() []*filter.AtomRule {
	rules := []*filter.AtomRule{tools.RuleEqual("is_ignore", false)}
	if len(l.excludedProjectIDs) != 0 {
		rules = append(rules, tools.RuleNotIn("project_id", l.excludedProjectIDs))
	}

	return rules
}

// listHostConcurrencyLimit 并发拉取本地裁撤主机分页的最大协程数
const listHostConcurrencyLimit = 10

// listLocalHosts 先取总数，再并发分页拉取本地裁撤主机表数据
func (l *logics) listLocalHosts(kt *kit.Kit, f *filter.Expression, fields []string) (
	[]hostdefine.RecycleHostTable, error) {

	total, err := l.countLocalHosts(kt, f)
	if err != nil {
		logs.Errorf("count local recycle host failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	if total == 0 {
		return make([]hostdefine.RecycleHostTable, 0), nil
	}

	limit := core.DefaultMaxPageLimit
	starts := make([]uint32, 0)
	for start := uint64(0); start < total; start += uint64(limit) {
		starts = append(starts, uint32(start))
	}

	pageResult, err := concurrence.BaseExecWithResult(listHostConcurrencyLimit, starts,
		func(start uint32) ([]hostdefine.RecycleHostTable, error) {
			opt := &types.ListOption{
				Filter: f,
				Fields: fields,
				Page:   &core.BasePage{Start: start, Limit: limit, Sort: "id"},
			}
			list, err := l.recycledHost.List(kt, opt)
			if err != nil {
				logs.Errorf("list local recycle host failed, err: %v, rid: %s", err, kt.Rid)
				return nil, err
			}
			return list.Details, nil
		})
	if err != nil {
		logs.Errorf("list local recycle host failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	result := make([]hostdefine.RecycleHostTable, 0, total)
	for _, subResult := range pageResult {
		result = append(result, subResult...)
	}

	return result, nil
}

// countLocalHosts 统计本地裁撤主机表命中条数
func (l *logics) countLocalHosts(kt *kit.Kit, f *filter.Expression) (uint64, error) {
	opt := &types.ListOption{Filter: f, Page: core.NewCountPage()}
	list, err := l.recycledHost.List(kt, opt)
	if err != nil {
		logs.Errorf("count local recycle host failed, err: %v, rid: %s", err, kt.Rid)
		return 0, err
	}

	return list.Count, nil
}

// overviewStat 业务聚合中间统计
type overviewStat struct {
	originHostCount  int64
	originCpuCore    int64
	currentHostCount int64
	currentCpuCore   int64
}

// ListResDissolveTable 总览：仅查本地表按业务聚合，附一行合计
func (l *logics) ListResDissolveTable(kt *kit.Kit, req *dissolve.ResDissolveReq) ([]dissolve.BizDetail, error) {
	f := l.buildOverviewFilter(req)

	hosts, err := l.listLocalHosts(kt, f, []string{"bk_biz_id", "cpu_core", "abolish_phase"})
	if err != nil {
		return nil, err
	}

	bizMap := aggregateOverview(hosts)
	if len(bizMap) == 0 {
		return make([]dissolve.BizDetail, 0), nil
	}

	deliveredMap, err := l.listBizDeliveredCpuCore(kt, maps.Keys(bizMap))
	if err != nil {
		logs.Errorf("list biz delivered cpu core failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return buildOverviewResult(bizMap, deliveredMap), nil
}

// ListExpectAbolishTime 查询裁撤截止时间列表：套用查询侧恒定条件，经 data-service 去重升序返回
func (l *logics) ListExpectAbolishTime(kt *kit.Kit, req *dissolve.ExpectAbolishTimeListReq) ([]string, error) {
	rules := convertAtomRulesToFactories(l.baseRules())
	if req.Filter != nil {
		rules = append(rules, req.Filter)
	}
	expr := &filter.Expression{Op: filter.And, Rules: rules}

	resp, err := l.cliSet.DataService().TCloudZiyan.Dissolve.ListRecycleHostExpectAbolishTime(kt,
		&dsproto.ListRecycleHostExpectAbolishTimeReq{Filter: expr})
	if err != nil {
		logs.Errorf("list expect abolish time failed, err: %v, req: %+v, rid: %s", err, req, kt.Rid)
		return nil, err
	}

	return resp.ExpectAbolishTimes, nil
}

// buildOverviewFilter 构造总览查询条件，恒附 ignore=false + 排除指定项目
func (l *logics) buildOverviewFilter(req *dissolve.ResDissolveReq) *filter.Expression {
	rules := convertAtomRulesToFactories(l.baseRules())
	if len(req.ProjectIDs) != 0 {
		rules = append(rules, tools.RuleIn("project_id", req.ProjectIDs))
	}
	if len(req.GroupIDs) != 0 {
		rules = append(rules, buildGroupIDInRule(req.GroupIDs))
	}
	if len(req.BizIDs) != 0 {
		rules = append(rules, tools.RuleIn("bk_biz_id", req.BizIDs))
	}
	if len(req.Operators) != 0 {
		rules = append(rules, tools.RuleJsonOverlaps("operators", req.Operators))
	}
	if len(req.Regions) != 0 {
		rules = append(rules, tools.RuleIn("region", req.Regions))
	}
	if len(req.ExpectAbolishTimes) != 0 {
		rules = append(rules, tools.RuleIn("expect_abolish_time", req.ExpectAbolishTimes))
	}

	return &filter.Expression{Op: filter.And, Rules: rules}
}

// convertAtomRulesToFactories 将 AtomRule 切片转为 RuleFactory 切片，便于与嵌套 Expression 组合
func convertAtomRulesToFactories(rules []*filter.AtomRule) []filter.RuleFactory {
	factories := make([]filter.RuleFactory, 0, len(rules))
	for _, rule := range rules {
		factories = append(factories, rule)
	}

	return factories
}

// buildGroupIDInRule 组织数量可能超过 DB 的 IN 限制，按 DefaultMaxInLimit 切分为多个 IN 并用 OR 组合成单条规则
func buildGroupIDInRule(groupIDs []int64) filter.RuleFactory {
	inRules := make([]filter.RuleFactory, 0)
	for _, batch := range slice.Split(groupIDs, int(filter.DefaultMaxInLimit)) {
		inRules = append(inRules, tools.RuleIn("group_id", batch))
	}

	return &filter.Expression{Op: filter.Or, Rules: inRules}
}

// aggregateOverview 按业务聚合：原始=全部命中，当前=未裁撤(abolish_phase != complete)
func aggregateOverview(hosts []hostdefine.RecycleHostTable) map[int64]*overviewStat {
	bizMap := make(map[int64]*overviewStat)
	for _, h := range hosts {
		bizID := cvt.PtrToVal(h.BkBizID)
		cpuCore := int64(cvt.PtrToVal(h.CPUCore))

		stat, ok := bizMap[bizID]
		if !ok {
			stat = &overviewStat{}
			bizMap[bizID] = stat
		}

		stat.originHostCount++
		stat.originCpuCore += cpuCore

		if cvt.PtrToVal(h.AbolishPhase) != enumor.Complete {
			stat.currentHostCount++
			stat.currentCpuCore += cpuCore
		}
	}

	return bizMap
}

// ListHostDetail 明细：分页查本地表，四态↔两态转换
func (l *logics) ListHostDetail(kt *kit.Kit, req *dissolve.HostDetailListReq) (*dissolve.HostDetailListResult, error) {
	f := buildDetailFilter(l.baseRules(), req)
	opt := &types.ListOption{Filter: f, Page: req.Page}

	list, err := l.recycledHost.List(kt, opt)
	if err != nil {
		logs.Errorf("list host detail failed, err: %v, req: %+v, rid: %s", err, req, kt.Rid)
		return nil, err
	}

	if req.Page.Count {
		return &dissolve.HostDetailListResult{Count: int64(list.Count)}, nil
	}

	return &dissolve.HostDetailListResult{Details: toHostDetails(list.Details)}, nil
}

// ListExportHostDetail 查询导出的裁撤主机明细：复用明细查询，可选按 ES 快照补充扩展字段
func (l *logics) ListExportHostDetail(kt *kit.Kit, req *dissolve.HostDetailExportListReq) (
	*dissolve.HostDetailListResult, error) {

	listReq := &dissolve.HostDetailListReq{
		BizIDs:             req.BizIDs,
		ProjectIDs:         req.ProjectIDs,
		GroupIDs:           req.GroupIDs,
		Operators:          req.Operators,
		Modules:            req.Modules,
		InnerIPs:           req.InnerIPs,
		AssetIDs:           req.AssetIDs,
		Status:             req.Status,
		ExpectAbolishTimes: req.ExpectAbolishTimes,
		Page:               req.Page,
	}

	if req.Page.Count {
		return l.ListHostDetail(kt, listReq)
	}

	res, err := l.listExportHostDetail(kt, listReq)
	if err != nil {
		logs.Errorf("list export host detail failed, err: %v, listReq: %+v, rid: %s", err, listReq, kt.Rid)
		return nil, err
	}

	if req.SnapshotDate != "" {
		if res.Details, err = l.enrichExtensionFromES(kt, res.Details, req.SnapshotDate); err != nil {
			logs.Errorf("enrich extension from es failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
	}

	return res, nil
}

func (l *logics) listExportHostDetail(kt *kit.Kit, listReq *dissolve.HostDetailListReq) (
	*dissolve.HostDetailListResult, error) {

	page := listReq.Page
	result := &dissolve.HostDetailListResult{Details: make([]dissolve.HostDetail, 0, page.Limit)}
	for uint(len(result.Details)) < page.Limit {
		batchLimit := page.Limit - uint(len(result.Details))
		if batchLimit > core.DefaultMaxPageLimit {
			batchLimit = core.DefaultMaxPageLimit
		}

		listReq.Page = &core.BasePage{
			Start: page.Start + uint32(len(result.Details)),
			Limit: batchLimit,
			Sort:  page.Sort,
			Order: page.Order,
		}
		batch, err := l.ListHostDetail(kt, listReq)
		if err != nil {
			logs.Errorf("list export host detail failed, err: %v, listReq: %+v, rid: %s", err, listReq, kt.Rid)
			return nil, err
		}

		result.Details = append(result.Details, batch.Details...)
		if uint(len(batch.Details)) < batchLimit {
			break
		}
	}

	return result, nil
}

// buildDetailFilter 构造明细查询条件，含四态过滤
func buildDetailFilter(base []*filter.AtomRule, req *dissolve.HostDetailListReq) *filter.Expression {
	rules := convertAtomRulesToFactories(base)
	if len(req.BizIDs) != 0 {
		rules = append(rules, tools.RuleIn("bk_biz_id", req.BizIDs))
	}
	if len(req.ProjectIDs) != 0 {
		rules = append(rules, tools.RuleIn("project_id", req.ProjectIDs))
	}
	if len(req.GroupIDs) != 0 {
		rules = append(rules, buildGroupIDInRule(req.GroupIDs))
	}
	if len(req.Operators) != 0 {
		rules = append(rules, tools.RuleJsonOverlaps("operators", req.Operators))
	}
	if len(req.Modules) != 0 {
		rules = append(rules, tools.RuleIn("module", req.Modules))
	}
	if len(req.InnerIPs) != 0 {
		rules = append(rules, tools.RuleIn("inner_ip", req.InnerIPs))
	}
	if len(req.AssetIDs) != 0 {
		rules = append(rules, tools.RuleIn("asset_id", req.AssetIDs))
	}
	if phases := req.Status.ToAbolishPhases(); len(phases) != 0 {
		rules = append(rules, tools.RuleIn("abolish_phase", phases))
	}
	if len(req.ExpectAbolishTimes) != 0 {
		rules = append(rules, tools.RuleIn("expect_abolish_time", req.ExpectAbolishTimes))
	}

	return &filter.Expression{Op: filter.And, Rules: rules}
}

// toHostDetails 本地表数据转明细，含四态归并两态
func toHostDetails(hosts []hostdefine.RecycleHostTable) []dissolve.HostDetail {
	result := make([]dissolve.HostDetail, 0, len(hosts))
	for _, h := range hosts {
		result = append(result, dissolve.HostDetail{
			ID:                h.ID,
			AssetID:           cvt.PtrToVal(h.AssetID),
			InnerIP:           cvt.PtrToVal(h.InnerIP),
			DeviceType:        cvt.PtrToVal(h.DeviceType),
			Module:            cvt.PtrToVal(h.Module),
			Status:            dissolve.FromAbolishPhase(cvt.PtrToVal(h.AbolishPhase)),
			ProjectID:         cvt.PtrToVal(h.ProjectID),
			ProjectName:       cvt.PtrToVal(h.ProjectName),
			Region:            cvt.PtrToVal(h.Region),
			BkBizID:           cvt.PtrToVal(h.BkBizID),
			GroupID:           cvt.PtrToVal(h.GroupID),
			Operators:         []string(h.Operators),
			CPUCore:           cvt.PtrToVal(h.CPUCore),
			ExpectAbolishTime: cvt.PtrToVal(h.ExpectAbolishTime),
		})
	}

	return result
}

// enrichExtensionFromES 按 ES 快照补充明细扩展字段
func (l *logics) enrichExtensionFromES(kt *kit.Kit, details []dissolve.HostDetail, date string) (
	[]dissolve.HostDetail, error) {

	if l.esCli == nil || len(details) == 0 {
		return details, nil
	}

	assetIDs := make([]string, 0, len(details))
	for _, d := range details {
		assetIDs = append(assetIDs, d.AssetID)
	}

	index := es.GetIndex(date)
	fields := esExtensionFields()

	hostMap := make(map[string]es.Host)
	for _, batch := range slice.Split(assetIDs, int(core.DefaultMaxPageLimit)) {
		cond := map[string][]interface{}{es.AssetID: cvt.StrSliceToInterfaceSlice(batch)}
		hosts, err := l.esCli.SearchWithCond(kt.Ctx, cond, index, 0, len(batch), es.AssetID, fields)
		if err != nil {
			logs.Errorf("search es snapshot failed, err: %v, index: %s, rid: %s", err, index, kt.Rid)
			return nil, err
		}
		for _, h := range hosts {
			hostMap[h.ServerAssetID] = h
		}
	}

	for i := range details {
		if h, ok := hostMap[details[i].AssetID]; ok {
			details[i].Extension = buildHostExtension(h)
		}
	}

	return details, nil
}

// esExtensionFields 返回补充扩展字段需从 ES 查询的字段列表
func esExtensionFields() []string {
	return []string{es.AssetID, "outer_ip", "device_type", "module_name", "idc_unit_name",
		"sfw_name_version", "go_up_date", "raid_name", "logic_area",
		"device_layer", "cpu_score", "mem_score", "inner_net_traffic_score", "disk_io_score", "disk_util_score",
		"is_pass", "mem4linux", "inner_net_traffic", "outer_net_traffic", "disk_io", "disk_util", "disk_total",
		"group_name", "center"}
}

// buildHostExtension 将 ES 主机快照映射为明细扩展字段
func buildHostExtension(h es.Host) *dissolve.HostExtension {
	// ES 的 is_pass 为字符串，脏值降级为 false
	isPass, _ := strconv.ParseBool(h.IsPass)
	return &dissolve.HostExtension{
		OuterIP:              h.OuterIP,
		DeviceType:           h.DeviceType,
		ModuleName:           h.ModuleName,
		IdcUnitName:          h.IdcUnitName,
		SfwNameVersion:       h.SfwNameVersion,
		GoUpDate:             h.GoUpDate,
		RaidName:             h.RaidName,
		LogicArea:            h.LogicArea,
		DeviceLayer:          h.DeviceLayer,
		CPUScore:             h.CPUScore,
		MemScore:             h.MemScore,
		InnerNetTrafficScore: h.InnerNetTrafficScore,
		DiskIoScore:          h.DiskIoScore,
		DiskUtilScore:        h.DiskUtilScore,
		IsPass:               isPass,
		Mem4linux:            h.Mem4linux,
		InnerNetTraffic:      h.InnerNetTraffic,
		OuterNetTraffic:      h.OuterNetTraffic,
		DiskIo:               h.DiskIo,
		DiskUtil:             h.DiskUtil,
		DiskTotal:            h.DiskTotal,
		GroupName:            h.GroupName,
		Center:               h.Center,
	}
}

// buildOverviewResult 组装各业务统计行并追加合计行（合计行 bk_biz_id 为 -1）
func buildOverviewResult(bizMap map[int64]*overviewStat, deliveredMap map[int64]int64) []dissolve.BizDetail {
	result := make([]dissolve.BizDetail, 0, len(bizMap)+1)
	total := dissolve.BizDetail{BkBizID: -1}

	for bizID, stat := range bizMap {
		delivered := deliveredMap[bizID]
		result = append(result, dissolve.BizDetail{
			BkBizID:          bizID,
			OriginHostCount:  stat.originHostCount,
			OriginCpuCore:    stat.originCpuCore,
			CurrentHostCount: stat.currentHostCount,
			CurrentCpuCore:   stat.currentCpuCore,
			DeliveredCpuCore: delivered,
			Progress:         getProgress(stat.originHostCount, stat.currentHostCount),
		})

		total.OriginHostCount += stat.originHostCount
		total.OriginCpuCore += stat.originCpuCore
		total.CurrentHostCount += stat.currentHostCount
		total.CurrentCpuCore += stat.currentCpuCore
		total.DeliveredCpuCore += delivered
	}

	total.Progress = getProgress(total.OriginHostCount, total.CurrentHostCount)
	result = append(result, total)

	return result
}

func getProgress(origin, current int64) string {
	if origin == 0 {
		return ""
	}

	return fmt.Sprintf("%.2f%%", (float64(origin-current))/float64(origin)*100)
}

// ListBizCpuCoreSummary list business cpu core summary.
func (l *logics) ListBizCpuCoreSummary(kt *kit.Kit, bizIDs []int64) (map[int64]dissolve.CpuCoreSummary, error) {
	// 1. 初始化数据，确保每个业务都有值
	bizCpuCoreSummaryMap := make(map[int64]dissolve.CpuCoreSummary, len(bizIDs))
	for _, bizID := range bizIDs {
		bizCpuCoreSummaryMap[bizID] = dissolve.CpuCoreSummary{}
	}

	// 2. 获取业务需要裁撤的总核数
	bizTotalCpuCoreMap, err := l.listBizTotalCpuCore(kt, bizIDs)
	if err != nil {
		logs.Errorf("list biz total cpu core failed, err: %v, bizIDs: %v, rid: %s", err, bizIDs, kt.Rid)
		return nil, err
	}

	// 3. 获取业务机房裁撤已交付的总核数
	bizDeliveredCpuCoreMap, err := l.listBizDeliveredCpuCore(kt, bizIDs)
	if err != nil {
		logs.Errorf("list biz delivered cpu core failed, err: %v, bizIDs: %v, rid: %s", err, bizIDs, kt.Rid)
		return nil, err
	}

	// 4. 获取统计机房裁撤主机的开始时间
	hostApplyTime, err := l.dissolveConfig.GetDissolveHostApplyTime(kt)
	if err != nil {
		logs.Errorf("get host apply time failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 5. 获取配额系数
	quotaCoefficient, err := l.dissolveConfig.GetQuotaCoefficient(kt)
	if err != nil {
		logs.Errorf("get quota coefficient failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 6. 获取偏移配置
	quotaOffsets, err := l.dissolveConfig.GetQuotaOffsetsMap(kt)
	if err != nil {
		logs.Errorf("get quota offsets failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 7. 组装业务数据
	for _, bizID := range bizIDs {
		quotaOffset := l.calcQuotaOffset(bizID, quotaOffsets)
		availableQuota := l.calcAvailableQuota(bizTotalCpuCoreMap[bizID], quotaCoefficient, quotaOffset,
			bizDeliveredCpuCoreMap[bizID])

		bizCpuCoreSummaryMap[bizID] = dissolve.CpuCoreSummary{
			TotalCore:        bizTotalCpuCoreMap[bizID],
			DeliveredCore:    bizDeliveredCpuCoreMap[bizID],
			HostApplyTime:    cvt.PtrToVal(hostApplyTime),
			QuotaCoefficient: quotaCoefficient,
			QuotaOffset:      quotaOffset,
			AvailableQuota:   availableQuota,
		}
	}

	return bizCpuCoreSummaryMap, nil
}

// calcQuotaOffset 计算业务偏移额度
func (l *logics) calcQuotaOffset(bizID int64, quotaOffsets map[int64]dissolve.QuotaOffsetItem) int64 {
	offsetItem, exists := quotaOffsets[bizID]
	if !exists {
		return 0
	}
	if offsetItem.Type == enumor.DissolveQuotaOffsetTypeIncrease {
		return offsetItem.Offset
	}
	return -offsetItem.Offset
}

// calcAvailableQuota 计算可申请额度: max(0, 裁撤原始核数 × 配额系数/100 + 业务偏移额度 - 已交付核数)
func (l *logics) calcAvailableQuota(totalCpuCore int64, quotaCoefficient float64,
	quotaOffset, deliveredCpuCore int64) int64 {

	availableQuota := int64(float64(totalCpuCore)*quotaCoefficient/100) + quotaOffset - deliveredCpuCore
	if availableQuota < 0 {
		return 0
	}
	return availableQuota
}

// listBizTotalCore list business dissolve total cpu core.
func (l *logics) listBizTotalCpuCore(kt *kit.Kit, bizIDs []int64) (map[int64]int64, error) {
	result := make(map[int64]int64, len(bizIDs))
	for _, bizID := range bizIDs {
		result[bizID] = 0
	}
	if len(bizIDs) == 0 {
		return result, nil
	}

	rules := append(l.baseRules(), tools.RuleIn("bk_biz_id", bizIDs))
	hosts, err := l.listLocalHosts(kt, tools.ExpressionAnd(rules...), []string{"bk_biz_id", "cpu_core"})
	if err != nil {
		return nil, err
	}

	for _, h := range hosts {
		bizID := cvt.PtrToVal(h.BkBizID)
		if _, ok := result[bizID]; ok {
			result[bizID] += int64(cvt.PtrToVal(h.CPUCore))
		}
	}

	return result, nil
}

// listBizDeliveredCpuCore list business dissolve delivered cpu core.
func (l *logics) listBizDeliveredCpuCore(kt *kit.Kit, bizIDs []int64) (map[int64]int64, error) {
	bizCpuCoreMap := make(map[int64]int64, len(bizIDs))
	for _, bizID := range bizIDs {
		bizCpuCoreMap[bizID] = 0
	}

	// 1. 查询“统计机房裁撤需求类型主机”开始时间
	time, err := l.dissolveConfig.GetDissolveHostApplyTime(kt)
	if err != nil {
		logs.Errorf("get host apply time failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 2. 查询从统计时间开始，申请的机房裁撤类型的主机
	filterExpr := tools.ExpressionAnd(
		tools.RuleIn("bk_biz_id", bizIDs),
		tools.RuleEqual("require_type", enumor.RequireTypeDissolve),
		tools.RuleEqual("is_delivered", true),
		tools.RuleGreaterThanEqual("created_at", time),
	)
	hosts, err := l.listDeliveredDeviceInfo(kt, filterExpr)
	if err != nil {
		logs.Errorf("list delivered device info failed, err: %v, filterExpr: %v, rid: %s", err, filterExpr, kt.Rid)
		return nil, err
	}

	bizIDDeviceTypeHostCountMap := make(map[int64]map[string]int64)
	deviceTypeMap := make(map[string]struct{})
	for _, host := range hosts {
		bizID := int64(host.BkBizId)
		if _, ok := bizIDDeviceTypeHostCountMap[bizID]; !ok {
			bizIDDeviceTypeHostCountMap[bizID] = make(map[string]int64)
		}
		bizIDDeviceTypeHostCountMap[bizID][host.DeviceType]++
		deviceTypeMap[host.DeviceType] = struct{}{}
	}

	// 3. 查询机型对应的核心数
	deviceTypes := maps.Keys(deviceTypeMap)
	deviceTypeInfos, err := l.configLogics.Device().ListCvmInstanceInfoByDeviceTypes(kt, deviceTypes)
	if err != nil {
		logs.Errorf("list device type info failed, err: %v, deviceTypes: %v, rid: %s", err, deviceTypes, kt.Rid)
		return nil, err
	}
	deviceTypeCpuCoreMap := make(map[string]int64, len(deviceTypeInfos))
	for _, info := range deviceTypeInfos {
		deviceTypeCpuCoreMap[info.DeviceType] = info.CPUAmount
	}

	// 4. 统计已交付给业务的核心数
	for bizID := range bizCpuCoreMap {
		deviceTypeHostCount, ok := bizIDDeviceTypeHostCountMap[bizID]
		if !ok {
			continue
		}
		for deviceType, hostCount := range deviceTypeHostCount {
			if _, ok = deviceTypeCpuCoreMap[deviceType]; !ok {
				logs.Errorf("can not find device type info, type: %s, bizID: %d", deviceType, bizID)
				return nil, fmt.Errorf("can not find device type info, type: %s, bizID: %d", deviceType, bizID)
			}
			bizCpuCoreMap[bizID] += deviceTypeCpuCoreMap[deviceType] * hostCount
		}
	}

	return bizCpuCoreMap, nil
}

// listDeviceInfoConcurrencyLimit 并发拉取已交付裁撤主机分页的最大协程数
const listDeviceInfoConcurrencyLimit = 10

// listDeliveredDeviceInfo 先取总数，再并发分页拉取已交付的机房裁撤类型主机
func (l *logics) listDeliveredDeviceInfo(kt *kit.Kit, filterExpr *filter.Expression) (
	[]*taskTypes.DeviceInfo, error) {

	total, err := model.Operation().DeviceInfo().CountDeviceInfo(kt, filterExpr)
	if err != nil {
		logs.Errorf("count device info failed, err: %v, filter: %+v, rid: %s", err, filterExpr, kt.Rid)
		return nil, err
	}
	if total == 0 {
		return make([]*taskTypes.DeviceInfo, 0), nil
	}

	limit := core.DefaultMaxPageLimit
	starts := make([]uint32, 0)
	for start := uint64(0); start < total; start += uint64(limit) {
		starts = append(starts, uint32(start))
	}

	pageResult, err := concurrence.BaseExecWithResult(listDeviceInfoConcurrencyLimit, starts,
		func(start uint32) ([]*taskTypes.DeviceInfo, error) {
			page := &core.BasePage{Start: start, Limit: limit}
			hosts, err := model.Operation().DeviceInfo().FindManyDeviceInfo(kt, filterExpr, page)
			if err != nil {
				logs.Errorf("list device info failed, err: %v, filter: %+v, rid: %s", err, filterExpr, kt.Rid)
				return nil, err
			}
			return hosts, nil
		})
	if err != nil {
		logs.Errorf("list delivered device info failed, err: %v, filterExpr: %v, rid: %s", err, filterExpr, kt.Rid)
		return nil, err
	}

	result := make([]*taskTypes.DeviceInfo, 0, total)
	for _, subResult := range pageResult {
		result = append(result, subResult...)
	}

	return result, nil
}
