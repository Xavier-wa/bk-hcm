## Why

`woa-server` 中的 Redis 集成路径属于历史遗留代码，当前没有有效业务读写链路接入，但仍然引入了启动依赖、配置负担和部署复杂度。现在清理这条死代码路径，可以在不影响现有功能的前提下降低运维风险和维护成本。

## What Changes

- 移除 `woa-server` 的 Redis 启动依赖，包括初始化路径和相关耦合代码。
- 移除服务配置模型中 Redis 的必填项与校验逻辑。
- 移除 Mongo 本地 DAL 中 `TxnManager` 对 Redis 的事务号/错误信息依赖路径。
- 删除 `cmd/woa-server/storage/driver/redis` 与 `cmd/woa-server/storage/dal/redis` 目录下的 Redis 遗留实现。
- 删除 `pkg/tools/lock` 中未使用的 Redis 锁死代码。
- 删除 `woa-server` YAML 与 Helm 模板中的 Redis 配置段。
- 从模块依赖中移除 `github.com/go-redis/redis/v7`。

## Capabilities

### New Capabilities
- `woa-redis-decommission`: 在保持当前 Mongo 事务行为和服务启动行为不变的前提下，完成 `woa-server` Redis 遗留能力下线。

### Modified Capabilities
- *(none)*

## Impact

- 影响代码范围：`cmd/woa-server/service`、`cmd/woa-server/storage/dal/mongo/local`、`pkg/cc`、部署配置模板。
- 影响依赖：Go 模块依赖图中移除 `go-redis`。
- 影响运行时：`woa-server` 启动阶段不再依赖外部 Redis 服务。
- API 影响：预期无外部接口行为变化。
