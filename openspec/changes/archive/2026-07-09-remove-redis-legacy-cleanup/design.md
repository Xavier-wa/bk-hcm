## Context

`woa-server` 仍保留了一条历史 Redis 集成路径，主要体现在：
- 启动阶段会执行 Redis 初始化；
- 配置模型中存在 Redis 必填校验；
- Mongo 本地事务管理器仍引用 Redis 事务号/错误键；
- 仓库中仍有 Redis DAL/driver 与 Redis 锁工具包。

当前生产路径不依赖 CMDB 风格的跨请求分布式事务语义，`woa-server` 实际业务事务已统一到原生 Mongo 事务流。因此 Redis 路径属于无效遗留代码，会持续带来不必要的运维耦合与部署负担。

## Goals / Non-Goals

**Goals:**
- 移除 `woa-server` 的 Redis 启动依赖。
- 移除 `woa-server` 配置契约中的 Redis 必填项及部署配置段。
- 在不改变当前有效事务行为的前提下，移除 Mongo 事务管理器中的 Redis 耦合。
- 删除 Redis 遗留代码目录与模块依赖（`go-redis`）。
- 保持对外 API 行为不变。

**Non-Goals:**
- 不对事务语义做额外重构（仅移除未使用的 Redis 支撑路径）。
- 不引入新的缓存、锁或分布式协调机制。
- 不改动非 `woa-server` 服务的功能行为。

## Decisions

### 1) Remove Redis from startup initialization path
- **Decision:** 删除 `woa-server` 启动阶段 Redis 客户端初始化逻辑。
- **Rationale:** 服务启动仅应依赖真实生效的基础设施。
- **Alternative considered:** 保留初始化并加开关。否决原因是仍保留死代码复杂度和隐式耦合。

### 2) Remove Redis from configuration contract
- **Decision:** 移除 `woa-server` 配置结构中的 `redis` 字段使用及必填校验，同时删除对应 YAML/Helm 配置段。
- **Rationale:** 配置契约应与运行时真实依赖一致；“必填但无效”的配置属于运维债务。
- **Alternative considered:** 保留字段但改为可选。否决原因是可选死配置仍会造成认知干扰和维护成本。

### 3) Decommission Redis-backed TxnManager path
- **Decision:** 移除 Mongo 本地事务管理器中基于 Redis 的事务号/错误记账逻辑，保留当前业务使用的原生 Mongo 事务流。
- **Rationale:** 现网有效链路不依赖 Redis 事务键；原生 Mongo session/transaction 对当前场景已足够。
- **Alternative considered:** 保留 Redis 记账作为兜底。否决原因是继续背负未使用且未验证的死分支。

### 4) Remove Redis implementation artifacts and dependency
- **Decision:** 删除 Redis DAL/driver 目录和死代码锁工具包，并移除 `go-redis` 模块依赖。
- **Rationale:** 保证清理彻底，避免通过陈旧包被误引用而回流。
- **Alternative considered:** 保留包仅断开引用。否决原因是会继续增加代码库熵值。

## Risks / Trade-offs

- **[Risk] 仍存在隐藏链路依赖 Redis 事务键** → **Mitigation:** 核查事务 Header 驱动的分布式语义是否有有效调用链，并补充事务相关回归验证。
- **[Risk] 模板外部署脚本仍注入 Redis 假设** → **Mitigation:** 全局检索并清理 `woa-server` 的 redis 配置引用，执行模板渲染检查。
- **[Risk] 清理范围误伤其他服务** → **Mitigation:** 变更严格收敛在 `woa-server` 与明确列出的共享配置/模型面。
- **[Trade-off] 清理后若需恢复需重新实现** → **Mitigation:** 通过 OpenSpec 完整记录设计与回滚策略，保证可追溯。

## Migration Plan

1. 移除 `woa-server` 中 Redis 运行时初始化路径。
2. 移除 Redis 配置结构、校验逻辑与配置模板段。
3. 重构 Mongo `TxnManager`，去除 Redis 事务键逻辑并保持当前有效事务语义。
4. 删除 Redis DAL/driver 与死代码锁包。
5. 更新 `go.mod/go.sum`，移除 `go-redis`。
6. 执行 `woa-server` 启动与事务相关关键路径回归验证。

**Rollback strategy:**
- 若发现未识别依赖，可按单次变更集整体回滚。
- 本次不涉及数据迁移，回滚风险主要集中在代码路径。

## Open Questions

- 是否存在仓库外部署 overlay 仍渲染 `woa-server.redis` 配置，需要同步调整？
- 是否存在内部运维看板依赖将被删除的 Redis 健康项命名？
