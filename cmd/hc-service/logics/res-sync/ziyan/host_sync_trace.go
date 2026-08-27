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

package ziyan

import (
	"context"
	"sync"
	"time"

	"hcm/cmd/hc-service/logics/res-sync/common"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// 同步流水线各段的 step 名，会出现在汇总日志的 steps 片段中，命名即日志检索关键词。
const (
	// stepListBizHost HostWithRelRes 第一段，带业务从 cc 查主机。
	stepListBizHost = "list_biz_host"
	// stepListCloudCvm getCVM 按 region 从云上查 cvm。
	stepListCloudCvm = "list_cloud_cvm"

	// stepListCCHost Host 内部不带业务的 cc 重查，与 stepListBizHost 区分。
	stepListCCHost = "list_cc_host"
	// stepFillCloudField getCloudHost 补云上字段（含重复的 ListCvm 与 vpc/subnet 映射查询）。
	stepFillCloudField = "fill_cloud_field"
	// stepReadHostDB diff 前从 db 预查存量主机。
	stepReadHostDB = "read_host_db"
	// stepCreateHostDB diff 后新增主机写库。
	stepCreateHostDB = "create_host_db"
	// stepUpdateHostDB diff 后更新主机写库。
	stepUpdateHostDB = "update_host_db"
	// stepDeleteHostDB diff 后删除主机写库。
	stepDeleteHostDB = "delete_host_db"
)

// SyncHostTrace is the observability record of one ziyan host sync batch:
// 通用机制由内嵌的 common.ResSyncTrace 承载，本结构只保留 ziyan 特有日志字段。
// 通过 kt.Ctx 传递，全部方法 nil 安全。
type SyncHostTrace struct {
	// ResSyncTrace 通用观测基座。
	*common.ResSyncTrace

	mu sync.RWMutex
	// accountID 本批主机归属的账号。
	accountID string
	// bizID 本批主机归属的业务。
	bizID int64
	// hostCount 本批请求同步的主机数。
	hostCount int
	// ccHostCount 第一段从 cc 查到的主机数。
	ccHostCount int
	// cvmCount 云上查到的 cvm 数。
	cvmCount int
	// regionCount cvm 分布的 region 数。
	regionCount int
	// dbCreateCount 主机 diff 后新增写库行数。
	dbCreateCount int
	// dbUpdateCount 主机 diff 后更新写库行数。
	dbUpdateCount int
	// dbDeleteCount 主机 diff 后删除写库行数。
	dbDeleteCount int
}

type syncHostTraceCtxKey struct{}

// newSyncHostTrace creates the trace of one sync batch and returns a kit
// carrying it in the context.
func newSyncHostTrace(kt *kit.Kit, params *SyncHostParams) (*kit.Kit, *SyncHostTrace) {
	tr := &SyncHostTrace{
		ResSyncTrace: common.NewResSyncTrace(kt, enumor.TCloudZiyan, enumor.CvmCloudResType,
			[]enumor.ResSyncStep{enumor.ResSyncStepHost, enumor.ResSyncStepRelRes, enumor.ResSyncStepHostRel}),
		accountID: params.AccountID,
		bizID:     params.BizID,
		hostCount: len(params.HostIDs),
	}

	return kt.NewSubKitWithCtx(context.WithValue(kt.Ctx, syncHostTraceCtxKey{}, tr)), tr
}

// syncHostTraceFromCtx extracts the trace carried by the context, nil if absent.
// 供深层调用点在不改函数签名的前提下写入观测数据。
func syncHostTraceFromCtx(ctx context.Context) *SyncHostTrace {
	if ctx == nil {
		return nil
	}
	tr, _ := ctx.Value(syncHostTraceCtxKey{}).(*SyncHostTrace)
	return tr
}

// Track starts timing a step and returns the function that records it.
// A nil check is required here: a nil shell panics on promoted methods.
func (t *SyncHostTrace) Track(name string) func() {
	if t == nil {
		return func() {}
	}
	return t.ResSyncTrace.Track(name)
}

// AddStep appends a step whose cost is measured by the caller; same nil check as Track.
func (t *SyncHostTrace) AddStep(name string, cost time.Duration) {
	if t == nil {
		return
	}
	t.ResSyncTrace.AddStep(name, cost)
}

// SetCCResult records the cc fetch outcome: how many hosts cc returned.
func (t *SyncHostTrace) SetCCResult(ccHostCount int) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.ccHostCount = ccHostCount
}

// SetCloudResult records the cloud fetch outcome: cvm count and region spread.
func (t *SyncHostTrace) SetCloudResult(cvmCount, regionCount int) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.cvmCount = cvmCount
	t.regionCount = regionCount
}

// SetHostWrite records the db write counts of the host diff.
func (t *SyncHostTrace) SetHostWrite(createCount, updateCount, deleteCount int) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.dbCreateCount = createCount
	t.dbUpdateCount = updateCount
	t.dbDeleteCount = deleteCount
}

// logSummary outputs the one-line overview of this batch. 在 HostWithRelRes
// 的 defer 中调用，因此提前返回的路径也会带着已记录的步骤打出来。
func (t *SyncHostTrace) logSummary(kt *kit.Kit, syncErr error) {
	if t == nil {
		return
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	result := "success"
	if syncErr != nil {
		result = "failed"
	}

	logs.Infof("sync ziyan host %s, account: %s, biz: %d, hosts: %d, cc hosts: %d, cvms: %d, "+
		"regions: %d, db create: %d, update: %d, delete: %d, cost: %s, steps: [%s], rid: %s",
		result, t.accountID, t.bizID, t.hostCount, t.ccHostCount, t.cvmCount, t.regionCount,
		t.dbCreateCount, t.dbUpdateCount, t.dbDeleteCount, t.Cost(), t.Summary(), kt.Rid)
}
