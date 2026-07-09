# Coding：机房裁撤-功能优化重构-前端

> 本文档记录实际的编码实现，基于 PRD + Design + API 三份产物。
>
> **HCM 编码红线遵守清单**：路由 name 用符号常量 ✓ / 菜单路由解耦 ✓ / 视图权限 `meta.auth.view`（待接入）✓ / 操作权限 `<hcm-auth>` ✓ / 表单 `reactive(initialState)` ✓ / API 相对路径 ✓ / CSS kebab-case ✓（无 BEM 风格）/ 不改 @deprecated 文件 ✓

## 0. 实际目录结构

```
front/src/views/server-dissolve/          （新建模块目录，取代旧的 views/ziyanScr/recycle-server-room/）
├── index.vue                              # 页面入口：Tab 容器 + 顶栏按钮（Teleport 到 #breadcrumbExtra）
├── route-config.ts                        # 路由配置
├── api-doc/                               # API 文档（7 份，供参考）
│   ├── get_dissolve_config.md
│   ├── list_dissolve_host_detail.md
│   ├── list_dissolve_projects.md
│   ├── list_dissolve_table.md
│   ├── list_export_dissolve_host_detail.md
│   ├── sync_recycled_host.md
│   └── upsert_dissolve_config.md
├── children/
│   ├── detail/                            # "裁撤明细" Tab
│   │   ├── index.vue                      #   明细页容器：搜索 + 数据列表
│   │   ├── data-list/
│   │   │   ├── column.ts                  #   表格列 @Model/@Column 定义
│   │   │   ├── data-list.vue              #   表格渲染 + 批量复制 + 导出
│   │   │   └── export-column.ts           #   导出 extension 列定义（22 个字段）
│   │   └── search/
│   │       ├── condition.ts               #   搜索条件 @Model/@Column 定义
│   │       └── search.vue                 #   搜索表单组件
│   ├── overview/                          # "裁撤总览" Tab
│   │   ├── index.vue                      #   总览页容器：搜索 + 数据列表
│   │   ├── data-list/
│   │   │   ├── column.ts                  #   表格列 @Model/@Column 定义（含 group 多级表头）
│   │   │   └── data-list.vue              #   表格渲染 + 汇总行 + 本地排序
│   │   └── search/
│   │       ├── condition.ts               #   搜索条件 @Model/@Column 定义
│   │       └── search.vue                 #   搜索表单组件
│   └── quota-offset/                      # 裁撤额度调整表格
│       ├── index.vue                      #   Ediatable 容器
│       └── render-row.vue                 #   单行渲染
└── components/
    ├── config-dialog/                     # 裁撤配置侧滑面板
    │   ├── index.vue                      #   bk-sideslider + Panel 布局
    │   └── time-period-block.vue          #   单个时间段卡片（Ediatable）
    ├── export-dissolve-dialog/            # 导出裁撤明细对话框
    │   └── index.vue                      #   附带主机利用率选项 + 日期选择
    ├── org-tree-selector/                 # 组织树选择器
    │   └── index.vue                      #   封装 TreeSelector + tof_dept_id 提取
    ├── sync-dialog/                       # 同步对话框
    │   └── index.vue                      #   只读展示项目类型列表
    └── time-period-selector/              # 时间段选择器
        └── index.vue                      #   bk-tag-input + bk-popover 面板
```

**关键架构差异 vs 旧 coding.md**：
- **无 model/ 目录**：condition.ts 和 column.ts 直接放在 `children/*/search/` 和 `children/*/data-list/` 下
- **无 hooks/ 目录**：使用 `@/hooks/` 已有 hooks（usePage、useSearchQs、useTableSettings 等），无需新增
- **无 typings.ts**：类型定义在 `@/store/dissolve/quota.ts` 中
- **store 复用**：`@/store/dissolve/quota.ts` 扩展而非重建
- **data 共享**：`provide/inject`（dissolveProjects、projectTypeList、fetchDissolveConfig）

## 1. 路由与菜单

### 1.1 route-config.ts

```typescript
// front/src/views/server-dissolve/route-config.ts
import type { RouteRecordRaw } from 'vue-router';
import Meta from '@/router/meta';
import { MENU_SERVICE_DISSOLVE, MENU_SERVICE } from '@/constants/menu-symbol';

export default [
  {
    path: 'dissolve',                           // 相对路径，注册到 service 路由下
    name: MENU_SERVICE_DISSOLVE,                 // 'menu_service_dissolve'
    component: () => import('@/views/server-dissolve/index.vue'),
    meta: {
      ...new Meta({
        owner: MENU_SERVICE,                     // 属于 service 视图
        title: '机房裁撤',
        activeKey: MENU_SERVICE_DISSOLVE,
        layout: {
          breadcrumbs: { show: true, back: false },
        },
        icon: 'hcm-icon bkhcm-icon-dissolve',
      }),
    },
  },
] as RouteRecordRaw[];
```

### 1.2 menu-symbol.ts

```typescript
// front/src/constants/menu-symbol.ts 第 49 行
export const MENU_SERVICE_DISSOLVE = 'menu_service_dissolve';
```

### 1.3 auth-symbols.ts

```typescript
// front/src/constants/auth-symbols.ts 第 78-79 行
export const AUTH_FIND_DISSOLVE = Symbol.for('auth_find_dissolve');
export const AUTH_UPDATE_DISSOLVE = Symbol.for('auth_update_dissolve');
```

### 1.4 路由注册

路由在 `front/src/router/module/service.ts` 第 14 行导入并合并到 `serviceMenus` 路由数组中。

## 2. Store（类型定义 + API 调用）

文件：`front/src/store/dissolve/quota.ts`（pinia setup store，id: `'dissolve-quota'`）

### 2.1 核心类型

```typescript
// 裁撤配置
interface IDissolveConfig {
  host_apply_time: string;                   // ISO 8601，统计开始时间
  approval_limit: number;                    // 自动审批配置占比 0-100
  quota_coefficient: number;                 // 裁撤申领上限占比 1-100
  quota_offsets: IQuotaOffset[];             // 业务偏移配置
  dissolve_projects: IDissolveProjectCycle[]; // 裁撤项目周期
}

interface IDissolveProjectCycle {
  start: string;                             // 开始日期 yyyy-MM-dd
  end: string;                               // 结束日期 yyyy-MM-dd
  default: boolean;                          // 是否为当前裁撤周期（可多个）
  projects: IDissolveProject[];              // 裁撤项目列表
}

interface IDissolveProject {
  id: number;
  memo: string;
}

interface IQuotaOffset {
  bk_biz_id: number;
  type: 'increase' | 'decrease';
  offset: number;
  memo: string;
}

// 裁撤总览行数据
interface IDissolveOverview {
  bk_biz_id: number;                         // 汇总行为 -1
  bk_biz_name?: string;                      // 前端映射字段
  progress: string | number;                 // API 返回 "100.00%"，前端转数字
  progress_str?: string;                     // 保留字符串格式显示
  origin_host_count?: number;
  origin_cpu_core?: number;
  current_host_count?: number;
  current_cpu_core?: number;
  delivered_cpu_core?: number;
}

// 裁撤明细行数据
interface IDissolveDetail {
  id: string;
  asset_id: string;
  inner_ip: string;
  module: string;
  status: 'complete' | 'incomplete';
  project_id: number;
  project_name: string;
  region: string;
  bk_biz_id: number;
  group_id: number;
  operators: string[];
  cpu_core: number;
}

// 项目类型选项
interface IDissolveProjectType {
  id: number;
  projectName: string;
  projectType: string;
}
```

### 2.2 API 方法

| 方法 | 方法+路径 | 说明 |
|------|----------|------|
| `getDissolveConfig()` | GET `/api/v1/woa/dissolve/config` | 获取裁撤配置 |
| `upsertDissolveConfig(data)` | PUT `/api/v1/woa/dissolve/config/upsert` | 保存裁撤配置 |
| `getOverviewList(params)` | POST `/api/v1/woa/dissolve/table/list` | 总览列表（无分页） |
| `getDetailList(params)` | POST `/api/v1/woa/dissolve/host/detail/list` | 明细列表（分页，双请求拆 count） |
| `syncDissolve()` | POST `/api/v1/woa/dissolve/recycled_host/sync` | 同步主机 |
| `getProjectTypes()` | GET `/api/v1/woa/dissolve/projects` | 项目类型下拉选项 |
| `getIdcNames(params)` | POST `/api/v1/woa/dissolve/idc_names/list` | 机房名称下拉选项 |
| `getCpuCoreSummary(bizId, params)` | POST `/api/v1/woa/{biz}/dissolve/cpu_core/summary` | CPU 核数汇总 |

### 2.3 关键实现细节

- **明细分页双请求**：`enableCount(params, false)` 获取列表，`enableCount(params, true)` 获取 count，用 `Promise.all` 并发
- **项目类型格式化**：`getProjectTypes()` 内部将 `{ id, projectName, projectType }` map 为 `{ value, label }` 格式
- **所有方法内部 try/catch + loading 状态管理**

## 3. index.vue（页面入口）

文件：`front/src/views/server-dissolve/index.vue`

### 3.1 Tab 切换机制

使用 URL `query.tab` 双向绑定，通过 `computed get/set`：

```typescript
const activeTab = computed<string>({
  get() {
    const tab = route.query.tab as string;
    if (tab === 'overview' || tab === 'detail') return tab;
    return 'overview';                       // 默认总览 Tab
  },
  set(value: string) {
    const query = { ...route.query, tab: value } as LocationQueryRaw;
    delete query.sort;                       // 切换 Tab 时清除排序参数
    delete query.order;
    routerAction.redirect({ query }, { replace: true });
  },
});
```

### 3.2 Data 共享机制

通过 `provide/inject` 向整个组件树共享三个响应式数据：

| 注入 key | 类型 | 来源 |
|----------|------|------|
| `dissolveProjects` | `Ref<IDissolveProjectCycle[]>` | `getDissolveConfig().dissolve_projects` |
| `projectTypeList` | `Ref<{value: number; label: string}[]>` | `getProjectTypes()` |
| `fetchDissolveConfig` | `() => Promise<void>` | 刷新配置函数，子组件在操作成功后调用 |

### 3.3 顶栏按钮

使用 `Teleport` 将按钮投射到 `#breadcrumbExtra` 插槽（面包屑右侧区域）：

```vue
<Teleport defer to="#breadcrumbExtra">
  <hcm-auth :sign="{ type: AUTH_UPDATE_DISSOLVE }" v-slot="{ noPerm }">
    <template v-if="!noPerm">
      <bk-button @click="syncDialogVisible = true">同步</bk-button>
      <bk-button @click="configDialogVisible = true">裁撤配置</bk-button>
    </template>
  </hcm-auth>
</Teleport>
```

### 3.4 Tab 渲染

```vue
<bk-tab v-model:active="activeTab" type="unborder-card">
  <bk-tab-panel
    v-for="tab in tabComps"
    render-directive="if"                   <!-- 切换时销毁/重建，而非 v-show -->
    :key="tab.name" :label="tab.label" :name="tab.name"
  >
    <component :is="tab.component" />
  </bk-tab-panel>
</bk-tab>
```

使用 `render-directive="if"` 确保切换 Tab 时销毁/重建组件（`v-if` 语义），而非隐藏（`v-show`）。

### 3.5 配置刷新

```typescript
const handleSyncSuccess = () => {
  fetchDissolveConfig();                    // 同步后刷新时间段选项
  fetchProjectTypes();                      // 同步后刷新项目类型
};
// config 同样在 success 回调中刷新
```

## 4. 裁撤总览 Tab

### 4.1 搜索条件（overview/search/condition.ts）

```typescript
@Model('dissolve-overview/search-condition')
export class SearchCondition {
  @Column('string', { name: '裁撤时间段', index: 0, meta: { search: { op: QueryRuleOPEnum.IN } } })
  time_periods: string[];

  @Column('number', { name: '项目类型', index: 1, meta: { search: { op: QueryRuleOPEnum.IN } } })
  project_ids: number[];

  @Column('number', { name: '组织', index: 2, meta: { search: { op: QueryRuleOPEnum.IN } } })
  group_ids: number[];

  @Column('business', { name: '业务名称', index: 3 })
  bk_biz_ids: number[];

  @Column('user', { name: '负责人', index: 4 })
  operators: string[];

  @Column('region', { name: '地域', index: 5, props: { vendor: VendorEnum.ZIYAN, multiple: true } })
  regions: string[];
}
```

**注意**：搜索条件中 `time_periods` 仅为前端内部使用（驱动时间段选择器），`fetchList()` 中已解构排除此字段（`const { time_periods, ...cond } = ...`），不传给接口。

### 4.2 表格列（overview/data-list/column.ts）

```typescript
@Model('dissolve-overview/table-column')
export class TableColumn {
  @Column('business', { name: '业务名称', fixed: 'left', index: 0 })
  bk_biz_id: number;

  @Column('string', { name: '裁撤进度', fixed: 'left', sort: true, index: 1 })
  progress: string;

  // 以下两列使用 group 实现多级表头：
  @Column('number', { name: '裁撤设备数', group: '当前进度', sort: true, index: 2 })
  current_host_count: number;

  @Column('number', { name: '裁撤CPU总核数', group: '当前进度', sort: true, index: 3 })
  current_cpu_core: number;

  @Column('number', { name: '已申领CPU总核数', sort: true, index: 4 })
  delivered_cpu_core: number;

  @Column('number', { name: '裁撤设备数', group: '原计划', sort: true, index: 5 })
  origin_host_count: number;

  @Column('number', { name: '裁撤CPU总核数', group: '原计划', sort: true, index: 6 })
  origin_cpu_core: number;
}
```

`groupedColumns` computed 在同 index.vue 中：连续同 `group` 的列折叠为 `bk-table-column` 嵌套结构（父列 label=group 名，子列为 children）。

### 4.3 数据映射（overview/index.vue）

```typescript
// API 返回的 progress 是 "100.00%" 字符串，前端映射为数字 + 保留字符串
list.value = (res?.items || []).map((item) => ({
  ...item,
  progress: parseFloat(String(item.progress || '0.00%')) || 0,  // 数字
  progress_str: String(item.progress || '0.00%'),               // 字符串
}));
```

### 4.4 汇总行处理（overview/data-list/data-list.vue）

- **标识**：`bk_biz_id === -1`
- **样式**：`.summary-cell` 类 → `background-color: #fdf4e8`（浅橙背景）+ 加粗字体
- **内容**：业务名称列显示 `hcm-icon bkhcm-icon-vector` + 「汇总」文字，不可点击
- **排序**：通过 `sortedList` computed 始终将汇总行移至第一位
- **禁用 bk-table 自动排序**：`sort: { sortFn: () => 0 }`

### 4.5 进度条渲染

```vue
<div v-if="col.id === 'progress'" class="progress-cell">
  <span class="progress-text">{{ row.progress_str }}</span>
  <bk-progress color="#699DF4" size="small" :percent="row.progress" :show-text="false" />
</div>
```

### 4.6 业务名称点击跳转

```typescript
const handleBizClick = (row: IDissolveOverview) => {
  const filter = searchQs.build({ bk_biz_ids: [row.bk_biz_id] });
  routerAction.redirect({ query: { ...route.query, filter, tab: 'detail' } }, { replace: true });
};
```

将当前总览的 `bk_biz_ids` 写入 URL filter，切换 Tab 到 detail 后由明细的 `useSearchQs` 解析并触发查询。

### 4.7 本地排序

```typescript
// 不通过路由触发，避免重新请求接口（总览无分页）
const handleLocalSort = ({ column, type }) => {
  if (type === 'null') {
    sortField.value = undefined;
    sortOrder.value = undefined;
  } else {
    sortField.value = column?.field;
    sortOrder.value = type as 'asc' | 'desc';
  }
};
```

`@column-sort` 事件 + 自定义排序，不修改 URL query。

## 5. 裁撤明细 Tab

### 5.1 搜索条件（detail/search/condition.ts）

```typescript
@Model('dissolve-detail/search-condition')
export class SearchCondition {
  @Column('string', { name: '裁撤时间段', index: 0, meta: { search: { op: QueryRuleOPEnum.IN } } })
  time_periods: string[];

  @Column('number', { name: '项目类型', index: 1, meta: { search: { op: QueryRuleOPEnum.IN } } })
  project_ids: number[];

  @Column('enum', { name: '裁撤状态', option: { complete: '已裁撤', incomplete: '未裁撤' }, index: 2,
    props: { multiple: false },
    meta: { search: { op: QueryRuleOPEnum.EQ } } })
  status: string;

  @Column('number', { name: '组织', index: 2, meta: { search: { op: QueryRuleOPEnum.IN } } })
  group_ids: number[];

  @Column('business', { name: '业务名称', index: 4 })
  bk_biz_ids: number[];

  @Column('user', { name: '负责人', index: 5 })
  operators: string[];

  @Column('string', { name: '内网IP', index: 6, meta: { search: { op: QueryRuleOPEnum.IN } } })
  inner_ips: string;

  @Column('string', { name: '固资号', index: 7, meta: { search: { op: QueryRuleOPEnum.IN } } })
  asset_ids: string;
}
```

**注意**：`status` 使用 `enum` 类型 + `multiple: false`（单选），与 API 的 `status` 字段（字符串 `'complete'`/`'incomplete'`）对应。内网IP 和固资号使用 `QueryRuleOPEnum.IN`（支持批量输入）。

### 5.2 表格列（detail/data-list/column.ts）

```typescript
@Model('dissolve-detail/table-column')
export class TableColumn {
  @Column('string', { name: '内网IP', minWidth: 120, index: 0 })
  inner_ip: string;

  @Column('string', { name: '固资号', minWidth: 120, index: 1 })
  asset_id: string;

  @Column('array', { name: '负责人', minWidth: 100, index: 2 })
  operators: string[];

  @Column('business', { name: '业务名称', minWidth: 100, index: 3 })
  bk_biz_id: number;

  @Column('region', { name: '地域', minWidth: 100, index: 4 })
  region: string;

  @Column('string', { name: '所属业务模块', minWidth: 120, index: 5 })
  module: string;

  @Column('string', { name: '项目类型', minWidth: 120, index: 6 })
  project_name: string;

  @Column('enum', { name: '裁撤状态',
    option: { complete: '已裁撤', incomplete: '未裁撤' },
    minWidth: 100, index: 7,
    meta: { display: { appearance: 'status' } } })
  status: string;
}
```

**注意**：状态列使用 `appearance: 'status'` 触发 `display-value` 的 status 渲染模式（圆点图标 + 文本）。列中没有「操作」列（PRD 已移除），代码中保留了注释掉的操作列模板以备后续扩展。

### 5.3 分页查询（detail/index.vue）

```typescript
const fetchList = async (searchCondition?: Record<string, any>) => {
  const { time_periods, ...cond } = searchCondition ?? condition.value;
  const sort = (route.query.sort as string) || 'inner_ip,id';
  const order = (route.query.order as string) || 'ASC';
  const page = getPageParams(pagination, { sort, order });

  const apiParams: Record<string, any> = { page, ...cond };
  const { list: dataList, count } = await store.getDetailList(apiParams as any);

  list.value = dataList || [];
  pagination.count = count || 0;
};
```

- 使用 `usePage().getPageParams()` 生成 `IPageQuery`
- store 内部双请求拆 count（`enableCount(params, false/true)`）
- 搜索条件中的 `time_periods` 被排除（不传给接口）

### 5.4 批量复制

```typescript
// 使用封装好的 CopyToClipboard 组件
const selectedInnerIps = computed(() =>
  selections.value.map(row => row.inner_ip).filter(Boolean).join('\n')
);
const selectedAssetIds = computed(() =>
  selections.value.map(row => row.asset_id).filter(Boolean).join('\n')
);
```

```vue
<hcm-dropdown :disabled="!selections.length">
  批量复制
  <template #menus>
    <copy-to-clipboard type="dropdown-item" text="内网IP" :content="selectedInnerIps" error-msg="没有可复制内容" />
    <copy-to-clipboard type="dropdown-item" text="固资号" :content="selectedAssetIds" error-msg="没有可复制内容" />
  </template>
</hcm-dropdown>
```

使用项目已有的 `CopyToClipboard` 组件 + `HcmDropdown` 组件。

### 5.5 导出

实现于 `detail/data-list/data-list.vue`：

```typescript
// 导出列 = 基础列 + 条件性 extension 列
const exportColumns = computed(() => {
  const base = props.columns.map(col => ({ label: col.name, field: col.id })) as ExportColumn[];
  if (exportExtraParams.value.includeUtilization) {
    return [...base, ...EXTENSION_EXPORT_COLUMNS];
  }
  return base;
});

// 导出请求：使用 rollRequest 分页拉取全部数据
const exportAllRequest = async (signal: AbortSignal, extraParams?: Record<string, any>) => {
  const params: Record<string, any> = {};
  // ... 从 condition 提取搜索参数 ...
  if (extraParams?.includeUtilization && extraParams?.utilizationDate) {
    params.snapshot_date = (extraParams.utilizationDate as string).replace(/\//g, '');
  }

  return await rollRequest({
    httpClient: http,
    pageEnableCountKey: 'count',
  }).rollReqUseTotalCount(
    '/api/v1/woa/dissolve/host/detail/export/list',
    params,
    { limit: 5000, total: props.pagination.count,
      listGetter: (res) => res.data.details,
      countGetter: (res) => res.data.count },
    { signal },
  );
};
```

```vue
<ExportToExcelBatchButton
  use-custom-dialog
  v-model:extra-params="exportExtraParams"
  :confirm-disabled="isExportConfirmDisabled"
  :request="exportAllRequest"
  :columns="exportColumns"
  filename="裁撤明细信息"
  text="导出"
  name="裁撤主机"
>
  <template #dialog-content>
    <ExportDissolveDialog v-model:params="exportExtraParams" :total-count="pagination.count" />
  </template>
</ExportToExcelBatchButton>
```

使用项目已有的 `ExportToExcelBatchButton` 组件 + `rollRequest` 滚动分页。

**export-column.ts**（22 个 extension 字段）：

| 字段 | 路径 |
|------|------|
| 公网IP | `extension.outer_ip` |
| SCM设备类型 | `extension.device_type` |
| 裁撤模块名称 | `extension.module_name` |
| 存放机房管理单元 | `extension.idc_unit_name` |
| 操作系统 | `extension.sfw_name_version` |
| 上架时间 | `extension.go_up_date` |
| RAID结构 | `extension.raid_name` |
| 逻辑区域 | `extension.logic_area` |
| 设备技术分类 | `extension.device_layer` |
| CPU得分 | `extension.cpu_score` |
| 内存得分 | `extension.mem_score` |
| 内网流量得分 | `extension.inner_net_traffic_score` |
| 磁盘IO得分 | `extension.disk_io_score` |
| 磁盘IO使用率得分 | `extension.disk_util_score` |
| 是否达标 | `extension.is_pass` |
| 内存使用量(G) | `extension.mem4linux` |
| 内网流量(Mb/s) | `extension.inner_net_traffic` |
| 外网流量(Mb/s) | `extension.outer_net_traffic` |
| 磁盘IO(Blocks/s) | `extension.disk_io` |
| 磁盘IO使用率 | `extension.disk_util` |
| 磁盘总量(G) | `extension.disk_total` |
| 运维小组 | `extension.group_name` |
| 业务中心 | `extension.center` |

## 6. 时间选择器（time-period-selector）

文件：`components/time-period-selector/index.vue`

### 6.1 实现方式

- 使用 `bk-popover` + `bk-tag-input` 组合
- Tag Input 作为触发器，显示已选时间段
- Popover 面板：全选 checkbox + 选项列表 + 底部已选计数 + 确定/取消按钮

### 6.2 核心逻辑

```typescript
// 选项值：`${p.start}~${p.end}`
const optionValue = (p: IDissolveProjectCycle) => `${p.start}~${p.end}`;

// 选项标签：`${p.start} 至 ${p.end}`
const optionLabel = (p: IDissolveProjectCycle) => `${p.start} 至 ${p.end}`;

// 默认选中：default === true 的周期
watch(() => props.list, (projects) => {
  const defaultSelected = projects.filter(p => p.default).map(optionValue);
  model.value = defaultSelected;
  emit('set-default', defaultSelected);
}, { immediate: true, deep: true });
```

### 6.3 Props & Emits

```typescript
const props = defineProps<{
  list?: IDissolveProjectCycle[];            // 从 inject('dissolveProjects') 获取
}>();

const model = defineModel<string[]>({ default: () => [] });

const emit = defineEmits<{
  'set-default': [value: string[]];          // 通知父组件默认选中值
}>();
```

## 7. 同步弹窗（sync-dialog）

文件：`components/sync-dialog/index.vue`

### 7.1 数据来源

通过 `inject` 获取父组件的 `dissolveProjects` 和 `projectTypeList`，汇总所有时间段内的项目类型：

```typescript
const projectTypes = computed(() => {
  const merged = new Map<number, { value: string; label: string }>();
  dissolveProjects.value.forEach(p => {
    (p.projects || []).forEach(proj => {
      const projectType = projectTypeList.value.find(pt => pt.value === proj.id);
      merged.set(proj.id, {
        value: String(proj.id),
        label: projectType?.label || proj.memo || String(proj.id),
      });
    });
  });
  return [...merged.values()];
});
```

### 7.2 展示方式

- 不使用 checkbox（PRD 变更为只读展示），直接使用灰色背景块列表
- 空状态显示「暂无项目类型」

### 7.3 操作

```typescript
const handleSync = async () => {
  await store.syncDissolve();                // POST /api/v1/woa/dissolve/recycled_host/sync
  Message({ theme: 'success', message: '同步成功' });
  isShow.value = false;
  emit('success');                           // 父组件刷新数据
};
```

## 8. 裁撤配置弹窗（config-dialog）

文件：`components/config-dialog/index.vue`

### 8.1 容器类型

使用 `bk-sideslider`（侧滑面板），宽度 960px，`background-color="#f5f7fa"`。

### 8.2 布局结构

三个 `Panel` 区域纵向排列：

1. **基础配置**：三列横排（`display: flex; gap: 32px`）
   - 申领统计开始时间（hcm-form-datetime + 修改提示 warning）
   - 自动审批配置占比（bk-input number + % 后缀 + InfoLine tooltip）
   - 裁撤申领上限占比（bk-input number + % 后缀 + InfoLine tooltip）

2. **裁撤时间段与项目类型**：动态列表
   - 每个 `TimePeriodBlock` 组件渲染一个时间段卡片
   - 「新增时间段」按钮追加空白时间段块
   - 每个卡片头部：时间段名称 +「当前裁撤时间」checkbox + 删除按钮
   - 每个卡片内部：Ediatable 表格（裁撤时间 / 项目类型 / 备注 / 操作）

3. **裁撤额度调整**：复用 `children/quota-offset/index.vue`
   - Ediatable 表格 + 行内编辑
   - 支持新增/删除行、业务校验（不允许重复）

### 8.3 校验逻辑

```typescript
const handleValidate = async (hint = true): Promise<boolean> => {
  // 1. 检查裁撤额度调整是否重复业务
  const { duplicateBizIds } = getDuplicateBizIds();
  
  // 2. 基础表单校验 + 额度调整校验 + 时间段块校验
  const [formValid, quotaOffsetValid] = await Promise.all([
    formRef.value?.validate(),
    quotaOffsetTableRef.value?.validate?.(),
  ]);
  if (timePeriodBlockRefs.value.length > 0) {
    await Promise.all(timePeriodBlockRefs.value.map(block => block?.getValue()));
  }

  return !!formValid && !!quotaOffsetValid && duplicateBizIds.size === 0;
};
```

### 8.4 重置功能

打开弹窗时深拷贝记录原始数据到 `originalData`，点击「重置」恢复原始值。

### 8.5 申领开始时间变更提示

```typescript
const isHostApplyTimeChanged = computed(() =>
  timeUTCFormatter(formData.host_apply_time) !== timeUTCFormatter(originalData.value?.host_apply_time)
);
```

当时间改变时，展示红色提示：「请关注，裁撤周期变化，裁撤基数有 N 个业务调整了额度」。

### 8.6 底部按钮

「保存配置」（primary，loading 状态）+「重置」+「取消」。

## 9. 时间段卡片组件（time-period-block）

文件：`components/config-dialog/time-period-block.vue`

### 9.1 实现方式

使用 `@blueking/ediatable` 的 `Ediatable` 组件 + `HeadColumn` / `DateTimePickerColumn` / `SelectColumn` / `InputColumn` / `OperationColumn`。

### 9.2 表格列

| 列 | 组件 | 绑定 |
|----|------|------|
| 裁撤时间 | `DateTimePickerColumn` type="daterange" | `dateRange`（双向同步到 `model.start/end`） |
| 项目类型 | `SelectColumn` + 校验 `id > 0` | `project.id` |
| 备注 | `InputColumn` | `project.memo` |
| 操作 | `OperationColumn` show-add + removeable | 新增/删除行 |

### 9.3 日期范围特殊处理

- `dateRange` 为本地 ref `[string, string]`，通过 watch 双向同步到 `model.start/end`
- 使用 `rowspan` 让日期选择器跨所有行（仅第一行显示）

### 9.4 校验

```typescript
const getValue = async () => {
  // 日期范围校验 + 每行项目类型校验
  await Promise.all(allRefs);
  return { ...model.value };
};
```

## 10. 组织树选择器（org-tree-selector）

文件：`components/org-tree-selector/index.vue`

### 10.1 实现方式

封装 `@/components/tree-selector/index.vue`（支持搜索、多选的组织架构树选择器）。

### 10.2 数据流

```typescript
// 正向：树选中 → 提取叶子节点 tof_dept_id → model
const groupIds = computed(() => {
  const leafIds = new Set<number>();
  const collectLeafCodes = (dept: ITreeItem) => {
    if (!dept?.has_children && dept?.tof_dept_id) leafIds.add(dept.tof_dept_id);
    if (dept?.has_children && dept?.children) dept.children.forEach(collectLeafCodes);
  };
  orgChecked.value.forEach(org => collectLeafCodes(org));
  return [...leafIds];
});

// 反向：model 外部赋值 → 查找叶子节点 → 恢复 orgChecked
```

### 10.3 数据源

`POST /api/v1/woa/metas/org_topos/list`，参数 `{ view: 'ieg' }`。

## 11. 裁撤额度调整表格（quota-offset）

文件：`children/quota-offset/index.vue` + `render-row.vue`

### 11.1 实现方式

使用 `@blueking/ediatable` 的可编辑表格。

### 11.2 表格列

| 列 | 类型 | 校验 |
|----|------|------|
| 业务 | `BusinessSelectorColumn` | 必填，不允许重复 |
| 调整类型 | `SelectColumn`（调增/调减） | 必填 |
| CPU核数 | `InputColumn`（number） | 必填 |
| 备注 | `InputColumn` | 选填 |

### 11.3 对外接口

```typescript
defineExpose({
  validate,                                  // 校验并返回结果
  addRow,                                    // 新增行
});
```

## 12. 与旧 coding.md 的关键差异总结

| 项目 | 旧 coding.md（假设） | 实际实现 |
|------|---------------------|---------|
| 目录 | `views/ziyanScr/recycle-server-room/` | `views/server-dissolve/` |
| model/ 目录 | 有 `properties.ts`, `overview.view.ts`, `detail.view.ts` 等 | **无**，condition/column 放在各自 `search/` `data-list/` 下 |
| hooks/ 目录 | 有 `use-dissolve-tabs.ts`, `use-dissolve-options.ts` | **无**，Tab 逻辑内联在 index.vue |
| typings.ts | 有独立文件 | **无**，类型在 `store/dissolve/quota.ts` |
| 配置弹窗 | `bk-dialog` width 900 | **`bk-sideslider`** width 960 |
| Tab 切换 | `useDissolveTabs` composable + `shallowRef` | URL `query.tab` + `computed get/set` + `routerAction` |
| Data 共享 | `useDissolveTabs` 共享状态 | **`provide/inject`** 三个 key |
| 汇总行 | `bk_biz_id === "total"` | **`bk_biz_id === -1`** |
| 状态枚举 | `pending/running/done` | **`incomplete/complete`** |
| 配置字段名 | `time_periods`, `is_current`, `project_types` | **`dissolve_projects`**, **`default`**, **`projects`** |
| 导出 | 未实现 | 完整实现（ExportToExcelBatchButton + rollRequest + extension 列） |
| 组织选择 | 直接使用 TreeSelector | 封装 **`org-tree-selector`**（叶子 tof_dept_id 提取） |
| 时间段选择器 | bk-popover + checkbox-group + tag 手动渲染 | **`bk-tag-input`** + **`bk-popover`** 自定义面板 |
| 时间段块 | bk-checkbox + bk-date-picker + bk-select | **`@blueking/ediatable`** 可编辑表格 |
| API 路径 | `/api/v1/woa/dissolve/overview/list` 等 | 全部匹配真实 API 路径 |
| 装饰器导入 | `@/decorator/columns/column` | **`@/decorator`** |
| 顶栏按钮 | 页面内标题行 | **`Teleport` 到 `#breadcrumbExtra`** |
| 配置校验 | `validateTimeOverlap()`（时间重叠） | **`getDuplicateBizIds()`**（业务重复）+ 三路 `Promise.all` |

## 13. 遵守的 HCM 编码规范

| 规范 | 状态 |
|------|------|
| Vue SFC 结构：script → template → style | ✅ |
| 显式导入组件 PascalCase，全局组件 kebab-case | ✅ |
| CSS kebab-case，无 BEM，无工具类 class | ✅ |
| 路由跳转使用 `routerAction` | ✅ |
| 路由 name 使用符号常量 | ✅ |
| 不修改 @deprecated 文件 | ✅ |
| 禁止 `useFormModel` hook | ✅（使用 `reactive(initialState)`） |
| 禁止 `notMenu`/`isShowBreadcrumb`/`icon` 废弃 meta | ✅ |
| 表单权限使用 `<hcm-auth>` | ✅ |
| API 相对路径 | ✅ |
| 文件命名 kebab-case | ✅ |
| `<script setup lang="ts">` | ✅ |
| `defineModel<T>()` | ✅ |
| `useTemplateRef<T>()` | ✅ |

## 14. 已知的待办（TODO）

- 明细表操作列（申领/回收按钮）已注释，后续迭代恢复
- 视图权限 `meta.auth.view` 待后续接入路由守卫
- 內网IP/固资号批量输入的前端 split + trim 处理逻辑（当前由后端处理）
