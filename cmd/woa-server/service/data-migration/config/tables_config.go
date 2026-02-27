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

package config

import (
	"fmt"

	tasktable "hcm/cmd/woa-server/dal/task/table"
	"hcm/pkg"
	"hcm/pkg/dal/table"
)

func init() {
	// 注册CVM申请单相关表配置
	registerCvmApplyBasicTables()
	registerCvmApplyProcessTables()
	registerCvmApplyDeviceTables()
	registerCvmApplyOtherTables()

	// TODO: 后续添加主机回收相关表的迁移配置
}

// registerCvmApplyBasicTables 注册CVM申请单基础表配置（主单、子单）
func registerCvmApplyBasicTables() {
	// 注册CVM申请单主单配置
	Register(&TableMigrationConfig{
		Name: pkg.BKTableNameApplyTicket,
		Description: fmt.Sprintf("CVM申请单主单表-迁移（%s -> %s）",
			pkg.BKTableNameApplyTicket, table.ZiyanCvmApplyOrderTable),
		Enabled:           true,
		SourceDB:          "mongodb",
		SourceTable:       pkg.BKTableNameApplyTicket,
		TargetDB:          "mysql",
		TargetTable:       table.ZiyanCvmApplyOrderTable,
		SourcePrimaryKeys: []string{"order_id"},
		TargetPrimaryKeys: []string{"order_id"},
		ConverterName:     table.ZiyanCvmApplyOrderTable + "_converter",
		CompareFields:     []string{"stage"},
		BatchSize:         100,
		Concurrency:       5,
	})

	// 注册CVM申请单子单配置
	Register(&TableMigrationConfig{
		Name: pkg.BKTableNameApplyOrder,
		Description: fmt.Sprintf("CVM申请单子单表-迁移（%s -> %s）",
			pkg.BKTableNameApplyOrder, table.ZiyanCvmApplySuborderTable),
		Enabled:           true,
		SourceDB:          "mongodb",
		SourceTable:       pkg.BKTableNameApplyOrder,
		TargetDB:          "mysql",
		TargetTable:       table.ZiyanCvmApplySuborderTable,
		SourcePrimaryKeys: []string{"suborder_id"},
		TargetPrimaryKeys: []string{"suborder_id"},
		ConverterName:     table.ZiyanCvmApplySuborderTable + "_converter",
		CompareFields: []string{"stage", "status", "total_num", "success_num", "pending_num", "applied_core",
			"delivered_core", "region", "zone", "image_id", "device_type", "charge_type", "charge_months"},
		BatchSize:   100,
		Concurrency: 5,
	})
}

// registerCvmApplyProcessTables 注册CVM申请单流程相关表配置（步骤、生产记录）
func registerCvmApplyProcessTables() {
	// 注册CVM申请单执行步骤表配置（复合主键：suborder_id + step_name）
	Register(&TableMigrationConfig{
		Name: pkg.BKTableNameApplyStep,
		Description: fmt.Sprintf("CVM申请单执行步骤表-迁移（%s -> %s）",
			pkg.BKTableNameApplyStep, table.ZiyanCvmApplyStepTable),
		Enabled:           true,
		SourceDB:          "mongodb",
		SourceTable:       pkg.BKTableNameApplyStep,
		TargetDB:          "mysql",
		TargetTable:       table.ZiyanCvmApplyStepTable,
		SourcePrimaryKeys: []string{"suborder_id", "step_name"},
		TargetPrimaryKeys: []string{"suborder_id", "step_name"},
		ConverterName:     table.ZiyanCvmApplyStepTable + "_converter",
		CompareFields:     []string{"status", "total_num", "success_num", "failed_num", "running_num"},
		BatchSize:         100,
		Concurrency:       5,
	})

	// 注册CVM申请单生产任务记录表配置
	Register(&TableMigrationConfig{
		Name: pkg.BKTableNameGenerateRecord,
		Description: fmt.Sprintf("CVM申请单生产任务记录表-迁移（%s -> %s）",
			pkg.BKTableNameGenerateRecord, table.ZiyanCvmGenerateRecordTable),
		Enabled:           true,
		SourceDB:          "mongodb",
		SourceTable:       pkg.BKTableNameGenerateRecord,
		TargetDB:          "mysql",
		TargetTable:       table.ZiyanCvmGenerateRecordTable,
		SourcePrimaryKeys: []string{"suborder_id", "generate_id"},
		TargetPrimaryKeys: []string{"suborder_id", "generate_id"},
		ConverterName:     table.ZiyanCvmGenerateRecordTable + "_converter",
		CompareFields:     []string{"status", "is_matched", "total_num", "success_num"},
		BatchSize:         100,
		Concurrency:       5,
	})

	// 注册CVM申请单初始化任务记录表配置（复合主键：suborder_id + ip + task_id）
	Register(&TableMigrationConfig{
		Name: pkg.BKTableNameInitRecord,
		Description: fmt.Sprintf("CVM申请单初始化任务记录表-迁移（%s -> %s）",
			pkg.BKTableNameInitRecord, table.ZiyanCvmApplyInitTaskTable),
		Enabled:           true,
		SourceDB:          "mongodb",
		SourceTable:       pkg.BKTableNameInitRecord,
		TargetDB:          "mysql",
		TargetTable:       table.ZiyanCvmApplyInitTaskTable,
		SourcePrimaryKeys: []string{"suborder_id", "ip", "task_id"},
		TargetPrimaryKeys: []string{"suborder_id", "ip", "task_id"},
		ConverterName:     table.ZiyanCvmApplyInitTaskTable + "_converter",
		CompareFields:     []string{"status", "message", "task_id", "task_link"},
		BatchSize:         100,
		Concurrency:       5,
	})
}

// registerCvmApplyDeviceTables 注册CVM申请单设备相关表配置（初始化、交付、设备信息）
func registerCvmApplyDeviceTables() {
	// 注册CVM设备交付记录表配置（复合主键：order_id + suborder_id + generate_id + ip + asset_id）
	Register(&TableMigrationConfig{
		Name: pkg.BKTableNameDeviceInfo,
		Description: fmt.Sprintf("CVM设备交付记录表-迁移（%s -> %s）",
			pkg.BKTableNameDeviceInfo, table.ZiyanCvmDeviceInfoTable),
		Enabled:           true,
		SourceDB:          "mongodb",
		SourceTable:       pkg.BKTableNameDeviceInfo,
		TargetDB:          "mysql",
		TargetTable:       table.ZiyanCvmDeviceInfoTable,
		SourcePrimaryKeys: []string{"order_id", "suborder_id", "generate_id", "ip", "asset_id"},
		TargetPrimaryKeys: []string{"order_id", "suborder_id", "generate_id", "ip", "asset_id"},
		ConverterName:     table.ZiyanCvmDeviceInfoTable + "_converter",
		CompareFields: []string{"is_matched", "is_inited", "is_delivered", "generate_task_id",
			"init_task_id", "device_type", "resource_type"},
		BatchSize:   100,
		Concurrency: 5,
	})

	// 注册CVM设备交付任务记录表配置（复合主键：suborder_id + ip）
	Register(&TableMigrationConfig{
		Name: pkg.BKTableNameDeliverRecord,
		Description: fmt.Sprintf("CVM设备交付任务记录表-迁移（%s -> %s）",
			pkg.BKTableNameDeliverRecord, table.ZiyanCvmDeliverRecordTable),
		Enabled:           true,
		SourceDB:          "mongodb",
		SourceTable:       pkg.BKTableNameDeliverRecord,
		TargetDB:          "mysql",
		TargetTable:       table.ZiyanCvmDeliverRecordTable,
		SourcePrimaryKeys: []string{"suborder_id", "ip"},
		TargetPrimaryKeys: []string{"suborder_id", "ip"},
		ConverterName:     table.ZiyanCvmDeliverRecordTable + "_converter",
		CompareFields:     []string{"status", "message", "generate_task_id", "init_task_id"},
		BatchSize:         100,
		Concurrency:       5,
	})

	// 注册CVM变更记录表配置（单字段主键：id）
	Register(&TableMigrationConfig{
		Name: tasktable.ModifyRecordTable,
		Description: fmt.Sprintf("CVM变更记录表-迁移（%s -> %s）",
			tasktable.ModifyRecordTable, table.ZiyanCvmModifyRecordTable),
		Enabled:           true,
		SourceDB:          "mongodb",
		SourceTable:       tasktable.ModifyRecordTable,
		TargetDB:          "mysql",
		TargetTable:       table.ZiyanCvmModifyRecordTable,
		SourcePrimaryKeys: []string{"id"},
		TargetPrimaryKeys: []string{"id"},
		ConverterName:     table.ZiyanCvmModifyRecordTable + "_converter",
		CompareFields: []string{
			"status", "approver", "pre_total_num", "pre_replicas", "pre_region", "pre_zone", "pre_device_type",
			"pre_vpc", "pre_subnet", "cur_total_num", "cur_replicas", "cur_region", "cur_zone", "cur_device_type",
			"cur_vpc", "cur_subnet",
		},
		BatchSize:   100,
		Concurrency: 5,
	})
}

// registerCvmApplyOtherTables 注册CVM申请单其他表配置（变更记录、生产申请单）
func registerCvmApplyOtherTables() {
	// 注册CVM申请单（cr_CvmApplyOrder）配置（单字段主键：order_id -> suborder_id）
	Register(&TableMigrationConfig{
		Name: pkg.BKTableNameCvmApplyOrder,
		Description: fmt.Sprintf("CVM生产申请单表-迁移（%s -> %s）",
			pkg.BKTableNameCvmApplyOrder, table.ZiyanCvmApplySuborderTable),
		Enabled:           true,
		SourceDB:          "mongodb",
		SourceTable:       pkg.BKTableNameCvmApplyOrder,
		TargetDB:          "mysql",
		TargetTable:       table.ZiyanCvmApplySuborderTable,
		SourcePrimaryKeys: []string{"order_id"},
		TargetPrimaryKeys: []string{"suborder_id"},
		ConverterName:     pkg.BKTableNameCvmApplyOrder + "_converter",
		CompareFields: []string{
			"status", "total_num", "success_num", "pending_num", "failed_num", "region", "zone",
		},
		BatchSize:   100,
		Concurrency: 5,
	})

	// 注册CVM设备信息表配置（复合主键：order_id + ip）
	Register(&TableMigrationConfig{
		Name: pkg.BKTableNameCvmInfo,
		Description: fmt.Sprintf("CVM设备信息表-迁移（%s -> %s）",
			pkg.BKTableNameCvmInfo, table.ZiyanCvmDeviceInfoTable),
		Enabled:           true,
		SourceDB:          "mongodb",
		SourceTable:       pkg.BKTableNameCvmInfo,
		TargetDB:          "mysql",
		TargetTable:       table.ZiyanCvmDeviceInfoTable,
		SourcePrimaryKeys: []string{"order_id", "ip"},
		TargetPrimaryKeys: []string{"order_id", "ip"},
		ConverterName:     pkg.BKTableNameCvmInfo + "_converter",
		CompareFields:     []string{"order_id", "ip", "asset_id"},
		BatchSize:         100,
		Concurrency:       5,
	})
}
