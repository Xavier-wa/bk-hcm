## ADDED Requirements

### Requirement: 调账资源类别为四值枚举

调账明细的资源类别 `res_class` MUST 取自 `cpu`、`gpu_card`、`gpu_api`、`gpu_other` 四值之一。原 `gpu` 值 MUST 视为非法值。

四值语义：`cpu` 为 CPU 算力相关支出；`gpu_card` 为具体 GPU 卡的算力成本，需指定卡型；`gpu_api` 为大模型 API 调用费，需指定模型厂商；`gpu_other` 为能识别为 GPU、但识别不出具体卡型的支出。

本要求对写入接口构成破坏性变更。迁移方式：调用方原先传 `gpu` 的场景改传 `gpu_card`；存量数据由刷新流程统一处理。

#### Scenario: 创建时传入合法类别

- **GIVEN** 调账创建请求的 `res_class` 为 `gpu_card`、`gpu_api`、`gpu_other` 或 `cpu` 之一
- **WHEN** 调用创建接口
- **THEN** 类别校验通过

#### Scenario: 创建时传入已下线的 gpu 值

- **GIVEN** 调账创建请求的 `res_class` 为 `gpu`
- **WHEN** 调用创建接口
- **THEN** 返回参数非法错误，错误信息为 `unsupported bill adjustment res class: gpu`

### Requirement: 调账明细承载资源子类

调账明细 MUST 提供一个可为空的资源子类字段 `res_sub_class`，以单一字段承载卡型或模型厂商。该字段的语义 MUST 由同一条记录的 `res_class` 决定——`gpu_card` 下表示卡型，`gpu_api` 下表示模型厂商。

该字段 MUST 为全链路单字段：存储一列，创建、更新、列表、导出共用，不在读写侧做拆分或合并。

#### Scenario: 资源子类随类别表达不同语义

- **GIVEN** 库中存在一条 `res_class=gpu_card`、`res_sub_class=H200` 的记录，以及一条 `res_class=gpu_api`、`res_sub_class=gemini` 的记录
- **WHEN** 读取这两条记录
- **THEN** 前者的资源子类表示卡型 `H200`，后者的资源子类表示模型厂商 `gemini`，两者存于同一列

### Requirement: 资源子类的必填性与取值域由资源类别决定

创建与更新接口 MUST 按下列规则校验 `res_sub_class`：

- `res_class` 为 `gpu_card` 时，`res_sub_class` MUST 非空，且取值 MUST 在该记录所属云厂商的卡型清单内
- `res_class` 为 `gpu_api` 时，`res_sub_class` MUST 非空，且取值 MUST 在该记录所属云厂商的模型厂商清单内
- `res_class` 为 `cpu` 或 `gpu_other` 时，`res_sub_class` MUST 为空

违反必填、违反必须为空、取值不在对应清单内，三种情况 MUST 一律返回参数非法错误，MUST NOT 静默忽略或静默置空。

取值域 MUST 按 `res_class` 区分判定，MUST NOT 仅校验取值是否落在卡型清单与厂商清单的并集内。

取值域 MUST 按云厂商隔离。校验所需的云厂商来源：创建接口取自 URL 路径参数；更新接口的 URL 不含云厂商，MUST 先按记录 ID 读出记录再取其云厂商。支持的云厂商为 AWS、GCP、华为云。

取值比对 MUST 严格，包括大小写——取值须与清单中的写法完全一致。实现 MUST NOT 做大小写归一，MUST NOT 在落库前把取值改写为清单中的写法，落库值即请求值。

大小写约定上 `gpu_card` 的取值为大写、`gpu_api` 为小写，但该约定由清单来源保证，不由校验环节纠正。

#### Scenario: GPU 卡类别下补齐卡型

- **GIVEN** 创建请求为 AWS 厂商、`res_class=gpu_card`、`res_sub_class` 取自该厂商卡型清单中的任一值
- **WHEN** 调用创建接口
- **THEN** 创建成功，落库的资源子类与请求值一致

#### Scenario: GPU 卡类别下漏填卡型

- **GIVEN** 创建请求 `res_class=gpu_card` 且 `res_sub_class` 为空
- **WHEN** 调用创建接口
- **THEN** 返回参数非法错误，错误信息指明资源子类必填

#### Scenario: GPU API 类别下补齐模型厂商

- **GIVEN** 创建请求为 GCP 厂商、`res_class=gpu_api`、`res_sub_class=gemini`
- **WHEN** 调用创建接口
- **THEN** 创建成功，落库的资源子类为 `gemini`

#### Scenario: 非 GPU 细分类别下携带资源子类

- **GIVEN** 创建请求 `res_class=cpu` 且 `res_sub_class=H200`，或 `res_class=gpu_other` 且 `res_sub_class=gemini`
- **WHEN** 调用创建接口
- **THEN** 返回参数非法错误，且 MUST NOT 将资源子类静默置空后创建成功

#### Scenario: 类别与子类语义错配

- **GIVEN** 创建请求 `res_class=gpu_card` 且 `res_sub_class=gemini`，即把模型厂商值用在卡型类别下
- **WHEN** 调用创建接口
- **THEN** 返回参数非法错误

#### Scenario: 取值跨厂商越界

- **GIVEN** 创建请求为 AWS 厂商、`res_class=gpu_card`，`res_sub_class` 取一个仅 GCP 侧存在的卡型
- **WHEN** 调用创建接口
- **THEN** 返回参数非法错误

#### Scenario: 取值大小写与清单不一致

- **GIVEN** 某云厂商的卡型清单中存在 `H200`
- **WHEN** 以该厂商、`res_class=gpu_card`、`res_sub_class=h200` 调用创建接口
- **THEN** 返回参数非法错误，且 MUST NOT 归一为 `H200` 后创建成功

#### Scenario: 更新接口按记录所属厂商校验

- **GIVEN** 库中存在一条 AWS 厂商的 `gpu_card` 记录
- **WHEN** 调用不含云厂商路径参数的更新接口，把 `res_sub_class` 改为一个仅 GCP 侧存在的卡型
- **THEN** 返回参数非法错误

#### Scenario: 华为云无法使用 GPU 细分类别

- **GIVEN** 创建请求为华为云厂商，`res_class` 为 `gpu_card` 或 `gpu_api`，`res_sub_class` 传任意值或留空
- **WHEN** 调用创建接口
- **THEN** 一律返回参数非法错误，因为华为云的卡型清单与模型厂商清单均为空

#### Scenario: 华为云使用允许的类别

- **GIVEN** 创建请求为华为云厂商，`res_class` 为 `cpu` 或 `gpu_other`，`res_sub_class` 为空
- **WHEN** 调用创建接口
- **THEN** 创建成功

### Requirement: 类别由 GPU 细分改为非细分时须显式置空子类

更新接口把 `res_class` 从 `gpu_card` 或 `gpu_api` 改为 `cpu` 或 `gpu_other` 时，请求 MUST 同时显式把 `res_sub_class` 置空，否则 MUST 返回参数非法错误。服务端 MUST NOT 自动清空资源子类。

该约束要求实现能区分「请求未携带该字段」与「请求显式传了空值」两种情形。

#### Scenario: 改类别但未处理子类

- **GIVEN** 库中存在一条 `res_class=gpu_card`、`res_sub_class=H200` 的记录
- **WHEN** 调用更新接口只把 `res_class` 改为 `cpu`，请求中不携带 `res_sub_class`
- **THEN** 返回参数非法错误

#### Scenario: 改类别并显式置空子类

- **GIVEN** 同上记录
- **WHEN** 调用更新接口把 `res_class` 改为 `cpu` 且显式把 `res_sub_class` 传为空值
- **THEN** 更新成功，落库的资源子类为空

#### Scenario: 只改子类不改类别

- **GIVEN** 库中存在一条 `res_class=gpu_card`、`res_sub_class=H200` 的记录
- **WHEN** 调用更新接口只传 `res_sub_class` 为该厂商卡型清单内的另一个值
- **THEN** 按记录当前的 `res_class` 校验取值域，校验通过后更新成功

### Requirement: 列表查询返回资源子类

调账明细列表查询接口的响应 MUST 包含 `res_sub_class` 字段，直接返回存储值，MUST NOT 做加工或映射。

本要求不新增查询筛选条件。

#### Scenario: 四类记录的资源子类返回

- **GIVEN** 库中存在 `cpu`、`gpu_card`、`gpu_api`、`gpu_other` 四类调账记录各一条
- **WHEN** 调用列表查询接口
- **THEN** `gpu_card` 记录返回卡型值，`gpu_api` 记录返回模型厂商值，`cpu` 与 `gpu_other` 记录返回空字符串

### Requirement: 导出文件包含资源子类列

调账明细导出的 Excel MUST 在「资源类别」列之后包含「资源子类」列，取值直接来自存储值。资源子类为空时 MUST 输出空单元格，MUST NOT 输出占位符。

#### Scenario: 导出含资源子类列

- **GIVEN** 库中存在四类调账记录各一条
- **WHEN** 导出调账明细
- **THEN** Excel 的「资源类别」列后存在「资源子类」列，且各行取值与列表查询返回的资源子类一致，空值行为空单元格

### Requirement: 存量资源类别数据刷新

存量 `res_class='gpu'` 的调账明细记录 MUST 一次性刷新为 `res_class='gpu_card'`，`res_sub_class` MUST 留空且 MUST NOT 填入默认值。

刷新范围 MUST 限于调账明细表，MUST NOT 追溯修正已同步到 OBS 的历史账单数据。

刷新 MUST 可重复执行（幂等）。刷新 MUST 在新版本服务上线前执行完成。

刷新产出的记录为 `gpu_card` 且资源子类为空，不满足必填校验。这是已接受的既有数据状态：校验只在写入路径生效，不影响这些记录的查询、导出与 OBS 同步；这些记录被编辑时须由运营补齐卡型，华为云的此类记录须先把类别改为 `gpu_other` 才能保存。校验 MUST NOT 为此放宽。

#### Scenario: 首次执行刷新

- **GIVEN** 库中存在 N 条 `res_class='gpu'` 的调账明细记录
- **WHEN** 执行存量刷新
- **THEN** N 条记录全部变为 `res_class='gpu_card'` 且 `res_sub_class` 为空

#### Scenario: 重复执行刷新

- **GIVEN** 刷新已执行过一次，库中不再存在 `res_class='gpu'` 的记录
- **WHEN** 再次执行刷新
- **THEN** 影响 0 行且不报错

#### Scenario: 刷新后的记录可正常读取

- **GIVEN** 一条刷新产出的 `gpu_card` 且资源子类为空的记录
- **WHEN** 查询、导出或同步 OBS
- **THEN** 均正常处理，不因资源子类为空而报错
