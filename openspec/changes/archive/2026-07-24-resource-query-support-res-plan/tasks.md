## 1. Skill：资源预测单据 / 子单

- [x] 1.1 新增 `list_biz_res_plan_ticket.md`
- [x] 1.2 新增 `list_biz_res_plan_sub_ticket.md`

## 2. Skill：GPU 需求主单 / 明细（列表复用）

- [x] 2.1 新增 `list_biz_res_plan_gpu_order.md`
- [x] 2.2 新增 `list_biz_res_plan_gpu_suborder.md`（初版含客户端聚合说明）
- [x] 2.3 **修订** `list_biz_res_plan_gpu_suborder.md`：标明明细用途；数据汇总主路径改为 summary 工具
- [x] 2.4 新增 `list_biz_res_plan_gpu_summary.md`：对接 `.../suborders/summary`，完整参数/返回/`months`/案例；注明 MCP-only

## 3. Skill：索引与描述

- [x] 3.1 `SKILL.md`「### 6. 资源预测」登记 demand + ticket + sub_ticket + gpu_order + gpu_suborder
- [x] 3.2 索引补充 `list_biz_res_plan_gpu_summary`；description/触发词覆盖「数据汇总」
- [x] 3.3 返回结构等通用说明的最小补充（列表类）

## 4. 提示词

- [x] 4.1 轻量更新 `resource_query_system_prompt.md` 职责范围
- [x] 4.2 确认行为约束段仍禁止写操作与编造数据

## 5. 后端：GPU 数据汇总接口（F-009）

- [x] 5.1 实现 `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/gpu/demands/suborders/summary`（body：`order_id` 必填；可选 `statuses`/`demand_year`/`demand_month`/`demand_type`）
- [x] 5.2 聚合口径对齐前端 `summaryRows`（按 `demand_type`；`gpu_num`/`qpm_max`；`months`）
- [x] 5.3 新增 api-docs（biz/scr/resource-plan 下），声明 MCP/助手用途、前端不切换
- [x] 5.4 单测 / 关键路径自测（聚合单测；`go build ./cmd/woa-server` 通过）

## 6. 自检与验收说明

- [x] 6.1 列表类 skill references 自检（既有完成项）
- [x] 6.2 对照 specs 自检：summary API + skill + 索引齐全；AC-006 走直出汇总
- [x] 6.3 标注：MCP 工具注册另轨（须含 summary）；前端不切换
- [ ] 6.4 （另轨）`hcm-res-manager` 注册后按需求 AC 端到端验收
