# 任务清单：GPU 机型同步增加 gpu_amount 字段

## 1. 数据库模型层

- [x] 1.1 修改 `pkg/dal/table/cloud/device-type/device_type.go`，在 `DeviceTypeTable` 结构体中添加 `GpuAmount` 字段（类型 `float64`，db tag: `gpu_amount`）
- [x] 1.2 修改 `pkg/api/data-service/cloud/device_type.go`，在 `DeviceTypeCreate` 结构体中添加 `GpuAmount` 字段（类型 `float64`）
- [x] 1.3 修改 `pkg/api/data-service/cloud/device_type.go`，在 `DeviceTypeUpdate` 结构体中添加 `GpuAmount` 字段（类型 `*float64`）
- [x] 1.4 修改 `pkg/dal/table/cloud/device-type/device_type.go`，在 `DeviceTypeTable` 常量中添加 `GpuAmount` 字段定义

## 2. API 协议层

- [x] 2.1 修改 `pkg/api/core/cloud/device-type/tcloud_ziyan.go`，在 `DeviceType` 结构体中添加 `GpuAmount` 字段（类型 `float64`，json tag: `gpu_amount`）
- [x] 2.2 修改 `pkg/api/core/cloud/device-type/tcloud_ziyan.go`，在 `DistinctDeviceType` 结构体中添加 `GpuAmount` 字段（类型 `float64`，json tag: `gpu_amount`）
- [x] 2.3 修改 `pkg/api/data-service/cloud/device_type.go`，在 `DeviceTypeCreate` 结构体中添加 `GpuAmount` 字段（类型 `float64`，json tag: `gpu_amount`）
- [x] 2.4 修改 `pkg/api/data-service/cloud/device_type.go`，在 `DeviceTypeUpdate` 结构体中添加 `GpuAmount` 字段（类型 `*float64`，json tag: `gpu_amount,omitempty`）

## 3. 数据服务层

- [x] 3.1 修改 `cmd/data-service/service/cloud/device-type/query.go`，在机型查询结果转换函数中添加 `GpuAmount` 字段映射
- [x] 3.2 修改 `cmd/data-service/service/cloud/device-type/create.go`，在机型创建逻辑中添加 `GpuAmount` 字段处理
- [x] 3.3 修改 `cmd/data-service/service/cloud/device-type/update.go`，在机型更新逻辑中添加 `GpuAmount` 字段处理

## 4. 同步服务层 - 数据构建

- [x] 4.1 修改 `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go`，在构建 `coredevicetype.DeviceType` 时从 CRP 响应中提取 `gpuAmount` 字段
- [x] 4.2 添加 `gpuAmount` 字段类型转换逻辑：CRP 返回的 `float64` 直接赋值，无需截断

## 5. 同步服务层 - 差异检测与操作

- [x] 5.1 修改 `cmd/hc-service/logics/res-sync/ziyan/device_type.go`，在 `isDeviceTypeChanged` 函数中添加 `gpu_amount` 字段比较逻辑
- [x] 5.2 修改 `cmd/hc-service/logics/res-sync/ziyan/device_type.go`，在 `createDeviceType` 函数中包含 `gpu_amount` 字段
- [x] 5.3 修改 `cmd/hc-service/logics/res-sync/ziyan/device_type.go`，在 `updateDeviceType` 函数中包含 `gpu_amount` 字段

## 6. WOA 服务层

- [x] 6.1 修改 `cmd/woa-server/service/config/device.go`，在机型查询接口返回结果中添加 `gpu_amount` 字段
- [x] 6.2 确保 `/config/createmany/config/cvm/device` 接口返回包含 `gpu_amount` 字段

## 7. 接口文档更新

- [x] 7.1 更新 `docs/api-docs/web-server/docs/scr/meta/list_device_type.md`，在响应参数中添加 `gpu_amount` 字段说明（类型：float，描述：GPU卡数）
- [x] 7.2 更新 `docs/api-docs/web-server/docs/scr/config-manage/create_config_cvm_device.md`，在响应参数中添加 `gpu_amount` 字段说明（类型：float64，描述：GPU卡数）

## 8. 数据库迁移

- [x] 8.1 创建 SQL 迁移脚本，在 `device_type` 表添加 `gpu_amount` 字段：
  ```sql
  ALTER TABLE device_type ADD COLUMN gpu_amount DOUBLE NOT NULL DEFAULT 0 COMMENT 'GPU卡数' AFTER memory;
  ```

## 9. 测试验证

- [x] 9.1 验证机型同步流程能正确从 CRP 获取 `gpuAmount` 并保存到数据库
- [x] 9.2 验证 `ListDeviceType` 接口返回包含 `gpu_amount` 字段
- [x] 9.3 验证 `ListDistinctDeviceType` 接口返回包含 `gpu_amount` 字段
- [x] 9.4 验证差异检测逻辑能正确识别 `gpu_amount` 字段变更
- [x] 9.5 验证 GPU 机型（gpu_amount > 0）和非 GPU 机型（gpu_amount = 0）数据正确性

---

## 文件修改清单

| 序号 | 文件路径 | 修改类型 | 依赖 |
|------|----------|----------|------|
| 1 | `pkg/dal/table/cloud/device-type/device_type.go` | 修改 | 无 |
| 2 | `pkg/api/core/cloud/device-type/tcloud_ziyan.go` | 修改 | 1 |
| 3 | `pkg/api/data-service/cloud/device_type.go` | 修改 | 1 |
| 4 | `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go` | 修改 | 2 |
| 5 | `cmd/hc-service/logics/res-sync/ziyan/device_type.go` | 修改 | 2, 4 |
| 6 | `cmd/data-service/service/cloud/device-type/query.go` | 修改 | 3 |
| 7 | `cmd/data-service/service/cloud/device-type/create.go` | 修改 | 3 |
| 8 | `cmd/data-service/service/cloud/device-type/update.go` | 修改 | 3 |
| 9 | `cmd/woa-server/service/config/device.go` | 修改 | 2 |
| 10 | `docs/api-docs/web-server/docs/scr/meta/list_device_type.md` | 修改 | 6 |
| 11 | `docs/api-docs/web-server/docs/scr/config-manage/create_config_cvm_device.md` | 修改 | 9 |

---

## 执行顺序建议

1. **第一阶段 - 基础模型**（任务 1-3）：先完成数据库模型和 API 协议定义
2. **第二阶段 - 同步逻辑**（任务 4-5）：实现从 CRP 同步 GPU 卡数逻辑
3. **第三阶段 - 数据服务**（任务 3, 6）：实现查询和写入逻辑
4. **第四阶段 - 接口与文档**（任务 6-7）：完成接口返回和文档更新
5. **第五阶段 - 验证**（任务 8-9）：数据库迁移和功能验证
