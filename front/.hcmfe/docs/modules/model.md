# 模型驱动与页面模式（model）

> status: drafted · kind: module
> globs: `src/model/**`
> 系统基础特色能力。本文档讲清「装饰器字段模型」的原理、「多云」编码模式，以及「展示/表单/列表」三场景组件模式，让 agent 知道存在并复用这套模式，而非从零手写页面。
> 操作手册（分步 how-to）见 skill：`.cursor/skills/page-list`、`.cursor/skills/page-form`、`.cursor/skills/page-detail`。

## 一、模型原理（装饰器驱动的字段元数据）

核心思想：**一次声明字段元数据，多场景投影复用**。同一批字段既能渲染成列表列、搜索项，也能渲染成表单项、详情展示项，避免在列表/表单/详情里重复定义字段。

### 关键文件

- `src/decorator/index.ts` — 导出 `@Model`、`@Column` 装饰器。
- `src/model/manager.ts` — `getModel(ModelClass)` 构造 `Model` 实例。
- `src/model/model.ts` — `Model` 类：`getProperties()` / `getPropertiesByGroup()` / `createInstance()`。
- `src/model/typings.ts` — 字段类型与各场景属性视图类型定义。
- `src/model/<域>/*.ts` — 各业务域的模型定义（如 `model/task/detail-cvm.view.ts`）。

### 声明与消费

```typescript
import { Model, Column } from '@/decorator';
import { getModel } from '@/model/manager';

@Model('task/detail-cvm.view')            // 注册模型（name 可选，用于标识/继承）
export class DetailCvmView {
  @Column('datetime', { name: '开始时间', sort: true })
  created_at: string;

  @Column('enum', { name: '任务状态', option: TASK_DETAIL_STATUS_NAME, meta: { display: { appearance: 'status' } } })
  state: TaskDetailStatus;

  @Column('array', { name: '内网IP', render: ({ data }) => getPrivateIPs(data.param) })
  'param.private_ipv4_addresses': string[];   // 支持点号路径字段名
}

const model = getModel(DetailCvmView);
const columns = model.getProperties();         // 按 index 排序、过滤 hidden 的属性数组
const grouped = model.getPropertiesByGroup();  // 按 group 分组（详情分面板用）
const formData = model.createInstance();       // 造带初始值的表单数据对象
```

### 字段类型与场景视图

- **字段类型** `ModelPropertyType`：`string` / `datetime` / `enum` / `number` / `array` / `bool` / `json` / `account` / `user` / `region` / `business` / `cert` / `ca` / `cloud-area` / `device-family` / `list` 等。
- **场景视图**：同一 `ModelProperty` 联合各场景配置，投影出四类属性——
  | 视图类型 | 场景 | 配置入口 |
  |---|---|---|
  | `ModelPropertyColumn` | 列表列 | `meta.column` / 顶层 `sort`、`render`、`width`、`fixed` |
  | `ModelPropertySearch` | 搜索项 | `meta.search`（`op`、`filterRules`、`converter`） |
  | `ModelPropertyForm` | 表单项 | `meta.form`（`required`、`rules`、`readonly`、`props`） |
  | `ModelPropertyDisplay` | 详情展示 | `meta.display`（`appearance`、`render`、`format`） |
- **常用 `@Column` 选项**：`name`（标签）、`option`（枚举映射，可为异步）、`render`（自定义渲染 `({ row, data, cell }) => VNode|string`）、`sort`、`group`（详情分组）、`apiOnly`（仅 API 用、表单过滤掉）、`hidden`、`meta.display.appearance`（`status`/`link`/`tag`/`cvm-status` 等预置外观）。
- **模型继承**：`getProperties()` 会收集基类字段（`prototype instanceof`），因此可抽公共字段基类再派生——这是多云模式的基础。

## 二、多云 / 多资源编码模式（Factory）

**判断闸口**：同一资源在不同云厂商（`VendorEnum`）或资源类型（`ResourceTypeEnum`）下**字段列表存在差异** → 用 Factory；字段完全一致 → 简化模式直接 `getModel(单类)`。

```typescript
// field-factory.ts —— 按 vendor 返回对应模型
export class FieldFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD: return getModel(FieldTcloud);
      default: throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}
```

- 公共字段抽基类（`@Model('module/form-field')`），各厂商类 `extends` 基类只加特有字段。
- 入口 `provide` 当前 `currentVendor` / `currentResourceType`，子组件 `inject` 后调 `FieldFactory.createModel(vendor)` 拿字段。
- **切换 vendor/resourceType 时必须清空当前搜索条件与列表数据**，避免旧数据残留。

## 三、展示 / 表单 / 列表 三场景组件模式

每个资源模块用一套「通用组件 + 字段模型」组合，通用组件只认 `ModelProperty*` 数组，业务差异全部落在模型定义里。

### 通用组件三+一件套

| 组件 | 职责 | 输入 | 关联 skill |
|---|---|---|---|
| `search.vue` | 搜索区，通常零改动 | `fields`（`ModelPropertySearch[]`）、`condition`；emit `search`/`reset` | page-list |
| `data-list.vue` | 表格区 | `columns`（`ModelPropertyColumn[]`）、`list`、`pagination` | page-list |
| `form.vue` | 新建/编辑表单，字段模型驱动动态组件 | 由 `getModel`/Factory 得到字段模型 | page-form |
| `details.vue` | 详情展示，GridContainer/GridItem + `display-value` 按 `group` 分面板 | `data` + 字段模型 | page-detail |

### 目录约定

```
views/<模块>/
├── index.vue                 # 入口：组装 Search + DataList，管理 sideslider/路由
├── typings.ts / utils.ts     # 模块内闭环
└── children/
    ├── list/{search, data-list}/
    ├── form/{form.vue, create.vue, edit.vue}
    └── details/{details.vue}
      （多类型时各目录追加 *-factory.ts + *-<type>.ts）
```

### 承载方式（两种，模式一致）

- **Sideslider 模式**：入口 `reactive` 管 `isShow`，模板直接渲染 `<CreateForm>`/`<EditForm>`/`<Details>`。
- **独立路由模式**：入口用 `routerAction.redirect()` 跳子页面（禁用 `router.push`，见 menu-route 模块）。

### 关键约定（务必遵守）

- 列表列 / 搜索项 / 表单项 / 展示项一律来自模型 `getProperties()`，不要在模板里手写字段。
- 表单：`form.vue` 用 `fields.filter(f => !f.apiOnly)` 过滤纯 API 字段；编辑回填**逐字段显式赋值**，禁止 `Object.assign(formData.value, props.data)`；表单数据用 `model.createInstance()`。
- 搜索条件通过 `useSearchQs` 与 URL query 同步（刷新保留筛选）。
- 操作按钮（新建/编辑/删除）必须用 `hcm-auth` 包裹并按行数据控 `disabled`（见 auth 模块）。
- 详情特殊字段：`display-value` 不满足时用 `<template>` 覆盖 `<grid-item>`；`extension.xxx` 字段的 `value` 需传整个 `data`。
- 模块内闭环：`typings.ts`/`utils.ts`/字段定义只在模块内引用，跨模块复用抽到 `common/`、`utils/`。
- **选项类组件的 `list` / `list-generator` 必须传稳定引用**。`hcm-form-list`（`components/form/list.vue`，`hcm-search-list` 亦转发到它）内部用 `watchEffect` 追踪该 prop（`localList.value = await props.list()`），追踪的是**函数身份**而非返回值：模板里写 `:list="() => getXxx()"` 或 `:list-generator="getGen(row)"` 时，父组件每次重渲染都产生新引用，effect 判定依赖变化即重新拉一次接口。触发源是任意重渲染——同表单里敲一下输入框、窗口 resize 引起布局重算都算，症状是「改无关字段却狂打下拉接口」。正确写法是把工厂提到 `<script setup>` 作用域（或用 `computed`）固化引用；**级联刷新不会因此失效**，因为 `props.list()` 是在 effect 内同步调用的，函数体对响应式数据的读取仍落在追踪窗口内（首个 `await` 之前），如 `const deviceTypeList = () => getDeviceTypeOptions({ region: formModel.region })` 仍会随地域变更重拉。传数组（含 `ModelProperty.list` 的数组形态）不受影响，重渲染最多是重新赋值、无网络开销。

## 注意事项

- 这是本系统区别于普通 CRUD 页面的**特色能力**：新增列表/表单/详情前，先判断能否用「模型 + 通用组件」落地，再决定是否手写。
- 分步落地严格以三个 skill 为准（含可复制的 assets 模板）；本文档负责「原理与全景 + 何时用」。
- 改造老模块不删老文件，新建迁移并加 `@deprecated`。

## 任务模型接缝（外部更新）

- 任务列表账号字段：`account_ids` 查询 op 为 `json_overlaps`（数组字段重叠，不是 `in`）。模型在 `src/model/task/search.view.ts`。
- 同步任务详情列由 detail 模型 / action-list fields 对齐 `cloud_lb_id` 等协议字段，不混用其它任务类型的 `cloud_clb_id`。
