## 1. 移除 Redis 运行时与配置耦合

- [x] 1.1 移除 `woa-server` Redis 启动初始化路径及相关 import/调用点。
- [x] 1.2 从 `woa-server` 配置模型中移除 Redis 字段及必填校验。
- [x] 1.3 删除 `cmd/woa-server/etc/woa_server.yaml` 与 Helm 模板中的 Redis 配置段。

## 2. 移除 Redis 事务耦合与死代码路径

- [x] 2.1 重构 Mongo 本地事务管理器，移除 Redis 事务号/错误键逻辑并保持当前生效的原生 Mongo 事务行为。
- [x] 2.2 更新因 `TxnManager` 去 Redis 耦合而受影响的 DAL 接口与调用方。
- [x] 2.3 删除 Redis 遗留目录：`cmd/woa-server/storage/driver/redis` 与 `cmd/woa-server/storage/dal/redis`。
- [x] 2.4 删除死代码锁工具包 `pkg/tools/lock` 并清理残余引用。

## 3. 清理依赖并完成安全验证

- [x] 3.1 从 `go.mod/go.sum` 移除 `github.com/go-redis/redis/v7`，并执行模块整理与校验。
- [x] 3.2 对 `woa-server` 与受影响包执行编译/测试，确认无行为回归。
- [x] 3.3 验证本地与部署配置下，`woa-server` 在无 Redis 依赖时可正常启动，事务相关关键路径正常。
