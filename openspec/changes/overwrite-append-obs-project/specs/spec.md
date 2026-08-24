# overwrite-append-obs-project

## ADDED Requirements

### Requirement: 项目类型校验只认形态

系统 MUST 用 `ValidateResPlan()` 校验资源预测相关项目类型。已知形态 MUST 仅为：`常规项目`、`滚服项目`、`改造复用`、`轻量云徙`、`短租项目`、`YYYY春节保障`、`YYYY机房裁撤`，其中 `YYYY` MUST 为四位数字。空值、中间空格、错写或未知文案 MUST 视为非法形态。校验 MUST NOT 按当前时间窗口拒绝已知形态。

#### Scenario: 固定类型通过

- **WHEN** 校验值为 `常规项目`
- **THEN** `ValidateResPlan()` 通过

#### Scenario: 窗口外春保通过

- **WHEN** 当前时间为 2026-08 且校验值为 `2029春节保障`
- **THEN** `ValidateResPlan()` 通过

#### Scenario: 非法形态被拒绝

- **WHEN** 校验值为 `2029春保` 或 `foobar` 或空字符串
- **THEN** 校验失败，错误语义与现网「不支持的项目类型」一致

### Requirement: 选择列表仍按当前时间窗口

给用户选择项目类型的接口 MUST 继续返回 `GetObsProjectMembersForResPlan()`，MUST NOT 把窗口外年份加入下拉枚举。

#### Scenario: 下拉不含远期春保

- **WHEN** 当前时间为 2026-08，调用项目类型列表接口
- **THEN** 返回集合含窗口内春保（如 `2027春节保障` / `2028春节保障`），不含 `2029春节保障`

### Requirement: 覆盖追加接受窗口外已知形态

资源预测与退回计划覆盖追加 MUST 按 `ValidateResPlan()` 校验项目类型，值 MUST 原样落库，MUST NOT 静默改写成窗口内邻近年份。

#### Scenario: 窗口外春保覆盖追加成功

- **WHEN** 调用资源预测覆盖追加且明细 `obs_project` 为 `2029春节保障`，其余参数合法
- **THEN** 接口接受，落库项目类型为 `2029春节保障`

#### Scenario: 非法形态整单不落库

- **WHEN** 覆盖追加明细 `obs_project` 为 `2029春保` 或 `foobar` 或空字符串
- **THEN** 接口返回参数错误，不新增本单主单或明细

### Requirement: 已准入数据不再因窗口失败

覆盖追加已接受的项目类型在 persist、拆单、dispatch 出站时 MUST 保持原值，MUST NOT 因「不在当前时间窗口」失败。系统 MUST NOT 沿链路堆互不相干的 skip 开关。

#### Scenario: persist 与拆单保持原值

- **WHEN** 覆盖追加已接受 `2029春节保障` 并进入 persist / 拆单
- **THEN** 主单、demand、子单项目类型仍为 `2029春节保障`，且不因项目类型窗口失败

### Requirement: CRP 回包项目类型信任

将 CRP 订单变更回包转换为本地结构时，系统 MUST NOT 对 `ProjectName` 做项目类型校验，MUST 按回包原值写入。其它回包字段的既有校验 MUST 保持。

#### Scenario: 回包未来春保不因校验失败

- **WHEN** CRP 回包 `ProjectName` 为 `2029春节保障`
- **THEN** 转换成功并按该值继续流转

#### Scenario: 非项目类型失败保持现网

- **WHEN** dispatch / split 因机型不存在或 CRP 业务错误失败
- **THEN** 按现网方式失败，不改写为成功

### Requirement: 覆盖追加权限不因项目类型放宽

覆盖追加的鉴权 MUST 与现网一致。项目类型属于已知形态 MUST NOT 使无权限调用方通过鉴权。

#### Scenario: 无权限调用仍被拒绝

- **WHEN** 调用方不具备现网覆盖追加权限，即使传入 `2029春节保障`
- **THEN** 仍按现网无权限处理
