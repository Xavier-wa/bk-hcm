# 变更提案：预测需求聚合接口 - 带可通配机型明细

## 背景

当前 AI Agent 通过 MCP 调用 `list_biz_res_plan_demand` 接口获取预测余量数据，但该接口返回的每条预测记录仅包含录入预测时的原始机型（`device_type`）。实际申领时，机型可在一定范围内通配（同机型族/同机型分类下的其他机型也可申领），因此 Agent 还需要额外调用两个接口来获取完整的可申领机型信息：

1. `list_config_cvm_charge_type_device_type` - 根据通配规则查询所有可申领机型及计费模式
2. `get_cvm_device` - 获取机型详细信息（核心数、内存等）

这导致 Agent 需要发起多次调用、组合多源数据，增加了调用复杂度和延迟。

## 目标

在 woa-server 中提供一个**聚合接口**，一次调用即可返回预测余量数据及每条预测对应的全部可通配机型明细。

## 方案概述

### 新增接口

**POST** `/api/v1/woa/plans/resources/demands/list_with_device_types`

- 以 `list_biz_res_plan_demand` 的查询逻辑为基础
- 对返回的每条预测记录，根据其 `bk_biz_id`、`region_id`、`zone_id` 等信息，调用 `list_config_cvm_charge_type_device_type` 的通配规则逻辑
- 从 `get_cvm_device` 数据源中获取可通配机型的详细信息（cpu_core、memory 等）
- 将预测数据与机型明细聚合后统一返回

### 接口路径设计

放在非 biz 路径组下（与 `list_config_cvm_charge_type_device_type` 同级），因为调用方是 MCP 服务而非前端业务页面。

### 核心数据流

```
list_biz_res_plan_demand (预测主数据)
        │
        ├── 提取 bk_biz_id, region_id, zone_id, device_type 等关键维度
        │
        ▼
list_config_cvm_charge_type_device_type 的通配规则逻辑
        │
        ├── 匹配得到所有可通配的 device_type 列表
        │
        ▼
get_cvm_device 数据源查询
        │
        ├── 获取可通配机型的详细信息 (cpu_core, memory, device_class 等)
        │
        ▼
聚合结果返回
```

### 分页策略

由于每条预测记录会展开为多条机型明细，数据量大于原始 `list_biz_res_plan_demand`，接口需支持分页：
- 支持现有的 `count/start/limit` 分页模式
- 分页粒度为展开后的明细记录

## 约束

- 服务层级：woa-server
- 调用方：MCP 服务（对外提供 AI 接口）
- 需支持分页
- 保持与现有接口一致的响应风格
