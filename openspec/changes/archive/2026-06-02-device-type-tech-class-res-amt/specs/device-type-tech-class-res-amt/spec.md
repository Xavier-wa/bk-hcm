## ADDED Requirements

### Requirement: 数据库迁移脚本

系统 SHALL 提供数据库迁移脚本，为 `device_type` 表新增 `tech_class_res_amt` 字段。字段类型 SHALL 为 `DECIMAL(10,2)`，默认值 SHALL 为 0，注释 SHALL 为「技术分类资源量」。字段位置 SHALL 在 `technical_class` 字段之后。迁移脚本 SHALL 同步更新 `hcm_version` 视图。仅涉及 MySQL 数据库。

#### Scenario: 迁移脚本正确执行

- **WHEN** 执行迁移脚本 `scripts/sql/0072_20260602_device_type_tech_res_amt.sql`
- **THEN** `device_type` 表成功新增 `tech_class_res_amt` 字段，类型为 DECIMAL(10,2)，默认值为 0

#### Scenario: 历史数据处理

- **WHEN** 查询历史机型数据
- **THEN** `tech_class_res_amt` 字段值为 0（默认值）

### Requirement: 数据层结构体新增字段

数据层 SHALL 在 `DeviceTypeTable` 结构体新增 `TechClassResAmt` 字段，类型 SHALL 为 `float64`，`db` tag SHALL 为 `tech_class_res_amt`。SHALL 同步更新 `DeviceTypeColumns` 常量和 `DeviceTypeColumnDescriptor` 数组。仅涉及 `pkg/dal/table/cloud/device-type/device_type.go` 文件。

#### Scenario: 结构体字段正确定义

- **WHEN** 检查 `DeviceTypeTable` 结构体定义
- **THEN** 存在 `TechClassResAmt float64 \`db:"tech_class_res_amt"\`` 字段

#### Scenario: 列描述符正确更新

- **WHEN** 检查 `DeviceTypeColumnDescriptor` 数组
- **THEN** 包含 `tech_class_res_amt` 字段的列描述符

### Requirement: 接口层结构体新增字段

接口层 SHALL 在 `DeviceTypeCreate`、`DeviceTypeUpdate`、`DeviceTypeResult` 三个结构体新增 `TechClassResAmt` 字段，类型 SHALL 为 `float64`。仅涉及 `pkg/api/data-service/cloud/device_type.go` 文件。

#### Scenario: 创建接口支持新字段

- **WHEN** 调用机型创建接口，请求体包含 `tech_class_res_amt` 字段
- **THEN** 接口正确接收并写入数据库

#### Scenario: 更新接口支持新字段

- **WHEN** 调用机型更新接口，请求体包含 `tech_class_res_amt` 字段
- **THEN** 接口正确接收并更新数据库

#### Scenario: 查询接口返回新字段

- **WHEN** 调用机型查询接口
- **THEN** 返回数据包含 `tech_class_res_amt` 字段

### Requirement: Core 层结构体新增字段

Core 层 SHALL 在 `DeviceType` 结构体新增 `TechClassResAmt` 字段，类型 SHALL 为 `float64`。仅涉及 `pkg/api/core/cloud/device-type/tcloud_ziyan.go` 文件。

#### Scenario: Core 层结构体正确定义

- **WHEN** 检查 Core 层 `DeviceType` 结构体定义
- **THEN** 存在 `TechClassResAmt float64` 字段

### Requirement: 第三方接口结构体新增字段

第三方接口层 SHALL 在 `QueryCvmInstanceTypeItem` 结构体新增 `DiskBlockNum` 和 `DiskBlockSize` 字段，类型 SHALL 为 `int`，`json` tag SHALL 分别为 `DiskBlockNum` 和 `DiskBlockSize`。仅涉及 `pkg/thirdparty/cvmapi/cvmapi_response.go` 文件。

#### Scenario: 磁盘数量字段正确定义

- **WHEN** 检查 `QueryCvmInstanceTypeItem` 结构体定义
- **THEN** 存在 `DiskBlockNum int \`json:"DiskBlockNum"\`` 字段

#### Scenario: 单盘容量字段正确定义

- **WHEN** 检查 `QueryCvmInstanceTypeItem` 结构体定义
- **THEN** 存在 `DiskBlockSize int \`json:"DiskBlockSize"\`` 字段（单位 GB）

### Requirement: 资源量计算函数

同步层 SHALL 新增 `calcTechClassResAmt` 函数，根据机型技术分类自动计算资源量。计算规则 SHALL 为：内存型取 `RamAmount`（单位 GB），高IO型/大数据型取 `DiskBlockNum * DiskBlockSize / 1024`（单位 TB），其他所有场景（含未知分类、空值、磁盘异常）SHALL fallback 到 `CPUAmount`。仅涉及 `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go` 文件。

#### Scenario: 内存型机型资源量计算

- **WHEN** 机型技术分类为「内存型」
- **THEN** 资源量 = `RamAmount`（单位 GB）

#### Scenario: 高IO型机型资源量计算

- **WHEN** 机型技术分类为「高IO型」
- **THEN** 资源量 = `DiskBlockNum * DiskBlockSize / 1024`（单位 TB）

#### Scenario: 大数据型机型资源量计算

- **WHEN** 机型技术分类为「大数据型」
- **THEN** 资源量 = `DiskBlockNum * DiskBlockSize / 1024`（单位 TB）

#### Scenario: 其他类型机型资源量计算

- **WHEN** 机型技术分类为其他类型（含计算型、通用型等）
- **THEN** 资源量 = `CPUAmount`

#### Scenario: 未知分类机型资源量计算

- **WHEN** 机型技术分类为空或未知
- **THEN** 资源量 = `CPUAmount`（fallback）

#### Scenario: 磁盘信息异常时 fallback

- **WHEN** 机型技术分类为高IO型/大数据型，但磁盘信息缺失或异常
- **THEN** 打印 Warn 级别日志，资源量 = `CPUAmount`（fallback）

### Requirement: 同步数据赋值

同步层 SHALL 在 `listDeviceTypeFromCloud` 函数中，为每条机型数据调用 `calcTechClassResAmt` 函数并赋值 `TechClassResAmt` 字段。仅涉及 `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go` 文件。

#### Scenario: 同步数据正确赋值

- **WHEN** 从 CRP 接口拉取机型数据
- **THEN** 每条机型数据的 `TechClassResAmt` 字段已正确赋值

### Requirement: 变更检测逻辑

同步层 SHALL 在 `isDeviceTypeChanged` 函数中新增 `tech_class_res_amt` 字段变更检测。浮点数比较 SHALL 使用 `math.Abs(a-b) > 0.01` 判断。仅涉及 `cmd/hc-service/logics/res-sync/ziyan/device_type.go` 文件。

#### Scenario: 检测到字段变化

- **WHEN** 数据库中的 `tech_class_res_amt` 与同步数据中的 `tech_class_res_amt` 差值 > 0.01
- **THEN** `isDeviceTypeChanged` 函数返回 `true`

#### Scenario: 未检测到字段变化

- **WHEN** 数据库中的 `tech_class_res_amt` 与同步数据中的 `tech_class_res_amt` 差值 <= 0.01
- **THEN** `isDeviceTypeChanged` 函数不考虑该字段变化（返回结果由其他字段决定）

### Requirement: 创建逻辑

同步层 SHALL 在 `createDeviceType` 函数中，将 `tech_class_res_amt` 字段跟随创建请求落库。仅涉及 `cmd/hc-service/logics/res-sync/ziyan/device_type.go` 文件。

#### Scenario: 创建机型数据包含新字段

- **WHEN** 调用 `createDeviceType` 函数创建机型
- **THEN** 创建请求包含 `tech_class_res_amt` 字段，并成功写入数据库

### Requirement: 更新逻辑

同步层 SHALL 在 `updateDeviceType` 函数中，将 `tech_class_res_amt` 字段跟随更新请求落库。仅涉及 `cmd/hc-service/logics/res-sync/ziyan/device_type.go` 文件。

#### Scenario: 更新机型数据包含新字段

- **WHEN** 调用 `updateDeviceType` 函数更新机型
- **THEN** 更新请求包含 `tech_class_res_amt` 字段，并成功更新数据库

## MODIFIED Requirements

### Requirement: 自研云机型同步流程

自研云机型同步流程 SHALL 在现有逻辑基础上，新增技术分类资源量计算与赋值。同步流程 SHALL 保持现有触发机制和调度逻辑不变。仅涉及 `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go` 和 `cmd/hc-service/logics/res-sync/ziyan/device_type.go` 文件。

#### Scenario: 同步流程正常执行

- **WHEN** 触发自研云机型同步任务
- **THEN** 同步流程正常执行，每条机型数据正确计算并赋值 `tech_class_res_amt` 字段

#### Scenario: 同步流程异常处理

- **WHEN** 计算资源量时发生异常（如磁盘信息缺失）
- **THEN** 打印 Warn 级别日志，fallback 到 CPU 核数，同步流程继续

## REMOVED Requirements

（无）
