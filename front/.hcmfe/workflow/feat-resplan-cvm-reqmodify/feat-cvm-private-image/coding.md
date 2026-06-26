# Coding: 自研云-主机申领-私有镜像支持

## 实施概述

**变更范围**：修改 2 个文件 —— `cvm-image-selector.vue`（基础组件）+ `form-cvm-image-selector.vue`（表单包装组件）

变更本质：
1. 给 `cvm-image-selector.vue` 新增可选的 `bizId` prop，用于适配不同场景的业务 ID 来源
2. 将 API 从旧公共镜像接口替换为按业务 ID 查询的统一接口（返回公共 + 私有镜像）
3. 通过 prop 透传，让父组件自行决定 bizId 来源（路由 query / BusinessSelector / 表单 model / 全局 hook）

## 使用场景与 bizId 来源

| # | 使用页面 | 组件调用 | bizId 来源 |
|---|----------|----------|-----------|
| 1 | 主机库存匹配面板 (`match-panel/common-resource`) | `CvmImageSelector` 直接使用 | `useWhereAmI().getBizsId()`（全局）—— **无需传 prop** |
| 2 | 主机申领表单 (`application-form`) | `FormCvmImageSelector` | `order.value.model.bkBizId`（BusinessSelector 选择值）或 `getBizsId()` —— **需要透传** |
| 3 | CVM生产单据 (`cvm-produce/create-order`) | `form-cvm-image-selector` | `formModel.bk_biz_id`（表单 model，如 931）—— **需要透传** |

## 涉及文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `front/src/views/ziyanScr/components/ostype-selector/cvm-image-selector.vue` | **修改** | 新增 `bizId` 可选 prop + 替换 API 接口 |
| `front/src/views/ziyanScr/components/ostype-selector/form-cvm-image-selector.vue` | **修改** | 透传 `bizId` prop 到子组件 |
| `front/src/views/ziyanScr/hostApplication/components/application-form/index.tsx` | **修改** | 传 `computedBiz` 作为 `:biz-id` |
| `front/src/views/ziyanScr/cvm-produce/component/create-order/index.vue` | **修改** | 传 `formModel.bk_biz_id` 作为 `:biz-id` |

### 不改动的文件

| 文件 | 原因 |
|------|------|
| `match-panel/common-resource/index.tsx` | 已使用 `useWhereAmI().getBizsId()` 场景，无需传 prop |
| `image-selector.tsx` (service-apply) | 多厂商镜像选择器，使用不同 API，不在范围内 |

## 详细改动

### 文件 1: `cvm-image-selector.vue`

#### 改动 1: 新增 import

在 `<script setup>` 顶部添加：

```typescript
import { useWhereAmI } from '@/hooks/useWhereAmI';
```

#### 改动 2: IProps 增加 `bizId?` 可选 prop

```typescript
interface IProps {
  region: string[];
  idKey?: string;
  displayKey?: string;
  multiple?: boolean;
  clearable?: boolean;
  filterable?: boolean;
  disabled?: boolean;
  transform?: (options: ICvmImage[]) => ICvmImage[];
  bizId?: number | string;   // 新增：外部传入的业务 ID；不传则用 getBizsId() 获取
}
```

#### 改动 3: 更新 ICvmImage 接口

```typescript
export interface ICvmImage {
  image_id: string;
  image_name: string;
  type?: string;        // PUBLIC_IMAGE | PRIVATE_IMAGE
  bk_biz_id?: number;   // -1 表示公共镜像
  [key: string]: any;
}
```

#### 改动 4: 获取业务 ID（优先 prop > 全局 hook）

在 `defineOptions` 之后：

```typescript
const { getBizsId } = useWhereAmI();
```

在 `getOptions` 函数中：

```typescript
// bizId 优先取 props 传入值，否则从 useWhereAmI 全局获取
const bizId = props.bizId ?? getBizsId();
let res: IQueryResData<{ info: ICvmImage[] }>;
if (bizId) {
  res = await http.post(`/api/v1/woa/bizs/${bizId}/config/cvm/image`, { region });
} else {
  // 兜底: 无业务 ID 时走旧公共镜像接口
  res = await http.post('/api/v1/woa/config/findmany/config/cvm/image', { region });
}
```

### 文件 2: `form-cvm-image-selector.vue`

#### 改动: 透传 `bizId` prop

**IProps 新增**:

```typescript
interface IProps {
  region: string[];
  idKey?: string;
  displayKey?: string;
  multiple?: boolean;
  disabled?: boolean;
  bizId?: number | string;   // 新增：透传给 cvm-image-selector
}
```

**模板** `<cvm-image-selector>` 标签新增:

```html
:biz-id="bizId"
```

### 文件 3: `application-form/index.tsx`（主机申领）

**已有变量**（第 603-604 行）:
```typescript
const computedBiz = computed(() => {
  return whereAmI.value === Senarios.business ? getBizsId() : order.value.model.bkBizId;
});
```

**`<FormCvmImageSelector>` 标签新增 prop**（约第 1522 行）:
```tsx
<FormCvmImageSelector
  class={'commonCard-form-select'}
  v-model={QCLOUDCVMForm.value.spec.image_id}
  region={[resourceForm.value.region]}
  :biz-id={computedBiz}          <!-- 新增 -->
/>
```

### 文件 4: `create-order/index.vue`（CVM生产）

**`<form-cvm-image-selector>` 标签新增 prop**（约第 379 行）:
```vue
<form-cvm-image-selector
  class="form-controls-item"
  v-model="formModel.spec.image_id"
  :region="[formModel.spec.region]"
  :biz-id="formModel.bk_biz_id"    <!-- 新增 -->
/>
```

## 兜底策略总结

| 条件 | 使用接口 | 说明 |
|------|----------|------|
| 有 bizId（prop 或全局）且 truthy | `/api/v1/woa/bizs/{bk_biz_id}/config/cvm/image` | 新接口，返回公共+私有 |
| 无 bizId 或 falsy | `/api/v1/woa/config/findmany/config/cvm/image` | 旧接口兜底，仅公共镜像 |

## 数据流

```
父组件（application-form / create-order / common-resource）
  → 传入 bizId（可选，不传则组件内部 getBizsId()）
  → watch(region) 触发 getOptions()
  → bizId 存在 → 新接口 / 不存在 → 旧接口
  → 返回 info[] 包含 PUBLIC_IMAGE + PRIVATE_IMAGE（如有）
  → options.value 赋值 → 下拉框渲染
  → form-cvm-image-selector 的 transformOptions() 处理排序 + 状态标签
  → option.type 已被模板中的 <bk-tag> 自动展示
```

## 遵守的编码红线

- [x] API 调用使用相对路径（无 BK_HCM_AJAX_URL_PREFIX 前缀）
- [x] 不修改 @deprecated 文件
- [x] CSS 无新增（零 UI 变更）
- [x] 向后兼容：不传 bizId 时行为与改造前一致（getBizsId() + 降级到旧接口）
