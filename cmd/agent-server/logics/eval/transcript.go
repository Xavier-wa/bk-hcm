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

package eval

import (
	"encoding/json"
	"strings"
	"time"

	"hcm/pkg/criteria/enumor"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

type textBuf struct {
	role string
	text strings.Builder
}

type toolBuf struct {
	name string
	args strings.Builder
	res  string
}

// ReduceAGUIEvents converts a time-ordered AG-UI event slice into transcript items.
// THINKING_* events are dropped. Multiple assistant bubbles stay multiple items.
func ReduceAGUIEvents(evts []aguievents.Event) Transcript {
	r := newAGUIReducer()
	for _, evt := range evts {
		r.apply(evt)
	}
	r.flushPendingTexts()
	return r.out
}

type aguiReducer struct {
	out   Transcript
	texts map[string]*textBuf
	tools map[string]*toolBuf
	order []string
}

func newAGUIReducer() *aguiReducer {
	return &aguiReducer{
		out:   Transcript{Items: make([]TranscriptItem, 0)},
		texts: make(map[string]*textBuf),
		tools: make(map[string]*toolBuf),
		order: make([]string, 0),
	}
}

func (r *aguiReducer) apply(evt aguievents.Event) {
	if evt == nil || isThinkingAGUIEvent(evt.Type()) {
		return
	}
	switch evt.Type() {
	case aguievents.EventTypeTextMessageStart:
		r.onTextStart(evt)
	case aguievents.EventTypeTextMessageContent:
		r.onTextContent(evt)
	case aguievents.EventTypeTextMessageEnd:
		r.onTextEnd(evt)
	case aguievents.EventTypeToolCallStart:
		r.onToolStart(evt)
	case aguievents.EventTypeToolCallArgs:
		r.onToolArgs(evt)
	case aguievents.EventTypeToolCallResult:
		r.onToolResult(evt)
	case aguievents.EventTypeCustom:
		r.onCustom(evt)
	default:
	}
}

func isThinkingAGUIEvent(typ aguievents.EventType) bool {
	switch typ {
	case aguievents.EventTypeThinkingStart, aguievents.EventTypeThinkingEnd,
		aguievents.EventTypeThinkingTextMessageStart, aguievents.EventTypeThinkingTextMessageContent,
		aguievents.EventTypeThinkingTextMessageEnd:
		return true
	default:
		return false
	}
}

func (r *aguiReducer) onTextStart(evt aguievents.Event) {
	e, _ := evt.(*aguievents.TextMessageStartEvent)
	if e == nil {
		return
	}
	role := "assistant"
	if e.Role != nil && *e.Role != "" {
		role = *e.Role
	}
	r.texts[e.MessageID] = &textBuf{role: role}
	r.order = append(r.order, e.MessageID)
}

func (r *aguiReducer) onTextContent(evt aguievents.Event) {
	e, _ := evt.(*aguievents.TextMessageContentEvent)
	if e == nil {
		return
	}
	r.text(e.MessageID).text.WriteString(e.Delta)
}

func (r *aguiReducer) onTextEnd(evt aguievents.Event) {
	e, _ := evt.(*aguievents.TextMessageEndEvent)
	if e == nil {
		return
	}
	r.flushText(e.MessageID)
}

func (r *aguiReducer) onToolStart(evt aguievents.Event) {
	e, _ := evt.(*aguievents.ToolCallStartEvent)
	if e == nil {
		return
	}
	r.tools[e.ToolCallID] = &toolBuf{name: e.ToolCallName}
}

func (r *aguiReducer) onToolArgs(evt aguievents.Event) {
	e, _ := evt.(*aguievents.ToolCallArgsEvent)
	if e == nil {
		return
	}
	r.tool(e.ToolCallID).args.WriteString(e.Delta)
}

func (r *aguiReducer) onToolResult(evt aguievents.Event) {
	e, _ := evt.(*aguievents.ToolCallResultEvent)
	if e == nil {
		return
	}
	tb := r.tool(e.ToolCallID)
	tb.res = e.Content
	r.out.Items = append(r.out.Items, TranscriptItem{
		Type:       enumor.AiagentTranscriptItemTool,
		ToolName:   tb.name,
		ToolArgs:   tb.args.String(),
		ToolResult: tb.res,
	})
	delete(r.tools, e.ToolCallID)
}

func (r *aguiReducer) onCustom(evt aguievents.Event) {
	e, _ := evt.(*aguievents.CustomEvent)
	if e == nil {
		return
	}
	r.out.Items = append(r.out.Items, TranscriptItem{
		Type:  enumor.AiagentTranscriptItemCustom,
		Name:  e.Name,
		Value: e.Value,
	})
}

func (r *aguiReducer) text(id string) *textBuf {
	if _, ok := r.texts[id]; !ok {
		r.texts[id] = &textBuf{role: "assistant"}
	}
	return r.texts[id]
}

func (r *aguiReducer) tool(id string) *toolBuf {
	if _, ok := r.tools[id]; !ok {
		r.tools[id] = &toolBuf{}
	}
	return r.tools[id]
}

func (r *aguiReducer) flushText(id string) {
	buf, ok := r.texts[id]
	if !ok {
		return
	}
	item := TranscriptItem{Text: buf.text.String()}
	switch buf.role {
	case "user":
		item.Type = enumor.AiagentTranscriptItemUser
	default:
		item.Type = enumor.AiagentTranscriptItemAssistant
	}
	if item.Text != "" {
		r.out.Items = append(r.out.Items, item)
	}
	delete(r.texts, id)
}

func (r *aguiReducer) flushPendingTexts() {
	for _, id := range r.order {
		r.flushText(id)
	}
}

// FirstUserText returns the first user bubble text.
func FirstUserText(t Transcript) string {
	for _, item := range t.Items {
		if item.Type == enumor.AiagentTranscriptItemUser && item.Text != "" {
			return item.Text
		}
	}
	return ""
}

// LastAssistantText returns the last assistant bubble, truncated to limit runes.
func LastAssistantText(t Transcript, limit int) string {
	text := ""
	for _, item := range t.Items {
		if item.Type == enumor.AiagentTranscriptItemAssistant && item.Text != "" {
			text = item.Text
		}
	}
	return truncateRunes(text, limit)
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "..."
}

// SessionTrack is one invocation reconstructed from the session event log.
type SessionTrack struct {
	Transcript Transcript
	StartedAt  time.Time
	EndedAt    time.Time
}

// SplitSessionEvents groups framework events by InvocationID into transcripts.
func SplitSessionEvents(events []event.Event) map[string]Transcript {
	tracks := SplitSessionTracks(events)
	out := make(map[string]Transcript, len(tracks))
	for id, track := range tracks {
		out[id] = track.Transcript
	}
	return out
}

// SplitSessionTracks groups framework events by InvocationID and records time bounds.
func SplitSessionTracks(events []event.Event) map[string]SessionTrack {
	out := make(map[string]SessionTrack)
	for i := range events {
		e := &events[i]
		if e.InvocationID == "" || e.Response == nil {
			continue
		}
		items := transcriptFromResponse(e)
		if len(items) == 0 {
			continue
		}
		cur := out[e.InvocationID]
		cur.Transcript.Items = append(cur.Transcript.Items, items...)
		cur.StartedAt, cur.EndedAt = mergeTrackTimes(cur.StartedAt, cur.EndedAt, e.Timestamp)
		out[e.InvocationID] = cur
	}
	return out
}

func mergeTrackTimes(startedAt, endedAt, ts time.Time) (time.Time, time.Time) {
	if ts.IsZero() {
		return startedAt, endedAt
	}
	if startedAt.IsZero() || ts.Before(startedAt) {
		startedAt = ts
	}
	if endedAt.IsZero() || ts.After(endedAt) {
		endedAt = ts
	}
	return startedAt, endedAt
}

func transcriptFromResponse(e *event.Event) []TranscriptItem {
	items := make([]TranscriptItem, 0)
	for _, choice := range e.Choices {
		msg := choice.Message
		switch msg.Role {
		case trpcmodel.RoleUser:
			if msg.Content != "" {
				items = append(items, TranscriptItem{
					Type: enumor.AiagentTranscriptItemUser, Text: msg.Content,
				})
			}
		case trpcmodel.RoleAssistant:
			if msg.Content != "" {
				items = append(items, TranscriptItem{
					Type: enumor.AiagentTranscriptItemAssistant, Text: msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				items = append(items, TranscriptItem{
					Type:     enumor.AiagentTranscriptItemTool,
					ToolName: tc.Function.Name,
					ToolArgs: string(tc.Function.Arguments),
				})
			}
		case trpcmodel.RoleTool:
			items = append(items, TranscriptItem{
				Type:       enumor.AiagentTranscriptItemTool,
				ToolResult: msg.Content,
			})
		default:
		}
	}
	return items
}

func decodeTranscript(raw string) Transcript {
	if raw == "" || raw == "{}" || raw == "null" {
		return Transcript{}
	}
	var t Transcript
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return Transcript{}
	}
	return t
}
