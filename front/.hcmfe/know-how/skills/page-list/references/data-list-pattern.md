# 表格区域模式

## 核心组件 `data-list.vue`

表格组件是**完全通用的**，只负责根据 `columns` 定义渲染表格，并通过 emit 将交互事件抛给入口处理。

### Props & Emits

```typescript
export interface IDataListProps<T> {
  columns: ModelPropertyColumn[];    // 列定义
  list: T[];                          // 数据列表
  pagination: PaginationType;         // 分页配置
}

const emit = defineEmits<{
  'view-details': [row: T];
  'edit': [row: T];
  'delete': [row: T];
}>();
```

### 动态列渲染

```vue
<bk-table-column
  v-for="(column, index) in columns"
  :key="index"
  :prop="column.id"
  :label="column.name"
  :sort="column.sort"
  :render="column.render"
>
  <template #default="{ row }">
    <!-- 特殊列：名称可点击查看详情 -->
    <template v-if="column.id === 'name'">
      <bk-button theme="primary" text @click="emit('view-details', row)">
        {{ row.name || '--' }}
      </bk-button>
    </template>
    <!-- 其他列：使用 display-value 通用渲染 -->
    <template v-else>
      <display-value :property="column" :value="row[column.id]" :display="column?.meta?.display" />
    </template>
  </template>
</bk-table-column>
```

### 操作列

操作列固定在最右侧，每个操作按钮用 `hcm-auth` 包裹，并根据行数据控制禁用状态：

```vue
<bk-table-column :show-overflow-tooltip="false" label="操作">
  <template #default="{ row }">
    <div class="actions">
      <hcm-auth :sign="{ type: AUTH_UPDATE_XXX, relation: [bizId] }" v-slot="{ noPerm }">
        <bk-button theme="primary" text :disabled="noPerm || row.someDisableCondition" @click="emit('edit', row)">
          编辑
        </bk-button>
      </hcm-auth>
      <hcm-auth :sign="{ type: AUTH_DELETE_XXX, relation: [bizId] }" v-slot="{ noPerm }">
        <bk-button theme="primary" text :disabled="noPerm || row.someDisableCondition" @click="emit('delete', row)">
          删除
        </bk-button>
      </hcm-auth>
    </div>
  </template>
</bk-table-column>
```

### 分页与排序

```vue
<bk-table
  :data="list"
  :pagination="pagination"
  :settings="settings"
  remote-pagination
  show-overflow-tooltip
  @page-limit-change="handlePageSizeChange"
  @page-value-change="handlePageChange"
  @column-sort="handleSort"
  row-key="id"
>
```

`usePage()` hook 提供分页处理函数，`useTableSettings(props.columns)` 提供表格设置（列显隐控制）。

## 表格列定义

表格列同样使用 `@Model()` + `@Column()` 装饰器定义：

```typescript
import { Model, Column } from '@/decorator';

@Model('module/table-column')
export class TableColumnTcloud {
  @Column('string', { name: '名称' })
  name: string;

  @Column('string', { name: '类型', render: ({ row }) => h(Tag, { theme: getTheme(row) }, getLabel(row)) })
  type: number;

  @Column('string', { name: '数量', sort: true })
  count: number;

  @Column('user', { name: '创建人' })
  creator: string;

  @Column('datetime', { name: '创建时间', sort: true })
  created_at: string;
}
```

### render 属性

当列需要自定义渲染（如 tag、按钮、格式化值）时，使用 `render` 属性：

```typescript
@Column('string', {
  name: '状态',
  render: ({ row }: { row: ItemType }) => {
    const { label, theme } = getStatusData(row);
    return h(Tag, { radius: '4px', theme }, label);
  },
})
status: number;
```

### 多类型时的 Factory 模式

```typescript
export class TableColumnFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD: return getModel(TableColumnTcloud);
      case VendorEnum.AWS:    return getModel(TableColumnAws);
      default: throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}
```

入口中使用：

```typescript
const columnModel = TableColumnFactory.createModel(currentVendor.value);
const dataListColumns = computed(() => columnModel.getProperties());
```

## 单类型时的简化

如果模块只支持单一类型（如仅腾讯云），不需要 factory，直接在入口中定义列：

```typescript
// 方式 1：直接写数组（简单场景）
const dataListColumns = ref<ModelPropertyColumn[]>([
  { id: 'name', name: '名称', type: 'string' },
  { id: 'status', name: '状态', type: 'string' },
]);

// 方式 2：用装饰器定义后 getModel（保持与多类型一致的结构）
const columnModel = getModel(TableColumn);
const dataListColumns = computed(() => columnModel.getProperties());
```
