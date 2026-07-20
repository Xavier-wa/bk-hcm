## Purpose

Track sync status for TCloud Ziyan public resources so account sync details reflect successful and failed `region`, `zone`, and `image` synchronization.

## Requirements

### Requirement: 公共资源同步状态记录

自研云公共资源 `region`、`zone`、`image` 同步流程 SHALL 写入账号资源同步状态，状态语义 SHALL 与现有私有资源同步状态一致。

#### Scenario: 公共资源同步开始时记录同步中

- **GIVEN** 自研云账号资源同步任务触发，且公共资源同步被启用
- **WHEN** 系统开始同步 `region`、`zone` 或 `image`
- **THEN** 系统 SHALL 为对应资源写入 `syncing` 状态

#### Scenario: 公共资源同步成功时记录成功

- **GIVEN** 自研云公共资源 `region`、`zone` 或 `image` 已开始同步
- **WHEN** 对应资源同步成功完成
- **THEN** 系统 SHALL 为对应资源写入 `sync_success` 状态并刷新最近同步时间

#### Scenario: 公共资源同步失败时记录失败

- **GIVEN** 自研云公共资源 `region`、`zone` 或 `image` 已开始同步
- **WHEN** 对应资源同步失败
- **THEN** 系统 SHALL 为对应资源写入 `sync_failed` 状态、刷新最近同步时间并保存失败原因

### Requirement: 公共资源状态项自动创建

自研云公共资源同步状态记录不存在时，系统 SHALL 在首次写入同步状态时自动创建状态项。

#### Scenario: 可用区首次成功同步创建状态项

- **GIVEN** 自研云账号没有 `zone` 同步状态记录
- **WHEN** `zone` 同步成功完成
- **THEN** 系统 SHALL 创建 `zone` 状态项，状态为 `sync_success`，并记录最近同步时间

#### Scenario: 地域和镜像历史失败状态被成功结果刷新

- **GIVEN** 自研云账号已有 `region` 或 `image` 历史失败状态
- **WHEN** 对应公共资源后续同步成功
- **THEN** 系统 SHALL 将状态更新为 `sync_success`，并将最近同步时间刷新为本次同步完成时间

### Requirement: 公共资源同步顺序与范围保持不变

系统 SHALL 保持现有自研云公共资源同步顺序和触发范围，仅补充状态记录行为。

#### Scenario: 公共资源同步顺序保持不变

- **GIVEN** 自研云公共资源同步被启用
- **WHEN** 系统执行公共资源同步
- **THEN** 系统 SHALL 仍按 `region -> zone -> image` 顺序执行同步

#### Scenario: 公共资源仅随首个账号同步一次

- **GIVEN** 某租户和自研云厂商下存在多个可同步账号
- **WHEN** 周期同步任务遍历这些账号
- **THEN** 系统 SHALL 保持公共资源仅随第一个账号触发一次的既有行为

### Requirement: 失败传播保持兼容

公共资源同步失败时，系统 SHALL 保持既有错误返回语义，使上层调度能够识别失败资源类型和错误原因。

#### Scenario: 地域同步失败返回地域资源类型

- **GIVEN** 自研云公共资源同步被启用
- **WHEN** `region` 同步失败
- **THEN** 系统 SHALL 返回 `region` 资源类型及原始错误

#### Scenario: 可用区同步失败返回可用区资源类型

- **GIVEN** 自研云公共资源同步被启用，且 `region` 同步成功
- **WHEN** `zone` 同步失败
- **THEN** 系统 SHALL 返回 `zone` 资源类型及原始错误

#### Scenario: 镜像同步失败返回镜像资源类型

- **GIVEN** 自研云公共资源同步被启用，且 `region`、`zone` 同步成功
- **WHEN** `image` 同步失败
- **THEN** 系统 SHALL 返回 `image` 资源类型及原始错误
