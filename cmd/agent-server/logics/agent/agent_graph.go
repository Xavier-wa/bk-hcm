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

package agent

import (
	"database/sql"
	"fmt"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/enumor"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/graphagent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	ckptinmem "trpc.group/trpc-go/trpc-agent-go/graph/checkpoint/inmemory"
	ckptsqlite "trpc.group/trpc-go/trpc-agent-go/graph/checkpoint/sqlite"
)

// NewGraphAgent creates a graphagent.GraphAgent from a compiled graph.
// The caller is responsible for providing a checkpoint saver (e.g. via BuildCheckpointSaver).
// When subAgents is non-empty, they are registered via graphagent.WithSubAgents so the framework
// can find the execution context for each sub-agent node (e.g. resource_query subgraph).
func NewGraphAgent(name string, compiledGraph *graph.Graph, saver graph.CheckpointSaver,
	subAgents []trpcagent.Agent) (trpcagent.Agent, error) {

	if compiledGraph == nil {
		return nil, fmt.Errorf("compiled graph is nil")
	}
	if saver == nil {
		return nil, fmt.Errorf("checkpoint saver is nil")
	}

	opts := []graphagent.Option{
		graphagent.WithDescription("HCM ReAct graph agent"),
		graphagent.WithCheckpointSaver(saver),
		graphagent.WithInitialState(graph.State{}),
	}
	if len(subAgents) > 0 {
		opts = append(opts, graphagent.WithSubAgents(subAgents))
	}

	gagent, err := graphagent.New(name, compiledGraph, opts...)
	if err != nil {
		return nil, fmt.Errorf("create graph agent: %w", err)
	}

	return gagent, nil
}

// BuildCheckpointSaver creates a CheckpointSaver based on the configuration.
// Supported backends: "inmemory", "sqlite".
func BuildCheckpointSaver(cfg cc.AgentCheckpointStorage) (graph.CheckpointSaver, error) {
	switch cfg.Backend {
	case enumor.GraphCheckpointBackendSQLite:
		db, err := sql.Open("sqlite3", cfg.DBPath)
		if err != nil {
			return nil, fmt.Errorf("open sqlite db %s: %w", cfg.DBPath, err)
		}
		saver, err := ckptsqlite.NewSaver(db)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("create sqlite checkpoint saver: %w", err)
		}
		return saver, nil
	case enumor.GraphCheckpointBackendInMemory:
		return ckptinmem.NewSaver(), nil
	default:
		return nil, fmt.Errorf("unsupported checkpoint backend: %s (supported: inmemory, sqlite)", cfg.Backend)
	}
}
