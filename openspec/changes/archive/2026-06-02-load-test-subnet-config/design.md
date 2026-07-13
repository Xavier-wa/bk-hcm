## Context

主机申领流程（自研云 CVM 申领）的"网络信息"步骤中，压测子网的 VPC/子网范围当前无法动态配置。woa-server 作为运营管理服务，已有完整的 global_config 表访问链路（通过 data-service）。本次变更在 woa-server 新增两个接口，使运营人员可通过管理页面维护压测子网映射。

## Goals / Non-Goals

**Goals:**
- woa-server 新增 Upsert 接口，支持全量替换 region→vpc_id→[subnet_id] 三层配置
- woa-server 新增 List 接口，返回完整压测子网映射
- 复用 global_config 表存储，不引入新表或新的外部依赖
- 与现有 SpringResPool 等同类配置实现模式保持一致

**Non-Goals:**
- 不做 subnet_id 合法性校验（不验证是否存在于 DB）
- 不支持按 region/vpc 粒度的局部更新
- 不做前端页面（由前端子需求负责）

## Decisions

### 决策 1：复用 global_config 表而非新建专用表

**选择**：复用现有 `global_config` 表，以 `config_type=cvm_apply`、`config_key=load_test_subnet` 存储整个映射 JSON。

**理由**：
- 数据量极小（≤100 条子网），单行 JSON 完全满足
- 无需 DDL 变更，上线风险为零
- 与 SpringResPool、RegionDefaultVpc 等配置保持一致的存储模式

**备选方案**：新建 `load_test_subnet` 专用表 → 引入 DDL、DAO、data-service CRUD 全套工作量，对当前数据量过度设计，否决。

### 决策 2：Upsert 语义为全量替换

**选择**：每次 Upsert 都完整覆盖 config_value，不支持局部更新。

**理由**：
- 压测子网数量少，全量传输数据量可接受
- 避免了合并逻辑的复杂性和潜在的并发问题
- 需求已明确：传入空 `{}` 等同于清空配置

### 决策 3：代码分层遵循现有 SpringResPool 模式

**选择**：types → logics → service 三层分离，与项目现有模式完全对齐。

```
types/config/load_test_subnet.go    ← 请求/响应类型定义
logics/config/load_test_subnet.go   ← 业务逻辑（CRUD 封装）
service/config/load_test_subnet.go  ← HTTP Handler + 路由注册
```

**理由**：降低新贡献者的理解成本，与代码审查时的预期一致。

### 决策 4：List 接口不鉴权

**选择**：List 接口仅要求 woa-server 登录态，不做额外权限校验。

**理由**：该数据用于申领表单展示，面向所有 CVM 申领用户，不含敏感信息。

## Risks / Trade-offs

- **JSON 字段大小限制**：MySQL JSON 字段上限 65535 字节，当前配置体积远小于此，但若未来子网数量极度膨胀（>1000 条）需考虑分行存储。当前数据量下风险可忽略。→ **缓解**：文档注明此存储方式的规模限制。
- **全量替换的并发风险**：若两个管理员同时 Upsert，后者会覆盖前者。→ **缓解**：Upsert 接口仅限管理员操作，并发写入概率极低，且业务接受此语义。
- **config_value 反序列化失败**：若 DB 中 JSON 格式损坏，List 接口会返回 500。→ **缓解**：在反序列化处加 Error 日志，便于排查。
