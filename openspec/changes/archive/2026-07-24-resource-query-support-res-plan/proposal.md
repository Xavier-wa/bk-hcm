## Why

HCM「资源查询助手」职责范围已包含主机库存与 CVM 预测查询，意图识别也将「查预测余量 / CVM 预测需求」归为 `resource_query`，但 `hcm-resource-search` skill **仅登记了**资源预测需求列表工具；资源预测单据、子单、GPU 需求主单/明细等尚未暴露。用户在助手侧无法完整查询海垒「资源预测」能力，与产品预期不一致。

另：GPU「数据汇总」在前端由子单 list 聚合，既有 `suborders/list` 的 filter/page 对 MCP/助手过重。经澄清修订后，本期**新增一条 MCP 专用汇总接口**，直出按需求分类的卡数/QPM（含按月列）；前端不切换。

## What Changes

- 在 `hcm-resource-search` skill 中新增/保留 reference，并在 `SKILL.md`「资源预测」分类下登记索引：
  - 资源预测单据列表（对接 woa `.../plans/resources/tickets/list`）
  - 资源预测子单列表（对接 woa `.../plans/resources/sub_tickets/list`，必填 `ticket_id`）
  - GPU 需求主单列表（对接 woa `.../gpu/demands/orders/list`）
  - GPU 子单明细列表（对接 woa `.../gpu/demands/suborders/list`；明细场景，可选）
  - **GPU 数据汇总**（对接**新增** woa `.../gpu/demands/suborders/summary`；助手查汇总的主路径）
- 保留并纳入验收：既有 `list_biz_res_plan_demand`（资源预测需求列表，含核数概览）。
- **新增** woa 业务视角只读接口：GPU 子单数据汇总（`order_id` 在 body；可选 statuses 多值 in、demand_year、demand_month、demand_type；响应含各类别总数 + months）。
- 轻量更新 `resource_query_system_prompt.md`：标明**资源预测**与 **GPU 预测**属于可查询范围即可。
- `hcm-res-manager` MCP 工具注册**不在本仓库本变更完成标准内**，由负责人另轨交付（须含汇总工具）。
- **无前端改动**；不加详情 get；仅只读查询。

## Capabilities

### New Capabilities

- `resource-search-res-plan-tools`: 在 hcm-resource-search skill 中补齐资源预测相关查询 reference（单据 / 子单 / GPU 主单 / GPU 子单明细 / **GPU 数据汇总**）与索引；MCP 工具注册另轨。
- `resource-query-res-plan-prompt`: 轻量更新资源查询系统提示词，将资源预测与 GPU 预测纳入可查询职责范围说明。
- `resource-plan-gpu-summary-api`: 新增业务视角 GPU 需求数据汇总只读接口（MCP-only 使用约定；前端不切换）。

### Modified Capabilities

<!-- 无：不修改 openspec/specs/ 下既有 capability 的需求条款。 -->

## Impact

- **本仓后端**：woa-server（及必要的 data-service/cloud 调用链，以实现为准）新增 `.../gpu/demands/suborders/summary`；配套接口文档。
- **本仓 AI / Agent**：`.cursor/skills/hcm-resource-search/`（SKILL.md + references，含汇总工具）；`cmd/agent-server/etc/prompts/resource_query_system_prompt.md`。
- **复用接口**（零契约变更）：
  - `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/demands/list`
  - `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/list`
  - `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/sub_tickets/list`
  - `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/gpu/demands/orders/list`
  - `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/gpu/demands/suborders/list`
- **新增接口**：
  - `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/gpu/demands/suborders/summary`
- **另轨**：`hcm-res-manager` MCP 工具注册与联通。
- **不涉及**：前端页面/组件（含 GPU 详情「数据汇总」Tab 不切换）、资源预测写操作、独立详情 get。
- **关联需求**：TAPD `1069995598136112812`。
