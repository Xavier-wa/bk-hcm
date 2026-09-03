/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may obtain a copy of the License at http://opensource.org/licenses/MIT
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

package runobserve

import (
	"time"

	"hcm/pkg/api/core"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	croncore "hcm/pkg/cron/core"
	"hcm/pkg/dal/dao/tools"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/metrics"
	"hcm/pkg/runtime/filter"
)

const (
	sweepURL = "/aiagent/runs/sweep"
	// sweepPageLimit 是单次 List 页大小。
	sweepPageLimit = 100
	// maxSweepPerRound 是一轮 Do 最多尝试的孤儿行数，避免积压把 cron worker 卡住。
	maxSweepPerRound = 1000
)

// SweepTask 清扫账本里卡住的 running 行。
//
// 入选：status=running 且 updated_at 早于 now-orphanThreshold（默认 4h）。
// 按 updated_at 升序 CAS 成 unknown（reason=orphan_sweep）；成功则打点，eval 开启时入评估队列。
// 一轮循环分页直到取空或扫满 maxSweepPerRound；CAS 成功后行会离开 running 过滤条件，
// 所以下一页始终 start=0，不会因偏移跳号。
type SweepTask struct {
	cli *client.ClientSet
	// interval 是两次 cron 执行的间隔，来自 aiagentRunSweep.interval。
	interval time.Duration
	// onHit 在 CAS 成功后入评估队列；eval 关闭时不会调用。
	onHit OnCASHit
}

// Name returns the cron task identifier.
func (t *SweepTask) Name() string {
	return string(enumor.CronTaskSweepAiagentRun)
}

// Next returns the next scheduled execution time.
func (t *SweepTask) Next() (time.Time, error) {
	return time.Now().Add(t.interval), nil
}

// GetURL returns the HTTP path for the manual sweep trigger.
func (t *SweepTask) GetURL() string {
	return sweepURL
}

// Do 将超时仍为 running 的账本行 CAS 为 unknown。
// leftover=true 表示本轮被上限截断或整页 CAS 失败，下一周期继续清。
func (t *SweepTask) Do(kt *kit.Kit) error {
	cfg := cc.AgentServer().AiagentRunSweep
	cutoff := time.Now().Add(-cfg.OrphanThresholdDur()).Format(constant.TimeStdFormat)
	expr := tools.ExpressionAnd(
		tools.RuleEqual("status", string(enumor.AiagentRunStatusRunning)),
		tools.RuleLessThan("updated_at", cutoff),
	)
	affected, leftover, err := t.sweepOrphans(kt, expr)
	if err != nil {
		return err
	}
	if leftover {
		logs.Warnf("sweep aiagent run leftover, count: %d, leftover: true, rid: %s",
			affected, kt.Rid)
		return nil
	}
	logs.Infof("sweep aiagent run success, count: %d, leftover: false, rid: %s",
		affected, kt.Rid)
	return nil
}

// sweepOrphans 分页清扫，返回本轮真正改为 unknown 的条数，以及是否还有剩余。
func (t *SweepTask) sweepOrphans(kt *kit.Kit, expr *filter.Expression) (int, bool, error) {
	scanned := 0
	affected := 0
	for {
		limit := nextSweepLimit(scanned)
		if limit == 0 {
			leftover, err := t.hasOrphanLeftover(kt, expr)
			return affected, leftover, err
		}
		result, err := t.listOrphans(kt, expr, uint(limit))
		if err != nil {
			return affected, false, err
		}
		listed := len(result.Details)
		if listed == 0 {
			return affected, false, nil
		}
		pageHit, progressed := t.sweepPage(kt, result.Details)
		affected += pageHit
		scanned += listed
		if !progressed {
			// 整页都 CAS 失败时仍会命中同一批 oldest 行，必须停，避免空转。
			return affected, true, nil
		}
		done, leftoverHint := sweepPageOutcome(listed, limit, scanned)
		if !done {
			continue
		}
		if !leftoverHint {
			return affected, false, nil
		}
		// 满页且触达上限，Count 确认是否还有孤儿。
		leftover, err := t.hasOrphanLeftover(kt, expr)
		return affected, leftover, err
	}
}

// sweepPage 处理一页。hit 是 CAS 成功数；progressed 为 false 表示整页都没推进。
func (t *SweepTask) sweepPage(kt *kit.Kit, runs []tableaiagent.RunTable) (int, bool) {
	hit := 0
	progressed := 0
	for _, run := range runs {
		casHit, moved := t.markUnknown(kt, run)
		if casHit {
			hit++
		}
		if moved {
			progressed++
		}
	}
	return hit, progressed > 0
}

// markUnknown CAS running→unknown。返回
//
//	casHit     本轮是否改成 unknown（计入 count、入评估队列）
//	progressed 该行是否已离开 running 过滤（成功或 RecordNotUpdate）；false 表示下次还会扫到它
func (t *SweepTask) markUnknown(kt *kit.Kit, run tableaiagent.RunTable) (bool, bool) {
	err := t.cli.DataService().Aiagent.Run.UpdateStatus(kt, &dsaiagent.UpdateAiagentRunStatusReq{
		RunID:  run.RunID,
		Status: enumor.AiagentRunStatusUnknown,
		Reason: "orphan_sweep",
	})
	if err != nil {
		if errf.Error(err).Code == errf.RecordNotUpdate {
			return false, true
		}
		logs.Errorf("cas unknown aiagent run failed, err: %v, run_id: %s, rid: %s",
			err, run.RunID, kt.Rid)
		return false, false
	}
	metrics.IncAiagentRunTotal(run.BkBizID, enumor.AiagentRunStateUnknown, string(run.Scene))
	if t.onHit != nil && cc.AgentServer().Eval.IsEnabled() {
		t.onHit(kt.NewSubKit(), run.RunID)
	}
	return true, true
}

func (t *SweepTask) listOrphans(kt *kit.Kit, expr *filter.Expression, limit uint) (
	*dsaiagent.ListAiagentRunResult, error) {

	// 始终 start=0：CAS 成功后该行不再匹配 running，剩余孤儿会滑到队头。
	result, err := t.cli.DataService().Aiagent.Run.List(kt, &core.ListReq{
		Filter: expr,
		Page: &core.BasePage{
			Start: 0, Limit: limit, Sort: "updated_at", Order: core.Ascending,
		},
	})
	if err != nil {
		logs.Errorf("list orphan aiagent runs failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	return result, nil
}

// hasOrphanLeftover 在触达单轮上限后 Count 一次，确认过滤条件下是否还有行。
// Count 失败时按还有剩余处理，避免漏报积压。
func (t *SweepTask) hasOrphanLeftover(kt *kit.Kit, expr *filter.Expression) (bool, error) {
	result, err := t.cli.DataService().Aiagent.Run.List(kt, &core.ListReq{
		Filter: expr,
		Page:   core.NewCountPage(),
	})
	if err != nil {
		logs.Errorf("count leftover orphan aiagent runs failed, err: %v, rid: %s", err, kt.Rid)
		return true, nil
	}
	return result.Count > 0, nil
}

// nextSweepLimit 本页 List 条数：不超过 sweepPageLimit，且不超过本轮剩余额度。
func nextSweepLimit(scanned int) int {
	remain := maxSweepPerRound - scanned
	if remain <= 0 {
		return 0
	}
	if remain > sweepPageLimit {
		return sweepPageLimit
	}
	return remain
}

// sweepPageOutcome 根据本页结果决定是否结束本轮。
// 短页说明已经扫完；满页且达到上限则提示可能还有剩余，由 Count 确认。
func sweepPageOutcome(listed, limit, scanned int) (done, leftoverHint bool) {
	if listed == 0 || listed < limit {
		return true, false
	}
	if scanned >= maxSweepPerRound {
		return true, true
	}
	return false, false
}

// NewSweepCronTask returns the sweep task when enabled, otherwise nil.
func NewSweepCronTask(cli *client.ClientSet, onHit OnCASHit) (croncore.Task, error) {
	return newSweepCronTask(cc.AgentServer().AiagentRunSweep, cli, onHit)
}

func newSweepCronTask(cfg cc.AiagentRunSweepConfig, cli *client.ClientSet, onHit OnCASHit) (
	croncore.Task, error) {

	if !cfg.IsEnabled() {
		return nil, nil
	}
	interval := cfg.IntervalDur()
	if interval <= 0 {
		return nil, errf.New(errf.InvalidParameter, "invalid aiagentRunSweep.interval")
	}
	return &SweepTask{cli: cli, interval: interval, onHit: onHit}, nil
}
