## ADDED Requirements

### Requirement: 业务视角 GPU 数据汇总只读接口

系统 SHALL 提供业务视角只读接口：`POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/gpu/demands/suborders/summary`。调用 MUST 校验业务访问权限。接口 MUST 仅查询与聚合，MUST NOT 修改任何 GPU 需求/子单数据。

#### Scenario: 仅传 order_id 返回汇总

- **WHEN** 调用方在 path 提供合法 `bk_biz_id`，body 提供存在的 `order_id`，且未传可选过滤
- **THEN** 系统 SHALL 返回该主单下按 `demand_type` 聚合的汇总列表（含 `gpu_num`、`qpm_max`、`months`）

#### Scenario: 缺少 order_id 时失败

- **WHEN** 调用方未提供 `order_id` 或 `order_id` 为空
- **THEN** 系统 SHALL 返回参数错误，且 MUST NOT 返回全业务盲查汇总

### Requirement: 可选过滤条件

接口 body MUST 支持可选字段：`statuses`（string 数组，多值 in 语义）、`demand_year`（int）、`demand_month`（int）、`demand_type`（string）。未提供的可选字段 MUST 不限制对应维度。`order_id` MUST 放在 body（非 path）。

#### Scenario: 多状态过滤

- **WHEN** body 提供 `statuses: ["DONE", "PENDING"]`
- **THEN** 汇总 MUST 仅基于状态落在该集合内的子单

#### Scenario: 年月与分类过滤

- **WHEN** body 同时提供 `demand_year`、`demand_month`、`demand_type` 中的若干项
- **THEN** 汇总 MUST 仅基于同时满足已提供条件的子单

### Requirement: 响应结构含总数与按月列

成功响应 `data` MUST 包含 `order_id` 与 `details` 数组。`details` 每一项 MUST 包含：`demand_type`、`gpu_num`、`qpm_max`、`months`（object，key 为 `YYYY-MM`，value 为该月度量数值）。聚合口径 MUST 对齐海垒 GPU 需求详情页「数据汇总」语义（按 `demand_type` 分行；卡数/QPM 求和；按月拆分）。

#### Scenario: 响应可直接呈现为数据汇总表

- **WHEN** 调用方取得成功响应
- **THEN** 无需再翻页拉取子单明细即可得到各类别总数与按月列

### Requirement: MCP-only 产品约定与文档

接口文档与 skill 文档 MUST 标明该接口面向 MCP/资源查询助手使用；MUST 说明海垒前端「数据汇总」Tab 可继续使用既有子单 list 前端聚合，**不要求**前端切换本接口。本要求不强制在网关层隔离前端调用，但本变更范围 MUST NOT 包含前端切换改造。

#### Scenario: 文档声明使用范围

- **WHEN** 阅读新增接口的 api-docs 或 skill reference
- **THEN** 文档 SHALL 写明 MCP/助手用途，并说明前端不切换

### Requirement: 接口文档落地

系统 SHALL 在 `docs/api-docs/web-server/docs/biz/scr/resource-plan/` 下新增（或等价路径）业务视角汇总接口 Markdown 文档，覆盖 URL、权限、入参、出参与至少 1 个调用示例。

#### Scenario: 文档可独立查阅

- **WHEN** 开发或 MCP 接入方查阅 api-docs
- **THEN** 能找到 summary 接口的完整参数说明
