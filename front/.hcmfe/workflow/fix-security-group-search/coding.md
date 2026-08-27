# Coding — fix-security-group-search

**TAPD**: [#1069995598137336978](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137336978)

**文件**:
- `src/common/resource-constant.ts`
- `src/components/resource-search-select/index.vue`
- `src/components/resource-search-select/option-common.ts`
- `src/views/resource/resource-manage/children/manage/security-manage.vue`

## 根因

业务 / 资源接入共用 `security-manage.vue`。安全组综合查询仍走旧 `bk-search-select`：`名称 / 云厂商 / 云账号ID` 来自 `FILTER_DATA`，经 `useFilter.searchData` 拼进 `selectSearchData`。

`searchData` 只在 `accountStore.accountList.length > 0` 时赋值。资源接入进页会预拉账号；业务视角不会。因此直接进业务视角缺名称等条件，先逛资源再切回来则条件被 Pinia 带过去。

`ResourceSearchSelect` 只负责**条件项和下拉数据**（云账号 ID 用组件内 `get-menu-list` 按需请求）。安全组列表的选中值、URL、默认回填、filter rules、列表接口仍是另一套，不能照搬主机的 v-model → `useFilterHost`。

## 查询链路（必须保持）

当前安全组（三个 tab 共用）是 **URL `filter` 为真相源**，不是 searchValue 直接打接口：

1. 首次进入：`onMounted` 若 query 无 `filter`，`searchQs.set({ mgmt_type: [业务管理, 未确认] })` 写入 URL。
2. 用户改条件：`handleUpdate` → `searchQs.set` → 更新 query。
3. `watch(route.query)`：`searchQs.get`（按 properties 的 type 做 convertValue）→ `transformSimpleCondition`（走 `meta.search.filterRules` / `getQueryOperator`）→ `filter.value.rules`；同时 `buildSearchSelectValueBySearchQsCondition` 回填 `searchValue`（默认管理类型要能显示成 tag）。
4. 资源接入若选了云厂商/账号：同一 watch 里把 vendor / account_id 追加进 rules。
5. `useQueryCommonList` deep watch `filter` → `security_groups/list`（业务走 biz 前缀）。

主机列表是 `v-model=searchValue` → `useFilterHost` 直接拼 rules，且几乎不写 `filter` query。安全组若改成那套，会丢掉默认管理类型 URL 回填，以及名称 CS、管理类型 IN、使用业务 number 等现有 op。

因此 group tab **只换条件组件**，绑定保持 `:model-value="searchValue"` + `@update:model-value="handleUpdate"`，不改成主机那种纯 v-model。

## 执行顺序

1. 为 `ResourceTypeEnum.SECURITY_GROUP` 补 option：公共项用已有 `base`（云账号 ID 保持 `async: true`，展开时走组件 `getOptionMenu`，**不要**在安全组页预拉 `accountList`）。专有项：安全组 ID、名称、使用业务、管理类型、管理业务、地域。
2. 共用工厂只 `optionMap.set` 字段和 children。`type` / `meta.search.filterRules` / 中文 option 由安全组页挂到 `searchQs` properties（`enrichGroupSearchItem`），否则 URL 解析丢字段、默认管理类型无法回填、接口 op 会错。
3. 三个 tab 都用 `ResourceSearchSelect`；地域异步检索扩在 `getOptionMenu`（`id === 'region'`）。
4. 保留 onMounted 默认 `mgmt_type`、query watch 拼 rules / 回填 tag、资源侧栏追加 vendor/account_id。三个 tab 都不再用 `FILTER_DATA` + `accountList.length` 决定有没有名称等条件。`useFilter` 的 searchValue watch 不能再当接口拼装器（避免和 `transformSimpleCondition` 双写）。

## 改动点

- 选项工厂：`SECURITY_GROUP` / `GCP_FIREWALL` / `ARGUMENT_TEMPLATE` 只登记字段和 children；GCP 不含云厂商。
- `ResourceSearchSelect` 增加 `exclude`，使用方按场景剔除条件。
- 安全组管理页三个 tab 都用 `ResourceSearchSelect`；`searchQs` 所需 `type` / `filterRules` 留在页面；业务视角安全组 tab `exclude` 使用业务/管理业务。

不改 `FILTER_DATA` 常量，不改其它资源的 useFilter 账号列表闸门，不调用 `getResourceAccountList`。

## 验收对照

- 清空站点数据后直接进业务视角安全组列表 → 条件项含名称，**不含使用业务/管理业务**；**默认 tag 仍为「管理类型: 业务管理 | 未确认」**；列表请求 filter 含对应 mgmt_type
- 资源接入安全组列表仍有使用业务/管理业务
- 展开「云账号 ID」有可选项（`get-menu-list` 按需请求），无需先访问资源接入
- 按名称搜索 → 请求对 name 使用与现网一致的 CS/包含语义
- 先资源接入再切业务 → 两边安全组条件与默认回填仍一致
- 清空站点数据后直接进业务视角 GCP 防火墙 / 参数模板 → 条件项与资源接入对齐（含名称、云账号 ID；GCP 无云厂商）
- GCP / 参数模板按名称搜索 → 请求对 name 使用与现网一致的 CS/包含语义
