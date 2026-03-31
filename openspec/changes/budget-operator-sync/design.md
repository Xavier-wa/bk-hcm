# 预算提报人同步 - 设计文档

## 1. 背景

### 1.1 业务背景
目前，`res_plan_demand` 表中由 admin 创建的单据，其 `creator`（提单人）字段无有效值。此前，提单人信息仅在邮件发送流程中临时调用「FinOps 预算提报人查询接口（get_dm_budget_declaration_operator）」获取，未持久化至数据库。

### 1.2 涉及层级
本次变更涉及 **两个服务层**：
- **服务层**：`cmd/woa-server` - 业务逻辑、批量处理、FinOps API 调用
- **资源层**：`cmd/data-service` - `res_plan_demand` 表的数据持久化操作

### 1.3 外部集成
- **FinOps 系统**：使用现有 `pkg/thirdparty/api-gateway/finops` 客户端，调用 `GetBudgetDeclarationOperator` 接口

---

## 2. 目标 / 非目标

### 目标
1. 开发标准化接口 `/plans/resources/demands/budget_operator/sync/by_time`，将 admin 单据的提报人信息同步至 `res_plan_demand` 表的 `creator` 字段
2. 实现幂等更新，防止重复更新
3. 支持按时间范围灰度执行

### 非目标
- 实时同步（仅支持批量处理）
- 自动调度/Cron 任务（仅支持手动触发）
- 回滚功能（通过 SQL 手动处理）

---

## 3. 架构设计

### 3.1 分层架构

```
┌─────────────────────────────────────────────────────────────────┐
│                     接入层 (api-server)                           │
│         POST /plans/resources/demands/budget_operator/           │
│                        sync/by_time                               │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                     服务层 (woa-server)                           │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │            BudgetOperatorSyncService                       │
│  │  - 参数校验                                                │
│  │  - 查询 admin 单据                                         │
│  │  - 按 OpProductID + 月份 分组                              │
│  │  - 批量处理（100条/批，200ms间隔）                         │
│  │  - 调用 FinOps API                                         │
│  │  - 幂等更新 creator 字段                                    │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                     资源层 (data-service)                         │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │              ResPlanDemandDao                              │
│  │  - ListWithTx: 按时间范围查询单据                           │
│  │  - UpdateCreator: 幂等更新 creator                          │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                      外部系统                                     │
│              FinOps 预算提报人 API                                │
│         get_dm_budget_declaration_operator                       │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 数据流程

```
1. 请求：start_time, end_time
        │
        ▼
2. 查询：creator = 'hcm-backend-admin'
   AND expect_time BETWEEN start_time AND end_time
        │
        ▼
3. 分组：按 OpProductID + 月份分组（按 expect_time ASC 排序）
        │
        ▼
4. 批量处理（每组）：
   - 先收集所有需要更新的 demand IDs
   - 再按 100 条/批批量提交
   - 子批次间隔 200ms
        │
        ▼
5. 对每个分组：
   a. 调用 FinOps API → 获取候选提报人列表
   b. 按提报人 ID 去重，过滤空字符串和 backend 用户
   c. 取第一个非空候选人
   d. 无候选人 → 跳过整个分组，记录日志警告
   e. 有候选人 → 收集 IDs 后批量更新
        │
        ▼
6. 响应：total_count, processed_count, success_count,
   failed_count, failed_demand_ids, skipped_count, skipped_demand_ids
```

---

## 4. 关键决策

### 决策 1：批量处理策略
**方案**：按 `OpProductID + 月份` 分组，然后拆分为 100 条子批次，间隔 200ms。

**理由**：
- 防止数据库锁升级
- 控制 FinOps API 调用频率
- 支持进度追踪

### 决策 2：幂等更新逻辑
**方案**：仅当 `id IN (:demand_ids) AND creator = 'hcm-backend-admin'` 时批量更新。

**理由**：
- 防止覆盖有效的 creator 值（仅当 creator 为 admin 时更新）
- 查询条件与更新条件对齐，避免统计不准确
- 支持失败后安全重试
- 保持数据完整性

### 决策 3：提报人选择策略
**方案**：调用 FinOps API → 按提报人 ID 去重 → 过滤空字符串和 backend 用户 → 取第一个有效候选人。

**理由**：
- FinOps 是提报人信息的权威来源
- 去重防止重复提报人
- 过滤空字符串和 backend 用户（`hcm-backend-admin`），确保只使用有效的真实用户
- 第一个有效候选人是主要联系人

---

## 5. 接口规范

### 5.1 端点
- **路径**：`/plans/resources/demands/budget_operator/sync/by_time`
- **方法**：POST

### 5.2 请求参数
| 字段 | 类型 | 必填 | 示例 | 说明 |
|-------|------|------|------|------|
| start_time | string | 是 | "2026-01-01" | 开始时间，格式 YYYY-MM-DD |
| end_time | string | 是 | "2026-12-31" | 结束时间，格式 YYYY-MM-DD |

### 5.3 响应参数
| 字段 | 类型 | 示例 | 说明 |
|------|------|------|------|
| code | int | 0 | 0=成功，500=系统异常，400=参数错误 |
| msg | string | "处理完成" | 响应信息 |
| data | object | - | 处理结果详情 |
| data.total_count | int | 100 | 符合条件的 admin 单据总数 |
| data.processed_count | int | 100 | 已处理单据数 |
| data.success_count | int | 96 | 更新成功数 |
| data.failed_count | int | 2 | 更新失败数 |
| data.failed_demand_ids | array | ["demand_001"] | 失败单据 ID 列表 |
| data.skipped_count | int | 2 | 因无候选提报人跳过的单据数 |
| data.skipped_demand_ids | array | ["demand_003"] | 跳过单据 ID 列表 |

---

## 6. 数据库操作

### 6.1 查询（通过 pkg/dal/dao）
```go
// 筛选条件：creator 为 admin，expect_time 在时间范围内
WHERE creator = 'hcm-backend-admin'
AND expect_time >= :start_time
AND expect_time < :end_time + 1 day
ORDER BY expect_time ASC
```

### 6.2 幂等更新（通过 pkg/dal/dao）
```go
// 仅当条件匹配时批量更新
WHERE id IN (:demand_ids)
AND creator = 'hcm-backend-admin'
```

---

## 7. 异常处理

| 异常类型 | 处理方式 |
|----------|----------|
| 时间格式无效 | 返回 400 错误 |
| FinOps API 调用失败 | 返回 500 错误，并在 `data` 中返回当前已累计的处理结果（processed/success/failed/skipped 及对应 ID） |
| 数据库更新失败 | 记录失败单据 ID，继续处理下一条 |
| FinOps 返回空候选 | 跳过更新，继续处理下一批次 |


---

## 8. 监控与日志

### 8.1 指标
- 接口调用量
- 成功率
- 平均响应时间
- 500 错误率

### 8.2 数据监控
- `res_plan_demand.creator` 字段空值率（按时间/产品维度）

### 8.3 告警
- FinOps API 调用失败
- 批量更新失败率 > 1%
- 数据库锁等待超时
