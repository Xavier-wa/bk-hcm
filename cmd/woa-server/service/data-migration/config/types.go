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

// Package config provides table migration configurations
package config

import (
	"fmt"
	"sync"
)

// TableMigrationConfig 表迁移配置
type TableMigrationConfig struct {
	// 基本信息
	Name        string `json:"name"`        // 配置名称
	Description string `json:"description"` // 描述
	Enabled     bool   `json:"enabled"`     // 是否启用

	// 源表（MongoDB）
	SourceDB    string `json:"source_db"`    // mongodb
	SourceTable string `json:"source_table"` // cr_ApplyTicket

	// 目标表（MySQL）
	TargetDB    string `json:"target_db"`    // mysql
	TargetTable string `json:"target_table"` // ziyan_cvm_apply_order

	// 主键配置（支持单字段和多字段联合唯一键）
	SourcePrimaryKeys []string `json:"source_primary_keys"` // ["order_id"] 或 ["suborder_id", "step_name"]
	TargetPrimaryKeys []string `json:"target_primary_keys"` // ["order_id"] 或 ["suborder_id", "step_name"]

	// 转换器名称
	ConverterName string `json:"converter_name"` // cvm_apply_order_converter

	// 对比字段（用于判断是否需要更新）
	CompareFields []string `json:"compare_fields"` // [stage, status, total_num]

	// 依赖关系（级联迁移）
	Dependencies []*DependencyConfig `json:"dependencies,omitempty"`

	// 性能配置
	BatchSize   int `json:"batch_size"`  // 批次大小
	Concurrency int `json:"concurrency"` // 并发数
}

// DependencyConfig 依赖关系配置
type DependencyConfig struct {
	ConfigName   string `json:"config_name"`   // 依赖的配置名称
	RelationType string `json:"relation_type"` // one_to_many, many_to_one
	ForeignKey   string `json:"foreign_key"`   // 外键字段名
}

// TableMappingRegistry 表映射配置注册表
type TableMappingRegistry struct {
	configs map[string]*TableMigrationConfig
	mu      sync.RWMutex
}

// globalRegistry 全局配置注册表
var globalRegistry = &TableMappingRegistry{
	configs: make(map[string]*TableMigrationConfig),
}

// Register 注册表映射配置
func Register(config *TableMigrationConfig) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.configs[config.Name] = config
}

// Get 获取表映射配置
func Get(name string) (*TableMigrationConfig, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	config, ok := globalRegistry.configs[name]
	if !ok {
		return nil, fmt.Errorf("table migration config %s not found", name)
	}

	return config, nil
}

// List 列出所有配置
func List() []*TableMigrationConfig {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	configs := make([]*TableMigrationConfig, 0, len(globalRegistry.configs))
	for _, config := range globalRegistry.configs {
		if config.Enabled {
			configs = append(configs, config)
		}
	}

	return configs
}

// GetSourcePrimaryKeys 获取源表主键字段列表
func (c *TableMigrationConfig) GetSourcePrimaryKeys() []string {
	if len(c.SourcePrimaryKeys) == 0 {
		return []string{"id"} // 默认
	}
	return c.SourcePrimaryKeys
}

// GetTargetPrimaryKeys 获取目标表主键字段列表
func (c *TableMigrationConfig) GetTargetPrimaryKeys() []string {
	if len(c.TargetPrimaryKeys) == 0 {
		return []string{"id"} // 默认
	}
	return c.TargetPrimaryKeys
}

// IsCompositePrimaryKey 判断是否为复合主键（主键字段数量 > 1）
func (c *TableMigrationConfig) IsCompositePrimaryKey() bool {
	return len(c.SourcePrimaryKeys) > 1 || len(c.TargetPrimaryKeys) > 1
}
