# 基础/共享接缝

> status: drafted · kind: base
> globs: `src/api/**`, `src/assets/**`, `src/common/**`, `src/components/**`, `src/constants/**`, `src/css/**`, `src/decorator/**`, `src/directive/**`, `src/hooks/**`, `src/http/**`, `src/language/**`, `src/store/**`, `src/style/**`, `src/types/**`, `src/typings/**`, `src/utils/**`, `src/vendor/**`
>
> 注：`index.md` 由工具从 blueprint 自动生成（仅模块 TOC），架构级叙述沉淀在本文档。

跨模块共享层：HTTP 客户端、API 封装、全局组件、hooks、store、常量/类型、样式与工具函数等。非业务域，供所有 views 模块复用。

## 从 base 拆出的关注点/能力

以下横切关注点/基础能力已独立成模块，动手前优先读它们：

- **模型驱动与页面模式** → [model](model.md)：装饰器字段模型、多云 Factory、展示/表单/列表通用组件模式（系统特色能力）。
- **菜单与路由** → [menu-route](menu-route.md)：`src/router/**`、`src/constants/menu-symbol.ts`。
- **权限控制** → [auth](auth.md)：`src/common/auth-service.ts`、`src/constants/auth-symbols.ts`、`src/components/auth|permission/**`。

其中 menu-route/auth 的部分文件物理上仍落在 `common/constants/components/hooks/store/views` 下，属概念分层（glob 与 base 宽 glob 有意重叠）。命中 `src/store/common.ts`、`src/hooks/useVerify.ts` 或 `src/views/error-pages/403.tsx` 的鉴权改动，优先阅读并更新 [auth](auth.md)；命中 `src/components/chatbot/**`、`src/components/ai-assistant/**`、`src/hooks/chatbot/**`、`src/store/chatbot/**` 的业务行为优先归 [chatbot](chatbot.md)；命中 `src/store/prepaid-bill/**` 或 `src/views/prepaid-bill/**` 优先归 [bill](bill.md)，base 只记录共享层边界。

## 共享层边界（踩过的坑，动这些文件前先读）

base 的 `src/components/**` 宽 glob 会把各业务模块的组件一并命中，判断归属见上一节。真正属于共享层、且有**非直觉约束**的接缝记录在此：

- **选项类通用组件的 `list` / `list-generator` 必须传稳定函数引用**（`src/components/form/list.vue` = `hcm-form-list`，`src/components/search/list.vue` 转发到它）。组件内部用 `watchEffect` 追踪该 prop 的**函数身份**，模板里写内联箭头函数会导致任意重渲染（改同表单的无关输入框、窗口 resize）都重新拉一次接口。完整机理、正确写法与「为何不会破坏级联刷新」见 [model](model.md) 的「关键约定（务必遵守）」。

## 架构北极星（目标形态）

- **目标**：所有功能模块在 `views/` 下**打平**，每个模块**原子自洽**——无论其菜单挂在业务/资源/服务哪个入口，模块本身的代码归属唯一、边界清晰。正面样板见 [operation-log](operation-log.md)。
- **现状**：受历史组织形式影响，多数模块尚未原子化：同一功能因菜单挂载位置不同而**物理分散**（如负载均衡分布在顶层与 `business/` 下）；`ziyanScr/`、`plugin-handler/` 等为**退场中**的历史结构。
- **文档底座取舍（方案 A）**：module 划分**照现状物理结构**落，保证 glob 真实命中、导航与漂移检测可信；**原子化目标**写在各模块文档正文与下方映射表中，作为迁移方向而非当前 glob。
- **迁移原则**：退场中的结构**只迁不增**——新需求不得往里加，命中即就近迁往对应原子模块；物理结构真正合并后，重新 `docs_scan` / `docs_init` 调整蓝图。

## 一级视图容器（现状聚合，待拆解 —— 原子原则要打破的对象）

`views/business`、`views/resource`、`views/service` 对应三个一级视图（`/business/:bizId`、`/resource/`、`/service/`）。它们是**按菜单笼统归一的容器，不是原子模块**——蓝图里保留它们只为如实刻画现状物理结构，**不代表它们是合法的功能单元**。原子原则的目标就是把容器里的功能**打平**成各自独立的原子模块，容器最终只留一级视图入口 / 该视图自身的管理页。

| 容器（一级视图） | 现状聚合的功能 | 应拆出的原子模块（目标） |
|---|---|---|
| [business](business.md) 业务视图 | 业务管理自身、主机/主机清单、证书、负载均衡、资源预测、滚服 | host（主机）、cert（证书）、load-balancer、resource-plan、rolling-server；容器仅留业务管理 |
| [resource](resource.md) 资源运营 | 账号管理、回收站、资源纳管（host/vpc/subnet/安全组/cvm 等多种 IaaS 资源） | 账号管理、recyclebin（回收站）、各 IaaS 资源类型各自打平 |
| [service](service.md) 工作台 | 我的申请、我的审批、服务申请、资源预测 | apply-approval（申请审批/服务单据）；resource-plan 已独立 |

> 与下方「逻辑特性映射表」互补：映射表看的是同一功能**散落在多个容器**，本表看的是同一容器**塞了多个功能**。两者都指向同一个终点——打平的原子模块。

## 逻辑特性 → 物理落点映射表

同一逻辑特性当前散落在多个物理目录，下表为真实落点索引（目标是各自收敛为单一原子模块）：

| 逻辑特性 | 物理落点 | 归属模块文档 |
|---|---|---|
| 负载均衡 | `src/views/load-balancer/**`、`src/views/business/load-balancer/**` | [load-balancer](load-balancer.md) |
| 资源预测 | `src/views/resource-plan/**`、`src/views/business/resource-plan/**`、`src/views/service/resource-plan/**` | [resource-plan](resource-plan.md) |
| 滚服 | `src/views/rolling-server/**`、`src/views/business/rolling-server/**`、`src/views/ziyanScr/rolling-server/**` | [rolling-server](rolling-server.md) |
| 资源管理 | `src/views/resource/**`、`src/views/ziyanScr/resource-manage/**` | [resource](resource.md) |

> 映射表随迁移进展更新；物理合并完成后，从表中移除对应行并精简相关模块 glob。

## 资源综合搜索（ResourceSearchSelect）

入口：`src/components/resource-search-select/`。组件只提供条件项与异步下拉（`getOptionMenu` 拉 children），不负责 URL 与列表 filter。`option-common.ts` 按 `ResourceTypeEnum` 只登记字段和 children（与 CVM 相同，`optionMap.set`）；`type` / `filterRules` / 默认回填由各资源列表页自己挂到 `searchQs`。使用方可通过 `exclude` 按场景剔除条件（如业务视角去掉使用业务/管理业务）。主机列表另走 `useFilterHost` 的 v-model 拼装，不要默认套到安全组。

## 业务上下文接缝（`useWhereAmI` / 业务相关常量）

业务视角下的接口路径统一由 `src/hooks/useWhereAmI.ts` 的 `getBusinessApiPath(bizId?)` 拼出（非业务视角返回空串）。它有两种用法，选错会导致跨业务请求：

- **不传参**：用全局业务（`accountStore` → URL `bizs` → localStorage）。适用于业务视角下的列表页等"页面业务 == 全局业务"的场景。
- **传 `bizId`**：用页面数据自带的业务。**页面 URL 上带 `bkBizId`（`PAGE_BIZ_KEY`）时必须传**——全局业务由业务选择器异步初始化，页面挂载阶段读到的可能还是上一次的业务，存在时序竞态。详见 [app-shell](app-shell.md)（全局业务如何产生）与 [ziyan-scr](ziyan-scr.md)（单据页面的落地约定）。

函数内部对传入值做了有效性判断（非有限数或 ≤ 0 一律回退全局业务），因此调用方可以直接把 `Number(route.query.bkBizId)`、`props.xxx.bk_biz_id` 这类可能缺失的值传进来，不会拼出 `bizs/NaN/`。**注意不要在调用方用 `??` 兜底**：`Number(undefined)` 是 `NaN`，`??` 兜不住，这正是曾经踩过的坑。

相关常量都在 `src/common/constant.ts`：`GLOBAL_BIZS_KEY = 'bizs'`（全局业务，URL 与 localStorage 共用）、`PAGE_BIZ_KEY = 'bkBizId'`（页面自身声明的业务）。原先的 `GLOBAL_BIZS_VERSION` / `GLOBAL_BIZS_VERSION_KEY` 已删除，不要恢复。

## 注意事项

- `src/router/**` 已整体划入 menu-route 模块（不再属于 base）。
- 列表搜索 filter 组装在 `src/utils/search.ts`：`transformSimpleCondition` 只组外层 `and`。`hcm-search-string` 默认 tag 多值；算子要求标量 value（`cs` / `json_contains`）时，由字段 `filterRules` 调 `buildFilterRulesWithSearchSelect`（嵌套 `or` 在字段侧生成）。业务页不要再包一层 flatten。细则见 skill `comp-data-list` → `references/search-pattern.md`。
- 由 workflow 在首次改动 base 相关文件时继续深化各共享子层（api/store/components/utils 等）的职责说明。
