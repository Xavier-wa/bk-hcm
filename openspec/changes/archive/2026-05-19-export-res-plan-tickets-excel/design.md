## Context

woa-server 管理资源计划单据（res_plan_ticket），目前支持列表查询但缺少导出能力。运营人员需要将单据数据导出为 Excel，用于汇报和离线分析。每张单据包含一个 Demands JSON 数组，导出时需将每条 demand 展开为独立一行。单据状态存储在 res_plan_ticket_status 表（ticket_id 关联），查询时可复用 `ListWithStatus` DAO join 查询。

## Goals / Non-Goals

**Goals:**
- 新增 `POST /plans/resources/tickets/export` 接口，接收 ticket_id 数组，返回 Excel 文件流
- 每条 `UpdatedRPDemandItem` 对应 Excel 一行，展开 ticket 的所有 demands
- 列头由 struct tag（`excel` tag）结合反射动态生成，字段映射关系见需求文档
- Updated 为 nil 时整体报错，不跳过
- 内部分批（每批 500）查询 data-service，无用户侧数量限制
- 鉴权复用 `meta.Application + meta.Find`

**Non-Goals:**
- 不支持异步下载（文件大时直接超时由调用方处理）
- 不支持自定义列头顺序
- 不新增前端页面，仅提供接口

## Decisions

### 决策1：Excel 列头生成方式
使用 struct tag + 反射的方式定义列头，在导出行结构体 `ticketExportRow` 上添加 `excel:"列头名"` tag，通过反射按字段顺序提取列头名和对应值，避免列头与数据错位。

**替代方案**：手动维护列头切片 + 数据切片，实现简单但容易与字段不同步。

### 决策2：数据查询策略
分两步查询：
1. 使用 `ResPlanTicket().ListWithStatus()` DAO 方法，通过 `RuleIn("id", batchIDs)` 按批次查询（每批 ≤ 500），同时获取 ticket 基础信息和 status。
2. 解析每个 ticket 的 demands JSON，展开为行，取 Updated 字段写入 Excel。

**替代方案**：分别查 ticket 和 ticket_status，需要自行 join，复杂度更高。

### 决策3：HTTP 响应方式
直接写入 HTTP Response，设置 `Content-Disposition: attachment; filename=...` header，使用 `excelize` 库写入 writer。与现有 GPU 模板导出类似。

### 决策4：文件位置
新建 `cmd/woa-server/service/plan/ticket_export.go`，职责单一，与现有 ticket.go 分离。

## Risks / Trade-offs

- [风险] ticket_id 数组过大时，分批查询耗时较长 → 接受，由调用方控制传入数量，接口不限制
- [风险] Demands JSON 解析失败 → 报错返回，不跳过
- [风险] Updated 为 nil → 整体报错，保证数据完整性
