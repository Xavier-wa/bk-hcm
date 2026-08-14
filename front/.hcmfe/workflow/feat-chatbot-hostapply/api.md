# API / 数据源 — 主机申领 chatbot 卡片优化（编码→名称 + C 弹窗真实下拉）

> 本文档定位「展示层编码→名称」与「C 调整配置弹窗下拉」两项优化所需的**数据来源与组件/接口契约**。
> 上一迭代协议见 `.hcmfe/workflow/iter-chatbot-custom-msg-command-template/api.md`（suborder 字段为后端编码）。
> **权威来源**：自研云申领表单入口 `src/views/ziyanScr/hostApplication/components/application-form/index.tsx` 及其引用的选择器/展示组件、ziyan transform 工具。展示写法可对照 `use-scr-columns.tsx` 的 `CHColumns`（L425-511）。

## 0. 已确认决策

| 项 | 决策 |
|----|------|
| 业务云 | **本期主机申领均为 ZIYAN 自研云**，`vendor = VendorEnum.ZIYAN`（application-form 给 ZoneTagSelector/DeviceTypeCvmSelector 传的就是 `VendorEnum.ZIYAN`） |
| 数据源 | **仅在数据/接口层复用**自研云申领的 WOA config store/transform，**不整体复用** `DeviceTypeCvmSelector` 等重组件（抽象不够通用）；下拉统一用通用组件 `hcm-form-list`（见 §2） |
| 名称形态 | **仅名称**（查不到回退原编码，PRD AC-2） |
| `device_type` 机型 | **直接展示编码**（如 `S3.MEDIUM4`，业务习惯名，本期不映射） |
| vendor 维度 | 本期固定 ZIYAN；选择器/映射均按 vendor 参数化，**预留公有云扩展**（见 §3） |

## 1. 展示层（A/B/D 卡片 + A 键值表）编码→名称

落点：`src/hooks/chatbot/host-apply-display.ts`（现状：8 字段 `String(val)` 原样输出）。改为按下表 id→name，**查不到回退原编码**。

| 字段 | 编码示例 | 映射来源（与 application-form / use-scr-columns 同域） |
|------|----------|------------------------------------------------------|
| `region` 地域 | `ap-nanjing` | `getRegionCn(cell)`（`@/views/ziyanScr/cvm-web/transform`，自研云申领同款） |
| `zone` 可用区 | `all` / `ap-nanjing-1` | `zone==='all' → '全部可用区'`，否则 `getZoneCn(zone)`；数组 join `，` |
| `device_type` 机型 | `S3.MEDIUM4` | 原样展示（不映射） |
| `image_id` 镜像 | `img-fjxtfi0n` | `getImageName(cell)`（`@/views/ziyanScr/cvm-produce/component/property-display/transform`） |
| `charge_type` 计费模式 | `PREPAID` | `ChargeTypeMap[cell as ChargeType]`（`@/typings/plan`）/ `useCvmChargeType` |
| `require_type` 需求类型 | `7` | `<ReqTypeValue :value>`（`@/components/display-value/req-type-value.vue`，内部走 `useConfigRequirementStore.getRequirementType()`）或 `useRequireTypes().getValueCn` |
| `res_assign` 资源分配方式 | `1` | `RES_ASSIGN_TYPE[cell]?.label ?? '--'`（`@/components/device-type-selector/constants`） |
| `system_disk` 系统盘 | `{CLOUD_PREMIUM,100,1}` | `<CvmSystemDiskDisplay :system-disk>`（`@/views/ziyanScr/components/cvm-system-disk/display.vue`） |
| `data_disk` 数据盘 | `[{...}]` | `<CvmDataDiskDisplay :data-disk-list>`（`@/views/ziyanScr/components/cvm-data-disk/display.vue`） |

实现要点：
- 异步来源（region/zone/image/requirement）需预加载后反查；展示用「编码兜底 + 异步替换」，避免闪烁/报错。
- region/zone/image 列表由 ziyan transform 内部按需加载（沿用现网机制）；`zone='all'` 特判文案。
- `host-apply-preorder-table.vue` 列（机型/操作系统/地域/可用区/计费模式/需求类型/磁盘）改走统一 formatter / 上述 render；磁盘直接用 `CvmSystemDiskDisplay`/`CvmDataDiskDisplay` 组件。

## 2. C「调整配置弹窗」下拉：`hcm-form-list` + `:list` 异步函数

落点：`src/components/chatbot/host-apply-adjust-dialog.vue`（现状：region/device_type/image_id/res_assign/require_type 手输；disk_type 静态枚举；无 zone）。

**方案**：**不整体复用** `DeviceTypeCvmSelector`/`AreaSelector` 等重组件（抽象不够通用）。下拉统一用通用组件 **`hcm-form-list`（`@/components/form/list.vue`）**，其 `list` prop 支持「数组」或「异步函数 `() => Promise<Array<{...}>>`」并自带 loading。每个下拉传一个 `:list` 函数，函数内**只复用数据/接口层**（WOA config store/transform），把结果映射为选项数组（`{ id/value, name/label }`，键名以 `hcm-form-list` 下游 `SelectColumn`/`bk-select` 约定为准，coding 阶段对齐）。

| 字段 | `:list` 函数的数据/接口来源（复用层） | 级联依赖 |
|------|----------------------------------------|----------|
| `region` | qcloud/ziyan 地域：`useConfigQcloudResourceStore.getQcloudRegionList()` 或 ziyan region 接口 | 无（vendor=ZIYAN） |
| `zone` | `getQcloudZoneList([region])`（`POST /api/v1/woa/config/findmany/config/qcloud/zone`） | region |
| `device_type` | 预测机型：`useCvmDeviceStore.getChargeTypeDeviceTypeList({ bk_biz_id, require_type, region })` | bizId、require_type、region |
| `image_id` | `POST /api/v1/woa/config/findmany/config/cvm/image { region }` | region |
| `require_type` | `useConfigRequirementStore.getRequirementType()` | 无 |
| `res_assign` | 静态：`RES_ASSIGN_TYPE` → 选项数组 | 无 |
| `disk_type` | `GET /api/v1/woa/config/find/config/cvm/disktype`（`getDiskTypes`） | 无 |
| `charge_type` | 保持只读 / 或静态计费选项（`cvmChargeTypes`） | 无 |

要点：
- 下拉控件统一替换为 `hcm-form-list`（`v-model` 绑 suborder 字段，`:list` 传函数），替代当前 `bk-input` 手输与静态 `DISK_TYPE_OPTIONS`。
- 级联用 `computed`/闭包：`:list` 函数引用当前 `region`/`bizId`/`require_type`，上层变更触发 `hcm-form-list` 内 `watchEffect` 重新拉取；上层变更后下层已选值失效则清空（沿用现网）。
- 选项映射函数建议集中在一个 chatbot 侧 hook（如 `use-host-apply-options.ts`）按 vendor 参数化，避免散落。
- 修正项：`addDataDisk` 默认 `disk_type:'SSD'` 不合法，改用合法默认（如 `CLOUD_PREMIUM`）。

## 3. vendor 维度与公有云扩展（用户明确要求预留）

- **本期**：所有 suborder = ZIYAN 自研云 → vendor 固定 `VendorEnum.ZIYAN`。
- **扩展点**：展示 resolver（`host-apply-display.ts`）与下拉 `:list` 函数（`use-host-apply-options.ts`）均以 `vendor` 入参；未来公有云只需按 suborder 来源传对应 vendor，并在函数内按 vendor 切换数据源（公有云走 `/api/v1/cloud/...` 域）。本期只实现 ZIYAN 分支，其它 vendor 兜底（展示编码 / 下拉空）。
- 因为下拉走 `hcm-form-list` + 函数式 `:list`，扩展 vendor 不需要替换组件，只需在 `:list` 函数内按 vendor 分流，改动收敛。

## 4. 上下文获取（C 弹窗选择器所需）

| 上下文 | 来源 | 说明 |
|--------|------|------|
| `vendor` | 本期固定 `VendorEnum.ZIYAN`（后续从 suborder 推导） | `:list` 函数 / 映射 vendor 入参 |
| `bk_biz_id` | D 卡 `value.data.path_param.bk_biz_id`；或会话级 bizId（`use-chatbot.ts`） | device_type 预测机型 `:list` 需要 |
| `require_type` | 当前编辑行 suborder.require_type | device_type 预测机型 `:list` 需要 |
| `region` | 当前编辑行 suborder.region | zone/device/image `:list` 级联依赖 |

> ⚠️ A/B 卡下行不含 `bk_biz_id`（仅 D 卡 path_param 有）。C 弹窗多由 B 触发，bizId 需从会话级兜底。**coding 阶段核对会话 bizId 可得性**。

## 5. 不引入新后端接口

所有数据源均为 hcm 前端**已存在**的 WOA config 接口/store/transform 与通用展示组件，**不需要后端新增接口**。chatbot 层仅新增「展示映射 resolver + 下拉 `:list` 选项函数（`hcm-form-list`）+ vendor 参数化」。

## 6. 待确认问题（coding 阶段核对）

| 编号 | 问题 | 默认假设 |
|------|------|----------|
| Q1 | C 弹窗 bizId 在 B 触发场景的来源（会话级 bizId 是否可得） | coding 阶段核对会话上下文 |
| Q2 | `hcm-form-list` 选项项键名（`id/name` vs `value/label`）与下游 `SelectColumn`/`bk-select` 约定 | coding 阶段对齐 |
| Q3 | 各 `:list` 函数返回结构与 store 现有方法签名是否需薄封装 | coding 阶段抽 `use-host-apply-options.ts` |
