## Context

**无设计稿（豁免）**：本变更为资源查询助手 skill / 提示词补齐 + 一条 MCP 专用只读汇总 API；无前端 UI 变更。UI 参照仅为海垒现网「资源预测」能力语义（CVM 预测 / GPU 需求列表与详情「数据汇总」口径），前端实现不改页面。

HCM「资源查询助手」通过 `hcm-resource-search` skill 对接业务视角只读查询。现状：

| 能力 | woa 接口 | skill reference |
|------|----------|-----------------|
| 资源预测需求列表 | 已有 | **已有** `list_biz_res_plan_demand.md` |
| 资源预测单据 | 已有 | **已有** `list_biz_res_plan_ticket.md` |
| 资源预测子单 | 已有 | **已有** `list_biz_res_plan_sub_ticket.md` |
| GPU 需求主单 | 已有 | **已有** `list_biz_res_plan_gpu_order.md` |
| GPU 子单明细 | 已有 | **已有** `list_biz_res_plan_gpu_suborder.md` |
| GPU 数据汇总（助手） | **待新增** summary | **待补** `list_biz_res_plan_gpu_summary.md` |

海垒前端「数据汇总」仍由子单 list + `summaryRows` 聚合；**不切换**新接口。

## Goals / Non-Goals

**Goals:**

- 补齐 skill references / 索引 / 提示词（列表类已基本完成）。
- **新增** GPU 数据汇总只读接口，供 MCP/助手用简化入参直出汇总。
- skill 将「查数据汇总」主路径指向新接口；子单 list 保留为明细可选能力。

**Non-Goals:**

- 不做前端改动；前端数据汇总 Tab 不切换。
- 不实现 `hcm-res-manager` MCP 工具注册（另轨）。
- 不暴露详情 get；不做写操作。

## Decisions

### D1. 交付模式：列表复用 + 汇总例外新增

**选择**：单据/子单/GPU 主单/子单明细复用既有 woa 列表；**仅** GPU 数据汇总新增 `suborders/summary`。

**理由**：列表接口已够用；子单 list 对助手拼参与聚合过重；汇总接口入参极简、出参直出。

### D2. reference 命名与索引

| 工具名 | 底层路径 |
|--------|----------|
| `list_biz_res_plan_ticket` | `.../tickets/list`（既有） |
| `list_biz_res_plan_sub_ticket` | `.../sub_tickets/list`（既有） |
| `list_biz_res_plan_gpu_order` | `.../gpu/demands/orders/list`（既有） |
| `list_biz_res_plan_gpu_suborder` | `.../gpu/demands/suborders/list`（既有，明细） |
| `list_biz_res_plan_gpu_summary` | `.../gpu/demands/suborders/summary`（**新增**） |

`SKILL.md`「### 6. 资源预测」登记上述工具 + 既有 demand；查汇总优先 summary。

### D3. 汇总接口契约（已确认）

**路径**：`POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/gpu/demands/suborders/summary`

**Body**：

| 字段 | 必填 | 说明 |
|------|------|------|
| order_id | 是 | 主单 ID（body，非 path） |
| statuses | 否 | string[]，状态多值 in |
| demand_year | 否 | int，与 month 拆开 |
| demand_month | 否 | int |
| demand_type | 否 | 需求分类 |

**响应**：

```json
{
  "order_id": "0000001k",
  "details": [
    {
      "demand_type": "大语言模型训练-文生文",
      "gpu_num": 128,
      "qpm_max": 0,
      "months": { "2026-03": 64, "2026-04": 64 }
    }
  ]
}
```

聚合口径对齐前端 `summaryRows`：按 `demand_type` 分行；卡数/QPM 求和；`months` 为 `YYYY-MM` → 数值。

### D4. 前置 ID 交互写进 skill

- 预测子单：缺 `ticket_id` → 追问 / 先查单据。
- GPU 汇总 / 明细：缺 `order_id` → 追问 / 先查 GPU 主单。

### D5. MCP-only 使用约定（非接口鉴权隔离）

**选择**：接口挂在业务视角 woa 路径（与其他 list 一致，需业务访问权限）；**产品/文档约定**仅 MCP/助手使用，前端不改造、不切换。

**理由**：避免单独鉴权通道；与「只给 MCP 用」的产品边界一致。

**备选**：独立内部 token 网关 — 超出本期，否决。

### D6. 提示词只做轻量职责声明

同前：职责范围标明资源预测 / GPU 预测可查；细节以 skill 为准。

### D7. MCP 另轨

tasks 以本仓 API + skill + 提示词为准；MCP 注册另轨验收。

### D8. 修订 D5 旧结论

原「不新增数据汇总 API」**作废**，以 D3/D5 为准。

## Risks / Trade-offs

- [汇总口径与前端公式字段不完全一致] → 默认对落库 `gpu_num`/`qpm_max` 求和；实现阶段对照 `summaryRows`/`tpl_config` 注明差异。
- [前端与 MCP 双路径] → 文档写清；前端不切换降低回归风险。
- [MCP 未就绪时助手仍查不了] → 验收分文档/API 完备与端到端两档。

## Migration Plan

1. 实现 summary 接口 + 接口文档。
2. 新增/调整 skill reference 与索引（汇总主路径）。
3. 提示词已轻量补充则保持。
4. 另轨 MCP 注册后端到端验收。
5. 回滚：下线 summary 路由 + 还原 skill；列表能力不受影响。

## Open Questions

- （无阻塞项）MCP 注册排期另轨确认。
