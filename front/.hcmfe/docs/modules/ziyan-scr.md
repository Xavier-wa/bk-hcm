# 自研上云（退场中）

> status: drafted · kind: module
> globs: `src/views/ziyanScr/**`

状态: 退场中。历史组织形式，内部混杂 cvm/主机申请/回收/滚动资源等。原则: 只迁不增，新需求就近迁往对应原子模块。

## 职责

目录内按功能分片，彼此独立、共用 `src/api` 与 `useWhereAmI`：

- `hostApplication/**`：主机申请单据（列表、详情、待匹配面板）。
- `host-recycle/**`：主机回收单据（预检详情 `pre-details`、单据详情 `bill-detail`、执行记录 `execute-record`）。
- `resource-manage/**`、`rolling-server/**`：见 [base](base.md) 的逻辑特性映射表，已有对应原子模块承接。
- `cvm-model/**`：CVM 机型配置管理（列表 + 创建弹窗）。

## 关键流程 / 注意事项

### 单据页面的业务归属：页面业务而非全局业务

单据详情类页面（`hostApplication/components/application-detail`、`host-recycle/bill-detail`）由外链或列表跳转进入，URL 上同时带两个业务：

- `bizs`：顶部业务选择器的全局业务，随用户操作变化；
- `bkBizId`（`PAGE_BIZ_KEY`）：**单据所属业务**，是这条数据的固有身份。

**这些页面的全部接口都必须用 `bkBizId` 拼路径**，即 `getBusinessApiPath(orderBizId)`，而不是不带参的 `getBusinessApiPath()`。二者不一致时用后者会把请求打到错误业务下（`/api/v1/woa/bizs/<错误业务>/task/audit/apply/ticket`），审批、撤单等写操作会失败或越权。

约定的写法是在页面顶层算一次、下发给所有调用点：

```ts
// 存量入口可能不带 bkBizId，回退全局业务以保持既有行为；
// 这里必须用 || 而不是 ??，Number(undefined) 是 NaN，?? 兜不住
const orderBizId = computed(() => Number(route.query[PAGE_BIZ_KEY]) || getBizsId());
const getOrderApiPath = () => getBusinessApiPath(orderBizId.value);
```

`orderBizId` 同时用于**请求体**里的 `bk_biz_id`，所以回退要放在这一层，保证路径业务与请求体业务同源；`getBusinessApiPath` 内部另有一道有效性兜底，属于双保险。

**子组件同样要覆盖**——`itsm-ticket-audit.vue`（审批提交）、`modify-record.tsx`（变更记录）、`execute-record/index.vue`（回收执行记录）都各自发请求，历史上就是漏了子组件导致主流程仍打错业务。子组件优先从 props 拿业务（如 `props.showObj.bkBizId`、`props.dataInfo.bk_biz_id`），拿不到时再从路由读。

反例边界：`match-panel/**` 由单据**列表页**和工作台匹配页使用，那里全局业务与页面业务本就一致，继续用无参 `getBusinessApiPath()` 是对的。判断标准是"页面是否声明了自己的 `bkBizId`"，不是"是否在 ziyanScr 下"。

另外注意请求体里的业务字段（`bk_biz_id`）要与路径业务同源，不要一个取 `bkBizId`、另一个取全局业务。

### CVM 机型配置管理

- 列表页 + 「创建新机型」弹窗（`CreateDevice/index.tsx`）。
- 创建表单字段：地域/园区/实例族/机型/机型类型/技术分类/机型分类/核心类型/机型代次/CPU/内存/GPU卡数/GPU卡类型/技术分类资源量。
- GPU卡类型（`gpu_type`）支持默认枚举 + 自定义输入，枚举来自 `GET /api/v1/woa/meta/gpu_type/list`（`src/api/scrApi/index.tsx` 的 `getGpuTypeList`）。
- 技术分类资源量（`tech_class_res_amt`）为数值字段（>=0）。
- 创建接口 `POST /api/v1/woa/config/createmany/config/cvm/device`，`device_types` 数组元素携带 `gpu_type`、`tech_class_res_amt`。
