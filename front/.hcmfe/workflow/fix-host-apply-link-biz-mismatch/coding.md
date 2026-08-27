# coding：修复主机申请详情外链外侧业务与单据内业务不匹配

> 需求文档：`front/docs/reqs/外链业务不匹配.md`
> TAPD：[1069995598137480239](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137480239)（doing）
> 预估 20 人时 / RICE 320

## 1. 根因

三段代码串成完整链路：

1. `views/home/business-selector/index.tsx` 的 `getGlobalBizsId`：本地无 `bizs_version` 记录时（首次访问、换浏览器、清缓存、无痕窗口），版本号必然判定失效，函数直接返回 `undefined`，地址栏携带的 `bizs` 被整段跳过。
2. 同文件 `fetchBusinessList`：拿不到业务时用有权限业务列表首项兜底，并通过 `saveGlobalBizsId` 写入 store、localStorage，再 `router.push` 把该业务写回地址栏。至此地址栏的 `bizs` 已被改写。
3. `hooks/useWhereAmI.ts` 的 `getBusinessApiPath`：读 `accountStore.bizs` 拼接 `bizs/<id>/`。详情页所有接口因此打到被改写后的业务，而请求体里的 `bk_biz_id` 取自 `route.query.bkBizId`，形成同一请求内两个业务并存。

`validateBizId` 未能拦截，是因为它在 `onBeforeMount` 执行，早于业务选择器的异步初始化完成，此刻读到的仍是链接原本的业务，校验通过之后业务才被改写。

## 2. 方案决策

| 决策项 | 结论 | 依据 |
|--------|------|------|
| Q-001 业务版本号机制 | 彻底移除 `GLOBAL_BIZS_VERSION` / `GLOBAL_BIZS_VERSION_KEY` 常量与相关逻辑 | 用户确认。该机制自 MR !1376 后未再更新，恒定判定失效，已无实际作用 |
| Q-003 详情页手动切业务 | 跳回目标业务的单据列表 | 用户确认。与 MR !2223 既有语义一致，不回归 story 123868572；若留在原页，新的优先级规则会让 `bkBizId` 覆盖切换结果，表现为「切不动」 |
| 详情页接口业务来源 | 直接使用单据业务，不经全局业务 | 详情页 `onBeforeMount` 早于业务选择器异步初始化，依赖全局业务必然存在竞态；直连 `bkBizId` 使请求正确性与初始化时序解耦 |

遗留的 `bizs_version` localStorage 键不做清理：该键移除后不再被任何代码读取，无害，专门为它保留一次性清理代码反而增加负担。

## 3. 改动清单（实际落地）

### 3.1 `src/common/constant.ts`

移除 `GLOBAL_BIZS_VERSION`、`GLOBAL_BIZS_VERSION_KEY` 两个常量；新增 `PAGE_BIZ_KEY = 'bkBizId'`，用于表达「页面数据自身所属业务」这一语义，与表示用户视角状态的 `GLOBAL_BIZS_KEY` 区分，两个常量的注释同步写明各自定位。

### 3.2 `src/hooks/useWhereAmI.ts`（基础能力）

`getBusinessApiPath` 增加可选参数 `bizId`：页面数据自带业务归属时传入，缺省仍取全局业务。这样「按页面业务拼接接口路径」成为共享能力，无需每个页面各写一套；参数可选也保证了未传入的既有调用点行为不变。

参数有效性在函数内部判定（`Number()` 归一后非有限数或 ≤ 0 一律回退全局业务），而不是用 `??`。原因是调用方普遍传入 `Number(route.query.bkBizId)` 这类可能缺失的值，`Number(undefined)` 得到 `NaN`，`??` 兜不住，会拼出 `bizs/NaN/` 直接请求失败——比修复前更糟。入参类型放宽为 `number | string`，兼容从接口数据直接透传的字段。

### 3.3 `src/views/home/business-selector/index.tsx`（F-001 / F-003 / R-001 / R-004）

- `getGlobalBizsId` 重排优先级：页面业务（`bkBizId`）> 地址栏 `bizs` > 本地缓存；整段版本号判定分支删除。
- `fetchBusinessList` 保持列表首项兜底，仅在上述三层都取不到时生效。
- `handleChange`：新增「路由 query 声明了页面业务」的判定，此类页面切换业务时按 `meta.activeKey` 跳回所属列表页，并从 query 中剔除 `bkBizId`，避免新业务的列表页仍挂着原页面数据的业务声明。`activeKey` 缺失时回落原有行为。

### 3.4 单据详情页及其子组件（F-002 / R-002 / R-003）

审批请求实际由子组件发出，父组件单独改造并不足以修复，因此三个文件一并处理：

| 文件 | 改动 |
|------|------|
| `application-detail/index.tsx` | 新增 `orderBizId` 与 `getOrderApiPath`，替换页面内 5 处接口路径；`getItsmTicketAudit` 的请求体也改用 `orderBizId`；移除 `validateBizId` 及其在 `onBeforeMount` 中的跳走拦截，连带清理因此失效的 `backRoute`、`router` 及相关 import。`orderBizId` 在 `bkBizId` 缺失时回退全局业务（`Number(...) || getBizsId()`），保证路径业务与请求体业务同源 |
| `application-detail/itsm-ticket-audit.vue` | **审批接口 `task/audit/apply/ticket` 的实际发出点**，改为按单据业务拼路径 |
| `application-detail/modify-record.tsx` | 变更记录接口改用已有的 `showObj.bkBizId` 拼路径 |

移除 `validateBizId` 的原因：改造后全局业务会与单据业务对齐使该校验恒真，而在初始化竞态下它反而会把存量链接误踢回列表（违反 AC-003）。

### 3.5 F-005 同类页面排查结果

排查全仓「路由 query 携带 `bkBizId` + 用全局业务拼接接口路径」的组合，发现主机回收单据详情存在完全相同的分裂（请求体取 `route.query.bkBizId`，路径取全局业务），一并修复：

| 文件 | 改动 |
|------|------|
| `ziyanScr/host-recycle/bill-detail/index.tsx` | 新增 `orderBizId` / `getOrderApiPath`，替换 7 处接口路径，请求体同步改用 `orderBizId`；同样按 `Number(...) || getBizsId()` 回退 |
| `ziyanScr/host-recycle/execute-record/index.vue` | 改用 `props.dataInfo.bk_biz_id` 拼路径；该组件也被回收预检详情复用，那里未传业务时自动回落全局业务，行为不变 |

其余命中 `bkBizId` 的位置均为组件 props、表单字段名或列表跳转时的赋值，不涉及路径与请求体的业务分裂，无需改动。

### 3.6 顺带修复的既有 lint 错误

`itsm-ticket-audit.vue` 中局部 `reactive` 变量与 props 同名（都叫 `data`），触发 `vue/no-dupe-keys`。该问题本已存在，因本次改动该文件而暴露并阻塞 lint，故将局部变量改名为 `auditData`（script 与 template 共 9 处引用同步更新）。模板中原本解析到的就是这个局部变量，行为不变。

## 4. 不做的事

- 不迁移 `ziyanScr`（退场中结构，遵循「只迁不增」，本次为 bugfix 不借机迁移）。
- 不调整业务选择器的交互样式与业务列表排序、收藏等既有功能。
- 不动后端接口。

## 5. 自验要点

对应需求文档 AC-001 至 AC-008，重点覆盖：无痕窗口首次访问（AC-001）、本地缓存与链接业务冲突（AC-002）、存量链接 `bizs` 与 `bkBizId` 不一致（AC-003）、审批请求路径业务（AC-004）、详情页手动切业务不报错（AC-008）。

因 3.5 扩大了改动范围，回归验证需补充主机回收单据详情页（`HostRecycleDocDetail`）的查看、审批与执行记录。

## 6. 静态检查结论

- `bkdevbuddy lint --fix`：8 个变更文件全部通过（exitCode 0）。
- IDE TypeScript 诊断：无报错。
- 本地未安装前端依赖（无 `node_modules`），未能执行 `tsc` 全量类型检查与本地启服务验证，功能验证依赖 test 阶段的手工用例。
