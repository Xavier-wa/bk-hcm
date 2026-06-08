## 1. 定义导出行结构体与请求结构体

- [x] 1.1 在 `cmd/woa-server/service/plan/ticket_export.go` 中定义 `ExportResPlanTicketsReq` 请求结构体（含 `TicketIDs []string` 字段和 `Validate()` 方法，要求非空）
- [x] 1.2 定义 `ticketExportRow` 结构体，字段顺序与列头一致，每个字段添加 `excel:"列头名"` struct tag，包含所有 23 列（含计算字段 总核心数）
- [x] 1.3 实现 `ticketExportRowHeaders()` 函数，通过反射从 `ticketExportRow` 的 struct tag 提取有序列头切片
- [x] 1.4 实现 `toExcelRow(row ticketExportRow)` 函数，通过反射按字段顺序将结构体转为 `[]interface{}` 用于写入 Excel

## 2. 实现数据查询逻辑

- [x] 2.1 实现 `fetchTicketsWithStatus(kt, ticketIDs)` 函数：将 ticketIDs 按 500 分批，循环调用 `c.dao.ResPlanTicket().ListWithStatus()` 传入 `RuleIn("id", batchIDs)` 过滤条件，聚合结果
- [x] 2.2 实现 `buildExportRows(tickets)` 函数：遍历每个 ticket，解析 Demands JSON（`json.Unmarshal` 到 `rpt.ResPlanDemands`），展开每条 demand，校验 Updated 非 nil，将 ticket 字段 + Updated 字段 + 计算字段填入 `ticketExportRow`

## 3. 实现 Excel 写入逻辑

- [x] 3.1 实现 `writeTicketExcel(w io.Writer, rows []ticketExportRow)` 函数：创建 `excelize.File`，写入列头行（`ticketExportRowHeaders()`），逐行写入数据行（`toExcelRow()`），最后调用 `file.Write(w)` 输出到 writer
- [x] 3.2 确保 Excel sheet 名称为 `"单据数据"`，文件名格式为 `res_plan_tickets_{unix_timestamp}.xlsx`（unix 时间戳在 Handler 中生成并传入）

## 4. 实现 Handler 并注册路由

- [x] 4.1 实现 `ExportResPlanTickets(cts *rest.Contexts) (interface{}, error)` Handler：解码请求 → 校验参数 → 鉴权（`meta.Application + meta.Find`）→ 查询数据 → 构建行 → 生成文件名（`fmt.Sprintf("res_plan_tickets_%d.xlsx", time.Now().Unix())`）→ 设置 HTTP header（Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet，Content-Disposition: attachment; filename={filename}）→ 写入 Response
- [x] 4.2 在 `cmd/woa-server/service/plan/service.go` 的 `initPlanService` 中注册路由：`h.Add("ExportResPlanTickets", http.MethodPost, "/plans/resources/tickets/export", s.ExportResPlanTickets)`
