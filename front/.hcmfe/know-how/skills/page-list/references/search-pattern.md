# 搜索区域模式

## 核心组件 `search.vue`

搜索组件是**完全通用的**，任何列表页都可以复用同一套结构。它只负责：
1. 根据 `fields` 定义渲染搜索表单
2. 收集表单值并 emit `search` / `reset`
3. 同步外部 `condition` 到表单（如 URL 回写）

### Props & Emits

```typescript
interface ISearchProps {
  fields: ModelPropertySearch[];    // 搜索字段定义
  condition: ISearchCondition;       // 当前搜索条件值
}

const emit = defineEmits<{
  (e: 'search', condition: ISearchCondition): void;
  (e: 'reset'): void;
}>();
```

### 动态渲染搜索控件

```vue
<component :is="`hcm-search-${field.type}`" v-bind="getSearchCompProps(field)" v-model="formValues[field.id]" />
```

`field.type` 决定渲染什么组件：`string` → `hcm-search-string`、`enum` → `hcm-search-enum`、`user` → `hcm-search-user` 等。

### 重置行为

重置不是清空表单，而是回到**第一次传入的 condition 值**：

```typescript
let conditionInitValues: ISearchCondition;

watch(() => props.condition, (condition) => {
  formValues.value = { ...condition };
  if (!conditionInitValues) {
    conditionInitValues = { ...formValues.value };
  }
}, { deep: true, immediate: true });

const handleReset = () => {
  formValues.value = { ...conditionInitValues };
  emit('reset');
};
```

## 搜索条件定义

搜索条件使用 `@Model()` + `@Column()` 装饰器定义，最终通过 `getModel()` 转换为 `ModelPropertySearch[]`：

```typescript
import { Model, Column } from '@/decorator';

@Model('module/search-condition')
export class SearchConditionTcloud {
  @Column('string', { name: '字段显示名' })
  fieldName: string;

  @Column('enum', { name: '状态', option: { active: '启用', disabled: '禁用' } })
  status: string;

  @Column('user', { name: '创建人' })
  creator: string;

  @Column('datetime', { name: '创建时间' })
  created_at: string;
}
```

### 特殊搜索逻辑（filterRules）

某些字段的搜索逻辑不单纯是等值匹配，需要自定义转换规则：

```typescript
@Column('string', {
  name: '资源名称',
  meta: {
    search: {
      filterRules(value: string | string[]) {
        if (Array.isArray(value) && value.length > 1) {
          return {
            op: QueryRuleOPEnum.OR,
            rules: value.map((val) => ({ field: 'res_name', op: QueryRuleOPEnum.CS, value: val })),
          };
        }
        return { field: 'res_name', op: QueryRuleOPEnum.CS, value: Array.isArray(value) ? value[0] : value };
      },
    },
  },
})
res_name: string;
```

### 多类型时的 Factory 模式

```typescript
export class SearchConditionFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD: return getModel(SearchConditionTcloud);
      case VendorEnum.AWS:    return getModel(SearchConditionAws);
      default: throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}
```

入口中使用：

```typescript
const searchModel = computed(() => SearchConditionFactory.createModel(currentVendor.value));
const searchFields = computed(() => searchModel.value.getProperties());
```

## URL 查询参数同步

入口使用 `useSearchQs` hook 实现搜索条件与 URL query 的双向同步：

```typescript
const searchQs = useSearchQs({ key: 'filter', properties: searchFields.value });

// 从 URL 解析条件
condition.value = searchQs.get(route.query);

// 搜索时写入 URL
const handleSearch = (condition: ISearchCondition) => {
  searchQs.set(condition);
};

// 重置时清空 URL
const handleReset = () => {
  searchQs.clear();
};
```

## 调用链

```
index.vue
├── SearchConditionFactory.createModel(vendor) → ModelPropertySearch[]
├── useSearchQs({ properties }) → searchQs
├── route.query → searchQs.get() → condition
├── Search(fields=searchFields, condition=condition)
│   ├── 用户点击查询 → emit('search', formValues)
│   └── index.vue: handleSearch → searchQs.set(condition) → URL 更新 → route.query 变化
└── route.query watch → 触发 API 请求
```
