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
	"encoding/json"
	"sync"
	"time"
	"unicode/utf8"

	"hcm/cmd/agent-server/logics/eval"
	"hcm/pkg/api/core"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

const (
	createWaitTimeout = 5 * time.Second
	// maxRunReasonLen 对齐 aiagent_run.reason VARCHAR(256) / validate:"max=256"（按字符，非字节）。
	maxRunReasonLen = 256
)

// OnCASHit is invoked after a successful running→terminal CAS.
type OnCASHit func(kt *kit.Kit, runID string)

// Ledger 是 aiagent_run 的写端：把一轮 AG-UI 生命周期落到账本。
//
//	RUN_STARTED              → 插入 running 行
//	中间事件                 → 堆进 RunMeta，终态时收成 query + transcript
//	RUN_FINISHED / RUN_ERROR → CAS running→终态，与 transcript 同一条 UPDATE；成功才 onHit
//	/cancel、孤儿清扫        → 只改 status，不写 transcript；CAS 成功同样 onHit
//
// eval.enabled=false 时仍写账本，submitEval 直接返回。
type Ledger struct {
	cli   *client.ClientSet
	onHit OnCASHit
}

var (
	ledgerMu sync.RWMutex
	ledger   *Ledger
)

// SetLedger installs the process-wide ledger used by ObserveEvents.
func SetLedger(l *Ledger) {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	ledger = l
}

func currentLedger() *Ledger {
	ledgerMu.RLock()
	defer ledgerMu.RUnlock()
	return ledger
}

// NewLedger creates a Ledger.
func NewLedger(cli *client.ClientSet, onHit OnCASHit) *Ledger {
	return &Ledger{cli: cli, onHit: onHit}
}

func (l *Ledger) handle(meta *RunMeta, evt aguievents.Event) {
	if l == nil || meta == nil || evt == nil || meta.RunID == "" {
		return
	}
	switch evt.Type() {
	case aguievents.EventTypeRunStarted:
		go l.createRunningRun(meta)
	case aguievents.EventTypeRunFinished:
		go l.casTerminalRun(meta, enumor.AiagentRunStatusFinished, "")
	case aguievents.EventTypeRunError:
		reason := ""
		if e, ok := evt.(*aguievents.RunErrorEvent); ok && e != nil {
			reason = e.Message
		}
		go l.casTerminalRun(meta, enumor.AiagentRunStatusError, reason)
	default:
		meta.appendEvent(evt)
	}
}

func (l *Ledger) createRunningRun(meta *RunMeta) {
	defer meta.markCreated()
	if l.cli == nil || meta.Kit == nil {
		logs.Errorf("create aiagent run skipped, client or kit is nil, run_id: %s, rid: %s",
			meta.RunID, meta.Rid)
		return
	}
	kt := meta.Kit.NewSubKit()
	_, err := l.cli.DataService().Aiagent.Run.Create(kt, &dsaiagent.CreateAiagentRunReq{
		RunID:       meta.RunID,
		SessionCode: meta.SessionCode,
		User:        meta.User,
		BkBizID:     meta.BkBizID,
		Scene:       enumor.NormalizeRunScene(enumor.IntentType(meta.SceneForMetric())),
		Query:       meta.Query,
	})
	if err != nil {
		if errf.Error(err).Code == errf.RecordDuplicated {
			logs.Infof("create aiagent run duplicated, run_id: %s, rid: %s", meta.RunID, kt.Rid)
			return
		}
		logs.Errorf("create aiagent run failed, err: %v, run_id: %s, rid: %s", err, meta.RunID, kt.Rid)
	}
}

func (l *Ledger) casTerminalRun(meta *RunMeta, status enumor.AiagentRunStatus, reason string) {
	if l.cli == nil || meta.Kit == nil {
		logs.Errorf("cas aiagent run skipped, client or kit is nil, run_id: %s, rid: %s",
			meta.RunID, meta.Rid)
		return
	}
	if !meta.waitCreated(createWaitTimeout) {
		logs.Errorf("cas aiagent run skipped, create not ready, run_id: %s, rid: %s",
			meta.RunID, meta.Rid)
		return
	}
	kt := meta.Kit.NewSubKit()
	// 把本轮缓冲的 AG-UI 事件收成 transcript，与终态 CAS 同一条 UPDATE 写入账本。
	tr := eval.ReduceAGUIEvents(meta.snapshotEvents())
	// SSE 出站事件通常没有 user 气泡；创建时从请求体写入的 query 才是本轮用户首句。
	query := meta.Query
	if query == "" {
		query = eval.FirstUserText(tr)
	}
	raw, err := json.Marshal(tr)
	if err != nil {
		logs.Errorf("marshal transcript failed, err: %v, run_id: %s, rid: %s", err, meta.RunID, kt.Rid)
		raw = []byte(`{"items":[]}`)
	}
	err = l.cli.DataService().Aiagent.Run.UpdateStatus(kt, &dsaiagent.UpdateAiagentRunStatusReq{
		RunID:      meta.RunID,
		Status:     status,
		Reason:     truncateReason(reason),
		Query:      query,
		Transcript: types.JsonField(raw),
	})
	if err != nil {
		if errf.Error(err).Code == errf.RecordNotUpdate {
			logs.Infof("cas aiagent run not update, run_id: %s, status: %s, rid: %s",
				meta.RunID, status, kt.Rid)
			return
		}
		logs.Errorf("cas aiagent run failed, err: %v, run_id: %s, rid: %s", err, meta.RunID, kt.Rid)
		return
	}
	l.submitEval(kt, meta.RunID)
}

// Cancel CAS-updates this run_id, or the latest running row of the session.
func (l *Ledger) Cancel(kt *kit.Kit, runID, sessionCode string) {
	if l == nil || l.cli == nil || kt == nil {
		return
	}
	if runID != "" {
		err := l.cli.DataService().Aiagent.Run.UpdateStatus(kt, &dsaiagent.UpdateAiagentRunStatusReq{
			RunID:  runID,
			Status: enumor.AiagentRunStatusCancel,
			Reason: "user_cancel",
		})
		if err == nil {
			l.submitEval(kt, runID)
			return
		}
		if errf.Error(err).Code != errf.RecordNotUpdate {
			logs.Errorf("cas cancel aiagent run failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
			return
		}
	}
	if sessionCode == "" {
		return
	}
	result, err := l.cli.DataService().Aiagent.Run.List(kt, &core.ListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("session_code", sessionCode),
			tools.RuleEqual("status", string(enumor.AiagentRunStatusRunning)),
		),
		Page: &core.BasePage{Start: 0, Limit: 1, Sort: "created_at", Order: core.Descending},
	})
	if err != nil {
		logs.Errorf("list running aiagent run failed, err: %v, session_code: %s, rid: %s",
			err, sessionCode, kt.Rid)
		return
	}
	if result == nil || len(result.Details) == 0 {
		return
	}
	latest := result.Details[0].RunID
	err = l.cli.DataService().Aiagent.Run.UpdateStatus(kt, &dsaiagent.UpdateAiagentRunStatusReq{
		RunID:  latest,
		Status: enumor.AiagentRunStatusCancel,
		Reason: "user_cancel",
	})
	if err != nil {
		if errf.Error(err).Code == errf.RecordNotUpdate {
			logs.Infof("cas cancel latest running not update, run_id: %s, rid: %s", latest, kt.Rid)
			return
		}
		logs.Errorf("cas cancel latest running failed, err: %v, run_id: %s, rid: %s", err, latest, kt.Rid)
		return
	}
	l.submitEval(kt, latest)
}

func (l *Ledger) submitEval(kt *kit.Kit, runID string) {
	if l.onHit == nil || !cc.AgentServer().Eval.IsEnabled() {
		return
	}
	// 复用 CAS / cancel 的 kit，不再 NewSubKit：入站 UUID(36) 叠三次会超过 Validate 的 50。
	l.onHit(kt, runID)
}

func truncateReason(s string) string {
	if utf8.RuneCountInString(s) <= maxRunReasonLen {
		return s
	}
	return string([]rune(s)[:maxRunReasonLen])
}
