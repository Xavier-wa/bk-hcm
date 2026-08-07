# Capability: resource-search-res-plan-tools

## Purpose

在 `hcm-resource-search` skill 中补齐资源预测相关查询 reference（单据 / 子单 / GPU 主单 / GPU 子单明细 / GPU 数据汇总）与索引，并约定 MCP 另轨交付。

## Requirements

### Requirement: 资源预测单据列表 skill reference

系统 SHALL 在 `hcm-resource-search` skill 的 references 目录新增 `list_biz_res_plan_ticket.md`，并在 `SKILL.md`「资源预测」分类下登记索引。文档 MUST 说明对接 `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/list`，包含 path 入参 `bk_biz_id`、body 过滤维度（如 ticket_ids / statuses / ticket_types / applicants / submit_time_range）与 `page`、主要返回字段及至少 1 个查询案例。文档 MUST 标明该能力为业务视角只读列表查询。

#### Scenario: 可通过索引定位单据查询文档

- **WHEN** 助手依据 SKILL.md 索引查找资源预测单据查询能力
- **THEN** 助手 SHALL 能定位到 `list_biz_res_plan_ticket.md` 并据此构造查询请求

### Requirement: 资源预测子单列表 skill reference

系统 SHALL 新增 `list_biz_res_plan_sub_ticket.md` 并在「资源预测」分类登记。文档 MUST 说明对接 `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/sub_tickets/list`，且 body **必填**主单 `ticket_id`；MUST 说明未提供 `ticket_id` 时应先追问或引导先查单据列表。文档 MUST 包含返回字段与至少 1 个查询案例。

#### Scenario: 缺少 ticket_id 时的文档指引

- **WHEN** 调用方阅读 `list_biz_res_plan_sub_ticket.md` 且尚未取得主单 ID
- **THEN** 文档 SHALL 明确要求先取得 `ticket_id`，禁止在无主单 ID 时盲查全量子单

### Requirement: GPU 需求主单列表 skill reference

系统 SHALL 新增 `list_biz_res_plan_gpu_order.md` 并在「资源预测」分类登记。文档 MUST 说明对接 `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/gpu/demands/orders/list`，body 为标准 `filter` + `page`，并列出可用过滤字段（至少含 id / status / creator / created_at 等接口支持字段）、返回字段（含 total_gpu_num / total_qpm_max）及至少 1 个查询案例。

#### Scenario: 可通过索引定位 GPU 主单查询文档

- **WHEN** 助手依据 SKILL.md 索引查找 GPU 需求列表查询能力
- **THEN** 助手 SHALL 能定位到 `list_biz_res_plan_gpu_order.md`

### Requirement: GPU 子单明细 skill reference

系统 SHALL 提供 `list_biz_res_plan_gpu_suborder.md` 并在「资源预测」分类登记。文档 MUST 说明对接 `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/gpu/demands/suborders/list`，典型过滤为 `order_id`；MUST 说明该工具用于**子单明细行**场景。文档 MUST 说明查「数据汇总」时**优先**使用 `list_biz_res_plan_gpu_summary`，而非要求助手翻页聚合本接口。文档 MUST 包含返回字段及至少 1 个按 `order_id` 查询的案例。

#### Scenario: 明细与汇总职责分离

- **WHEN** 调用方阅读 GPU 子单明细文档
- **THEN** 文档 SHALL 明确数据汇总主路径为 summary 工具，本工具用于明细

### Requirement: GPU 数据汇总 skill reference

系统 SHALL 新增 `list_biz_res_plan_gpu_summary.md` 并在「资源预测」分类登记。文档 MUST 说明对接 `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/gpu/demands/suborders/summary`，body **必填** `order_id`，可选 `statuses`（多值 in）、`demand_year`、`demand_month`、`demand_type`；MUST 说明未提供 `order_id` 时应追问或引导先查 GPU 主单列表。文档 MUST 描述响应中 `details[].demand_type` / `gpu_num` / `qpm_max` / `months`，并含至少 1 个查询案例。文档 MUST 标明该能力面向 MCP/助手（前端不切换）。

#### Scenario: 可通过索引定位 GPU 数据汇总文档

- **WHEN** 助手依据 SKILL.md 索引查找 GPU 数据汇总能力
- **THEN** 助手 SHALL 能定位到 `list_biz_res_plan_gpu_summary.md`

#### Scenario: 缺少 order_id 时的文档指引

- **WHEN** 调用方阅读 GPU 汇总文档且尚未取得主单需求 ID
- **THEN** 文档 SHALL 明确要求先取得 `order_id`

### Requirement: 既有预测需求列表纳入索引与验收口径

系统 SHALL 保留既有 `list_biz_res_plan_demand.md`，并将其置于扩展后的「资源预测」索引分类下。文档口径 MUST 继续覆盖必填 `expect_time_range`、返回 `overview` 核数概览与 `details` 列表。本变更 MUST NOT 删除或弱化该工具的登记。

#### Scenario: 需求列表仍可从资源预测分类找到

- **WHEN** 助手查看 SKILL.md「资源预测」分类
- **THEN** 索引 SHALL 包含 `list_biz_res_plan_demand` 条目

### Requirement: SKILL 描述与触发词覆盖资源预测全量查询

系统 SHALL 更新 `hcm-resource-search` 的 SKILL.md description / 触发词，使其覆盖资源预测单据、子单、GPU 需求列表、GPU 明细与 **GPU 数据汇总**等查询意图。

#### Scenario: 触发词包含 GPU 与单据

- **WHEN** 检索 skill 是否匹配「查预测单据」或「查 GPU 需求」或「查 GPU 数据汇总」类请求
- **THEN** SKILL description / 触发词 SHALL 足以命中本 skill

### Requirement: MCP 工具另轨交付

`hcm-res-manager` MCP 中上述业务视角只读工具的注册与接入 MUST 对接对应 woa 接口（含 summary），但其实现与注册**不在本仓库本变更的完成标准内**，由负责人另轨交付。工具 MUST 为只读。

#### Scenario: MCP 就绪后助手可查询预测相关列表与汇总

- **WHEN** MCP 工具已另轨注册且资源查询助手按 skill 文档调用对应工具
- **THEN** 工具 SHALL 返回符合业务范围与过滤条件的数据

#### Scenario: 只读约束

- **WHEN** 通过上述任一预测相关工具发起请求
- **THEN** 工具 MUST NOT 执行创建、调整、取消、审批或终止等写操作

### Requirement: 不加详情 get；允许新增汇总接口

本变更 MUST NOT 要求暴露独立详情 get 接口。本变更 MUST 允许并要求实现 GPU 数据汇总专用只读接口（见 capability `resource-plan-gpu-summary-api`）。列表类能力 MUST 复用既有 woa 列表路径。

#### Scenario: 实现范围含汇总 API、不含详情 get

- **WHEN** 审查本变更代码改动范围
- **THEN** 改动 MAY 包含新增 summary HTTP 路由；MUST NOT 以「详情 get」作为 GPU 汇总交付方式
