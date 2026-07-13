# 技术设计：预测需求聚合接口 - 带可通配机型明细

## 1. 接口定义

### 路径

```
POST /api/v1/woa/plans/resources/demands/list_with_device_types
```

### 请求参数

与 `list_biz_res_plan_demand` 基本一致，复用相同的过滤条件：

| 参数名 | 类型 | 必选 | 描述 |
|--------|------|------|------|
| bk_biz_id | int | 是 | 业务ID（本接口仅支持业务视角，必须传入单个业务ID） |
| demand_ids | string[] | 否 | 预测需求ID列表 |
| obs_projects | string[] | 否 | OBS项目类型列表 |
| core_types | string[] | 否 | 核心类型列表 |
| device_families | string[] | 否 | 机型族列表 |
| device_classes | string[] | 否 | 机型分类列表 |
| device_types | string[] | 否 | 机型规格列表 |
| region_ids | string[] | 否 | 地区/城市ID列表 |
| zone_ids | string[] | 否 | 可用区ID列表 |
| plan_types | string[] | 否 | 计划类型列表 |
| cpu_cores | int[] | 否 | CPU核心数列表（用于筛选机型规格） |
| memories | int[] | 否 | 内存大小列表(GB)（用于筛选机型规格） |
| expiring_only | bool | 否 | 是否只查询即将过期的需求 |
| expect_time_range | object | 是 | 期望交付时间范围 `{start, end}` |
| statuses | string[] | 否 | 状态枚举 |
| page | object | 是 | 分页设置 `{count, start, limit, sort?, order?}` |

**注意**：移除了 `demand_classes` 参数，本接口专用于 CVM 类型预测需求，在查询时自动硬编码 `demand_class = "CVM"`。

### 响应结构

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "count": 300,
    "details": [
      {
        // === 预测需求主数据（来自 list_biz_res_plan_demand） ===
        "demand_id": "0000001z",
        "bk_biz_id": 111,
        "bk_biz_name": "业务",
        "status": "can_apply",
        "status_name": "可申领",
        "demand_class": "CVM",
        "demand_res_type": "CVM",
        "expect_time": "2024-01-01",
        "can_apply_time": "2024-01-01",
        "expired_time": "2024-01-28",
        "return_plan_time": "2025-01-01",
        "total_cpu_core": 560,
        "applied_cpu_core": 440,
        "remained_cpu_core": 120,
        "region_id": "ap-shanghai",
        "region_name": "上海",
        "zone_id": "ap-shanghai-2",
        "zone_name": "上海二区",
        "plan_type": "预测内",
        "obs_project": "常规项目",
        "technical_class": "高IO型",
        "device_family": "高IO型",
        "core_type": "大核心",
        "disk_type": "CLOUD_PREMIUM",
        "disk_type_name": "高性能云硬盘",
        "disk_io": 15,

        // === 机型明细（展开后的字段） ===
        "device_type": "I6t.33XMEDIUM198",
        "is_original": true,
        "device_type_class": "CommonType",
        "device_class": "高IO型I6t",
        "cpu_core": 4,
        "memory": 16,
        "total_os": 140,
        "applied_os": 110,
        "remained_os": 30
      },
      {
        // === 预测需求主数据（来自 list_biz_res_plan_demand） ===
        "demand_id": "0000001z",
        "bk_biz_id": 111,
        "bk_biz_name": "业务",
        "status": "can_apply",
        "status_name": "可申领",
        "demand_class": "CVM",
        "demand_res_type": "CVM",
        "expect_time": "2024-01-01",
        "can_apply_time": "2024-01-01",
        "expired_time": "2024-01-28",
        "return_plan_time": "2025-01-01",
        "total_cpu_core": 560,
        "applied_cpu_core": 440,
        "remained_cpu_core": 120,
        "region_id": "ap-shanghai",
        "region_name": "上海",
        "zone_id": "ap-shanghai-2",
        "zone_name": "上海二区",
        "plan_type": "预测内",
        "obs_project": "常规项目",
        "technical_class": "高IO型",
        "device_family": "高IO型",
        "core_type": "大核心",
        "disk_type": "CLOUD_PREMIUM",
        "disk_type_name": "高性能云硬盘",
        "disk_io": 15,

        // === 机型明细（展开后的字段） ===
        "device_type": "I6t.2XLARGE32",
        "is_original": false,
        "device_type_class": "CommonType",
        "device_class": "高IO型I6t",
        "cpu_core": 8,
        "memory": 32,
        "total_os": 70,
        "applied_os": 55,
        "remained_os": 15
      }
    ]
  }
}
```

### 关键响应字段说明

| 字段路径 | 描述 |
|----------|------|
| `data.count` | 展开后的总记录条数（每条机型明细独立计数） |
| `data.details[]` | 展开后的每条记录，包含预测需求主数据 + 机型明细 |
| `data.details[].device_type` | 机型规格 |
| `data.details[].is_original` | 是否为预测录入时的原始机型 |
| `data.details[].cpu_core` | CPU核心数 |
| `data.details[].memory` | 内存大小(GB) |
| `data.details[].device_class` | 机型分类 |
| `data.details[].total_os` | 该机型对应的总 OS 数（基于 total_cpu_core / cpu_core 计算） |
| `data.details[].applied_os` | 该机型对应的已申领 OS 数（基于 applied_cpu_core / cpu_core 计算） |
| `data.details[].remained_os` | 该机型对应的剩余可申领 OS 数（基于 remained_cpu_core / cpu_core 计算） |

### 无通配机型时的处理

当预测需求的原始机型没有可通配的其他机型时，仅返回原始机型记录（`is_original = true`），不额外展开。这与现有逻辑保持一致。

## 2. 后端实现设计

### 2.1 路由注册

在 `cmd/woa-server/service/plan/service.go` 的 `initPlanService` 中新增：

```go
h.Add("ListResPlanDemandWithDeviceTypes", http.MethodPost,
    "/plans/resources/demands/list_with_device_types", s.ListResPlanDemandWithDeviceTypes)
```

### 2.2 核心处理流程

```
Handler (ListResPlanDemandWithDeviceTypes)
    │
    ├─ 1. 解析请求参数（复用 list_biz_res_plan_demand 的参数结构）
    │     - 硬编码 demand_class = "CVM"
    │     - 提取新增的 cpu_cores 和 memories 筛选条件
    │
    ├─ 2. 调用现有 list_biz_res_plan_demand 的查询逻辑，获取预测需求列表
    │     - 支持 count 模式（仅返回总数）
    │     - 支持 detail 模式（返回详情）
    │
    ├─ 3. 对每条预测记录，查询可通配机型（复用 GetCvmChargeTypeDeviceTypeV2 的逻辑）
    │     - 传入 bk_biz_id, require_type, region, zone
    │     - 获取通配后的 device_type 列表
    │
    ├─ 4. 对通配得到的 device_type 列表，批量查询机型详细信息
    │     - 从 DeviceTypesMap 缓存获取（TTL 1分钟，内存缓存）
    │     - 获取 cpu_core, memory, device_class 等信息
    │     - 应用 cpu_cores 和 memories 筛选条件过滤机型
    │
    ├─ 5. 展开、计算并排序数据
    │     - 将每条预测记录展开为多条机型明细记录
    │     - 计算每个机型的 total_os、applied_os、remained_os
    │     - 按 is_original 排序（原始机型在前）
    │
    ├─ 6. 分页处理
    │     - 对展开后的明细记录进行分页
    │     - 返回当前页的数据
    │
    └─ 7. 返回结果
```

### 2.3 性能优化策略

#### 2.3.1 多层并发优化

本接口采用两层并发策略，显著降低响应时间：

**Layer 1: 预测查询与消耗池查询并行**

- **前提条件**：本接口仅支持业务视角，`bk_biz_id` 必传且唯一
- **实现方式**：使用 goroutine 并发执行两个独立查询
  - 分支 A：查询预测需求列表（`listAllResPlanDemand`）
  - 分支 B：查询消耗池（`GetProdResConsumePoolV2`）
- **性能收益**：节省 `min(T_demand, T_consume)` 时间

**Layer 2: 通配规则查询并发**

- **实现方式**：按 `(region_id, zone_id)` 维度分组，并发查询通配规则
- **适用场景**：同一业务下有多个不同地区/可用区的预测需求
- **性能收益**：节省 `(N-1) × T_match` 时间（N 为分组数）

#### 2.3.2 机型信息缓存复用

复用现有的 `DeviceTypesMap` 缓存机制，避免重复查询数据库：

```go
// cmd/woa-server/types/device/device.go
type DeviceTypesMap struct {
    lock        sync.RWMutex
    client      *client.ClientSet
    DeviceTypes map[string]dt.DistinctDeviceType
    TTL         time.Time  // 缓存过期时间
}
```

**缓存特性**：
- **TTL**：1 分钟（`time.Now().Add(1 * time.Minute)`）
- **并发安全**：读写锁保护（`sync.RWMutex`）
- **自动更新**：过期时自动重新加载
- **存储方式**：内存缓存，无网络开销

**性能优势**：
- 机型信息查询几乎是零开销（内存读取）
- 自动处理缓存过期，无需手动维护

#### 2.3.3 其他优化策略

1. **批量查询通配规则**：对同一 `(bk_biz_id, region_id, zone_id)` 组合的预测记录，合并查询 `charge_type_device_type`，避免重复调用
2. **筛选优化**：
   - 先应用 `cpu_cores` 和 `memories` 筛选，减少需要展开的机型数量
   - 避免展开所有机型后再过滤
3. **分页处理**：
   - count 模式：需要统计展开后的总条数（可能需要预查询所有预测记录的通配机型）
   - detail 模式：对展开后的明细记录分页，支持按 `is_original` 排序

### 2.4 分页策略

由于一条预测可能对应多个可通配机型，分页有以下两种方案：

**方案 A：按预测记录分页**
- 分页粒度为预测记录，不展开机型
- 每条预测记录内嵌 `available_device_types` 数组
- count = 预测记录数
- 优点：实现简单，与现有 list 接口行为一致
- 缺点：单页数据量可能较大

**方案 B（采用）：按展开后的明细分页**
- 每个可通配机型作为独立一条记录
- 分页粒度更细
- 结果按 `is_original` 排序：原始机型（is_original=true）排在前面，通配机型排在后面
- 缺点：预测主数据会重复出现，count 计算复杂

**采用方案 B**，提供更细粒度的分页控制，方便前端按机型规格筛选和展示。

### 2.5 排序规则

默认排序规则：
1. **主排序**：按 `is_original` 降序排序（true 在前，false 在后）
2. **次排序**：在同一 `is_original` 分组内，可按其他字段（如 `device_type`、`cpu_core` 等）进行次要排序

## 3. 涉及文件

| 文件 | 变更类型 | 描述 |
|------|----------|------|
| `cmd/woa-server/service/plan/service.go` | 修改 | 新增路由注册 |
| `cmd/woa-server/service/plan/logic.go` | 新增 | Handler 实现 |
| `cmd/woa-server/logics/plan/` | 新增 | 核心聚合逻辑 |
| `docs/api-docs/api-server/api/tmp/woa_server_apis.yaml` | 修改 | 新增网关 API 定义 |
