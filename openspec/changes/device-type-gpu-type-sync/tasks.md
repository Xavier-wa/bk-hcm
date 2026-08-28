## 1. 数据库迁移

- [x] 1.1 新增 `scripts/sql/9999_*_device_type_gpu_type.sql`（`SQLVER=9999`、`HCMVER=v9.9.9.9`；真实编号出包前再定）
- [x] 1.2 为 `device_type` 表增加 `gpu_type VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'GPU卡类型' AFTER gpu_amount`
- [x] 1.3 同步更新 `hcm_version` 视图为占位：`hcm_ver='v9.9.9.9'`，`sql_ver='9999'`

## 2. 数据模型与协议

- [x] 2.1 在 `pkg/dal/table/cloud/device-type/device_type.go` 的 `DeviceTypeColumnDescriptor` 增加 `gpu_type`（String）
- [x] 2.2 在 `DeviceTypeTable` 增加 `GpuType string`，`db:"gpu_type"` / `json:"gpu_type"`，`validate:"lte=64"`
- [x] 2.3 在 `InsertValidate` / `UpdateValidate` 中校验 `gpu_type` 长度不超过 64
- [x] 2.4 在 `pkg/api/core/cloud/device-type/tcloud_ziyan.go` 的 `DeviceType`、`DistinctDeviceType` 增加 `GpuType`
- [x] 2.5 更新 `ConvTableToDeviceType`、`ConvTableToDistinctDeviceType` 映射 `GpuType`
- [x] 2.6 在 `pkg/api/data-service/cloud/device_type.go` 的 `DeviceTypeCreate` 增加 `GpuType string`（`json:"gpu_type" validate:"lte=64"`）
- [x] 2.7 在 `DeviceTypeUpdate` 增加 `GpuType *string`（`json:"gpu_type,omitempty" validate:"omitempty,lte=64"`）

## 3. data-service 读写

- [x] 3.1 在 `cmd/data-service/service/cloud/device-type/create.go` 创建记录时写入 `GpuType`
- [x] 3.2 在 `cmd/data-service/service/cloud/device-type/update.go` 当 `updateReq.GpuType != nil` 时更新该字段
- [x] 3.3 确认 `query.go` 经 `ConvTableTo*` 返回 `gpu_type`，无需额外组装逻辑

## 4. 自研云同步

- [x] 4.1 在 `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go` 构建 `DeviceType` 时赋值 `GpuType: item.GPUType`
- [x] 4.2 确保 `gpuType` 为空时不 `continue`，仅既有 `CalcTechClassResAmt` 失败等规则可跳过
- [x] 4.3 在 `isDeviceTypeChanged` 中比较 `cloud.GpuType != db.GpuType`
- [x] 4.4 在 `createDeviceType` 中写入 `GpuType`
- [x] 4.5 在 `updateDeviceType` 中写入 `GpuType`
- [x] 4.6 在 `cmd/hc-service/logics/res-sync/ziyan/device_type_test.go` 增加仅 `gpu_type` 变化应更新、相同则不因该字段更新的用例

## 5. 主动登记创建页

- [x] 5.1 在 `front/src/views/ziyanScr/cvm-model/CreateDevice/index.tsx` 的 `ICvmDeviceCreateModel` 增加 `gpu_type`、`tech_class_res_amt`
- [x] 5.2 表单增加「GPU 卡类型」：`hcm-form-enum` + `allowCreate`，预置常见卡型常量
- [x] 5.3 表单增加「技术分类资源量」非负数字输入（精度对齐 DECIMAL(10,2)）
- [x] 5.4 提交 `createCvmDevice` 时透传两字段；未填卡类型提交空字符串语义，未填资源量可不传（后端默认 0）

## 6. 配置页筛选（列表不加列）

- [x] 6.1 在 `front/src/views/ziyanScr/cvm-model/index.tsx` 的 filter / clearFilter 增加 `gpu_type`
- [x] 6.2 `queryRules` 在有值时追加 `{ field: 'gpu_type', op: 'eq', value }`
- [x] 6.3 筛选区增加「GPU 卡类型」表单项（可搜索，允许手输或预置选项）
- [x] 6.4 确认 `cvmModelColumns` 不新增 `gpu_type` 列

## 7. 规格详情展示

- [x] 7.1 在申领规格详情 `details-info.vue` 中，`gpu_type` 非空时随现有 extra-text 展示
- [x] 7.2 在 `device-type-dialog.vue` 已选机型 extra-text 中同样展示非空 `gpu_type`
- [x] 7.3 必要时在 `front/src/store/cvm/device.ts` 的 `ICvmDevicetypeItem` 声明 `gpu_type`

## 8. 接口文档

- [x] 8.1 更新 `docs/api-docs/web-server/docs/scr/config-manage/create_config_cvm_device.md`：入参增加可选 `gpu_type`、`tech_class_res_amt` 及示例；新字段描述标注 `v9.9.9.9+`，不改原「该接口提供版本」
- [x] 8.2 更新 `docs/api-docs/web-server/docs/scr/config-manage/list_config_cvm_device.md`：响应增加 `gpu_type`（字段标注 `v9.9.9.9+`）
- [x] 8.3 更新 `docs/api-docs/web-server/docs/scr/meta/list_device_type.md`：响应增加 `gpu_type`（字段标注 `v9.9.9.9+`）
- [x] 8.4 更新 `docs/api-docs/web-server/docs/scr/resource-query/list_config_cvm_devicetype.md`：响应增加 `gpu_type`（字段标注 `v9.9.9.9+`）

## 9. 验证

- [x] 9.1 核对 AC-001 / AC-002：同步写入非空与空 `gpu_type`，空值不导致任务失败
- [x] 9.2 核对 AC-005：仅卡类型变化触发更新
- [x] 9.3 核对 AC-003 / AC-004 / AC-006 / AC-007：创建页与接口默认值、超长/负数拒绝、选项外手输
- [x] 9.4 核对 AC-008：配置页可按卡类型筛选，表格无该列
- [x] 9.5 核对无「平台-CVM机型」权限时创建仍拒绝
