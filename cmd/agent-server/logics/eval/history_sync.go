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

package eval

import (
	"encoding/json"
	"time"

	proto "hcm/pkg/api/agent-server/eval"
	"hcm/pkg/api/core"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/slice"
	"hcm/pkg/tools/times"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

const (
	skipReasonEmptyRunID      = "empty run_id"
	skipReasonRunIDTooLong    = "run_id too long"
	skipReasonEmptyTranscript = "empty transcript"
	skipReasonAlreadyExists   = "already exists"
	// aguiTrack is the session track id used by AG-UI RUN_* events.
	// 对应框架表 {prefix}session_track_events，prefix 为 aiagent_ 时即 aiagent_session_track_events。
	aguiTrack session.Track = "agui"
)

// HistorySyncer reconstructs missing aiagent_run rows from session history.
type HistorySyncer struct {
	cli        *client.ClientSet
	sessionSvc session.Service
	appName    string
}

type sessionHistory struct {
	tracks   map[string]SessionTrack
	statuses map[string]enumor.AiagentRunStatus
}

// NewHistorySyncer creates a HistorySyncer.
func NewHistorySyncer(cli *client.ClientSet, sessionSvc session.Service, appName string) *HistorySyncer {
	return &HistorySyncer{cli: cli, sessionSvc: sessionSvc, appName: appName}
}

// Sync inserts missing ledger rows for the requested sessions.
func (s *HistorySyncer) Sync(kt *kit.Kit, req *proto.SyncHistoryReq) (*proto.SyncHistoryResp, error) {
	if s == nil || s.cli == nil {
		return nil, errf.New(errf.Aborted, "history syncer is not initialized")
	}
	if s.sessionSvc == nil {
		return nil, errf.New(errf.Aborted, "session service is not initialized")
	}
	sessions, err := s.listSessions(kt, req)
	if err != nil {
		return nil, err
	}
	resp := &proto.SyncHistoryResp{SessionScanned: uint64(len(sessions))}
	resp.SessionFailed += countMissingSessions(kt, req.SessionCodes, sessions)
	for i := range sessions {
		s.syncSession(kt, &sessions[i], resp)
	}
	logs.Infof("sync history success, scanned: %d, created: %d, skipped: %d, failed: %d, rid: %s",
		resp.SessionScanned, resp.RunCreated, resp.RunSkipped, resp.RunFailed, kt.Rid)
	return resp, nil
}

func (s *HistorySyncer) listSessions(kt *kit.Kit, req *proto.SyncHistoryReq) (
	[]tableaiagent.SessionTable, error) {

	result, err := s.cli.DataService().Aiagent.Session.List(kt, &dsaiagent.ListAiagentSessionReq{
		Filter: tools.ContainersExpression("session_code", req.SessionCodes),
		Page: &core.BasePage{
			Start: 0, Limit: uint(len(req.SessionCodes)), Sort: "created_at", Order: core.Ascending,
		},
	})
	if err != nil {
		logs.Errorf("list sessions for history sync failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.Details, nil
}

func (s *HistorySyncer) syncSession(kt *kit.Kit, sess *tableaiagent.SessionTable,
	resp *proto.SyncHistoryResp) {

	hist := s.loadHistory(kt, sess)
	if hist == nil {
		resp.SessionFailed++
		return
	}
	existing, err := s.listExistingRunIDs(kt, trackRunIDs(hist.tracks))
	if err != nil {
		logs.Warnf("list existing runs failed, err: %v, session_code: %s, rid: %s",
			err, sess.SessionCode, kt.Rid)
		resp.SessionFailed++
		return
	}
	scene := enumor.NormalizeRunScene(sess.SessionTag)
	for runID, track := range hist.tracks {
		reason := skipHistoryTrack(runID, track.Transcript, existing)
		if reason != "" {
			logSkipHistoryRun(kt, reason, runID, sess.SessionCode)
			resp.RunSkipped++
			continue
		}
		status := historyRunStatus(runID, hist.statuses)
		if err := s.createHistoryRun(kt, sess, scene, runID, track, status); err != nil {
			if errf.Error(err).Code == errf.RecordDuplicated {
				resp.RunSkipped++
				continue
			}
			logs.Warnf("create history run failed, err: %v, run_id: %s, session_code: %s, rid: %s",
				err, runID, sess.SessionCode, kt.Rid)
			resp.RunFailed++
			continue
		}
		resp.RunCreated++
	}
}

func (s *HistorySyncer) loadHistory(kt *kit.Kit, sess *tableaiagent.SessionTable) *sessionHistory {
	if sess == nil {
		return nil
	}
	threadID := sess.ThreadID
	if threadID == "" {
		threadID = sess.ID
	}
	if threadID == "" || sess.User == "" {
		logs.Warnf("skip session without thread or user, session_code: %s, rid: %s",
			sess.SessionCode, kt.Rid)
		return nil
	}
	key := session.Key{AppName: s.appName, UserID: sess.User, SessionID: threadID}
	got, err := s.sessionSvc.GetSession(kt.Ctx, key)
	if err != nil || got == nil {
		logs.Warnf("get session track failed, err: %v, session_code: %s, rid: %s",
			err, sess.SessionCode, kt.Rid)
		return nil
	}
	return &sessionHistory{
		tracks:   SplitSessionTracks(got.GetEvents()),
		statuses: statusFromAGUITrackEvents(aguiTrackEvents(got)),
	}
}

func (s *HistorySyncer) listExistingRunIDs(kt *kit.Kit, runIDs []string) (map[string]struct{}, error) {
	existing := make(map[string]struct{})
	runIDs = slice.Unique(runIDs)
	if len(runIDs) == 0 {
		return existing, nil
	}
	for _, batch := range slice.Split(runIDs, int(core.DefaultMaxPageLimit)) {
		result, err := s.cli.DataService().Aiagent.Run.List(kt, &core.ListReq{
			Filter: tools.ContainersExpression("run_id", batch),
			Page: &core.BasePage{
				Start: 0, Limit: uint(len(batch)), Sort: "created_at", Order: core.Ascending,
			},
		})
		if err != nil {
			return nil, err
		}
		if result == nil {
			continue
		}
		for _, row := range result.Details {
			existing[row.RunID] = struct{}{}
		}
	}
	return existing, nil
}

func (s *HistorySyncer) createHistoryRun(kt *kit.Kit, sess *tableaiagent.SessionTable,
	scene enumor.IntentType, runID string, track SessionTrack,
	status enumor.AiagentRunStatus) error {

	raw, err := json.Marshal(track.Transcript)
	if err != nil {
		return err
	}
	req := &dsaiagent.CreateAiagentRunReq{
		RunID:       runID,
		SessionCode: sess.SessionCode,
		User:        sess.User,
		BkBizID:     sess.BkBizID,
		Scene:       scene,
		Query:       FirstUserText(track.Transcript),
		Status:      status,
		Reason:      constant.AiagentRunHistorySyncReason,
		Transcript:  types.JsonField(raw),
		OccurredAt:  formatOccurredAt(track.StartedAt),
		EndedAt:     formatOccurredAt(track.EndedAt),
	}
	_, err = s.cli.DataService().Aiagent.Run.Create(kt, req)
	return err
}

func skipHistoryTrack(runID string, tr Transcript, existing map[string]struct{}) string {
	if runID == "" {
		return skipReasonEmptyRunID
	}
	if len(runID) > constant.AiagentRunIDMaxLen {
		return skipReasonRunIDTooLong
	}
	if len(tr.Items) == 0 {
		return skipReasonEmptyTranscript
	}
	if _, ok := existing[runID]; ok {
		return skipReasonAlreadyExists
	}
	return ""
}

func formatOccurredAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return times.FormatDateTimeCST(t)
}

func countMissingSessions(kt *kit.Kit, codes []string, found []tableaiagent.SessionTable) uint64 {
	exist := make(map[string]struct{}, len(found))
	for i := range found {
		exist[found[i].SessionCode] = struct{}{}
	}
	var n uint64
	for _, code := range codes {
		if _, ok := exist[code]; ok {
			continue
		}
		logs.Warnf("session not found, session_code: %s, rid: %s", code, kt.Rid)
		n++
	}
	return n
}

func trackRunIDs(tracks map[string]SessionTrack) []string {
	runIDs := make([]string, 0, len(tracks))
	for runID := range tracks {
		if runID == "" {
			continue
		}
		runIDs = append(runIDs, runID)
	}
	return runIDs
}

func aguiTrackEvents(sess *session.Session) []session.TrackEvent {
	if sess == nil {
		return nil
	}
	te, err := sess.GetTrackEvents(aguiTrack)
	if err != nil || te == nil {
		return nil
	}
	return te.Events
}

// statusFromAGUITrackEvents maps each run_id to finished/error/unknown from AG-UI track events.
// Last RUN_FINISHED or RUN_ERROR for that run wins; otherwise unknown.
func statusFromAGUITrackEvents(events []session.TrackEvent) map[string]enumor.AiagentRunStatus {
	out := make(map[string]enumor.AiagentRunStatus)
	currentRun := ""
	for i := range events {
		evt, err := aguievents.EventFromJSON(events[i].Payload)
		if err != nil || evt == nil {
			continue
		}
		applyAGUIRunStatus(out, &currentRun, evt)
	}
	return out
}

func applyAGUIRunStatus(out map[string]enumor.AiagentRunStatus, currentRun *string,
	evt aguievents.Event) {

	runID := evt.RunID()
	switch evt.Type() {
	case aguievents.EventTypeRunStarted:
		if runID == "" {
			return
		}
		*currentRun = runID
		if _, exists := out[runID]; !exists {
			out[runID] = enumor.AiagentRunStatusUnknown
		}
	case aguievents.EventTypeRunFinished:
		if runID == "" {
			runID = *currentRun
		}
		if runID != "" {
			out[runID] = enumor.AiagentRunStatusFinished
		}
	case aguievents.EventTypeRunError:
		if runID == "" {
			runID = *currentRun
		}
		if runID != "" {
			out[runID] = enumor.AiagentRunStatusError
		}
	default:
	}
}

func historyRunStatus(runID string, byRun map[string]enumor.AiagentRunStatus) enumor.AiagentRunStatus {
	if st, ok := byRun[runID]; ok && st != "" {
		return st
	}
	return enumor.AiagentRunStatusUnknown
}

func logSkipHistoryRun(kt *kit.Kit, reason, runID, sessionCode string) {
	if reason == skipReasonAlreadyExists {
		logs.V(2).Infof("skip history run, reason: %s, run_id: %s, session_code: %s, rid: %s",
			reason, runID, sessionCode, kt.Rid)
		return
	}
	logs.Infof("skip history run, reason: %s, run_id: %s, session_code: %s, rid: %s",
		reason, runID, sessionCode, kt.Rid)
}
