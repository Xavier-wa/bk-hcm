# Coding: CVM 申请 - 修改需求重试 - 数量显示优化

## 修改文件

`src/views/ziyanScr/hostApplication/components/application-modify/index.vue`

## 修改点

### 1. 未生产数计算（Line 52）

```typescript
// 修改前
const unProductNum = computed(() => (!details.value ? 0 : details.value.origin_num - details.value.product_num));

// 修改后
const unProductNum = computed(() => (!details.value ? 0 : details.value.total_num - details.value.product_num));
```

### 2. 表单初始化解构（Line 70）

```typescript
// 修改前
const { origin_num, product_num } = details.value || {};

// 修改后
const { total_num, product_num } = details.value || {};
```

### 3. 表单复本数量初始值（Line 76）

```typescript
// 修改前
replicas: origin_num - product_num,

// 修改后
replicas: total_num - product_num,
```

### 4. 生产情况-需求总数展示字段（Line 165）

```typescript
// 修改前
{ id: 'origin_num', name: '需求总数', type: 'number' },

// 修改后
{ id: 'total_num', name: '需求总数', type: 'number' },
```

### 5. 底部提示文本（Line 371）

```html
<!-- 修改前 -->
<span class="text-danger">{{ details?.origin_num }}</span>

<!-- 修改后 -->
<span class="text-danger">{{ details?.total_num }}</span>
```

## 无影响项

- API 响应参数不变，无需后端配合
- `origin_num` 在其他页面（列表、详情）仍保留，作为历史参考信息展示
- 当前分支已有足够代码上下文，无需新建文件
