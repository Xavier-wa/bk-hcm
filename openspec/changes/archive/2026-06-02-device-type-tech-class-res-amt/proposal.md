## Why

在机型转移场景中进行资源对冲预测时，当前缺乏统一的技术分类资源量度量标准，导致预测准确性不足。不同技术分类的机型（内存型、高IO型、大数据型等）需要采用不同的资源当量计算方式，但现有数据模型未记录这一关键指标。

现在需要在 `device_type` 表中新增 `tech_class_res_amt` 字段，在自研云机型同步时自动按技术分类核算资源量并落库，为后续资源对冲预测提供准确的数据基础。

## What Changes

- 为 `device_type` 表新增 `tech_class_res_amt` 字段（DECIMAL(10,2)，默认 0），用于存储技术分类资源量
- 数据层（dal/table）、接口层（api/data-service、api/core）、同步层（hc-service）全链路新增字段适配
- 补齐 CRP 机型接口返回结构体 `DiskBlockNum`、`DiskBlockSize` 字段，获取磁盘规格用于资源量核算
- 新增资源量计算函数 `calcTechClassResAmt`，根据技术分类自动选择计算规则
- 修改机型变更比对、新建/更新入库逻辑，确保新字段跟随同步流程正确落库

## Capabilities

### Modified Capabilities

- `device-type-sync`: 自研云机型同步能力增强，新增技术分类资源量自动计算与入库
- `device-type-query`: 机型查询接口返回新增字段，支持按技术分类资源量筛选和展示

## Impact

- `pkg/dal/table/cloud/device-type/device_type.go`: 表结构体新增 `TechClassResAmt` 字段
- `pkg/api/data-service/cloud/device_type.go`: 接口层创建/更新/查询结构体新增字段
- `pkg/api/core/cloud/device-type/tcloud_ziyan.go`: Core 层结构体新增字段
- `pkg/thirdparty/cvmapi/cvmapi_response.go`: CRP 接口返回结构体新增磁盘信息字段
- `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go`: 同步服务新增计算函数和字段赋值逻辑
- `cmd/hc-service/logics/res-sync/ziyan/device_type.go`: 变更检测、创建、更新逻辑适配新字段
- `scripts/sql/`: 新增数据库迁移脚本
- 查询接口返回数据新增 `tech_class_res_amt` 字段，前端可选适配展示
