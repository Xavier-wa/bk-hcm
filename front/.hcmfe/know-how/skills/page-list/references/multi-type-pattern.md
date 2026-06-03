# 多云 / 多资源模式

## 什么时候使用 Factory 模式？

当列表页需要支持**同一资源在不同云厂商下字段不同**，或**同一页面切换资源类型后展示不同字段**时，使用 Factory 模式。

典型场景：
- **多云管理**：VPC 列表在腾讯云/AWS/Azure 下的可筛选字段、表格列不同
- **多资源类型**：操作日志在 CLB/安全组/VPC 等资源类型下的搜索条件不同

## 核心机制

通过一个**工厂类**根据当前类型（vendor / resourceType）创建对应的模型：

```typescript
// search/condition-factory.ts
export class SearchConditionFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD: return getModel(SearchConditionTcloud);
      case VendorEnum.AWS:    return getModel(SearchConditionAws);
      case VendorEnum.AZURE:  return getModel(SearchConditionAzure);
      default: throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}

// data-list/column-factory.ts
export class TableColumnFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD: return getModel(TableColumnTcloud);
      case VendorEnum.AWS:    return getModel(TableColumnAws);
      case VendorEnum.AZURE:  return getModel(TableColumnAzure);
      default: throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}
```

## 入口中的类型注入

### 多云场景（注入 currentVendor）

子组件通过 `inject` 获取当前云厂商：

```typescript
// entry-biz.vue / index.vue
const currentVendor = ref(VendorEnum.TCLOUD);
provide('currentVendor', currentVendor);

// search.vue / data-list.vue（子组件）
const currentVendor = inject<Ref<VendorEnum>>('currentVendor', ref(VendorEnum.TCLOUD));
```

### 多资源场景（注入 currentResourceType）

```typescript
// entry.vue
const currentResourceType = ref(ResourceTypeEnum.CLB);
provide('currentResourceType', currentResourceType);

// factory.ts
export class SearchConditionFactory {
  static createModel(resourceType: ResourceTypeEnum | 'all') {
    switch (resourceType) {
      case ResourceTypeEnum.CLB: return getModel(SearchConditionClb);
      case 'all': return getModel(SearchConditionAll);
      default: return getModel(SearchConditionDefault);
    }
  }
}
```

## 具体定义文件的结构

每个类型定义文件是独立的 `@Model()` 类，包含该类型特有的字段：

```typescript
// condition-tcloud.ts
@Model('module/search-condition-tcloud')
export class SearchConditionTcloud {
  @Column('string', { name: '字段A' })
  field_a: string;

  @Column('enum', { name: '状态', option: STATUS_OPTIONS })
  status: string;
}

// condition-aws.ts
@Model('module/search-condition-aws')
export class SearchConditionAws {
  @Column('string', { name: '字段B（AWS 特有）' })
  field_b: string;

  @Column('string', { name: '字段A' })
  field_a: string;
}
```

## 继承公共字段（推荐）

当多种类型之间存在大量公共字段时，使用**继承**抽取基类，避免重复定义：

```typescript
// column.ts（基类：定义所有公共字段）
@Model('module/table-column')
export class TableColumn {
  @Column('datetime', { name: '操作时间', sort: true, index: 0 })
  created_at: string;

  @Column('string', { name: '资源类型', index: 0 })
  res_type: string;

  @Column('string', { name: '资源名称', index: 0 })
  res_name: string;

  @Column('enum', { name: '操作方式', option: ACTION_NAME, index: 0 })
  action: string;

  @Column('user', { name: '操作人', index: 0 })
  operator: string;
}

// column-all.ts（继承：无扩展，直接使用基类字段）
@Model('module/table-column-all')
export class TableColumnAll extends TableColumn {
  // 暂无扩展字段
}

// column-clb.ts（继承：添加/覆盖特定字段）
@Model('module/table-column-clb')
export class TableColumnClb extends TableColumn {
  @Column('enum', { name: 'CLB 状态', option: CLB_STATUS_OPTIONS, index: 0 })
  status: string;
}
```

**继承的使用原则**：
- 公共字段（如创建时间、操作人、资源名称）放在基类中
- 各类型子类通过 `extends` 继承基类，添加特有字段或覆盖 option
- 子类中可用 `static` 属性定义类型特定的常量（如 `static actionOption = {...}`）

## 单类型时的简化

如果确认模块**不需要**支持多云/多资源，直接**跳过 factory**：

1. 不创建 `condition-factory.ts` / `column-factory.ts`
2. 不创建 `condition-<type>.ts` / `column-<type>.ts`
3. 搜索条件直接写在 `condition.ts`
4. 表格列直接写在 `index.vue`（数组形式）或 `columns.ts`（装饰器形式）
5. 入口中不需要注入 `currentVendor`

## 决策流程

```
模块是否需要支持多云/多资源？
├── 是 → 使用 Factory 模式
│   ├── 创建 condition-factory.ts + condition-<type>.ts
│   ├── 创建 column-factory.ts + column-<type>.ts
│   ├── 入口注入 currentVendor / currentResourceType
│   └── 子组件通过 inject 获取当前类型
└── 否 → 简化模式
    ├── 搜索条件直接写在一个文件中
    ├── 表格列直接定义（数组或装饰器）
    └── 入口不需要注入类型
```
