# Coding - 机房裁撤总览-业务名称未知数据支持显示及明细跳转

## 改动文件清单

| # | 文件 | 改动类型 | 说明 |
|---|------|---------|------|
| 1 | `components/display-value/business-value.vue` | 修改 | 业务名称无法匹配时显示"未知" |
| 2 | `views/dissolve/children/overview/data-list/data-list.vue` | 修改 | 未知业务行可点击跳转 |
| 3 | `views/dissolve/children/overview/index.vue` | 修改 | 跳转时传 bk_biz_id=0 |
| 4 | `views/dissolve/children/overview/search/condition.ts` | 修改 | 搜索栏业务名称增加"未知"选项 |
| 5 | `views/dissolve/children/overview/search/search.vue` | 修改 | 传递 extraOptions 给 business-selector |

## 详细改动

### 1. business-value.vue - 显示"未知"

**改动位置**：`businessNames` 计算属性

**逻辑**：
- 当 `value` 存在（truthy）但在 `businessFullList` 中找不到匹配时，push `"未知"`
- 当 `value` 为 falsy（0、null、undefined）时，也 push `"未知"`

```typescript
const businessNames = computed(() => {
  const values = Array.isArray(props.value) ? props.value : [props.value];
  const names: string[] = [];
  for (const value of values) {
    if (value) {
      const name = businessGlobalStore.businessFullList.find((item) => item.id === value)?.name;
      if (name) {
        names.push(name);
      } else {
        names.push('未知'); // 新增：值存在但无法匹配
      }
    } else if (value === 0 || value === null || value === undefined) {
      names.push('未知'); // 新增：值为 0/null/undefined
    }
  }
  return names;
});
```

### 2. data-list.vue - 未知业务行可点击

**改动位置**：`isBizClickable` 函数

**逻辑**：
- 非汇总行始终可点击（移除对 `businessFullList` 的检查）
- 汇总行不渲染按钮（已有逻辑）

```typescript
const isBizClickable = (row: IDissolveOverview) => {
  // 非汇总行均可点击（包括未知业务）
  return !isSummaryRow(row);
};
```

### 3. overview/index.vue - 跳转时传 bk_biz_id=0

**改动位置**：`handleBizClick` 函数

**逻辑**：
- 判断该行是否为"未知"业务（`bk_biz_id` 不在 `businessFullList` 中）
- 如果是"未知"业务，传 `bk_biz_ids=[0]`
- 否则正常传 `bk_biz_ids=[row.bk_biz_id]`

```typescript
const handleBizClick = (row: IDissolveOverview) => {
  const isInList = businessGlobalStore.businessFullList.some((item) => item.id === row.bk_biz_id);
  const bizId = isInList ? row.bk_biz_id : 0;
  const filter = searchQs.build({ bk_biz_ids: [bizId] });
  routerAction.redirect({ query: { ...route.query, filter, tab: 'detail' } }, { replace: true });
};
```

### 4. search/condition.ts - 增加"未知"选项 props

**改动位置**：`bk_biz_ids` 字段定义

**逻辑**：
- 在 props 中增加 `extraOptions` 配置，用于在业务下拉中追加"未知"选项

```typescript
@Column('business', {
  name: '业务名称',
  index: 3,
  props: {
    scope: 'auth',
    extraOptions: [{ id: 0, name: '未知' }],
  },
})
bk_biz_ids: number[];
```

### 5. search/search.vue - 传递 extraOptions

**改动位置**：`getSearchCompProps` 函数

**逻辑**：
- 将 `field.props` 中的 `extraOptions` 透传给 `hcm-search-business` → `business-selector`

无需额外处理，`getSearchCompProps` 已通过 `...field.props` 展开所有 props。

### 6. business-selector.vue - 支持 extraOptions

**改动位置**：props 定义和 `list` 计算逻辑

**逻辑**：
- 新增 `extraOptions` prop
- 在 `watchEffect` 中，将 `extraOptions` 追加到列表末尾

```typescript
// 新增 prop
extraOptions?: IBusinessItem[];

// 在 watchEffect 中追加
if (props.extraOptions) {
  list.value = [...list.value, ...props.extraOptions];
}
```

## 风险点

1. **business-value.vue 影响范围**：该组件被多处使用（裁撤总览、裁撤明细、主机回收等）。修改后所有使用 `display-value` + `type: 'business'` 的地方，当 `bk_biz_id` 无法匹配时都会显示"未知"。需确认此行为在所有场景下都是合理的。

   **降级方案**：如果其他模块不希望显示"未知"，可以通过 `display` prop 传递自定义配置来控制。但根据需求描述和代码分析，所有场景下"未知"都是合理的行为（业务下线/无权限时显示占位名称）。

2. **business-selector.vue 影响范围**：该组件被多处使用。新增 `extraOptions` prop 是可选的，不传则无影响。只有裁撤总览搜索栏传 `extraOptions: [{ id: 0, name: '未知' }]`。

3. **后端兼容性**：前端传 `bk_biz_ids=[0]` 时，后端需能正确处理。前端不做额外校验。
