## ADDED Requirements

### Requirement: Batch export res plan tickets to Excel
The system SHALL provide a `POST /plans/resources/tickets/export` endpoint that accepts an array of ticket IDs and returns an Excel file stream with one row per demand.

#### Scenario: Successful export with multiple tickets
- **WHEN** a user sends a POST request with a valid ticket_ids array
- **THEN** the system returns an Excel file with header row and one data row per demand across all tickets

#### Scenario: Demand Updated field is nil
- **WHEN** any ticket's demand has a nil Updated field
- **THEN** the system SHALL return an error and no file is produced

#### Scenario: ticket_ids array is empty
- **WHEN** the request body contains an empty ticket_ids array
- **THEN** the system SHALL return an InvalidParameter error

#### Scenario: Large number of ticket IDs
- **WHEN** more than 500 ticket IDs are provided
- **THEN** the system SHALL query data-service in batches of 500 and aggregate results before writing Excel

#### Scenario: Unauthorized access
- **WHEN** the caller does not have meta.Application + meta.Find permission
- **THEN** the system SHALL return an unauthorized error

### Requirement: Excel column headers match field mapping
The Excel file SHALL contain the following columns in order, derived from struct tags on the export row struct:

| 列头 | 数据来源 |
|------|---------|
| 单据ID | res_plan_ticket.ID |
| 部门 | res_plan_ticket.VirtualDeptName |
| 规划产品ID | res_plan_ticket.PlanProductID |
| 规划产品名称 | res_plan_ticket.PlanProductName |
| 业务名称 | res_plan_ticket.BkBizName |
| 运营产品ID | res_plan_ticket.OpProductID |
| 运营产品名称 | res_plan_ticket.OpProductName |
| 单据类型 | res_plan_ticket.Type.Name()（中文） |
| 单据状态 | res_plan_ticket_status.Status（通过ticket_id关联） |
| 提单人 | res_plan_ticket.Applicant |
| 项目类型 | UpdatedRPDemandItem.ObsProject |
| 城市 | res_plan_ticket.RegionName（从UpdatedRPDemandItem.RegionName取） |
| 机型 | UpdatedRPDemandItem.Cvm.DeviceType |
| 数据盘类型 | UpdatedRPDemandItem.Cbs.DiskType |
| 核心类型 | UpdatedRPDemandItem.Cvm.CoreType |
| 预测用途 | res_plan_ticket.DemandClass |
| 单台数据盘容量（G） | UpdatedRPDemandItem.Cbs.DiskSize |
| 期望交付时间 | UpdatedRPDemandItem.ExpectTime |
| 业务需求OS数 | UpdatedRPDemandItem.Cvm.Os |
| 预测说明 | UpdatedRPDemandItem.Remark |
| 备注 | res_plan_ticket.Remark |
| 单OS核心数 | UpdatedRPDemandItem.Cvm.CpuCore |
| 总核心数 | 业务需求OS数 * 单OS核心数（计算字段） |

#### Scenario: Column headers are written as first row
- **WHEN** the export is generated
- **THEN** the first row of the Excel sheet SHALL contain all column headers in the order defined above

#### Scenario: Computed column total cores
- **WHEN** a row is written for a demand
- **THEN** the 总核心数 column SHALL equal UpdatedRPDemandItem.Cvm.Os * UpdatedRPDemandItem.Cvm.CpuCore
