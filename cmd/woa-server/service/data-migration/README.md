# 数据迁移框架

通用的数据迁移框架，支持从MongoDB到MySQL的数据迁移和同步。

## 特性

- ✅ **配置驱动**: 新增表迁移只需添加配置和转换器，无需修改核心代码
- ✅ **智能同步**: 自动对比数据差异，支持增量更新
- ✅ **批量处理**: 自动分批处理（data-service限制500条/批）
- ✅ **并发处理**: 支持多goroutine并发处理批次，可配置并发数（1-10）
- ✅ **级联迁移**: 支持主子表依赖关系的自动级联迁移
- ✅ **多种模式**: 支持创建、更新、同步三种模式
- ✅ **DryRun**: 支持模拟运行，预览迁移效果
- ✅ **详细报告**: 提供完整的迁移统计和错误详情

## 架构设计

```
cmd/woa-server/service/data-migration/
├── framework/           # 核心框架
│   ├── types.go        # 基础类型定义
│   └── migrator.go     # 迁移引擎
├── config/             # 配置层
│   ├── types.go        # 配置类型
│   └── tables_config.go     # 数据表迁移配置（CVM申请、回收等）
├── converters/         # 转换器层
│   ├── interface.go    # 转换器接口
│   ├── registry.go     # 转换器注册表
│   ├── cvm_apply_order.go      # 主单转换器
│   └── cvm_apply_suborder.go   # 子单转换器
├── api_router.go       # API路由器（支持自动分批）
├── service.go          # HTTP服务层
└── init.go             # 初始化
```

## 使用方法

### 1. API接口

#### 执行迁移

```bash
POST /api/v1/woa/data_migration/migrate

{
  "table_name": "cr_ApplyTicket",
  "filter": {
    "time_range": {
      "field": "create_at",
      "start_time": "2024-01-01T00:00:00Z",
      "end_time": "2024-12-31T23:59:59Z"
    },
    "fields": {
      "stage": {"$in": ["RUNNING", "DONE"]}
    }
  },
  "mode": "sync",
  "dry_run": false,
  "include_dependencies": true,
  "batch_size": 100,
  "concurrency": 5
}
```

#### 参数说明

- `table_name`: 配置名称（如 `cr_ApplyTicket`、`cr_ApplyOrder`）
- `filter`: 过滤条件
  - `time_range`: 时间范围过滤
  - `fields`: 字段过滤（MongoDB查询语法）
  - `custom_query`: 自定义MongoDB查询
- `mode`: 迁移模式
  - `create_only`: 只创建新数据，跳过已存在
  - `update_only`: 只更新已存在的数据
  - `sync`: 同步模式（创建+更新）
- `dry_run`: 是否模拟运行（不实际执行）
- `include_dependencies`: 是否包含依赖表（级联迁移）
- `batch_size`: 业务批次大小
- `concurrency`: 并发数

#### 响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "total": 1000,
    "success": 980,
    "failed": 20,
    "skipped": 500,
    "created": 400,
    "updated": 80,
    "duration": "2m30s",
    "start_time": "2026-01-15T10:00:00Z",
    "end_time": "2026-01-15T10:02:30Z",
    "errors": [
      {
        "primary_key": 12345,
        "table": "ziyan_cvm_apply_order",
        "action": "create",
        "error": "duplicate key"
      }
    ],
    "dependencies": {
      "cvm_apply_suborder": {
        "total": 5000,
        "created": 3000,
        "updated": 2000
      }
    }
  }
}
```

#### 列出所有配置

```bash
GET /api/v1/woa/data_migration/configs
```

### 2. 使用场景示例

#### 场景1: 首次全量迁移

```bash
# 1. DryRun预览
curl -X POST /api/v1/woa/data_migration/migrate -d '{
  "table_name": "cvm_apply_order",
  "mode": "sync",
  "dry_run": true,
  "include_dependencies": true
}'

# 2. 实际迁移
curl -X POST /api/v1/woa/data_migration/migrate -d '{
  "table_name": "cvm_apply_order",
  "mode": "sync",
  "include_dependencies": true,
  "batch_size": 50
}'
```

#### 场景2: 按时间范围增量同步

```bash
curl -X POST /api/v1/woa/data_migration/migrate -d '{
  "table_name": "cvm_apply_order",
  "filter": {
    "time_range": {
      "field": "create_at",
      "start_time": "2024-01-01T00:00:00Z"
    }
  },
  "mode": "sync",
  "include_dependencies": true
}'
```

#### 场景3: 按状态过滤迁移

```bash
curl -X POST /api/v1/woa/data_migration/migrate -d '{
  "table_name": "cvm_apply_suborder",
  "filter": {
    "fields": {
      "stage": {"$in": ["RUNNING", "DONE"]},
      "status": {"$in": ["DONE"]}
    }
  },
  "mode": "update_only"
}'
```

#### 场景4: 指定订单重新同步

```bash
curl -X POST /api/v1/woa/data_migration/migrate -d '{
  "table_name": "cvm_apply_order",
  "filter": {
    "fields": {
      "order_id": {"$in": [12345, 67890]}
    }
  },
  "mode": "sync",
  "include_dependencies": true
}'
```

## 新增表迁移

只需三步即可新增表的迁移支持：

### 步骤1: 定义转换器

```go
// converters/new_table_converter.go
type NewTableConverter struct{}

func (c *NewTableConverter) GetName() string {
    return "new_table_converter"
}

func (c *NewTableConverter) ConvertToCreate(source interface{}) (interface{}, error) {
    // 实现MongoDB -> MySQL Create Request转换
}

func (c *NewTableConverter) ConvertToUpdate(source, target interface{}) (interface{}, error) {
    // 实现MongoDB -> MySQL Update Request转换
}

func (c *NewTableConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
    // 提取主键
}

func (c *NewTableConverter) CompareData(source, target interface{}, fields []string) (bool, []string) {
    // 对比数据
}

func init() {
    Register(&NewTableConverter{})
}
```

### 步骤2: 注册配置

#### 单字段主键配置示例

```go
// config/new_table_config.go
func init() {
    Register(&TableMigrationConfig{
        Name:             "new_table",
        Description:      "新表迁移",
        Enabled:          true,
        SourceDB:         "mongodb",
        SourceTable:      "cr_NewTable",
        TargetDB:         "mysql",
        TargetTable:      "ziyan_new_table",
        // 单字段主键使用数组格式
        SourcePrimaryKeys: []string{"id"},
        TargetPrimaryKeys: []string{"id"},
        ConverterName:    "new_table_converter",
        CompareFields:    []string{"status", "count"},
        BatchSize:        100,
        Concurrency:      5,
    })
}
```

#### 复合主键配置示例

对于具有联合唯一键的表（如 `UNIQUE KEY (suborder_id, step_name)`），配置多个主键字段：

```go
// config/new_table_config.go
func init() {
    Register(&TableMigrationConfig{
        Name:             "new_table",
        Description:      "新表迁移（复合主键）",
        Enabled:          true,
        SourceDB:         "mongodb",
        SourceTable:      "cr_NewTable",
        TargetDB:         "mysql",
        TargetTable:      "ziyan_new_table",
        // 复合主键：多个字段组成联合唯一键
        SourcePrimaryKeys: []string{"suborder_id", "step_name"},
        TargetPrimaryKeys: []string{"suborder_id", "step_name"},
        ConverterName:     "new_table_converter",
        CompareFields:     []string{"status", "count"},
        BatchSize:         100,
        Concurrency:       5,
    })
}
```

**复合主键转换器实现要点**：

```go
// ExtractPrimaryKey 需要返回 map[string]interface{} 类型
func (c *NewTableConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
    // 从源数据提取
    if source, ok := data.(*tasktypes.SourceType); ok {
        return map[string]interface{}{
            "suborder_id": source.SubOrderId,
            "step_name":   source.StepName,
        }, nil
    }
    // 从目标数据提取
    if target, ok := data.(*table.TargetType); ok {
        return map[string]interface{}{
            "suborder_id": target.SuborderID,
            "step_name":   target.StepName,
        }, nil
    }
    return nil, fmt.Errorf("data type mismatch")
}
```

### 步骤3: 添加API路由支持

在 `api_router.go` 中添加对应的case分支。

## 关键设计

### 1. 自动分批

data-service接口限制单次最多100条，框架自动分批：

- 业务批次：由 `batch_size` 参数控制（如1000条）
- API批次：框架自动按100条分批调用API

### 2. 依赖关系（级联迁移）

配置中定义依赖关系，自动级联迁移：

```go
Dependencies: []*DependencyConfig{
    {
        ConfigName:   "cvm_apply_suborder",  // 子表配置
        RelationType: "one_to_many",         // 一对多
        ForeignKey:   "order_id",            // 外键
    },
},
```

迁移主表时，自动迁移所有关联的子表。

### 3. 智能对比

支持多种数据类型的对比：

- 字符串：精确匹配
- 数值：类型自动转换后对比
- JSON：深度对比
- 时间：允许1秒误差

### 4. 幂等性

- 已存在的数据：对比后决定是否更新
- 支持多次运行：不会重复创建数据

## 监控和日志

框架提供详细的日志输出：

```
[INFO] Start migration for table: cvm_apply_order (mode: sync)
[INFO] Found 1000 records from MongoDB table: cr_ApplyTicket
[INFO] Split into 10 batches (batch_size: 100)
[INFO] Processing batch 1/10 (size: 100)
[INFO] Batch creating 80 records for table ziyan_cvm_apply_order
[INFO] Batch updating 20 records for table ziyan_cvm_apply_order
[INFO] Processing dependency: cvm_apply_suborder (relation: one_to_many)
[INFO] Migration completed: total=1000, created=800, updated=200, skipped=0, failed=0, duration=2m30s
```

## 注意事项

1. **生产环境**：
   - 先使用 `dry_run: true` 预览
   - 建议设置合适的 `batch_size`（建议100-500）
   - 避免高峰期执行大批量迁移

2. **数据一致性**：
   - 迁移期间避免修改源数据
   - 建议在业务低峰期执行
   - 重要数据迁移后需要验证

3. **性能优化**：
   - 调整 `batch_size`（建议100-500，根据单条数据大小）
   - 调整 `concurrency`（并发数1-10，默认1串行，推荐3-5）
   - 大表分批迁移（按时间范围分段执行）
   - 避免过大的过滤条件
   - 并发数越高速度越快，但会增加数据库压力

4. **并发处理说明**：
   - `Concurrency=1`：串行处理，安全但较慢
   - `Concurrency=3-5`：推荐配置，平衡性能和稳定性
   - `Concurrency=10`：最大并发，适用于低峰期快速迁移
   - 并发处理使用goroutine池，自动分配任务到多个worker
   - 所有worker共享同一个kit.Kit，保持请求上下文一致

## 故障排查

### 问题1: 迁移失败

检查日志中的错误详情：

```json
{
  "errors": [
    {
      "primary_key": 12345,
      "action": "create",
      "error": "详细错误信息"
    }
  ]
}
```

### 问题2: 数据不一致

使用 `update_only` 模式重新同步：

```bash
curl -X POST /api/v1/woa/data_migration/migrate -d '{
  "table_name": "cvm_apply_order",
  "filter": {"fields": {"order_id": 12345}},
  "mode": "update_only"
}'
```

### 问题3: 性能问题

- 减小 `batch_size`
- 增加 `concurrency`（谨慎使用）
- 分时间段迁移

## 扩展性

框架设计支持：

- 新增表迁移（只需配置+转换器）
- 自定义对比规则
- 自定义转换逻辑
- 多级依赖关系
- 不同数据库类型

## 技术栈

- Go 1.18+
- MongoDB Driver
- HCM Data-service Client
- Filter Expression
- RESTful API
