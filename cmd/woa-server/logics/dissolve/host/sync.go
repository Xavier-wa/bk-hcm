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

package host

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"hcm/pkg"
	"hcm/pkg/api/core"
	zoneproto "hcm/pkg/api/data-service/cloud/zone"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	daotypes "hcm/pkg/dal/dao/types"
	define "hcm/pkg/dal/table/dissolve/host"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/thirdparty/caiche"
	"hcm/pkg/thirdparty/es"
	"hcm/pkg/tools/assert"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/querybuilder"
	"hcm/pkg/tools/slice"
)

// bizInfo 主机的业务相关补全信息
type bizInfo struct {
	bizID     int64
	groupID   int64
	operators []string
	found     bool
}

// Sync 从裁撤系统拉取裁撤主机，补全业务字段后回写本地表。
func (l *logics) Sync(kt *kit.Kit) error {
	start := time.Now()
	logs.Infof("start sync recycle host, time: %v, rid: %s", start, kt.Rid)

	projectIDs, err := l.getDissolveProjectIDs(kt)
	if err != nil {
		return err
	}
	if len(projectIDs) == 0 {
		logs.Warnf("no dissolve project configured, skip sync, rid: %s", kt.Rid)
		return nil
	}

	zoneRegionMap, err := l.getZoneRegionMap(kt)
	if err != nil {
		return err
	}

	dbHosts, err := l.getAllHostFromDB(kt)
	if err != nil {
		return err
	}

	caicheHosts, err := l.getAllHostFromCaiChe(kt, projectIDs, zoneRegionMap)
	if err != nil {
		return err
	}

	caicheHosts, err = l.enrichBizFields(kt, caicheHosts, dbHosts)
	if err != nil {
		logs.Errorf("enrich biz fields failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	create, update, deleteIDs := diff(dbHosts, caicheHosts)
	if err = l.applyDiff(kt, create, update, deleteIDs); err != nil {
		return err
	}

	logs.Infof("end sync recycle host, cost: %v, create: %d, update: %d, delete: %d, rid: %s", time.Since(start),
		len(create), len(update), len(deleteIDs), kt.Rid)
	return nil
}

// getDissolveProjectIDs 取所有裁撤周期配置的项目ID并集
func (l *logics) getDissolveProjectIDs(kt *kit.Kit) ([]int, error) {
	cycles, err := l.config.GetDissolveProjects(kt)
	if err != nil {
		logs.Errorf("get dissolve projects failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	idSet := make(map[int]struct{})
	for _, cycle := range cycles {
		for _, p := range cycle.Projects {
			idSet[p.ID] = struct{}{}
		}
	}

	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	return ids, nil
}

// getZoneRegionMap 经 data-service 查 zone 表，加载 zone_name(name_cn) -> region_id 映射
func (l *logics) getZoneRegionMap(kt *kit.Kit) (map[string]string, error) {
	req := &zoneproto.ZoneListReq{
		Field:  []string{"name_cn", "region"},
		Filter: tools.EqualExpression("vendor", enumor.TCloudZiyan),
		Page:   core.NewDefaultBasePage(),
	}

	result := make(map[string]string)
	for {
		zones, err := l.cliSet.DataService().TCloudZiyan.Zone.ListZoneExt(kt, req)
		if err != nil {
			logs.Errorf("list zone failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		for _, zone := range zones.Details {
			if zone.NameCn == "" {
				continue
			}
			result[zone.NameCn] = zone.Region
		}

		if uint(len(zones.Details)) < req.Page.Limit {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	return result, nil
}

// getAllHostFromDB 分页拉取本地裁撤主机表全部数据
func (l *logics) getAllHostFromDB(kt *kit.Kit) ([]define.RecycleHostTable, error) {
	hosts := make([]define.RecycleHostTable, 0)
	opt := &daotypes.ListOption{
		Filter: tools.AllExpression(),
		Page:   core.NewDefaultBasePage(),
	}

	for {
		result, err := l.List(kt, opt)
		if err != nil {
			logs.Errorf("get recycle host from db failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		hosts = append(hosts, result.Details...)
		if len(result.Details) < int(opt.Page.Limit) {
			break
		}
		opt.Page.Start += uint32(opt.Page.Limit)
	}

	return hosts, nil
}

// getAllHostFromCaiChe get all host from caiche
func (l *logics) getAllHostFromCaiChe(kt *kit.Kit, projectIDs []int, zoneRegionMap map[string]string) (
	[]define.RecycleHostTable, error) {

	virtualDepartmentName := []string{"IEG_Global", "IEG技术运营部"}
	abolishPhase := []enumor.AbolishPhase{enumor.Incomplete, enumor.Complete, enumor.BsiComplete, enumor.Retain}

	v2Req := &caiche.ListDeviceV2Req{
		PageNo:   1, // 裁撤系统该接口页码从 1 开始
		PageSize: core.DefaultMaxPageLimit,
		Filter: map[string]interface{}{
			"virtualDepartmentName": virtualDepartmentName,
			"projectId":             projectIDs,
			"abolishPhase":          abolishPhase,
			"svrTypeName":           l.svrTypeNames,
		},
	}

	hostMap := make(map[string]define.RecycleHostTable)
	for {
		resp, err := l.thirdCli.CaiChe.ListDeviceV2(kt, v2Req)
		if err != nil {
			logs.Errorf("get recycle v2 host failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		for _, device := range resp.Data {
			host := transferDevice(kt, device, zoneRegionMap)
			key := compositeKey(cvt.PtrToVal(host.ProjectID), cvt.PtrToVal(host.AssetID))
			hostMap[key] = host
		}

		if len(resp.Data) < int(core.DefaultMaxPageLimit) {
			break
		}
		v2Req.PageNo++
	}

	result := make([]define.RecycleHostTable, 0, len(hostMap))
	for _, v := range hostMap {
		result = append(result, v)
	}
	return result, nil
}

// transferDevice 裁撤设备转为本地表结构
func transferDevice(kt *kit.Kit, device caiche.DeviceV2, zoneRegionMap map[string]string) define.RecycleHostTable {
	if device.AvailabilityZoneName == "" {
		logs.Warnf("availabilityZoneName is empty, assetID: %s, rid: %s", device.SerAssetID, kt.Rid)
	}
	region, ok := zoneRegionMap[device.AvailabilityZoneName]
	if !ok {
		logs.Warnf("woa zone not found, availabilityZoneName: %s, assetID: %s, rid: %s", device.AvailabilityZoneName,
			device.SerAssetID, kt.Rid)
	}

	return define.RecycleHostTable{
		AssetID:           cvt.ValToPtr(device.SerAssetID),
		InnerIP:           cvt.ValToPtr(device.ServerLanIP),
		DeviceType:        cvt.ValToPtr(device.SvrDeviceClassName),
		Module:            cvt.ValToPtr(device.ModName),
		AbolishPhase:      cvt.ValToPtr(device.AbolishPhase),
		ProjectName:       cvt.ValToPtr(device.ProjectName),
		ProjectID:         cvt.ValToPtr(device.ProjectID),
		Region:            cvt.ValToPtr(region),
		CPUCore:           cvt.ValToPtr(device.CPULogicCoreNum),
		ExpectAbolishTime: cvt.ValToPtr(device.ExpectAbolishTime),
	}
}

// enrichBizFields enrich biz fields
func (l *logics) enrichBizFields(kt *kit.Kit, caicheHosts, dbHosts []define.RecycleHostTable) (
	[]define.RecycleHostTable, error) {

	dbMap := make(map[string]define.RecycleHostTable, len(dbHosts))
	ignoreEsFoundAssetIDs := make([]string, 0)
	for _, h := range dbHosts {
		dbMap[compositeKey(cvt.PtrToVal(h.ProjectID), cvt.PtrToVal(h.AssetID))] = h
		// 主机被销毁后，从cc查不到数据，如果之前有值，代表从cc或者es已经查询过数据进行补充了，不需要再进行ES查询
		if h.BkBizID != nil && h.GroupID != nil && len(h.Operators) != 0 {
			ignoreEsFoundAssetIDs = append(ignoreEsFoundAssetIDs, cvt.PtrToVal(h.AssetID))
		}
	}

	assetIDs := make([]string, 0, len(caicheHosts))
	for _, h := range caicheHosts {
		assetIDs = append(assetIDs, cvt.PtrToVal(h.AssetID))
	}
	assetIDs = slice.Unique(assetIDs)
	ccMap, noFoundAssetIDs, err := l.enrichFromCC(kt, assetIDs)
	if err != nil {
		logs.Errorf("enrich from cc failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	noFoundAssetIDs = slice.NotIn(ignoreEsFoundAssetIDs, noFoundAssetIDs)
	esMap := make(map[string]bizInfo)
	if len(noFoundAssetIDs) != 0 {
		esMap, err = l.enrichFromES(kt, noFoundAssetIDs)
		if err != nil {
			logs.Errorf("enrich from es failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
	}

	ignoreBizSet := make(map[int64]struct{}, len(l.ignoreBiz))
	for _, id := range l.ignoreBiz {
		ignoreBizSet[id] = struct{}{}
	}

	for i := range caicheHosts {
		caicheHosts[i] = fillBizFields(caicheHosts[i], ccMap, esMap, dbMap, ignoreBizSet)
	}

	return caicheHosts, nil
}

// fillBizFields 为单台主机按优先级填充业务字段
func fillBizFields(host define.RecycleHostTable, ccMap, esMap map[string]bizInfo,
	dbMap map[string]define.RecycleHostTable, ignoreBizSet map[int64]struct{}) define.RecycleHostTable {

	assetID := cvt.PtrToVal(host.AssetID)
	var info bizInfo
	switch {
	case ccMap[assetID].found:
		info = ccMap[assetID]
	case esMap[assetID].found:
		info = esMap[assetID]
	default:
		if old, ok := dbMap[compositeKey(cvt.PtrToVal(host.ProjectID), assetID)]; ok {
			info = bizInfo{
				bizID:     cvt.PtrToVal(old.BkBizID),
				groupID:   cvt.PtrToVal(old.GroupID),
				operators: old.Operators,
			}
		}
	}

	host.BkBizID = cvt.ValToPtr(info.bizID)
	host.GroupID = cvt.ValToPtr(info.groupID)
	host.Operators = info.operators

	_, ignored := ignoreBizSet[info.bizID]
	host.IsIgnore = cvt.ValToPtr(ignored)

	return host
}

// enrichFromCC 经 CC 按 asset_id 补全 biz_id/group_id/operators
func (l *logics) enrichFromCC(kt *kit.Kit, assetIDs []string) (map[string]bizInfo, []string, error) {
	result := make(map[string]bizInfo)
	noFound := make([]string, 0)
	if len(assetIDs) == 0 {
		return result, noFound, nil
	}

	assetOperators := make(map[string][]string)
	assetHostID := make(map[string]int64)
	hostIDs := make([]int64, 0)
	fields := []string{pkg.BKHostIDField, pkg.BKAssetIDField, pkg.BKOperatorField, pkg.BKBakOperatorField}
	for _, batch := range slice.Split(assetIDs, pkg.BKMaxPageSize) {
		hosts, err := l.listCCHostsByAssetID(kt, batch, fields)
		if err != nil {
			logs.Errorf("list cc hosts by asset id failed, err: %v, rid: %s", err, kt.Rid)
			return nil, nil, err
		}
		for _, h := range hosts {
			assetOperators[h.BkAssetID] = splitOperators(h.Operator, h.BkBakOperator)
			assetHostID[h.BkAssetID] = h.BkHostID
			hostIDs = append(hostIDs, h.BkHostID)
		}
	}

	hostBizMap, err := l.getHostBizMap(kt, hostIDs)
	if err != nil {
		logs.Errorf("get host biz map failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	bizIDs := make([]int64, 0, len(hostBizMap))
	for _, bizID := range hostBizMap {
		bizIDs = append(bizIDs, bizID)
	}
	bizGroupMap, err := l.getBizGroupMap(kt, slice.Unique(bizIDs))
	if err != nil {
		logs.Errorf("get biz group map failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	for _, assetID := range assetIDs {
		hostID, ok := assetHostID[assetID]
		if !ok {
			noFound = append(noFound, assetID)
			continue
		}
		bizID := hostBizMap[hostID]
		result[assetID] = bizInfo{
			bizID:     bizID,
			groupID:   bizGroupMap[bizID],
			operators: assetOperators[assetID],
			found:     true,
		}
	}

	return result, noFound, nil
}

// splitOperators 将主备负责人字段按英文逗号拆分并去重合并为负责人列表
func splitOperators(operators ...string) []string {
	result := make([]string, 0)
	for _, operator := range operators {
		for _, item := range strings.Split(operator, ",") {
			if item = strings.TrimSpace(item); item != "" {
				result = append(result, item)
			}
		}
	}
	return slice.Unique(result)
}

// listCCHostsByAssetID 按 asset_id 批量查询 CC 主机
func (l *logics) listCCHostsByAssetID(kt *kit.Kit, assetIDs []string, fields []string) ([]cmdb.Host, error) {
	req := &cmdb.ListHostReq{
		HostPropertyFilter: &cmdb.QueryFilter{
			Rule: querybuilder.CombinedRule{
				Condition: querybuilder.ConditionAnd,
				Rules: []querybuilder.Rule{
					querybuilder.AtomRule{Field: pkg.BKCloudIDField, Operator: querybuilder.OperatorEqual, Value: 0},
					querybuilder.AtomRule{
						Field: pkg.BKAssetIDField, Operator: querybuilder.OperatorIn, Value: assetIDs,
					},
				},
			},
		},
		Fields: fields,
		Page:   cmdb.BasePage{Start: 0, Limit: int64(len(assetIDs)), Sort: pkg.BKHostIDField},
	}

	resp, err := l.cmdbCli.ListHost(kt, req)
	if err != nil {
		logs.Errorf("list cc host by asset id failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return resp.Info, nil
}

// getHostBizMap 批量获取 host_id -> biz_id
func (l *logics) getHostBizMap(kt *kit.Kit, hostIDs []int64) (map[int64]int64, error) {
	result := make(map[int64]int64)
	for _, batch := range slice.Split(hostIDs, int(core.DefaultMaxPageLimit)) {
		relation, err := l.cmdbCli.GetHostBizIds(kt, batch)
		if err != nil {
			logs.Errorf("get host biz ids failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		for hostID, bizID := range relation {
			result[hostID] = bizID
		}
	}

	return result, nil
}

// getBizGroupMap 批量获取 biz_id -> group_id(bk_oper_grp_name_id)
func (l *logics) getBizGroupMap(kt *kit.Kit, bizIDs []int64) (map[int64]int64, error) {
	result := make(map[int64]int64)
	if len(bizIDs) == 0 {
		return result, nil
	}

	req := &cmdb.SearchBizParams{
		Fields: []string{"bk_biz_id", "bk_oper_grp_name_id"},
		Page:   cmdb.BasePage{Start: 0, Limit: pkg.BKMaxInstanceLimit},
	}
	for _, batch := range slice.Split(bizIDs, pkg.BKMaxInstanceLimit) {
		req.BizPropertyFilter = &cmdb.QueryFilter{
			Rule: cmdb.CombinedRule{
				Condition: cmdb.ConditionAnd,
				Rules: []cmdb.Rule{
					cmdb.AtomRule{Field: "bk_biz_id", Operator: cmdb.OperatorIn, Value: batch},
				},
			},
		}

		resp, err := l.cmdbCli.SearchBusiness(kt, req)
		if err != nil {
			logs.Errorf("search business failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		for _, info := range resp.Info {
			result[info.BizID] = info.BkOperGrpNameID
		}
	}

	return result, nil
}

// enrichFromES 经 ES 按 asset_id 兜底补全 biz_id/group_id/operators
func (l *logics) enrichFromES(kt *kit.Kit, assetIDs []string) (map[string]bizInfo, error) {
	result := make(map[string]bizInfo)
	if len(assetIDs) == 0 || l.esCli == nil {
		return result, nil
	}

	index := es.GetIndex(l.originDate)
	fields := []string{es.AssetID, es.BizID, es.GroupID, es.ServerOperator, es.ServerBakOperator}
	for _, batch := range slice.Split(assetIDs, int(core.DefaultMaxPageLimit)) {
		cond := map[string][]interface{}{es.AssetID: cvt.StrSliceToInterfaceSlice(batch)}
		hosts, err := l.esCli.SearchWithCond(kt.Ctx, cond, index, 0, len(batch), es.AssetID, fields)
		if err != nil {
			logs.Errorf("search with cond from es failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		for _, h := range hosts {
			result[h.ServerAssetID] = bizInfo{
				bizID:     h.BizID,
				groupID:   h.GroupID,
				operators: parseESOperators(h.ServerOperator, h.ServerBakOperator),
				found:     true,
			}
		}
	}

	return result, nil
}

// applyDiff 按 diff 结果回写本地表
func (l *logics) applyDiff(kt *kit.Kit, create, update []define.RecycleHostTable, deleteIDs []string) error {
	for _, batch := range slice.Split(deleteIDs, int(core.DefaultMaxPageLimit)) {
		if err := l.Delete(kt, batch); err != nil {
			return err
		}
	}

	for i := range update {
		if err := l.Update(kt, &update[i]); err != nil {
			return err
		}
	}

	if len(create) != 0 {
		if _, err := l.Create(kt, create); err != nil {
			return err
		}
	}

	return nil
}

func compositeKey(projectID int, assetID string) string {
	return fmt.Sprintf("%d|%s", projectID, assetID)
}

// diff 按 (project_id, asset_id) 复合键比对，返回新增/更新/删除
func diff(dbHosts, caicheHosts []define.RecycleHostTable) ([]define.RecycleHostTable,
	[]define.RecycleHostTable, []string) {

	dbMap := make(map[string]define.RecycleHostTable, len(dbHosts))
	for _, one := range dbHosts {
		dbMap[compositeKey(cvt.PtrToVal(one.ProjectID), cvt.PtrToVal(one.AssetID))] = one
	}

	create := make([]define.RecycleHostTable, 0)
	update := make([]define.RecycleHostTable, 0)
	for _, newHost := range caicheHosts {
		key := compositeKey(cvt.PtrToVal(newHost.ProjectID), cvt.PtrToVal(newHost.AssetID))
		dbHost, exist := dbMap[key]
		if !exist {
			create = append(create, newHost)
			continue
		}

		delete(dbMap, key)
		if isChange(newHost, dbHost) {
			newHost.ID = dbHost.ID
			update = append(update, newHost)
		}
	}

	deleteIDs := make([]string, 0)
	for _, one := range dbMap {
		deleteIDs = append(deleteIDs, one.ID)
	}

	return create, update, deleteIDs
}

// isChange 判断同步数据相对 DB 是否有变化
func isChange(newHost, old define.RecycleHostTable) bool {
	if cvt.PtrToVal(newHost.InnerIP) != cvt.PtrToVal(old.InnerIP) ||
		cvt.PtrToVal(newHost.DeviceType) != cvt.PtrToVal(old.DeviceType) ||
		cvt.PtrToVal(newHost.Module) != cvt.PtrToVal(old.Module) ||
		cvt.PtrToVal(newHost.AbolishPhase) != cvt.PtrToVal(old.AbolishPhase) ||
		cvt.PtrToVal(newHost.ProjectName) != cvt.PtrToVal(old.ProjectName) ||
		cvt.PtrToVal(newHost.Region) != cvt.PtrToVal(old.Region) ||
		cvt.PtrToVal(newHost.BkBizID) != cvt.PtrToVal(old.BkBizID) ||
		cvt.PtrToVal(newHost.GroupID) != cvt.PtrToVal(old.GroupID) ||
		cvt.PtrToVal(newHost.CPUCore) != cvt.PtrToVal(old.CPUCore) ||
		cvt.PtrToVal(newHost.IsIgnore) != cvt.PtrToVal(old.IsIgnore) ||
		cvt.PtrToVal(newHost.ExpectAbolishTime) != cvt.PtrToVal(old.ExpectAbolishTime) {
		return true
	}

	return !assert.IsStringSliceEqual(newHost.Operators, old.Operators)
}

// parseESOperators 解析 ES 中以 JSON 数组字符串存储的维护人字段，合并主备维护人并去重去空。
// ES 示例值：server_operator/server_bak_operator 均为 `["test1", "test2"]` 形式。
func parseESOperators(raws ...string) []string {
	operators := make([]string, 0)
	for _, raw := range raws {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		var list []string
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			// 兼容非 JSON 数组的纯字符串值
			operators = append(operators, raw)
			continue
		}
		operators = append(operators, list...)
	}

	return slice.Unique(operators)
}
