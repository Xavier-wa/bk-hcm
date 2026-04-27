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

package logics

import (
	"context"
	"fmt"
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

var _ memory.Service = (*loggingMemoryService)(nil)

const memoryContentLogLimit = 500

// loggingMemoryService wraps a memory.Service with structured logging for
// observability into memory extraction, storage, and retrieval operations.
type loggingMemoryService struct {
	inner memory.Service
}

func newLoggingMemoryService(inner memory.Service) *loggingMemoryService {
	return &loggingMemoryService{inner: inner}
}

func (s *loggingMemoryService) AddMemory(ctx context.Context, userKey memory.UserKey,
	memoryStr string, topics []string, opts ...memory.AddOption) error {

	logs.Infof("[memory:add] user=%q app=%q topics=%v content=%q",
		userKey.UserID, userKey.AppName, topics, truncate(memoryStr, memoryContentLogLimit))

	err := s.inner.AddMemory(ctx, userKey, memoryStr, topics, opts...)
	if err != nil {
		logs.Errorf("[memory:add] user=%q FAILED: %v", userKey.UserID, err)
	} else {
		logs.Infof("[memory:add] user=%q stored OK", userKey.UserID)
	}
	return err
}

func (s *loggingMemoryService) UpdateMemory(ctx context.Context, memoryKey memory.Key,
	memoryStr string, topics []string, opts ...memory.UpdateOption) error {

	logs.Infof("[memory:update] user=%q memoryID=%q topics=%v content=%q",
		memoryKey.UserID, memoryKey.MemoryID, topics, truncate(memoryStr, memoryContentLogLimit))

	err := s.inner.UpdateMemory(ctx, memoryKey, memoryStr, topics, opts...)
	if err != nil {
		logs.Errorf("[memory:update] user=%q memoryID=%q FAILED: %v", memoryKey.UserID, memoryKey.MemoryID, err)
	} else {
		logs.Infof("[memory:update] user=%q memoryID=%q updated OK", memoryKey.UserID, memoryKey.MemoryID)
	}
	return err
}

func (s *loggingMemoryService) DeleteMemory(ctx context.Context, memoryKey memory.Key) error {
	logs.Infof("[memory:delete] user=%q memoryID=%q", memoryKey.UserID, memoryKey.MemoryID)
	err := s.inner.DeleteMemory(ctx, memoryKey)
	if err != nil {
		logs.Errorf("[memory:delete] user=%q memoryID=%q FAILED: %v", memoryKey.UserID, memoryKey.MemoryID, err)
	}
	return err
}

func (s *loggingMemoryService) ClearMemories(ctx context.Context, userKey memory.UserKey) error {
	logs.Infof("[memory:clear] user=%q app=%q", userKey.UserID, userKey.AppName)
	err := s.inner.ClearMemories(ctx, userKey)
	if err != nil {
		logs.Errorf("[memory:clear] user=%q FAILED: %v", userKey.UserID, err)
	}
	return err
}

func (s *loggingMemoryService) ReadMemories(ctx context.Context, userKey memory.UserKey,
	limit int) ([]*memory.Entry, error) {

	logs.Infof("[memory:read] user=%q app=%q limit=%d", userKey.UserID, userKey.AppName, limit)

	entries, err := s.inner.ReadMemories(ctx, userKey, limit)
	if err != nil {
		logs.Errorf("[memory:read] user=%q FAILED: %v", userKey.UserID, err)
		return nil, err
	}

	logs.Infof("[memory:read] user=%q returned %d entries", userKey.UserID, len(entries))
	for i, e := range entries {
		logs.Infof("[memory:read]   [%d] id=%q kind=%q topics=%v updated=%v content=%q",
			i, e.ID, e.Memory.Kind, e.Memory.Topics, e.UpdatedAt.Format(constant.TimeStdFormat),
			truncate(e.Memory.Memory, memoryContentLogLimit))
	}
	return entries, nil
}

func (s *loggingMemoryService) SearchMemories(ctx context.Context, userKey memory.UserKey,
	query string, opts ...memory.SearchOption) ([]*memory.Entry, error) {

	logs.Infof("[memory:search] user=%q app=%q query=%q",
		userKey.UserID, userKey.AppName, truncate(query, memoryContentLogLimit))

	entries, err := s.inner.SearchMemories(ctx, userKey, query, opts...)
	if err != nil {
		logs.Errorf("[memory:search] user=%q query=%q FAILED: %v", userKey.UserID, truncate(query, 100), err)
		return nil, err
	}

	logs.Infof("[memory:search] user=%q returned %d results for query=%q",
		userKey.UserID, len(entries), truncate(query, 100))
	for i, e := range entries {
		scoreInfo := ""
		if e.Score > 0 {
			scoreInfo = fmt.Sprintf(" score=%.4f", e.Score)
		}
		logs.Infof("[memory:search]   [%d]%s id=%q kind=%q topics=%v content=%q",
			i, scoreInfo, e.ID, e.Memory.Kind, e.Memory.Topics,
			truncate(e.Memory.Memory, memoryContentLogLimit))
	}
	return entries, nil
}

func (s *loggingMemoryService) Tools() []tool.Tool {
	return s.inner.Tools()
}

func (s *loggingMemoryService) EnqueueAutoMemoryJob(ctx context.Context, sess *session.Session) error {
	userID := ""
	appName := ""
	eventCount := 0
	if sess != nil {
		userID = sess.UserID
		appName = sess.AppName
		eventCount = len(sess.Events)
	}
	logs.Infof("[memory:enqueue] user=%q app=%q events=%d", userID, appName, eventCount)

	err := s.inner.EnqueueAutoMemoryJob(ctx, sess)
	if err != nil {
		logs.Errorf("[memory:enqueue] user=%q FAILED: %v", userID, err)
	}
	return err
}

func (s *loggingMemoryService) Close() error {
	return s.inner.Close()
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("... (%d chars total)", len(s))
}
