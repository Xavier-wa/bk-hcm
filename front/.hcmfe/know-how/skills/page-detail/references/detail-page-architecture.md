# 详情页架构

## 目录组织

```
views/<模块>/
├── index.vue                    # 列表页入口（page-list skill）
├── children/
│   ├── list/                    # 列表子组件（page-list skill）
│   ├── details/                 # 【本 skill】详情子组件
│   │   ├── details.vue          # 详情组件（接收 data props 或自行获取）
│   │   ├── field-factory.ts     # 【多类型时】字段工厂
│   │   └── field-<type>.ts      # 【多类型时】各类型字段定义
│   ├── create-form/             # 新建表单（page-form skill）
│   └── edit-form/               # 编辑表单（page-form skill）
```

## 入口职责

详情页有两种承载方式：

1. **Sideslider 模式**：详情组件作为子组件被列表页入口直接引入，通过 `props` 接收数据
   ```vue
   <!-- index.vue 中 -->
   <Details :data="selectedRow" />
   ```

2. **独立路由模式**：详情页为独立 `.vue` 文件，通过 `route.params.id` 自行获取数据
   ```vue
   <!-- views/<module>/details.vue 或 children/details-page/details.vue -->
   <script setup>
   const route = useRoute();
   const data = ref({});
   onMounted(async () => {
     data.value = await store.getDetail(route.params.id);
   });
   </script>
   ```

无论哪种方式，**详情展示组件本身**（`details.vue`）的结构是一致的：
- 接收 `data`（对象）作为展示数据源
- 通过 Factory 获取字段定义（多类型时）
- 使用 `GridContainer` + `GridItem` 按分组展示字段

## 与列表页的关系

| 维度 | 列表页 | 详情页 |
|------|--------|--------|
| 数据获取 | 列表页自行获取 | Sideslider: props 传入; 独立路由: 自行获取 |
| 多类型 | 注入 currentVendor | 同样 inject currentVendor |
| Factory | condition-factory + column-factory | field-factory |
| 布局 | Search + DataList | GridContainer + GridItem 分组 |

## 字段分组

字段通过 `@Column` 装饰器的 `group` 属性分组：
```typescript
@Column('string', { name: '模板名称', group: '基本信息' })
name: string;

@Column('json', { name: '', group: '权限模板' })
policy_document: string;
```

在组件中通过 `model.getPropertiesByGroup<ModelPropertyDisplay>()` 获取分组后的字段：
```typescript
const properties = model.getPropertiesByGroup<ModelPropertyDisplay>();
// 返回值: { '基本信息': [...], '权限模板': [...] }
```

每个 group 渲染为一个 `details-panel`，内部使用一个 `GridContainer`。
