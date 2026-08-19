# ziyan-cvm-subnet-available-ip Specification

## Purpose

自研云 CVM 子网下拉接口返回与生产调度同源的实时可用 IP（CRP `leftIpNum`）。失败或未查询时字段为 `null`，明确剩余 0 时为数字 `0`；可用 IP 失败不阻断子网清单返回。

## Requirements

### Requirement: 下拉接口返回与调度同源的实时可用 IP

woa-server SHALL 在现有接口 `POST /api/v1/woa/config/findmany/config/cvm/subnet` 的每条 `info` 上返回 `available_ip_count`。该字段 MUST 取自 CRP `getRealSubnetInfo` 的 `leftIpNum`（客户端 `cvmapi.QueryRealCvmSubnet`，实例 MUST 为 `thirdCli.CVM`），与生产调度 `getCvmSubnet` 同一数据源与同一客户端。本能力仅覆盖自研云（`tcloud_ziyan`）。

请求参数保持 `{ region, zone, vpc }` 不变。子网清单仍由 MySQL `enable_cvm = true` 决定，CRP 结果不得新增或删除清单项。

`available_ip_count` MUST 为 `*uint64`。写入时将 CRP `leftIpNum`（`int`）转换为 `uint64` 指针。CRP 失败或未查询时为 `null`；CRP 明确返回 `0` 或单项未命中时为数字 `0`。

该接口的 CRP 填充 MUST 走独立方法（`GetAllSubnetWithAvailIP`），不得嵌入被调度复用的 `GetAllSubnet`。

#### Scenario: 正常返回 CRP 剩余 IP

- **GIVEN** 请求已带自研云 region、zone、vpc，且 DB 中存在 `enable_cvm = true` 的子网
- **WHEN** 调用 `POST /api/v1/woa/config/findmany/config/cvm/subnet`
- **THEN** 响应每条命中 CRP 的子网 `available_ip_count` 等于该子网 CRP `leftIpNum`，且等于调度器对同一 region/zone/vpc 取到的值

#### Scenario: DB 有而 CRP 未返回的子网写 0

- **GIVEN** DB 清单中存在子网 A，CRP 本次结果不含 A 的 `Id`
- **WHEN** 接口合并两侧数据
- **THEN** 子网 A 仍出现在 `info` 中，其 `available_ip_count` 为 `0`；不得从清单中删除 A

#### Scenario: CRP 有而 DB 无的子网不进入清单

- **GIVEN** CRP 返回子网 B，但 B 不在 `enable_cvm = true` 的 DB 清单中
- **WHEN** 接口合并两侧数据
- **THEN** `info` 中不出现子网 B

#### Scenario: 可用 IP 为 0 时返回数字 0

- **GIVEN** CRP 对某子网返回 `leftIpNum = 0`
- **WHEN** 接口填充该子网
- **THEN** `available_ip_count` 为 `0`

### Requirement: 可用 IP 获取失败按项降级且不阻断清单

可用 IP 是增量信息。CRP 超时、报错、region/zone/vpc 不全时，woa-server MUST 仍返回子网清单，受影响项的 `available_ip_count` 为 `null`。单个子网未命中时为数字 `0`。不得因此返回业务错误或空列表（清单本身查询成功的前提下）。

失败时 MUST 记 Warn 日志，并在成功调用时记录 CRP `TraceId`，便于与调度 `crpTraceID` 对账。

#### Scenario: CRP 调用失败时清单仍返回

- **GIVEN** `GetAllSubnet` 已成功返回至少一条子网
- **WHEN** `QueryRealCvmSubnet` 超时或返回错误
- **THEN** 接口仍返回该清单，全部 `available_ip_count` 为 `null`，HTTP 成功

#### Scenario: 部分子网未命中时其余项正常

- **GIVEN** CRP 只返回清单中的部分 `cloud_id`
- **WHEN** 接口合并数据
- **THEN** 命中项为对应 `leftIpNum` 转成的 `uint64`，未命中项为 `0`，清单条数不变

#### Scenario: 参数不全时跳过 CRP

- **GIVEN** 请求缺少 region、zone 或 vpc 之一
- **WHEN** 处理可用 IP 填充
- **THEN** 不调用 CRP，已有清单的 `available_ip_count` 全部为 `null`

### Requirement: CRP 只挂在新方法上，不改 GetAllSubnet

`GetAllSubnet` 与配置管理接口 `POST /api/v1/woa/config/findmany/config/cvm/subnet/list` MUST NOT 新增 CRP 调用。`GetAllSubnetWithAvailIP` MUST 先调用 `GetAllSubnet` 取清单，再单独查询 CRP 并左连接，不得把 CRP 嵌入 `GetAllSubnet` 方法体。

下拉接口响应中的 `available_ip_count` MUST 以本次 CRP 结果为准：用 CRP 值覆盖清单上可能已有的 IP 字段；CRP 失败或未查询则为 `null`，单项未命中则为 `0`，不得把其他数据源的可用 IP 留在下拉响应里。

本能力 MUST NOT 按 `leftIpNum = 0` 或子网名 `cvm_use` 前缀过滤或剔除清单项。

#### Scenario: GetAllSubnet 方法体不发起 CRP

- **GIVEN** 调度器通过 `GetAllSubnet` 取 `enable_cvm` 清单
- **WHEN** 完成本次查询
- **THEN** 不出现由 `GetAllSubnet` 方法体发起的 `QueryRealCvmSubnet`

#### Scenario: 配置管理列表不依赖 CRP

- **GIVEN** 运营请求 `POST /api/v1/woa/config/findmany/config/cvm/subnet/list`
- **WHEN** 列表返回
- **THEN** 行为与引入本能力之前一致（分页、过滤、`enable` / `comment`），且不调用 CRP

#### Scenario: 配置列表成功时 available_ip_count 仍为数字

- **GIVEN** 配置列表走 `GetSubnetList`，且 `ListCountIP` 成功或失败降级
- **WHEN** 列表返回
- **THEN** 每条 `available_ip_count` 为数字（成功为对应数量，`ListCountIP` 失败为 `0`），不得为 `null`

#### Scenario: 剩余 IP 为 0 的子网仍返回

- **GIVEN** CRP 对某 `enable_cvm = true` 的子网返回 `leftIpNum = 0`
- **WHEN** 下拉接口合并数据
- **THEN** 该项仍在 `info` 中，`available_ip_count` 为 `0`
