# 权限控制

> status: drafted · kind: module
> globs: `src/common/auth-service.ts`、`src/constants/auth-symbols.ts`、`src/components/auth/**`、`src/components/permission/**`、`src/hooks/useVerify.ts`、`src/store/common.ts`、`src/views/error-pages/403.tsx`
> 横切关注点，从 base 拆出。相关文件物理上仍落在 common/constants/components/hooks/store/views 目录内，属概念分层。

## 职责

管理两个层面的权限：

1. **视图级**：用户能否进入某页面（路由守卫拦截，无权限渲染权限申请页）。
2. **操作级**：用户能否执行某操作（按钮禁用 + 权限申请弹窗）。

> **迁移状态**：`useVerify` + `authVerifyData` + `/403/:id` + `403.tsx` 属 Deprecated 旧链路。它们在当前仓库仍实际运行，仅允许维护存量入口；新功能、新页面和新操作权限禁止继续接入，应使用 `auth.view` / `HcmAuth` 目标方案。本次智能助手鉴权调整沿用既有业务入口与 403 页面，属于存量链路维护，不改变其 Deprecated 定位。

## 关键文件

- `src/constants/auth-symbols.ts` — 权限类型 Symbol 常量（如 `AUTH_UPDATE_IAAS_RESOURCE`）。
- `src/common/auth-service.ts` — 权限定义与鉴权工具。
- `src/components/auth/auth.vue` — `HcmAuth` 组件，操作级权限的唯一正确入口。
- `src/components/permission/apply-dialog.vue` — 操作级权限申请弹窗，`window.hcmPermissionDialog.show(permission)` 调用。
- `src/store/common.ts` — 仍在运行的旧视图级预鉴权清单 `pageAuthData` 与批量 verify 结果 `authVerifyData`。
- `src/hooks/useVerify.ts` — 把 `pageAuthData` 转为 `/auth/verify` 请求，并把结果按条目 `id` 映射到 `permissionAction`。
- `src/views/error-pages/403.tsx` — 仍在运行的 `/403/:id` 权限说明页；不同权限可选择 IAM 自助申请或主动联系管理员。
- （规则引用但当前仓库尚未落地）`constants/view-auth.ts`、`views/status/permission.vue`、`views/status/business.vue` — 视图级预鉴权与权限申请页。

## 一、视图级权限

机制：`route-config.ts` 的 `meta.auth.view` 声明所需权限 → 路由守卫 `beforeEach` 检查 → 无权限时切换到权限申请页。

```typescript
// 静态权限
meta: { ...new Meta({ owner: MENU_RESOURCE, auth: { view: { type: AUTH_FIND_IAAS_RESOURCE } } }) }
// 动态参数（业务 ID）
auth: { view: (to) => ({ type: AUTH_ACCESS_BIZ, relation: [Number(to.params.bizId)] }) }
```

业务路由的 `AUTH_ACCESS_BIZ` 由业务拦截器统一处理，无需逐路由配置。无动态参数的静态权限可在 `constants/view-auth.ts` 加预鉴权（应用启动批量校验）。

### Deprecated 旧视图级链路（仍在运行）

当前仓库处于权限方案迁移期，两套视图级机制并存。Deprecated 表示禁止新代码接入，不表示这些文件已经删除或运行时不再调用：

1. `src/store/common.ts` 的 `pageAuthData` 声明 `{ type, action, id, path?, bk_biz_id? }`。
2. `src/hooks/useVerify.ts` 在启动/业务切换时批量调用 `/api/v1/web/auth/verify`，将结果写入 `authVerifyData.permissionAction[id]`，并为缺失权限生成 `urlParams[id]`。
3. `src/router/index.ts` 根据目标路径选择最具体的鉴权项；无权限跳转 `/403/:id`。
4. `src/views/error-pages/403.tsx` 根据 `id` 展示说明与操作。`agent_assistant` 属主动授权，不展示 IAM “申请权限”按钮，改用 `w-name` 联系管理员。

`agent_assistant` 是平台权限，不带 `bk_biz_id`；业务范围仍由 `biz_access` 控制。`biz_access` 当前没有 `bk_biz_id`，后端执行 `AuthorizeAny`，表示“对任意业务有访问权限”，并非当前业务精确鉴权；修正该历史行为需单独评估所有 `/business*` 页面的准入影响。

## 二、操作级权限（HcmAuth）

用 `HcmAuth` 包裹按钮，通过 scoped slot 拿到 `noPerm`：

```vue
<hcm-auth :sign="{ type: AUTH_MANAGE_RECYCLE_BIN }" v-slot="{ noPerm }">
  <bk-button :disabled="noPerm || otherDisabled" @click="doAction">操作</bk-button>
</hcm-auth>
```

- `IAuthSign`：`{ type: symbol, relation?: [...] }`。
- **资源场景**：`AUTH_*` + `relation: [accountId]`（从资源详情数据 `detail.account_id` 取）。
- **业务场景**：`AUTH_BIZ_*` + `relation: [bizId]`。列表页可用 `getAuthSignByBusinessId(bizId, AUTH_*, AUTH_BIZ_*)` 简化。

工作机制：相同 sign 的多个实例 CombineRequest 批量合并；无权限自动遮罩并弹权限申请；HTTP 层 403（code 2030403）全局兜底弹窗。

## Deprecated 约束（新代码禁止继续扩散）

- `useVerify` hook / `getAuthVerifyData` / `authVerifyData.permissionAction` 仍被旧视图级路由使用；除修复、迁移现有入口外，新代码禁止继续接入该链路。
- `provide/inject('authVerifyData')`、`bus.$emit('auth', ...)` 触发弹窗。
- `useGlobalPermissionDialog`、`components/permission-dialog/`、`components/global-permission-dialog/`。
- `/403/:id` 与 `403.tsx` 仍服务旧视图级链路；除维护现有入口外，不应再新增专用 403 分支。迁移到 `auth.view` 时应改用统一权限页。
- `useResourceAccountStore` 取 `account_id`（恒为 null，改从资源详情数据取）。

## 旧 auth ID → 新 symbol 映射（节选）

| 旧 ID | 新 symbol |
|---|---|
| `iaas_resource_operate` / `_delete` | `AUTH_UPDATE_IAAS_RESOURCE` / `AUTH_DELETE_IAAS_RESOURCE` |
| `biz_iaas_resource_operate` / `_delete` | `AUTH_BIZ_UPDATE_IAAS_RESOURCE` / `AUTH_BIZ_DELETE_IAAS_RESOURCE` |
| `recycle_bin_manage` | `AUTH_MANAGE_RECYCLE_BIN` |
| `account_edit` / `account_import` | `AUTH_UPDATE_ACCOUNT` / `AUTH_IMPORT_ACCOUNT` |
| `root_account_find` | `AUTH_FIND_ROOT_ACCOUNT` |

## 注意事项

- 本文档提炼自 `.cursor/rules/fe-auth-migration.mdc`；完整迁移清单、待迁移文件优先级以该规则为准。
- 视图级/操作级权限与路由守卫强相关，改动时对照 menu-route 模块。
