# Spec：预测需求聚合接口 - 带可通配机型明细

## 1. 概述

提供一个新的聚合接口，一次请求返回预测余量数据及其对应的全部可通配机型明细，减少 AI Agent 的调用次数。

## 2. 接口规格

### 2.1 基本信息

- **路径**: `POST /api/v1/woa/plans/resources/demands/list_with_device_types`
- **描述**: 查询预测需求列表，包含每条预测可通配的机型明细
- **Tags**: SCR, ResPlan

### 2.2 请求参数

```yaml
requestBody:
  required: true
  content:
    application/json:
      schema:
        type: object
        required:
          - expect_time_range
          - page
        properties:
          bk_biz_ids:
            type: array
            items:
              type: integer
            description: 业务ID列表，最大100
          demand_ids:
            type: array
            items:
              type: string
            description: 预测需求ID列表，最大100
          obs_projects:
            type: array
            items:
              type: string
            description: OBS项目类型列表，最大100
          demand_classes:
            type: array
            items:
              type: string
            description: 预测需求类型列表，最大100
          core_types:
            type: array
            items:
              type: string
            description: 核心类型列表，最大100
          device_families:
            type: array
            items:
              type: string
            description: 机型族列表，最大100
          device_classes:
            type: array
            items:
              type: string
            description: 机型分类列表，最大100
          device_types:
            type: array
            items:
              type: string
            description: 机型规格列表，最大100
          region_ids:
            type: array
            items:
              type: string
            description: 地区/城市ID列表，最大100
          zone_ids:
            type: array
            items:
              type: string
            description: 可用区ID列表，最大100
          plan_types:
            type: array
            items:
              type: string
            description: 计划类型列表，最大100
          expiring_only:
            type: boolean
            description: 是否只查即将过期需求
          expect_time_range:
            type: object
            required:
              - start
              - end
            properties:
              start:
                type: string
                description: 起始时间 YYYY-MM-DD
              end:
                type: string
                description: 结束时间 YYYY-MM-DD
          statuses:
            type: array
            items:
              type: string
            enum: [can_apply, not_ready, expired, spent_all, locked]
            description: 状态列表，最大5
          page:
            type: object
            required:
              - count
            properties:
              count:
                type: boolean
                description: true=仅返回总数; false=返回详情
              start:
                type: integer
                description: 起始位置，从0开始
              limit:
                type: integer
                description: 每页限制，最大500
              sort:
                type: string
                description: 排序字段
              order:
                type: string
                enum: [ASC, DESC]
                description: 排序顺序
```

### 2.3 响应结构

```yaml
responses:
  '200':
    description: 成功响应
    content:
      application/json:
        schema:
          type: object
          properties:
            code:
              type: integer
              example: 0
            message:
              type: string
              example: success
            data:
              type: object
              properties:
                count:
                  type: integer
                  description: 预测记录总数（仅在 count=true 时返回）
                details:
                  type: array
                  description: 预测详情列表（仅在 count=false 时返回）
                  items:
                    $ref: '#/components/schemas/DemandWithDeviceTypes'
```

### 2.4 数据模型

#### DemandWithDeviceTypes

```yaml
components:
  schemas:
    DemandWithDeviceTypes:
      type: object
      properties:
        # --- 预测需求主数据 ---
        demand_id:
          type: string
          description: 预测需求ID
        bk_biz_id:
          type: integer
          description: 业务ID
        bk_biz_name:
          type: string
          description: 业务名称
        status:
          type: string
          enum: [can_apply, not_ready, expired, spent_all, locked]
          description: 需求状态
        status_name:
          type: string
          description: 需求状态名称
        demand_class:
          type: string
          description: 预测需求类型
        demand_res_type:
          type: string
          description: 预测资源类型
        expect_time:
          type: string
          description: 期望交付日期 YYYY-MM-DD
        can_apply_time:
          type: string
          description: 可申领时间
        expired_time:
          type: string
          description: 预测申领截止日期
        return_plan_time:
          type: string
          description: 预期退回时间（短租项目）
        total_os:
          type: string
          description: 总OS数量
        applied_os:
          type: string
          description: 已申请OS数量
        remained_os:
          type: string
          description: 剩余OS数量
        total_cpu_core:
          type: integer
          description: 总CPU核数
        applied_cpu_core:
          type: integer
          description: 已申请CPU核数
        remained_cpu_core:
          type: integer
          description: 剩余CPU核数
        remained_memory:
          type: integer
          description: 剩余内存(GB)
        remained_disk_size:
          type: integer
          description: 剩余云盘大小(GB)
        region_id:
          type: string
          description: 地区ID
        region_name:
          type: string
          description: 地区名称
        zone_id:
          type: string
          description: 可用区ID
        zone_name:
          type: string
          description: 可用区名称
        plan_type:
          type: string
          description: 计划类型（预测内/预测外）
        obs_project:
          type: string
          description: OBS项目类型
        device_family:
          type: string
          description: 机型族
        device_class:
          type: string
          description: 机型分类
        device_type:
          type: string
          description: 原始机型规格
        core_type:
          type: string
          description: 核心类型
        disk_type:
          type: string
          description: 云盘类型
        disk_type_name:
          type: string
          description: 云盘类型名称
        disk_io:
          type: integer
          description: 云盘IO
        # --- 新增：可通配机型明细 ---
        available_device_types:
          type: array
          description: 可通配的机型列表
          items:
            $ref: '#/components/schemas/AvailableDeviceType'

    AvailableDeviceType:
      type: object
      properties:
        device_type:
          type: string
          description: 机型规格
        is_original:
          type: boolean
          description: 是否为预测录入时的原始机型
        device_type_class:
          type: string
          description: 机型分类（CommonType/SpecialType）
        device_class:
          type: string
          description: 机型类型
        device_family:
          type: string
          description: 机型族
        cpu_core:
          type: integer
          description: CPU核心数
        memory:
          type: integer
          description: 内存大小(GB)
        charge_types:
          type: array
          description: 各计费模式下的可用性
          items:
            $ref: '#/components/schemas/DeviceTypeChargeInfo'

    DeviceTypeChargeInfo:
      type: object
      properties:
        charge_type:
          type: string
          enum: [PREPAID, POSTPAID_BY_HOUR]
          description: 计费模式
        available:
          type: boolean
          description: 是否可用
        remain_core:
          type: integer
          description: 剩余核心数
```

## 3. 行为规格

### 3.1 WHEN 请求参数 page.count = true

**THEN** 仅返回 `data.count`（预测记录总数），不返回 details。

### 3.2 WHEN 请求参数 page.count = false

**THEN** 返回 `data.details` 列表，每条记录包含 `available_device_types`。

### 3.3 WHEN 预测记录的 demand_res_type 不是 CVM

**THEN** `available_device_types` 返回空数组（仅 CVM 类型存在机型通配）。

### 3.4 WHEN 多条预测记录属于同一 (bk_biz_id, region_id, zone_id)

**THEN** 合并查询通配规则，避免重复调用。

### 3.5 WHEN 查不到可通配机型

**THEN** `available_device_types` 仍包含原始机型（`is_original: true`），其他通配机型为空。

### 3.6 WHEN 未查询到指定筛选条件的预测记录

**THEN** 返回 `data.count: 0` 和 `data.details: []`。

## 4. 错误处理

| 场景 | code | message |
|------|------|---------|
| 参数校验失败 | 400 | 具体校验错误信息 |
| 期望时间范围缺失 | 400 | expect_time_range is required |
| 分页参数缺失 | 400 | page is required |
| 内部查询失败 | 500 | 内部服务错误 |
