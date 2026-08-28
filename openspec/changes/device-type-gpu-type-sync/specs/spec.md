# device-type-gpu-type

## ADDED Requirements

### Requirement: 机型主数据持久化 GPU 卡类型

系统 SHALL 在 `device_type` 表新增 `gpu_type` 字段，类型为字符串，最大长度 64，默认值为空字符串。非 GPU 机型或来源未给出卡类型时，`gpu_type` SHALL 允许为空字符串，且不得阻断现有查询。

#### Scenario: 迁移后历史机型可查询

- **WHEN** 执行数据库迁移脚本后查询已有机型
- **THEN** 历史记录的 `gpu_type` 为空字符串，查询不报错

#### Scenario: 超长卡类型拒绝写入

- **WHEN** 创建或更新机型时 `gpu_type` 长度大于 64
- **THEN** 系统 SHALL 拒绝该条写入并返回参数错误

### Requirement: 自研云同步写入 GPU 卡类型

自研云机型同步 SHALL 将 CRP `QueryCvmInstanceType` 返回的 `gpuType` 原样写入对应机型的 `gpu_type`。`gpuType` 为空时 SHALL 写入空字符串，不得因此跳过该机型。仅覆盖 vendor=`tcloud_ziyan` 的机型同步。

#### Scenario: GPU 机型同步写入卡类型

- **WHEN** CRP 对某 GPU 机型返回 `gpuType="NVIDIA A100"` 且完成该地域自研云机型同步
- **THEN** 对应机型主数据 `gpu_type` 等于 `NVIDIA A100`，且查询接口返回该值

#### Scenario: 空卡类型仍同步成功

- **WHEN** CRP 对某非 GPU 机型返回 `gpuType` 为空
- **THEN** 该机型仍被同步成功，`gpu_type` 为空字符串，同步任务不因空卡类型失败

#### Scenario: 详情查询失败仍终止同步

- **WHEN** `QueryCvmInstanceType` 调用失败
- **THEN** 该次同步失败并返回错误，不得部分静默丢弃 `gpu_type`

### Requirement: 仅卡类型变化也必须更新

同步差异检测 SHALL 将 `gpu_type` 纳入比较。已存在机型的 `gpu_type` 与云侧 `gpuType` 不一致时，系统 MUST 更新该字段。

#### Scenario: 空卡类型被 CRP 新值覆盖

- **WHEN** 库中某机型 `gpu_type` 为空，而 CRP 当次返回 `gpuType="V100"`
- **THEN** 该机型被更新，`gpu_type` 变为 `V100`

#### Scenario: 卡类型未变化不单独触发更新

- **WHEN** 云侧 `gpuType` 与库中 `gpu_type` 相同，且其他已纳入差异检测的字段也未变化
- **THEN** `isDeviceTypeChanged` 返回 false，该机型不被更新

### Requirement: 主动登记允许提交卡类型与技术分类资源量

主动登记创建接口 SHALL 允许可选字段 `gpu_type`（字符串，≤64）与 `tech_class_res_amt`（数值 ≥0）。未传 `gpu_type` 时按空字符串落库；未传 `tech_class_res_amt` 时按 0 落库。后端 MUST NOT 将 `gpu_type` 作为强制枚举拦截。两字段均为可选，不得改变现有必填项。创建接口权限仍为「平台-CVM机型」。

#### Scenario: 创建页提交两字段

- **WHEN** 机型配置管理员在创建新机型页填写 `gpu_type="NVIDIA A100"` 且 `tech_class_res_amt=1` 并提交
- **THEN** 新机型记录中两字段分别为 `NVIDIA A100` 与 1

#### Scenario: 未传字段使用默认值

- **WHEN** 创建请求不传 `gpu_type` 且不传 `tech_class_res_amt`，其余必填合法
- **THEN** 创建成功，`gpu_type` 为空字符串，`tech_class_res_amt` 为 0

#### Scenario: 非法入参拒绝创建

- **WHEN** 创建请求 `gpu_type` 长度大于 64 或 `tech_class_res_amt` 小于 0
- **THEN** 接口返回参数错误，该条机型未创建

#### Scenario: 选项外手输创建成功

- **WHEN** 创建页 GPU 卡类型下拉中没有目标卡型，管理员手输该卡型并提交（其余必填合法，字符串 ≤64）
- **THEN** 创建成功，`gpu_type` 等于手输原文，HCM 不做卡型号翻译或归一

#### Scenario: 无权限创建仍被拒绝

- **WHEN** 无「平台-CVM机型」权限的调用方提交创建机型
- **THEN** 请求被拒绝，与改造前一致

### Requirement: 查询返回 GPU 卡类型

对外机型查询（含列表、去重列表、机型规格查询）SHALL 返回 `gpu_type`。无卡类型时返回空字符串，不得因此报错。

#### Scenario: 查询接口返回卡类型

- **WHEN** 调用机型列表或去重列表查询且记录已有 `gpu_type`
- **THEN** 响应包含 `gpu_type` 字段，值为库中原文

#### Scenario: 无卡类型返回空字符串

- **WHEN** 查询 `gpu_type` 为空的机型
- **THEN** 响应中 `gpu_type` 为空字符串，接口不报错

### Requirement: 配置页可按卡类型筛选且列表不加列

机型配置页 SHALL 提供「GPU 卡类型」筛选条件，按已落库 `gpu_type` 过滤。机型配置列表表格 MUST NOT 增加 `gpu_type` 列。本期 MUST NOT 改造 CVM 申领筛选或推荐。

#### Scenario: 按卡类型筛选命中

- **WHEN** 库中已有 `gpu_type="NVIDIA A100"` 的机型，运营在机型配置页按 GPU 卡类型筛选该值
- **THEN** 结果仅包含匹配机型，且列表表格中不出现「GPU 卡类型」列

#### Scenario: 未填写筛选条件不过滤卡类型

- **WHEN** 运营未填写 GPU 卡类型筛选条件并查询
- **THEN** 结果不按 `gpu_type` 收窄，行为与改造前其他筛选条件组合一致

### Requirement: 已有规格详情可展示卡类型

已有机型规格详情展示（如申领选机型详情）SHALL 在有值时展示 `gpu_type`。MUST NOT 新增独立机型详情页。

#### Scenario: 规格详情展示卡类型

- **WHEN** 规格详情中的机型 `gpu_type` 非空
- **THEN** 详情文案中展示该卡类型

#### Scenario: 空卡类型不强制展示占位型号

- **WHEN** 规格详情中的机型 `gpu_type` 为空
- **THEN** 不展示虚假卡型号，现有卡数展示逻辑保持可用

# ziyan-device-type-sync

## MODIFIED Requirements

### Requirement: 机型详情查询保留

机型的详细规格信息 SHALL 继续由 CRP `QueryCvmInstanceType` 接口提供。该调用逻辑保持不变；字段映射 SHALL 在既有 CPU/内存/GPU 卡数/核心类型/机型代际等字段基础上增加 `gpuType` → `gpu_type`。`gpuType` 为空不得作为跳过该条机型的理由。因技术分类资源量计算失败等既有规则跳过的机型，仍按现有同步规则跳过并打日志。

#### Scenario: 保留 QueryCvmInstanceType 查询详情

- **WHEN** 收集到机型规格名称列表后需要查询机型详细信息
- **THEN** 系统 SHALL 调用 `QueryCvmInstanceType`（入参为机型规格名称列表）获取详情，并按既有字段映射填充 `device_type` 的 CPU/内存/GPU 卡数/GPU 卡类型/核心类型/机型代际等字段

#### Scenario: 详情查询失败终止同步

- **WHEN** `QueryCvmInstanceType` 调用出现 HTTP 错误或返回业务错误码（`error.code` 非 0）
- **THEN** 系统 SHALL 记录 `logs.Errorf` 告警并立即返回错误、终止本次同步

#### Scenario: 空 gpuType 不跳过机型

- **WHEN** `QueryCvmInstanceType` 某条记录 `gpuType` 为空且该条未触发既有跳过规则
- **THEN** 系统 SHALL 继续组装并同步该机型，`gpu_type` 为空字符串
