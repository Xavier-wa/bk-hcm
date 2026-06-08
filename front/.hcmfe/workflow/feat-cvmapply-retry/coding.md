# Coding - CVM申请-修改需求重试优化

## 实施方案

### 修改文件清单

1. **`src/store/cvm/device.ts`**
   - `getChargeTypeDeviceTypeList` 参数类型增加可选 `suborder_id: string`

2. **`src/components/device-type-selector/cvm-apply/children/use-device-type-plan.ts`**
   - `useDeviceTypePlan` 参数增加可选 `suborderId: Ref<string>`
   - 调用 `getChargeTypeDeviceTypeList` 时条件传递 `suborder_id`

3. **`src/components/device-type-selector/cvm-apply/cvm-apply.vue`**
   - `props` 增加可选 `suborderId: string`
   - `useDeviceTypePlan` 调用时传入 `suborderId`

4. **`src/views/ziyanScr/hostApplication/components/application-modify/index.vue`**
   - `device-type-cvm-selector` 组件绑定 `:suborder-id="suborderId"`

### 兼容性

- `application-form/index.tsx`（新建模式）不传 `suborderId`，保持现有行为
- `suborderId` 为可选 prop，不传时不影响
