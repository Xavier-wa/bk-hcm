## 1. 数据库迁移脚本

- [x] 1.1 创建迁移脚本 `scripts/sql/9999_20260602_device_type_tech_class_res_amt.sql`
- [x] 1.2 为 `device_type` 表新增 `tech_class_res_amt` 字段（DECIMAL(10,2)，默认 0）
- [x] 1.3 字段注释为「技术分类资源量」
- [x] 1.4 字段位置在 `technical_class` 字段之后
- [x] 1.5 同步更新 `hcm_version` 视图

## 2. 数据层改动

- [x] 2.1 修改 `pkg/dal/table/cloud/device-type/device_type.go`
- [x] 2.2 在 `DeviceTypeTable` 结构体新增 `TechClassResAmt float64 \`db:"tech_class_res_amt"\`` 字段
- [x] 2.3 更新 `DeviceTypeColumns` 常量，新增字段列名（自动生成）
- [x] 2.4 更新 `DeviceTypeColumnDescriptor` 数组，新增字段描述符

## 3. 接口层改动（data-service）

- [x] 3.1 修改 `pkg/api/data-service/cloud/device_type.go`
- [x] 3.2 在 `DeviceTypeCreate` 结构体新增 `TechClassResAmt` 字段
- [x] 3.3 在 `DeviceTypeUpdate` 结构体新增 `TechClassResAmt` 字段
- [x] 3.4 在 `DeviceTypeResult` 结构体新增 `TechClassResAmt` 字段（已在 Core 层 DeviceType 中完成）

## 4. Core 层改动

- [x] 4.1 修改 `pkg/api/core/cloud/device-type/tcloud_ziyan.go`
- [x] 4.2 在 Core 层 `DeviceType` 结构体新增 `TechClassResAmt` 字段

## 5. 第三方接口结构体改动

- [x] 5.1 修改 `pkg/thirdparty/cvmapi/cvmapi_response.go`
- [x] 5.2 在 `QueryCvmInstanceTypeItem` 结构体新增 `DiskBlockNum int \`json:"DiskBlockNum"\`` 字段
- [x] 5.3 在 `QueryCvmInstanceTypeItem` 结构体新增 `DiskBlockSize int \`json:"DiskBlockSize"\`` 字段（单位 GB）

## 6. 同步层改动（hc-service）

### 6.1 计算函数

- [x] 6.1.1 在 `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go` 新增 `calcTechClassResAmt` 函数
- [x] 6.1.2 实现内存型资源量计算：取 `RamAmount`（单位 GB）
- [x] 6.1.3 实现高IO型/大数据型资源量计算：取 `DiskBlockNum * DiskBlockSize / 1024`（单位 TB）
- [x] 6.1.4 实现其他类型 fallback 逻辑：取 `CPUAmount`
- [x] 6.1.5 磁盘信息异常时打印 Warn 日志并 fallback 到 CPU 核数

### 6.2 同步数据赋值

- [x] 6.2.1 修改 `listDeviceTypeFromCloud` 函数
- [x] 6.2.2 为每条机型数据赋值 `TechClassResAmt` 字段

### 6.3 变更检测

- [x] 6.3.1 修改 `cmd/hc-service/logics/res-sync/ziyan/device_type.go` 的 `isDeviceTypeChanged` 函数
- [x] 6.3.2 新增 `tech_class_res_amt` 字段变更检测
- [x] 6.3.3 使用 `math.Abs(a-b) > 0.01` 进行浮点数比较

### 6.4 创建逻辑

- [x] 6.4.1 修改 `createDeviceType` 函数
- [x] 6.4.2 新增字段跟随创建请求落库

### 6.5 更新逻辑

- [x] 6.5.1 修改 `updateDeviceType` 函数
- [x] 6.5.2 新增字段跟随更新请求落库

## 7. 单元测试

### 7.1 `calcTechClassResAmt` 函数单元测试

- [x] 7.1.1 测试内存型机型资源量计算：验证返回值 = `RamAmount`
- [x] 7.1.2 测试高IO型机型资源量计算：验证返回值 = `DiskBlockNum * DiskBlockSize / 1024`
- [x] 7.1.3 测试大数据型机型资源量计算：验证返回值 = `DiskBlockNum * DiskBlockSize / 1024`
- [x] 7.1.4 测试其他类型机型资源量计算：验证返回值 = `CPUAmount`（fallback）
- [x] 7.1.5 测试未知分类机型资源量计算：验证返回值 = `CPUAmount`（fallback）
- [x] 7.1.6 测试磁盘信息异常时 fallback：验证打印 Warn 日志且返回值 = `CPUAmount`
- [x] 7.1.7 测试磁盘信息缺失时 fallback：验证返回值 = `CPUAmount`

### 7.2 `isDeviceTypeChanged` 函数单元测试

- [x] 7.2.1 测试 `tech_class_res_amt` 字段变化检测：验证返回 `true`
- [x] 7.2.2 测试 `tech_class_res_amt` 字段未变化：验证返回 `false`
- [x] 7.2.3 测试浮点数精度比较：验证误差 <= 0.01 时返回 `false`

