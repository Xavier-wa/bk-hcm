# Coding — feat-cert-ziyan-delete-tip

## 执行顺序

1. `cert-manager` 操作列删除按钮增加自研云判定（唯一代码改动点）

> 排序依据：本需求只有一处代码改动，改动面收敛在单个文件的单个 render 函数内，无依赖、无风险分层。

## 共享改动 / 提交策略

- 跨单公共改动：无。本需求为单单据，且**不改动任何共享组件 / hooks / 工具函数 / 常量 / store**，只改页面组件自身的列渲染逻辑。
- 提交与关单策略：每单一提交（默认；与 git-commit skill 一致）。

---

## 单据 1: 自研云证书-云上不支持删除，页面需提示

**TAPD**: [#1069995598137815326](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137815326)

**文件**: `src/views/business/cert-manager/index.tsx`

**改动点**: 在证书列表操作列的删除按钮 `render` 内新增自研云判定——行数据 `vendor === VendorEnum.ZIYAN`（`tcloud-ziyan`）时按钮置灰不可点击，并把悬停 tips 按「自研云 > 已分配业务 > 无提示」的优先级切换为「自研云证书不允许删除，如有疑问，请联系C2000」。非自研云行保持原有判定与文案不变。

### 3.y 落码入口核对

`design.md` §3.y 判定为 `adjacent`（无 `page-*` / `comp-*` 落码入口），项目路径 `src/views/business/cert-manager/index.tsx` 操作列 `render`。已按该路径落码，未新建页面与组件。

### 实施细节

改动位于 `tableColumns` 中「操作」列的 `render`（改动前位于原文件第 72–92 行）：

1. `render` 由「隐式返回 JSX」改为「块级函数 + 显式 return」，以便在渲染前做一次判定；
2. 新增两个只读局部变量：
   - `isZiyanCert = data.vendor === VendorEnum.ZIYAN` —— 自研云判定（PRD R-001）；
   - `isAssignedBiz = isResourcePage && data.bk_biz_id !== -1` —— 抽出既有的「已分配业务」判定，语义与原来完全一致；
3. `Button` 的 `disabled` 追加 `isZiyanCert`，保证点击不可达（不弹确认框、不发请求，AC-001 / AC-S01）；
4. `v-bk-tooltips` 的 `content` 改为 `isZiyanCert ? '自研云证书不允许删除，如有疑问，请联系C2000' : '该证书已分配业务, 仅可在业务下操作'`，`disabled` 改为 `!isZiyanCert && !isAssignedBiz`（自研云优先，PRD R-003）。

**不需要新增 import**：`VendorEnum` 已在文件顶部从 `@/common/constant` 导入（第 5 行）。

### 关键前置确认

- `VendorEnum.ZIYAN = 'tcloud-ziyan'`（`src/common/constant.ts:20`），`VendorMap` 中映射为「自研云」——与列表「云厂商」列同源。
- 证书列表列定义 `certColumns`（`src/views/resource/resource-manage/hooks/use-columns.tsx:1592`）已包含 `vendor` 字段列，说明列表数据中该字段可用。
- `useWhereAmI()` 返回的 `isResourcePage` / `isBusinessPage` 是**普通 boolean**（非 ref，`src/hooks/useWhereAmI.ts:9,55`），无需 `.value`，既有用法正确，本次不改动其求值方式。

### 影响面分析（编码前完成）

| 维度 | 结论 |
|------|------|
| 改动文件 | `src/views/business/cert-manager/index.tsx`（**唯一**） |
| 是否改共享组件 / hooks / 工具 / 常量 / store | **否**。不新增、不修改任何共享代码 |
| 文件使用方 | 1. `src/router/module/business.ts:349` —— 业务视角路由 `/business/cert`（证书托管）<br>2. `src/views/resource/resource-manage/resource-manage.vue:13` —— 资源视角（资源管理 - 证书） |
| 使用方兼容性 | 两处复用**同一个页面组件**，本次改动落在组件内部，故两个视角同时生效（满足 AC-004），且生效口径完全一致 |
| 非自研云回归证明 | 当 `isZiyanCert === false` 时：<br>• `disabled` 退化为 `noPerm \|\| isAssignedBiz`，与原表达式 `noPerm \|\| (isResourcePage && data.bk_biz_id !== -1)` **完全等价**；<br>• `content` 退化为原常量文案；<br>• tooltip `disabled` 退化为 `!isAssignedBiz`，与原 `!(isResourcePage && data.bk_biz_id !== -1)` **完全等价**。<br>⇒ 非自研云厂商（腾讯云 / AWS / Azure / GCP / 华为云 / Zenlayer / 靠谱云 / 其他）行为**零变化** |
| 其它操作回归 | 上传证书、批量分配业务（`BatchDistribution`）、行多选（`isCurRowSelectEnable`）均**不触碰**（AC-007） |
| 接口 / 权限 | 不新增请求、不改接口、不改权限点与 `noPerm` 分支（AC-P01 / AC-006） |

### 未采纳的方案（影响面更大，已排除）

- **在 `handleDeleteCert` 内二次拦截**：按钮已不可达，属冗余校验，且会给调用方增加分支，不采纳。
- **抽成共享工具函数 / 常量**：本判定仅一处使用，抽公共代码反而扩大影响面，不采纳。
- **顺手修复 `isResourcePage` 语义问题**：经核实 `isResourcePage` 为普通 boolean，既有用法**正确**，不存在待修 bug，无需改动。
