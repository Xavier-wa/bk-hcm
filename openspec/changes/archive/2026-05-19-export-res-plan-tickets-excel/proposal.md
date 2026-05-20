## Why

运营人员需要批量导出资源计划单据（res_plan_ticket）数据，用于离线分析和汇报。目前系统缺少此导出能力，需要新增批量导出接口，支持按 ticket_id 数组指定导出范围，将单据及其 demand 展开行写入 Excel 文件。

## What Changes

- 新增 woa-server 接口 `POST /plans/resources/tickets/export`，接收 ticket_id 数组，返回 Excel 文件流
- 接口直接写入 HTTP Response（Content-Disposition: attachment），无需先存储文件
- 每条 demand（UpdatedRPDemandItem）对应 Excel 一行，一个 ticket 包含多条 demand 时展开为多行
- 若某条 demand 的 Updated 字段为 nil，接口整体报错
- ticket 数量无限制，但内部分批（每批 ≤ 500）查询 data-service

## Capabilities

### New Capabilities
- `res-plan-ticket-export`: 资源计划单据批量导出为 Excel，支持按 ticket_id 列表过滤，按 demand 展开行，列头包含单据信息、单据状态、demand 详情

### Modified Capabilities

## Impact

- `cmd/woa-server/service/plan/`：新增 `ticket_export.go` 文件，实现 Handler 及导出逻辑
- `cmd/woa-server/service/plan/service.go`：注册新路由
- 依赖已有的 `ResPlanTicket` DAO（ListWithStatus）和 `tools/excel` 包构建 Excel
- 鉴权逻辑复用 `ListResPlanTicket` 的 `meta.Application + meta.Find` 方式
