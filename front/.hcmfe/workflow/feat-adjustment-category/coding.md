# Coding：三方云核算-调账类别调整-前端

## 实现范围

已完成前端调账页面改造，未修改后端代码。

### 云厂商可用状态

- `front/src/components/vendor-radio-group/index.tsx` 新增 `enabledVendors` 可选属性，支持按业务场景逐项禁用云厂商按钮。
- 调账新增/编辑表单传入 `BILL_ADJUSTMENT_SUPPORTED_VENDORS`，仅 `aws`、`gcp`、`huawei` 可选；`azure`、`zenlayer` 等其余选项保留展示但置为 disabled。
- 调账表单新增默认云厂商调整为 `aws`，避免默认落在已 disabled 的厂商上。
- 其他复用 `VendorRadioGroup` 的页面不传 `enabledVendors`，保持原有行为。

### 资源类别与资源子类

- `front/src/constants/bill.ts` 将资源类别扩展为 `cpu`、`gpu_card`、`gpu_api`、`gpu_other`，同步更新下拉选项和列表展示映射。
- `front/src/views/bill/bill/adjust/create/RenderTable/index.tsx` 在“资源类别”后增加“资源子类”表头。
- `front/src/views/bill/bill/adjust/create/RenderTableRow/index.tsx` 增加资源子类选择列：
  - `gpu_card` 请求卡型候选；
  - `gpu_api` 请求模型厂商候选；
  - `cpu` / `gpu_other` **不渲染下拉**，展示为 `--`，避免可编辑列在空列表时的 "true" 异常；
  - 仅 `aws`、`gcp`、`huawei` 请求候选接口；
  - 类别或云厂商切换时清空资源子类；
  - 使用请求序号避免旧请求响应覆盖当前候选；
  - **下拉沿用 `@blueking/ediatable` 的 `SelectColumn`**（可编辑表格 row 内必须用 Column 系组件，保持行校验/布局一致）；参考项目 `secondary-account-selector.vue` 的异步 list 标准用法，传 `list`/`v-model`/`clearable`/`placeholder`，并以 `{...({ loading, filterable } as Record<string, unknown>)}` 透传库 `.d.ts` 未声明但运行时支持的属性；
  - 之前"下拉显示 true 且无候选"的真正根因是**接口响应结构未归一化**（见下方 API 段 `normalizeAdjustmentSubClassList`），而非组件选择；
  - `key={`subclass-${vendor}-${res_class}`}` 强制下拉在厂商/类别变化时重建；
  - 新增 `normalizeResClass` / `normalizeSubClass` 兜底：非法 `res_class` 归一为 `cpu`，`res_sub_class` 非字符串归一为空串；`handleResClassChange` 入参放宽为 `unknown` 以匹配 `SelectColumn` 的 `onUpdate:modelValue(IKey)` 签名；
  - 提交时 `res_sub_class` 始终归一为字符串，CPU/GPU其他显式提交空字符串。
- `front/src/views/bill/bill/adjust/index.tsx` 在资源类别后增加资源子类列表列，空值展示 `--`。

### API 与类型

- `front/src/api/bill/index.ts` 新增：
  - `GET /api/v1/account/vendors/{vendor}/bills/adjustment_items/gpu_cards`
  - `GET /api/v1/account/vendors/{vendor}/bills/adjustment_items/api_brands`
  - 两个接口统一经 `normalizeAdjustmentSubClassList` 归一化响应：兼容纯字符串数组、`{value/label/name/id}` 对象数组、以及被包裹在对象里的数组，统一输出 `string[]`，避免响应结构差异导致下拉候选为空。
- `front/src/typings/bill.ts`、`front/src/store/useBillStore.ts`、API 更新参数补充 `res_sub_class`。

## 校验结果

- 指定修改文件 ESLint：通过。
- `npm run build:bcc`：通过。
- 构建仅产生项目既有 Sass 弃用提示和资源体积 warning，无新增编译错误。

## 联调风险

后端兄弟需求明确华为候选接口可能返回空列表，且后端校验对华为的 `gpu_card` / `gpu_api` 可能拒绝；当前前端按已确认 PRD 保留四类资源类别并按接口结果展示，待接口就位后联调确认。