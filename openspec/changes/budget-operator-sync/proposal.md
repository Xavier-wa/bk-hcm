# 预算提报人同步 - 提案

## 背景与目标

目前，`res_plan_demand` 表中由 admin（用户：`hcm-backend-admin`）创建的单据，其 `creator`（提单人）字段无有效值。此前，提单人信息仅在邮件发送流程中临时调用「FinOps 预算提报人查询接口（get_dm_budget_declaration_operator）」获取，未持久化至数据库。

这导致 admin 创建的单据无法追溯到实际提单人，影响业务溯源和审计要求。

## 变更内容

1. 新增标准化接口：`POST /plans/resources/demands/budget_operator/sync/by_time`
2. 实现批量处理逻辑，将 FinOps 提报人信息同步至 `res_plan_demand.creator` 字段
3. 支持幂等更新，防止覆盖有效数据
4. 支持按时间范围灰度执行

## 新增能力

- **预算提报人同步接口**：按时间范围同步 admin 单据提报人信息
- **批量处理**：分批处理需求（100条/批，间隔200ms）
- **幂等更新**：仅在条件匹配时安全更新

## 涉及模块

### 影响的服务层
- **服务层 (woa-server)**：新增 API 接口、批量处理逻辑、FinOps API 集成
- **资源层 (data-service)**：新增数据查询和更新方法

### 涉及文件
- `cmd/woa-server/service/plan/budget_operator_sync.go` - 新增服务实现
- `cmd/woa-server/service/plan/service.go` - 新增路由注册
- `cmd/woa-server/types/plan/demand.go` - 新增请求/响应协议（BudgetOperatorSyncReq/BudgetOperatorSyncResp）
- `cmd/data-service/service/resource-plan/res-plan-demand/service.go` - 新增路由
- `pkg/client/data-service/global/resource_plan.go` - 使用现有 data-service 客户端
- `pkg/thirdparty/api-gateway/finops/` - 使用现有 FinOps 客户端

### 外部集成
- **FinOps 系统**：`get_dm_budget_declaration_operator` 接口，用于查询预算提报人
