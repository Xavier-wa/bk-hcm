# 权限控制

> status: drafted · kind: module
> globs: `src/common/auth-service.ts`、`src/constants/auth-symbols.ts`、`src/components/auth/**`、`src/components/permission/**`
> 横切关注点，从 base 拆出。相关文件物理上仍落在 common/constants/components 目录内，属概念分层。

## 职责

管理两个层面的权限：

1. **视图级**：用户能否进入某页面（路由守卫拦截，无权限渲染权限申请页）。
2. **操作级**：用户能否执行某操作（按钮禁用 + 权限申请弹窗）。

## 关键文件

- `src/constants/auth-symbols.ts` — 权限类型 Symbol 常量（如 `AUTH_UPDATE_IAAS_RESOURCE`）。
- `src/common/auth-service.ts` — 权限定义与鉴权工具。
- `src/components/auth/auth.vue` — `HcmAuth` 组件，操作级权限的唯一正确入口。
- `src/components/permission/apply-dialog.vue` — 操作级权限申请弹窗，`window.hcmPermissionDialog.show(permission)` 调用。
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

## 禁止使用（旧方案已废弃，store 方法已移除）

- `useVerify` hook / `getAuthVerifyData` / `authVerifyData.permissionAction` —— 权限数据恒为空，按钮永远禁用。
- `provide/inject('authVerifyData')`、`bus.$emit('auth', ...)` 触发弹窗。
- `useGlobalPermissionDialog`、`components/permission-dialog/`、`components/global-permission-dialog/`。
- `403.tsx` / `NoPermission.tsx` / `/403/:id` 路由做视图无权限控制（改由路由守卫 + 权限页）。
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
