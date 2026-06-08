# 列表页整体架构

## 目录组织

HCM 的列表页采用**入口组装 + 子组件拆分**的模式：

```
views/<模块>/
├── index.vue                    # 入口：组装 Search + DataList，管理页面级状态
├── typings.ts                   # 模块内闭环的类型定义
├── utils.ts                     # 模块内闭环的工具函数
├── children/
│   └── list/
│       ├── search/
│       │   ├── search.vue       # 通用搜索表单组件
│       │   ├── condition-factory.ts   # 【多类型时】条件工厂
│       │   └── condition-<type>.ts    # 【多类型时】具体类型的搜索条件定义
│       └── data-list/
│           ├── data-list.vue    # 通用表格组件
│           ├── column-factory.ts      # 【多类型时】列工厂
│           └── column-<type>.ts       # 【多类型时】具体类型的表格列定义
```

### 什么时候需要多视角入口？

当模块需要在多个一级视图（业务/资源/工作台）下呈现时，不直接用 `index.vue`，而是创建多个入口文件：

```
views/<模块>/
├── entry-biz.vue          # 业务视角入口
├── entry-rsc.vue          # 资源视角入口
├── entry-srv.vue          # 服务/工作台视角入口
├── children/
│   └── list/ ...
```

多视角入口的核心职责：
1. 注入当前视角标识：`provide('isBusinessPage', true)` 或 `provide('currentVendor', ref(VendorEnum.TCLOUD))`
2. 引入 `children/list/` 下的公共子组件
3. 各入口复用同一套 `children/` 子组件，子组件通过 `inject` 获取上下文实现差异化行为

> 详见 `fe-menu-route-architecture.mdc` 中的"多视角入口约定"。

## 入口 `index.vue` 的职责

### 1. 状态管理

入口管理所有页面级 UI 状态，典型结构：

```typescript
const createState = reactive({ isShow: false, data: null });
const editState   = reactive({ isShow: false, data: null });
const detailsState = reactive({ isShow: false, data: null });
const deleteState = reactive({ isShow: false, data: null });
```

每个状态对应一个交互：
- `createState` → `bk-sideslider` 新建表单
- `editState` → `bk-sideslider` 编辑表单
- `detailsState` → `bk-sideslider` 详情展示
- `deleteState` → 删除确认对话框

### 2. 搜索与表格的数据流

```
index.vue
├── 注入 currentVendor / currentResourceType
├── 通过 Factory 创建 searchModel + columnModel
├── 监听 route.query → 解析搜索条件 + 分页参数
├── 调用 Store API → 获取列表数据
├── 传递 list + columns + pagination 给 DataList
└── 传递 fields + condition 给 Search
```

### 3. 权限控制

所有操作按钮（新建、编辑、删除）必须用 `hcm-auth` 组件包裹：

```vue
<hcm-auth :sign="{ type: AUTH_CREATE_XXX, relation: [bizId] }" v-slot="{ noPerm }">
  <bk-button theme="primary" :disabled="noPerm" @click="handleCreate">新建</bk-button>
</hcm-auth>
```

## `children/list/search/search.vue` 的职责

通用搜索组件，**不感知业务**，只负责渲染和事件：

- Props: `fields` (`ModelPropertySearch[]`), `condition` (`ISearchCondition`)
- Emits: `search(condition)`, `reset()`
- 使用 `grid-container` 布局（4 列）
- 通过 `<component :is="`hcm-search-${field.type}`">` 动态渲染搜索控件
- `watch props.condition` 实现外部查询参数同步到表单

## `children/list/data-list/data-list.vue` 的职责

通用表格组件，**不感知业务**，只负责渲染和事件：

- Props: `columns` (`ModelPropertyColumn[]`), `list` (`T[]`), `pagination` (`PaginationType`)
- Emits: `view-details(row)`, `edit(row)`, `delete(row)`
- 使用 `bk-table` + `bk-table-column`
- `v-for columns` 动态渲染列，通过 `template #default="{ row }"` 处理特殊列渲染
- 操作列固定在最右侧，用 `hcm-auth` 包裹每个操作按钮
- 分页事件通过 `usePage` hook 处理

## 单类型 vs 多类型的简化差异

| 场景 | 搜索条件 | 表格列 | 入口注入 |
|------|---------|--------|---------|
| **单类型**（如云密钥） | 直接写在 `condition.ts`，无 factory | 直接写在 `index.vue` 或 `columns.ts` | 不需要注入 |
| **多类型**（如多云、多资源） | `condition-factory.ts` + `condition-<type>.ts` | `column-factory.ts` + `column-<type>.ts` | 注入 `currentVendor` / `currentResourceType` |

> 单类型时，入口中直接写 `const searchFields = [...]` 和 `const dataListColumns = [...]`，不需要 factory。
