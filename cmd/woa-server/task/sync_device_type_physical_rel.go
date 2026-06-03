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

package task

import (
	"time"

	"hcm/pkg/api/core"
	dataproto "hcm/pkg/api/data-service"
	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	croncore "hcm/pkg/cron/core"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/tools/slice"
)

// localRecord 本地映射表单条记录的内存视图，仅保留 diff 计算用到的字段。
type localRecord struct {
	ID                   string
	PhysicalDeviceFamily string
}

// SyncDeviceTypePhysicalRelTask 同步 CVM 机型与物理机机型族映射的定时任务。
// 全量 diff 拉取 CRP 数据并覆盖本地 `woa_device_type_physical_rel` 表。
type SyncDeviceTypePhysicalRelTask struct {
	clientSet *client.ClientSet
	crpCli    cvmapi.CVMClientInterface
	sd        serviced.State
}

// NewSyncDeviceTypePhysicalRelTask 创建同步任务实例。
func NewSyncDeviceTypePhysicalRelTask(clientSet *client.ClientSet, crpCli cvmapi.CVMClientInterface,
	sd serviced.State) (croncore.Task, error) {

	return &SyncDeviceTypePhysicalRelTask{
		clientSet: clientSet,
		crpCli:    crpCli,
		sd:        sd,
	}, nil
}

// Name 返回任务名。
func (t *SyncDeviceTypePhysicalRelTask) Name() string {
	return string(enumor.CronTaskSyncDeviceTypePhysicalRel)
}

// Next 返回下次执行时间，由 cc.WoaServer().ResourceSync.SyncDeviceTypePhysicalRel.Interval 控制。
func (t *SyncDeviceTypePhysicalRelTask) Next() (time.Time, error) {
	interval := cc.WoaServer().ResourceSync.SyncDeviceTypePhysicalRel.Interval
	return time.Now().Add(time.Duration(interval) * time.Minute), nil
}

// GetURL 返回手动触发 API 的子路径（在 service/res-sync 中被注册）。
func (t *SyncDeviceTypePhysicalRelTask) GetURL() string {
	return "/device_type_physical_rels/sync"
}

// Do 执行同步主流程：master 检查 → 拉 CRP → 加载本地 → diff → 写入。
func (t *SyncDeviceTypePhysicalRelTask) Do(kt *kit.Kit) error {
	if t.sd == nil || !t.sd.IsMaster() {
		logs.V(5).Infof("current node is not master, skip sync device type physical rel, rid: %s", kt.Rid)
		return nil
	}

	logs.Infof("start sync device type physical rel task, rid: %s", kt.Rid)

	crpMap, err := t.fetchCrpSnapshot(kt)
	if err != nil {
		return err
	}
	if len(crpMap) == 0 {
		return nil
	}

	localMap, err := t.loadLocalSnapshot(kt)
	if err != nil {
		return err
	}

	toCreate, toUpdate := t.diffSnapshots(crpMap, localMap)

	if err := t.applyCreate(kt, toCreate); err != nil {
		return err
	}
	if err := t.applyUpdate(kt, toUpdate); err != nil {
		return err
	}

	// 没有删除逻辑，原因：避免一开始某个机型支持生产，后面又不支持生产，导致把映射关系删掉，
	// 后续在回收等场景使用的时候查不到映射信息的问题

	logs.Infof("sync device type physical rel done, add: %d, update: %d, rid: %s", len(toCreate), len(toUpdate), kt.Rid)
	return nil
}

// fetchCrpSnapshot 调用 CRP queryCvmTypeList 拉取全量映射快照，过滤空字段并去重。
// 返回 key=cvmInstanceModel, value=deviceFamily 的 map。
func (t *SyncDeviceTypePhysicalRelTask) fetchCrpSnapshot(kt *kit.Kit) (map[string]string, error) {
	// 请求参数不带部门的条件，尽可能查到更多的机型信息
	resp, err := t.crpCli.QueryCvmTypeList(kt, &cvmapi.QueryCvmTypeListParams{})
	if err != nil {
		logs.Errorf("fetch crp cvm type list failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if resp == nil || len(resp.Result) == 0 {
		logs.Errorf("crp cvm type list returned 0 record, skip sync to keep last snapshot, rid: %s", kt.Rid)
		return nil, nil
	}

	return buildSnapshotFromCvmTypeItems(kt, resp.Result), nil
}

// buildSnapshotFromCvmTypeItems 从 CRP 返回的明细列表构建去重 map，
// 过滤掉 cvmInstanceModel/deviceFamily 任一字段为空的记录并打 warn 日志。
// 抽出为独立函数以便单元测试覆盖。
func buildSnapshotFromCvmTypeItems(kt *kit.Kit, items []cvmapi.CvmTypeItem) map[string]string {
	snapshot := make(map[string]string, len(items))
	for _, item := range items {
		if item.CvmInstanceModel == "" || item.DeviceFamily == "" {
			logs.Warnf("skip crp cvm type item with empty field, item: %+v, rid: %s", item, kt.Rid)
			continue
		}
		snapshot[item.CvmInstanceModel] = item.DeviceFamily
	}
	return snapshot
}

// loadLocalSnapshot 分页拉取本地 woa_device_type_physical_rel 表全量数据。
// 返回 key=device_type, value=localRecord{id, physical_device_family} 的 map。
func (t *SyncDeviceTypePhysicalRelTask) loadLocalSnapshot(kt *kit.Kit) (map[string]localRecord, error) {
	req := &core.ListReq{
		Filter: tools.AllExpression(),
		Page:   core.NewDefaultBasePage(),
	}
	result := make(map[string]localRecord)
	for {
		resp, err := t.clientSet.DataService().Global.ResourcePlan.ListWoaDeviceTypePhysicalRel(kt, req)
		if err != nil {
			logs.Errorf("list local woa device type physical rel failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		for _, one := range resp.Details {
			result[one.DeviceType] = localRecord{
				ID:                   one.ID,
				PhysicalDeviceFamily: one.PhysicalDeviceFamily,
			}
		}
		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}
	return result, nil
}

// diffSnapshots 只返回创建和更新的数据：
//   - toCreate: CRP 有，本地没有的 device_type → physical_device_family
//   - toUpdate: 双方都有但 physical_device_family 不同，key=本地 id，value=新值
func (t *SyncDeviceTypePhysicalRelTask) diffSnapshots(crpMap map[string]string, localMap map[string]localRecord) (
	map[string]string, map[string]string) {

	toCreate := make(map[string]string)
	toUpdate := make(map[string]string)
	for deviceType, family := range crpMap {
		local, ok := localMap[deviceType]
		if !ok {
			toCreate[deviceType] = family
			continue
		}
		if local.PhysicalDeviceFamily != family {
			toUpdate[local.ID] = family
		}
	}

	return toCreate, toUpdate
}

// applyCreate 分批调用 BatchCreate 接口写入新增记录。
func (t *SyncDeviceTypePhysicalRelTask) applyCreate(kt *kit.Kit, toCreate map[string]string) error {
	if len(toCreate) == 0 {
		return nil
	}
	records := make([]rpproto.WoaDeviceTypePhysicalRelCreateReq, 0, len(toCreate))
	for deviceType, family := range toCreate {
		records = append(records, rpproto.WoaDeviceTypePhysicalRelCreateReq{
			DeviceType:           deviceType,
			PhysicalDeviceFamily: family,
		})
	}
	for _, batch := range slice.Split(records, constant.BatchOperationMaxLimit) {
		req := &rpproto.WoaDeviceTypePhysicalRelBatchCreateReq{Records: batch}
		_, err := t.clientSet.DataService().Global.ResourcePlan.BatchCreateWoaDeviceTypePhysicalRel(kt, req)
		if err != nil {
			logs.Errorf("batch create woa device type physical rel failed, err: %v, batch_size: %d, rid: %s", err,
				len(batch), kt.Rid)
			return err
		}
	}
	return nil
}

// applyUpdate 分批调用 BatchUpdate 接口更新已变化记录。
func (t *SyncDeviceTypePhysicalRelTask) applyUpdate(kt *kit.Kit, toUpdate map[string]string) error {
	if len(toUpdate) == 0 {
		return nil
	}
	records := make([]rpproto.WoaDeviceTypePhysicalRelUpdateReq, 0, len(toUpdate))
	for id, family := range toUpdate {
		records = append(records, rpproto.WoaDeviceTypePhysicalRelUpdateReq{
			ID:                   id,
			PhysicalDeviceFamily: family,
		})
	}
	for _, batch := range slice.Split(records, constant.BatchOperationMaxLimit) {
		req := &rpproto.WoaDeviceTypePhysicalRelBatchUpdateReq{Records: batch}
		err := t.clientSet.DataService().Global.ResourcePlan.BatchUpdateWoaDeviceTypePhysicalRel(kt, req)
		if err != nil {
			logs.Errorf("batch update woa device type physical rel failed, err: %v, batch_size: %d, rid: %s", err,
				len(batch), kt.Rid)
			return err
		}
	}
	return nil
}

// applyDelete 分批调用 Delete 接口删除 CRP 不再返回的记录。
func (t *SyncDeviceTypePhysicalRelTask) applyDelete(kt *kit.Kit, toDelete []string) error {
	if len(toDelete) == 0 {
		return nil
	}
	for _, batch := range slice.Split(toDelete, constant.BatchOperationMaxLimit) {
		req := &dataproto.BatchDeleteReq{Filter: tools.ContainersExpression("id", batch)}
		if err := t.clientSet.DataService().Global.ResourcePlan.DeleteWoaDeviceTypePhysicalRel(kt, req); err != nil {
			logs.Errorf("delete woa device type physical rel failed, err: %v, batch_size: %d, rid: %s", err, len(batch),
				kt.Rid)
			return err
		}
	}
	return nil
}
