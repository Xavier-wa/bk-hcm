# CVM申请-修改需求重试优化

## 需求背景

在"修改申请"（修改需求重试）页面中，点击"编辑"按钮弹出机型选择弹窗时，弹窗会请求 `/api/v1/woa/config/findmany/config/cvm/charge_type/device_type` 接口获取计费模式与机型预测数据。当前请求缺少 `suborder_id` 参数，导致后端在预测核算时无法排除当前子订单的预测数据，从而机型选择列表中可用机型为空，用户无法修改机型。

## 用户故事

- 作为**资源申请者**，在"修改需求重试"页面点击"编辑"修改机型时，希望机型列表能正确排除当前子订单的预测数据，以便看到准确的可用机型并顺利完成修改。

## 功能需求

### 需求1：接口请求补充 suborder_id 参数

在"修改申请"页面的机型选择弹窗中，当请求 `/api/v1/woa/config/findmany/config/cvm/charge_type/device_type` 接口时：

- **编辑模式**（`editMode = true`）下：请求参数中携带当前子订单的 `suborder_id`
- **新建模式**（`editMode = false`）下：不携带 `suborder_id`

### 需求2：调用链参数透传

从"修改申请"页面到接口请求的整个调用链中，需要逐层透传 `suborder_id`：

1. `application-modify/index.vue` -> `DeviceTypeCvmSelector`（新增 `suborderId` prop）
2. `DeviceTypeCvmSelector` -> `useDeviceTypePlan` hook（新增 `suborderId` 参数）
3. `useDeviceTypePlan` -> `useCvmDeviceStore.getChargeTypeDeviceTypeList`（新增 `suborder_id` 参数）

## 验收标准

- [ ] 在"修改申请"页面点击"编辑"按钮后，弹窗请求机型数据时，Network 面板中可见请求体包含 `suborder_id` 字段
- [ ] 新建申请时，请求机型数据接口不携带 `suborder_id`
- [ ] 携带 `suborder_id` 后，修改需求重试场景下机型列表能正确展示可用机型
- [ ] 非编辑模式下的现有功能不受影响

## 边界场景

- 空 `suborder_id` 时（新建模式）：不传递该字段，保持现有行为
- 后端返回异常时：按现有错误处理逻辑执行

## 接口说明

### GET/POST /api/v1/woa/config/findmany/config/cvm/charge_type/device_type

**新增请求字段**：

| 字段名 | 类型 | 是否必填 | 说明 |
|--------|------|----------|------|
| suborder_id | string | 否 | 当前子订单ID，编辑模式时携带 |

**现有请求字段**（保持不变）：

| 字段名 | 类型 | 是否必填 | 说明 |
|--------|------|----------|------|
| bk_biz_id | number | 是 | 业务ID |
| require_type | number | 是 | 需求类型 |
| region | string | 是 | 地域 |
| zone | string | 否 | 可用区 |

## 影响范围

- `src/views/ziyanScr/hostApplication/components/application-modify/index.vue`
- `src/components/device-type-selector/cvm-apply/cvm-apply.vue`
- `src/components/device-type-selector/cvm-apply/children/use-device-type-plan.ts`
- `src/store/cvm/device.ts`
