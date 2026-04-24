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

// Package framework provides the core data migration framework
package framework

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// MigrationRequest 通用迁移请求
type MigrationRequest struct {
	// 表配置名称
	TableName string `json:"table_name" validate:"required"`

	// 执行方向，默认 forward（mongodb -> mysql）
	Direction MigrationDirection `json:"direction,omitempty"`

	// 任务ID（用于日志追踪与多次执行排障）
	TaskID string `json:"task_id,omitempty"`

	// 通用过滤条件
	Filter MigrationFilter `json:"filter"`

	// 迁移模式
	Mode MigrationMode `json:"mode" validate:"required"`

	// 是否DryRun（只分析不执行）
	DryRun bool `json:"dry_run"`

	// 性能控制
	BatchSize   int `json:"batch_size"`  // 业务批次大小
	Concurrency int `json:"concurrency"` // 并发数

	// 级联选项
	IncludeDependencies bool `json:"include_dependencies"` // 是否包含依赖表

	// 循环依赖检测（内部使用，不对外暴露）
	visitedTables map[string]bool `json:"-"` // 已访问的表（用于检测循环依赖）
}

// getVisitedPath 获取已访问的表路径（用于日志输出）
func (r *MigrationRequest) getVisitedPath() []string {
	if r.visitedTables == nil {
		return []string{}
	}
	path := make([]string, 0, len(r.visitedTables))
	for table := range r.visitedTables {
		path = append(path, table)
	}
	return path
}

// MigrationFilter 通用过滤器
type MigrationFilter struct {
	// 时间范围过滤
	TimeRange *TimeRangeFilter `json:"time_range,omitempty"`

	// 通用字段过滤
	Fields map[string]interface{} `json:"fields,omitempty"`

	// 自定义MongoDB查询
	CustomQuery bson.M `json:"custom_query,omitempty"`
}

// TimeRangeFilter 时间范围过滤器
type TimeRangeFilter struct {
	Field     string     `json:"field" validate:"required"` // 字段名，如 "create_at"
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}

// MigrationMode 迁移模式
type MigrationMode string

const (
	// MigrationModeCreateOnly 只创建，跳过已存在
	MigrationModeCreateOnly MigrationMode = "create_only"
	// MigrationModeUpdateOnly 只更新已存在
	MigrationModeUpdateOnly MigrationMode = "update_only"
	// MigrationModeSync 同步（创建+更新）
	MigrationModeSync MigrationMode = "sync"
)

// MigrationDirection 迁移方向
type MigrationDirection string

const (
	// MigrationDirectionForward 正向同步：mongodb -> mysql
	MigrationDirectionForward MigrationDirection = "forward"
	// MigrationDirectionReverse 反向同步：mysql -> mongodb
	MigrationDirectionReverse MigrationDirection = "reverse"
)

// MigrationResult 迁移结果
type MigrationResult struct {
	// 表信息
	TableName  string `json:"table_name,omitempty"`  // 表名
	BatchIndex int    `json:"batch_index,omitempty"` // 批次编号（用于并发处理）

	// 基本统计
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`

	// 操作统计
	Created int `json:"created"`
	Updated int `json:"updated"`

	// 详细信息
	Duration  time.Duration `json:"duration"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`

	// 错误详情
	Errors []*ErrorDetail `json:"errors,omitempty"`

	// 依赖表统计
	Dependencies map[string]*MigrationResult `json:"dependencies,omitempty"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	PrimaryKey interface{} `json:"primary_key"`
	Table      string      `json:"table"`
	Action     string      `json:"action"` // create, update
	Error      string      `json:"error"`
	SourceData interface{} `json:"source_data,omitempty"`
}

// Merge 合并另一个MigrationResult
func (r *MigrationResult) Merge(other *MigrationResult) {
	r.Total += other.Total
	r.Success += other.Success
	r.Failed += other.Failed
	r.Skipped += other.Skipped
	r.Created += other.Created
	r.Updated += other.Updated
	r.Errors = append(r.Errors, other.Errors...)
}

// MergeDependencies 合并依赖表结果
func (r *MigrationResult) MergeDependencies(deps map[string]*MigrationResult) {
	if r.Dependencies == nil {
		r.Dependencies = make(map[string]*MigrationResult)
	}
	for name, result := range deps {
		r.Dependencies[name] = result
	}
}

// CompareResult 数据对比结果
type CompareResult struct {
	IsEqual    bool                  `json:"is_equal"`
	DiffFields map[string]*FieldDiff `json:"diff_fields,omitempty"`
}

// FieldDiff 字段差异
type FieldDiff struct {
	SourceValue interface{} `json:"source_value"`
	TargetValue interface{} `json:"target_value"`
}

// UpdateStrategy 更新策略
type UpdateStrategy string

const (
	// UpdateStrategyFull 全量更新（覆盖所有字段）
	UpdateStrategyFull UpdateStrategy = "full"
	// UpdateStrategyIncremental 增量更新（只更新差异字段）
	UpdateStrategyIncremental UpdateStrategy = "incremental"
	// UpdateStrategyMerge 合并更新（源数据优先，但保留目标独有字段）
	UpdateStrategyMerge UpdateStrategy = "merge"
)
