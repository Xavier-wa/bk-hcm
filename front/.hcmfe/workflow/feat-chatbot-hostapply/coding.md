# Coding 实施方案 — 主机申领 chatbot 卡片优化

> 依据 prd.md / design.md / api.md（均已审批）。本期 vendor=ZIYAN；展示层编码→名称、C 弹窗下拉用 `hcm-form-list` + `:list` 函数；不新增后端接口。

## 1. 改动文件清单

| # | 文件 | 类型 | 说明 |
|---|------|------|------|
| 1 | `src/hooks/chatbot/host-apply-display.ts` | 改 | 8 字段编码→名称（展示层，A/B/D 共用） |
| 2 | `src/hooks/chatbot/use-host-apply-options.ts` | 新增 | C 弹窗各下拉的 `:list` 选项函数 + vendor 参数化（数据/接口层复用） |
| 3 | `src/components/chatbot/host-apply-adjust-dialog.vue` | 改 | 手输/静态枚举 → `hcm-form-list` 下拉；补 zone；修默认磁盘类型 |
| 4 | `src/components/chatbot/host-apply-preorder-table.vue` | 改 | region/zone/image/require_type/磁盘列改走映射后的展示值 |

> 原计划的 bizId 透传（chat-message-list / 卡片）已取消：机型改用单纯目录接口后不再需要 bizId。

> A 卡 `host-apply-recommend-card.vue` 走 `toSpecDisplayItems`→`getSpecFieldText`，改 #1 后自动生效，无需单独改。

## 2. 展示层映射（#1 host-apply-display.ts）

复用与 `use-scr-columns.tsx` `CHColumns` 同源的 ziyan 工具（均为「import 即自拉取」的模块单例，模板内调用可响应式更新）：

| 字段 | 改为 | 来源 |
|------|------|------|
| `region` | `getRegionCn(val)` | `@/views/ziyanScr/cvm-web/transform` |
| `zone` | `val==='all' ? '全部可用区' : getZoneCn(val)` | 同上 |
| `image_id` | `getImageName(val)` | `@/views/ziyanScr/cvm-produce/component/property-display/transform` |
| `charge_type` | `ChargeTypeMap[val] ?? String(val)` | `@/typings/plan` |
| `require_type` | **组件渲染** `<req-type-value :value>`（不走字符串映射） | `@/components/display-value/req-type-value.vue`（Q3） |
| `res_assign` | `RES_ASSIGN_TYPE[val]?.label ?? String(val)` | `@/components/device-type-selector/constants` |
| `disk_type`（磁盘内） | `getDiskTypesName(val)` | `@/views/ziyanScr/cvm-produce/component/property-display/transform`（diskType） |
| `device_type` | 原样（不映射） | — |

- `getSpecFieldText` 的 `default` 分支按字段 `switch` 分流到上述映射；查不到一律回退原编码（各 transform/Map 已内置 fallback）。
- `formatDisk`/`formatDataDisks` 把 `disk_type` 经 `getDiskTypesName` 映射，容量数字保留。
- **require_type 不进字符串映射**：从 `SPEC_FIELD_ORDER` 移除，改由卡片/表格模板内联 `<req-type-value :value="row.require_type" />`（Q3，复用现成组件，组件内已 `useConfigRequirementStore` 拉取并缓存）。A 键值卡 require_type 行单独渲染该组件（保持原排序首位）。
- 其余字段为「编码兜底 + 异步替换」：列表未加载完先显示编码，加载后响应式刷新，不报错/不空白（PRD AC-2）。

## 3. C 弹窗下拉（#2 + #3）

### 3.1 新增 `use-host-apply-options.ts`
导出按 vendor 参数化的选项函数，**统一归一化为 `Array<{ id; name }>`**，弹窗里 `hcm-form-list` 配 `:id-key="'id'" :display-key="'name'"`（Q2，透传给底层 `bk-select`，参考 `req-stage.vue:43-44`）：

| 函数 | 数据源 | 入参 |
|------|--------|------|
| `getRegionOptions(vendor)` | `getRegions('qcloud')`（`@/api/host/config-management`，仅 qcloud，不含 idc）→ `{ id: region, name: region_cn }` | vendor |
| `getDeviceTypeOptions({ vendor, region })` | `useCvmDeviceStore().getDeviceTypeFullList`（单纯机型目录接口 `/findmany/config/cvm/devicetype`，**非**计费/预测接口），filter `vendor=ZIYAN` + `region` → `{ id: device_type, name: device_type }` 去重 | vendor/region |
| `getImageOptions(region)` | `getImages({ region: region?[region]:[] })`（`@/api/host/cvm`）→ `{ id: image_id, name: image_name }` | region |
| `getRequireTypeOptions()` | `useConfigRequirementStore().getRequirementType()` → `{ id: require_type, name: require_name }` | — |
| `getResAssignOptions()` | `RES_ASSIGN_TYPE` 转数组 | — |
| `getDiskTypeOptions()` | `getDiskTypes()`（`@/api/host/cvm`）→ `{ id: disk_type, name: disk_name }` | — |

- 不含可用区下拉（C 弹窗不编辑 zone）；机型不使用 `getChargeTypeDeviceTypeList`（其与计费模式/预测绑定），改用单纯机型列表接口。
- vendor 本期固定 `VendorEnum.ZIYAN`；函数内预留 vendor 分支位（其它云返回 `[]`）。

### 3.2 改 `host-apply-adjust-dialog.vue`
- 新增 prop：`vendor?`（默认 `VendorEnum.ZIYAN`）。`hcm-form-list` 配 `:id-key="'id'" :display-key="'name'"`。（不需要 bizId，机型已改用单纯目录接口。）
- 控件替换（`bk-input`/静态 `bk-select` → `hcm-form-list`，`v-model` 绑 `formModel.*`）：
  - `require_type`：`:list="() => getRequireTypeOptions()"`
  - `region`：`:list="() => getRegionOptions(vendor)"`
  - `device_type`：`:list="() => getDeviceTypeOptions({ vendor, region: formModel.region })"`
  - `image_id`：`:list="() => getImageOptions(formModel.region)"`
  - `res_assign`：`:list="getResAssignOptions()"`
  - 系统盘/数据盘 `disk_type`：`:list="() => getDiskTypeOptions()"`
  - `charge_type`：保持只读 `bk-input`（展示用 `ChargeTypeMap` 文案）
  - **无可用区下拉**（zone 不在 C 弹窗编辑，仅 createState/handleSave 透传保留原值）
- **级联**：region 变更后清空 device_type/image_id（及透传的 zone）已选值（`watch(() => formModel.region)`，回填期不清空）。
- 修正：`addDataDisk` 默认 `disk_type` 由 `'SSD'` → `'CLOUD_PREMIUM'`。

## 4. 表格展示（#4 host-apply-preorder-table.vue）
- 机型列：保持 `row.device_type`（不映射）。
- 操作系统列：`getSpecFieldText(row, 'image_id')`。
- 地域列：`getSpecFieldText(row, 'region')`。
- 可用区列：`getSpecFieldText(row, 'zone')`。
- 计费模式/需求类型：已走/改走 `getSpecFieldText`。
- 系统盘/数据盘（showDisk）：`getSpecFieldText`（经 #2 磁盘名映射）。

## 5. bizId 透传（已取消）
- 机型改用单纯机型目录接口后不再依赖 bizId，故不透传 bizId（chat-message-list / 卡片 / 弹窗均无 bizId 相关代码）。
- 机型仅按 `vendor + region` 过滤；region 由弹窗内选择，cascade 自动刷新。

## 5.1 「添加到配置清单」跳转回填（新增）

目标：A/B/D 卡片点「添加到配置清单」→ 跳转老申领页 `applyCvm` → 把方案规格回填到页面的配置清单（云主机表格）。

| 文件 | 类型 | 说明 |
|------|------|------|
| `src/store/chatbot/host-apply-backfill.ts` | 新增 | 浮窗同页传参通道：`pushBackfill/consume`，仅浮窗场景用 |
| `src/components/chatbot/chat-message-list.vue` | 改 | `handleAddToList` 按 `useChatbotMode()` 分形态：floating 走 store + Message 提示；fullpage 走 `routerAction.open` 新标签页 |
| `src/views/ziyanScr/hostApplication/components/application-form/index.tsx` | 改 | `unReapply` 解析 query 回填；另 `watch` store.pending 实现浮窗同页追加；均复用 `backfillFromChatbot` |

实现要点（两种形态）：
- **浮窗（floating）**：chatbot 浮窗仅挂在申领页（`service-apply/cvm`），与 `ApplicationForm` 同页同应用实例。点击「添加到配置清单」→ `hostApplyBackfillStore.pushBackfill(suborders)` + `bkMessage` 提示「配置已添加到清单」，**不开新标签页**；`ApplicationForm` 内 `watch(() => store.pending)` 消费并 `backfillFromChatbot` 追加到当前页配置清单。
- **全页（fullpage）**：`routerAction.open` 新标签页打开 `applyCvm`；新标签页是独立实例无法用 store，规格经 URL query `backfill`（`JSON.stringify`）承载，表单页 `parseChatbotBackfill`（`JSON.parse` + try/catch）解析；并带 `sessionCode` 让申领页 `onMounted` 唤起对应会话浮窗。
- 形态判定用 `useChatbotMode()`（floating / fullpage，由壳层 provide）。
- 路由名沿用老的 `applyCvm`（渲染 `service-apply/cvm` → `<ApplicationForm>`）；参考 `applications/index.tsx:131` 去掉 `order_id`/`unsubmitted`。
- `buildCloudRowFromChatbot` 对齐 `cloudResourceForm()` 输出结构：spec 含 `device_type/image_id/system_disk/data_disk/region/zone/zones/charge_type/charge_months/res_assign/replicas...`；`resource_type` 缺省取 `QCLOUDCVM`（chatbot 本期仅产出 ZIYAN QCLOUDCVM）。
- `require_type` 为订单级共享字段，取首条带值 suborder 回填到 `order.model.requireType`。
- 单条/多条统一处理：单方案回填 1 行，确认提交多 suborder 回填多行。
- **CPU 核数补全**：chatbot 不下发每实例 `spec.cpu`，而页面「需求核数 / CPU 总数」依赖它（`replicas * spec.cpu`），缺失会 NaN。回填时按去重机型并发调用 `cvmDeviceStore.getOneDevicetype`（filter `vendor=ZIYAN + device_type EQ`）取 `cpu_core`，映射回填 `spec.cpu`。
- **预选账号透传**：申领页账号选择（`service-apply/cvm/index.tsx` 的 `AccountSelectorCard` → `cond.cloudAccountId`，bcc 插件提供）是 ApplicationForm 渲染的前置门槛，聊天里 agent 也前置确认账号。把聊天已选账号 id 带到申领页预选同一账号：
  - 聊天侧 `useAccountSelect` 新增 `selectedAccountId`（取最后一条已选定账号选择消息的 `account_id`）。
  - 全页：`handleAddToList` 在 `routerAction.open` 的 query 加 `accountId`；`cvm/index.tsx` setup 读 `route.query.accountId` 初始化 `presetAccountId`。
  - 浮窗：`pushBackfill(suborders, accountId)` 写入 store；`cvm/index.tsx` watch `store.pending` 时取 `store.accountId` 更新 `presetAccountId`（store `consume` 只清 pending、不清 accountId）。
  - `useAccountSelectorCard` 新增可选 `presetAccountId` prop：`businessAccountList` 加载时优先命中预选账号（未命中退化首个 ZIYAN）；并对 `presetAccountId` 变化加 watch（覆盖列表已加载后才透传的浮窗场景）。prop 可选、默认空 → 不影响其它调用方。
- **选择方案跳转后定位方案**（仅全页新标签）：在「选择方案」步骤点「添加到配置清单」跳转申领页、自动打开关联会话时，定位到跳转前所选方案。**保持卡片可交互**（用户仍可选择/切换方案继续添加到清单），不进只读。
  - `chat-message-list.vue`：A 卡（单方案）跳转时 query 加 `selectPlan=1`（方案规格仍复用 `backfill`）；模板把 `getInitialIndex(message)` 传给推荐卡 `:initial-index`。
  - `use-chatbot.ts`：新增并导出 `selectRecommendBySuborder(suborder)` —— 在最近一张方案推荐卡候选中按 `isEqual` 精确匹配，写 `__initialIndex`（仅定位、不写 `__selectedIndex`，故不触发只读）。
  - `types.ts`：`HostApplyRecommendMessage` 增 `__initialIndex`（定位展示用）；`use-host-apply.ts` 增 `getInitialIndex`。
  - `host-apply-recommend-card.vue`：新增 `initialIndex` prop，`pageIndex` 初值取它并 watch 同步（prop 在会话加载后才写入）；只读态走 `selectedIndex` 不受影响。
  - `ai-assistant/index.vue` + `types.ts`：`initSessions` 增加可选 `preselectSuborder`，`await initSessions` 后调用 `selectRecommendBySuborder`。
  - `cvm/index.tsx`：`onMounted` 解析 `selectPlan` + `backfill[0]`，作为 preselect 传给 `initSessions`。
  - 仅全页：浮窗不跳转、会话已在显示，不处理。

## 5.2 历史回放恢复「已选择/已确认」（新增）

问题：会话回放时，A 卡（方案推荐）总是高亮首条、B 卡（预提单）丢失 C 弹窗修改——因为运行态字段 `__selectedIndex`/`__confirmedSuborders` 不进 `MESSAGES_SNAPSHOT`。后端更新后，每次中断 resume 会在快照里附带一条 CUSTOM `after_tool_hitl.resume_forwarded` 回执，`payload.value` 为当时回写的 JSON。

| 文件 | 类型 | 说明 |
|------|------|------|
| `src/hooks/chatbot/types.ts` | 改 | 新增 `HOST_APPLY_RESUME_FORWARDED_EVENT` 常量 |
| `src/hooks/chatbot/use-stream.ts` | 改 | 快照遍历期间用 resume_forwarded 回填最近卡片的选择/确认；过滤内部 activity |
| `src/components/chatbot/chat-message-list.vue` | 改 | `handleSelectPlan` 去掉调试前缀「帮我基于此方案进行拆单」，resume 直接回传 suborder JSON |

实现要点：
- 快照按序遍历，记录最近一张 `host_apply.recommend` / `host_apply.preorder` 卡片；遇 `resume_forwarded` 即按 payload 形状路由：
  - **数组 → 模板 B**（可改）：用确认/编辑后的 suborders **覆盖** `__confirmedSuborders`（`getPreorderReadonlySuborders` 优先取它）。
  - **对象 → 模板 A**（不可改）：与候选 `recommendations[*].suborder` 做 `isEqual` 匹配得选中下标写入 `__selectedIndex`，理应精确命中，兜底首条。
- `payload.value` 解析兼容历史调试前缀：从首个 `{`/`[` 起截取再 `JSON.parse`。
- 内部 activity（`ACTIVITY_DELTA`、未在白名单的 CUSTOM 如 `resume_forwarded`）不渲染为「活动消息」气泡——`isInternalActivity` 过滤；可渲染白名单为 hitl/account_select/recommend/confirm/submit。

## 5.x 回填逻辑抽离 hook（可维护性重构）

| 文件 | 改动 | 说明 |
|------|------|------|
| `src/views/ziyanScr/hostApplication/components/application-form/use-chatbot-backfill.ts` | 新增 | 收敛 chatbot「添加到配置清单」回填逻辑：`buildCloudRowFromChatbot` / `fetchCpuCoreMap` / `backfillFromChatbot` / `parseChatbotBackfill`，对外暴露 `applyQueryBackfill()`，内部 watch `host-apply-backfill` store 处理浮窗同页回填 |
| `src/views/ziyanScr/hostApplication/components/application-form/index.tsx` | 改 | 移除上述函数与 store 监听，改为 `useChatbotBackfill({ appendCloudRow, setRequireType })`；`unReapply` 命中 `applyQueryBackfill()` 即短路；同步清理 `useCvmDeviceStore`/`useHostApplyBackfillStore`/`QueryRuleOPEnum`/`HostApplySuborder` 等仅回填用的 import |

实现要点：
- hook 通过回调（`appendCloudRow` / `setRequireType`）与主表单解耦，不直接持有表单 ref；`route` / `cvmDeviceStore` / `host-apply-backfill` store 均在 hook 内自取。
- 行为与重构前一致：全页新标签走 URL query，浮窗同页走 store；动机为主表单文件业务过载、降低后续维护成本。

## 6. 规范遵循
- Vue3 `<script setup lang="ts">`，`hcm-form-list` 为全局/局部 import 组件，模板 kebab-case。
- 表单状态沿用现有 `reactive(createState())` 模式（**不引入 `useFormModel`**）。
- 不改协议/不动只读态推断（`use-host-apply.ts` 不变）。
- 不引入新依赖、不新增后端接口。
- 完成后跑 `hcmfe_lint --fix`。

## 7. 验证点（对应 PRD AC）
- AC-1/AC-2：A/B/D 显示名称、未命中回退编码不报错。
- AC-3/AC-4：C 弹窗下拉为真实数据、级联刷新、保存写回 B。
- AC-5：保存后 B 回显名称。

## 8. 待确认问题（已全部定）
| 编号 | 问题 | 结论 |
|------|------|------|
| Q1 | C 弹窗机型数据源 | 用单纯机型目录接口 `getDeviceTypeFullList`（vendor+region），不依赖 bizId/计费预测接口 |
| Q2 | `hcm-form-list` 选项键名 | 归一化 `{id,name}` + `:id-key='id' :display-key='name'`（参考 req-stage.vue） |
| Q3 | require_type 展示 | 复用 `req-type-value.vue` 组件渲染（不写 sync resolver） |
