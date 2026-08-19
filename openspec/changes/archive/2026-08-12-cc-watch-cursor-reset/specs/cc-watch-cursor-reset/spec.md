# cc-watch-cursor-reset Specification

## Purpose

CC watch 事件消费 cursor 的管理员应急重置能力：消息堆积落后太久时，将消费位置重置到最新，跳过全部积压事件，并异步触发主机补偿同步。

## Requirements

### Requirement: 重置 cursor 接口

系统 SHALL 提供 admin 接口 `POST /api/v1/cloud/admin/system/cc_sync/watch/cursor/reset`（直连、不走网关、系统内部调用无需鉴权），按目标租户 + `resource` 限定重置范围。目标租户取请求头 `X-Bk-Tenant-Id`（直连场景由调用方在 header 中指定），body 仅传 `resource`。本期接口语义固定为「重置到最新」：将 etcd 中对应 cursor 置空，由 CC 在 cursor 为空时从当前时间起 watch。该接口为应急操作，允许跳过中间未消费事件。接口 MUST NOT 接受调用方传入 cursor 或时间参数。

#### Scenario: 重置到最新

- **WHEN** 以合法目标租户（header）+ `resource` 调用重置接口
- **THEN** 系统将 etcd 中 `/hcm/event/cc/{tenant_id}/{resource}` 的值置空，watch 下一轮从当前时间起消费，跳过全部积压事件

#### Scenario: 参数缺失或格式错误

- **WHEN** 请求缺少 `resource` 或 header 租户，或字段格式不合法
- **THEN** 系统返回 InvalidParameter 错误，不修改 etcd 中的 cursor，不触发主机补偿

### Requirement: 参数校验

系统 SHALL 在写入前完成校验：`resource` 必须在白名单（`host` / `host_relation`）内；header 中的目标租户必须存在。任一校验失败 MUST 拒绝且不写入。

#### Scenario: 非法 resource 类型

- **WHEN** `resource` 不在 `host` / `host_relation` 白名单内
- **THEN** 系统返回 InvalidParameter 错误，不修改 etcd 中的 cursor

#### Scenario: 租户不存在

- **WHEN** header 中的目标租户不存在
- **THEN** 系统返回 InvalidParameter 错误，不修改 etcd 中的 cursor

### Requirement: 写入与生效

系统 SHALL 复用 watcher 的 lease 将空 cursor 写入 etcd（与 watch 消费同一写路径），不写其他存储、不引入第二写者。生效方式为 watch 循环下一轮从 etcd 重读 cursor，MUST NOT 要求重启 master，也不对 watch goroutine 做通知/中断；正在执行中的当轮消费不被打断。

#### Scenario: 下一轮生效

- **WHEN** 重置接口成功写入 etcd
- **THEN** 对应 watch 流下一轮循环读取到空 cursor 并从当前时间起消费，全程无需重启

#### Scenario: etcd 写入失败

- **WHEN** 写 etcd 失败
- **THEN** 系统返回错误，cursor 保持原值，不触发主机补偿

### Requirement: 主机补偿同步

系统 SHALL 在 cursor 重置成功后异步触发该租户下的主机补偿同步。补偿范围 MUST 仅覆盖 CC 增量涉及的厂商（`tcloud-ziyan`、`other`）且仅同步主机资源。补偿 MUST NOT 要求 master 校验。补偿编排 SHALL 按账号维度执行，互斥信号以 DB `account_sync_detail` 中该账号 `res_name=cvm` 的 `res_status=syncing` 为主判断，并抢主机补偿专用锁 `lock.ResKey(accountID, cvm)` 防并发。进行中的账号 MUST 跳过且不报错，MUST NOT 打断定时全量同步。

#### Scenario: 重置成功后异步触发主机补偿

- **WHEN** cursor 重置写入 etcd 成功
- **THEN** 系统异步触发该租户下 `tcloud-ziyan` 与 `other` 的主机同步，接口立即返回，不等待补偿完成

#### Scenario: 账号主机同步已在进行时跳过

- **WHEN** 补偿编排处理某账号时，DB 中该账号 cvm 状态为 syncing，或主机补偿专用锁已被占用
- **THEN** 系统跳过该账号的主机补偿，继续处理其他账号，不返回错误、不影响定时全量

#### Scenario: 同账号其他资源同步不误伤主机补偿

- **WHEN** 补偿编排处理某账号时，该账号 cvm 未在 syncing，但账号级锁 `lock.Key(accountID)` 被其他资源同步（如条件同步）占用
- **THEN** 系统不检查账号级锁，正常触发该账号的主机补偿

#### Scenario: 补偿只同步主机

- **WHEN** 主机补偿对某账号执行同步
- **THEN** 系统仅同步主机资源，不同步该账号下的其他云资源类型

### Requirement: 审计日志与响应回显

系统 SHALL 在重置前、后各打一条 Info 级审计日志，内容包含旧 cursor、tenant、resource、rid。响应 MUST 回显重置前的 cursor，供操作员确认实际生效的重置结果。

#### Scenario: 审计留痕

- **WHEN** 一次重置操作执行（无论成功或失败）
- **THEN** 日志中可通过 rid 找到包含旧 cursor、tenant、resource 的审计记录

#### Scenario: 响应回显重置前 cursor

- **WHEN** 重置成功
- **THEN** 响应中携带重置前的 cursor，为空表示重置前已处于「从当前时间起」的状态
