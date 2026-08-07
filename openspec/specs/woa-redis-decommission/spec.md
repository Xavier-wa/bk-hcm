# woa-redis-decommission Specification

## Purpose
明确 `woa-server` 去除 Redis 遗留依赖后的行为契约：启动、配置校验与 Mongo 事务流程均不依赖 Redis，且 Redis 相关死代码与模块依赖已从仓库移除。

## Requirements

### Requirement: WOA server startup SHALL not require Redis
`woa-server` 进程 SHALL 在不存在 Redis 初始化路径的情况下完成启动和运行，并且 MUST NOT 因 Redis 服务缺失或 Redis 连通性问题导致启动失败。

#### Scenario: Startup without Redis service
- **GIVEN** a deployment where Redis is not provisioned
- **WHEN** `woa-server` starts with valid non-Redis dependencies
- **THEN** service startup succeeds and does not attempt Redis client initialization

### Requirement: WOA server configuration SHALL not require Redis section
`woa-server` 运行时配置结构与校验逻辑 SHALL NOT 要求存在 `redis` 配置段，部署配置模板也 SHALL NOT 为 `woa-server` 输出 Redis 配置。

#### Scenario: Config validation without redis block
- **GIVEN** a `woa-server` config file without `redis` fields
- **WHEN** configuration validation runs during startup
- **THEN** validation passes as long as other required sections are valid

### Requirement: Mongo transaction flow SHALL not depend on Redis transaction keys
`woa-server` 使用的 Mongo DAL 事务流程 SHALL 仅依赖原生 Mongo transaction/session 机制，并且 MUST NOT 依赖 Redis 中的事务号或事务错误键。

#### Scenario: Transaction execution through native Mongo path
- **GIVEN** a request path that executes Mongo DAL transaction logic
- **WHEN** transaction commit or abort handling is executed
- **THEN** behavior does not include Redis key read/write operations and remains functionally equivalent for currently used paths

### Requirement: Legacy Redis code and dependency SHALL be removed
`woa-server` 相关 Redis 遗留死代码与模块依赖 SHALL 被移除，包括 Redis DAL/driver 目录、未使用的锁工具代码，以及 Go 模块中的 `go-redis` 引用。

#### Scenario: Source tree and module graph after cleanup
- **GIVEN** the repository at this change revision
- **WHEN** inspecting Redis legacy directories and module dependencies
- **THEN** Redis DAL/driver and dead lock code are absent and `github.com/go-redis/redis/v7` is not referenced by `go.mod`
