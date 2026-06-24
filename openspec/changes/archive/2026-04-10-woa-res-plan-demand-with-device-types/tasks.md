# 任务清单：预测需求聚合接口 - 带可通配机型明细

## 后端开发

### 1. 路由与 Handler 层
- [x] 1.1 woa-server 路由注册：在 `initBizPlanService` 中新增 `ListBizResPlanDemandWithDeviceTypes` 路由（biz 路径组）
- [x] 1.2 Handler 层实现：
  - 解析请求参数（`bk_biz_id` 必选，单值）
  - 硬编码 `demand_class = "CVM"`
  - 提取新增的 `cpu_cores` 和 `memories` 筛选条件
  - 调用聚合逻辑，返回响应

### 2. 核心聚合逻辑实现

#### 2.1 并发查询 Layer 1：预测与消耗池并行
- [x] 2.1.1 实现并发查询逻辑（使用 goroutine）：
  - 分支 A：查询预测需求列表（`listAllResPlanDemand`，硬编码 `demand_class = "CVM"`）
  - 分支 B：查询消耗池（`GetProdResConsumePoolV2`，传入 `bk_biz_id`）
- [x] 2.1.2 错误处理：任一分支失败则返回错误
- [x] 2.1.3 等待所有并发查询完成（`sync.WaitGroup`）

#### 2.2 机型信息获取（缓存复用）
- [x] 2.2.1 从 `DeviceTypesMap` 缓存获取机型详细信息：
  - 调用 `deviceTypesMap.GetDeviceTypes(kt)`
  - 获取 `cpu_core`、`memory`、`device_class` 等字段
  - 缓存自动管理（TTL 1分钟，无需手动维护）

#### 2.3 通配规则查询与展开
- [x] 2.3.1 按 `(region_id, zone_id)` 分组预测记录，减少重复查询
- [x] 2.3.2 并发查询通配规则（Layer 2 优化）：
  - 对每个分组并发调用通配规则逻辑（复用 `GetCvmChargeTypeDeviceTypeV2`）
  - 传入 `bk_biz_id`、`region_id`、`zone_id` 等参数
  - 获取可通配机型列表
- [x] 2.3.3 应用 `cpu_cores` 和 `memories` 筛选条件过滤机型（在展开前过滤）
- [x] 2.3.4 展开预测记录为机型明细记录：
  - 原始机型：标记 `is_original = true`
  - 通配机型：标记 `is_original = false`
  - 无通配机型时：仅返回原始机型（不展开）
- [x] 2.3.5 计算每个机型的 `total_os`、`applied_os`、`remained_os`：
  - 基于 CPU 核心数计算：`os = cpu_core_total / cpu_core`
  - 处理整除和取整逻辑
- [x] 2.3.6 排序：
  - 主排序：`is_original` 降序（原始机型在前）
  - 次排序：可按 `device_type` 或 `cpu_core` 排序

### 3. 分页逻辑实现
- [x] 3.1 count 模式：
  - 统计展开后的总条数（需要预查询所有预测记录的通配机型）
  - 返回 count 值，不返回详情
- [x] 3.2 detail 模式：
  - 对展开后的明细记录分页
  - 支持排序（`is_original` 优先）
  - 返回当前页数据

### 4. 数据结构定义
- [x] 4.1 请求参数结构体：
  - `bk_biz_id`：必选，单值（int）
  - 新增 `cpu_cores` 和 `memories` 筛选字段
  - 移除 `demand_classes`（硬编码为 CVM）
- [x] 4.2 响应结构体：
  - 扁平化设计：每条机型明细独立成条
  - 包含预测主数据 + 机型明细字段
  - `is_original` 标识原始/通配机型

## API 网关

- [x] 5. woa_server_apis.yaml 新增 API 路由定义：
  - 路径：`POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/demands/list_with_device_types`
  - 请求参数定义
  - 响应结构定义

## 测试

### 6. 单元测试
- [x] 6.1 聚合逻辑测试：
  - 预测数据获取
  - 通配规则匹配
  - 机型展开逻辑
- [x] 6.2 OS 数计算逻辑测试：
  - 整除场景
  - 非整除场景
- [x] 6.3 排序逻辑测试：
  - `is_original` 排序
  - 多字段组合排序
- [x] 6.4 筛选逻辑测试：
  - `cpu_cores` 筛选
  - `memories` 筛选


## 文档

- [x] 10. API 文档编写（参考 `docs/api-docs/web-server/docs/biz/scr/resource-plan/list_biz_resource_plan_demand.md` 格式）：
  - [x] 10.1 创建文档：`docs/api-docs/web-server/docs/biz/scr/resource-plan/list_biz_resource_plan_demand_with_device_types.md`
  - [x] 10.2 描述部分：
    - 版本信息
    - 权限要求
    - 功能描述
  - [x] 10.3 URL 定义：
    - `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/demands/list_with_device_types`
  - [x] 10.4 输入参数说明：
    - 参数表格（参考现有格式）
    - `bk_biz_id` 标注为必选
    - 新增 `cpu_cores` 和 `memories` 参数
    - 移除 `demand_classes` 参数说明
  - [x] 10.5 调用示例：
    - 完整的 JSON 请求示例
    - 包含筛选条件示例
  - [x] 10.6 响应示例：
    - 完整的 JSON 响应结构
    - 包含展开后的多条机型明细
  - [x] 10.7 响应参数说明：
    - `data.count` 说明
    - `data.details[]` 字段说明
    - 机型明细字段（`device_type`、`is_original`、`cpu_core`、`memory` 等）
    - OS 计算字段说明

## 验收标准

- [ ] 11. 功能验收：
  - 单次接口调用返回预测数据 + 所有可通配机型明细
  - 硬编码 CVM 类型，无需传入 `demand_classes`
  - 无通配机型时仅返回原始机型
  - 筛选和分页功能正常
- [ ] 12. 性能验收：
  - 响应时间 < 500ms（正常数据量）
  - 并发查询生效，优于串行调用 3 个接口的总耗时
  - 机型缓存复用生效
- [ ] 13. 代码质量：
  - 单元测试覆盖率 > 80%
  - 代码符合项目规范
  - 无明显性能瓶颈
