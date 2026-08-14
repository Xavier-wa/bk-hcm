# Coding — fix-biz-chatbot-403-apply

> 工作流：`fix-biz-chatbot-403-apply`（lite）  
> 需求文档：`docs/reqs/AI助手无权限回退.md`  
> TAPD：[#1069995598136566294](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598136566294)

## 执行顺序

1. 路由守卫特殊处理 `biz_agent_assistant` 无权限去向（核心）
2. 冷启动 / 站内跳转统一传入 `to`，保证行为一致
3. 默认首页分流收窄：直链 `/business/host` 不再被拉到 chatbot（范围并进，用户确认）
4. 自测清单对齐 AC-001~006 / AC-S01

## 共享改动 / 提交策略

- 改动集中在 `src/router/index.ts`（必要时仅补常量 import）
- 单提交；不改 403.tsx / useVerify（本期明确不包含）

---

## 单据 1: AI对话-业务直链进入无权限页时「申请权限」按钮无响应

**TAPD**: [#1069995598136566294](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598136566294)  
**文件**: `src/router/index.ts`  
**改动点**:
1. 对主动授权型权限 `biz_agent_assistant` 做路由层特殊处理——无权限时不再进入该权限点的申请页；有业务访问则回退主机页并保留 `bizs`，无业务访问则改走 `biz_access` 申请页；冷启动与站内跳转共用同一套逻辑。
2. 冷启动默认首页分流收窄为仅 `/`、`/business`；直链 `/business/host` 有 chatbot 权限时也留在主机页。

### 根因（产品结论）

`biz_agent_assistant` 是主动授权模型，本就不该展示自助申请页。现状鉴权失败仍跳 `403` 且 `params.id=biz_agent_assistant`，该权限点无说明文案、也取不到申请链接 → 「申请权限」无响应。公共链路问题（丢 `bizs` / `urlParams` 覆盖）**本单不修**。

另：历史默认首页兼容把 `/business/host` 也算进分流，导致有权限用户直链主机页被误拉到 chatbot——与「仅默认首页触发跳转」不符，本单一并修正。

### 修复方案

#### 1. 扩展 `toCurrentPage` 的无权限分支

现有：

```ts
if (hasAuth) next();
else next({ name: '403', params: { id: currentFindAuthData?.id } });
```

改为（逻辑语义）：

```ts
if (hasAuth) {
  next();
  return;
}

// 主动授权型：业务 AI 助手不展示自助申请页
if (currentFindAuthData?.id === 'biz_agent_assistant') {
  const hasBizAccess = !!authVerifyData?.permissionAction?.biz_access;
  if (hasBizAccess) {
    // 回退业务主机页，保留原 query（含 bizs）
    next({
      name: MENU_BUSINESS_HOST_MANAGEMENT,
      query: to?.query,
    });
  } else {
    // 改走可自助申请的业务访问申请页
    next({
      name: '403',
      params: { id: 'biz_access' },
      query: to?.query,
    });
  }
  return;
}

next({ name: '403', params: { id: currentFindAuthData?.id }, query: to?.query });
```

注意：

- 需确认已 import `MENU_BUSINESS_HOST_MANAGEMENT`（若尚未引入则补上）。
- `to` 参数在冷启动与非冷启动分支**都必须传入**（见下），否则站内跳转时 `query` 丢失，违反 F-003 / AC-002。

#### 2. 统一传入 `to`（F-003）

`beforeEach` 里非冷启动分支始终传 `to`：

```ts
toCurrentPage(authVerifyData, currentFindAuthData as any, next, to);
```

> 说明：现有「403 且有 biz_access → 弹回 `/`」逻辑继续保留，作用于**其他**可申请权限点；`biz_agent_assistant` 已在更早分支处理，不会再落到该逻辑。

#### 3. 默认首页分流收窄（F-004）

冷启动守卫现有：

```ts
if (hasBizChatbotAccess && (to.path === '/' || to.path === '/business' || to.path === '/business/host')) {
  next({ name: MENU_BUSINESS_CHATBOT, query: to.query });
}
```

改为仅默认首页入口：

```ts
if (hasBizChatbotAccess && (to.path === '/' || to.path === '/business')) {
  next({ name: MENU_BUSINESS_CHATBOT, query: to.query });
}
```

- /business 的 redirect 分流（router/module/business.ts）保持不变：有权限 → chatbot，无权限 → host。
- 直链 / 菜单进入 /business/host：不再被冷启动守卫拦截。

### 非目标（与需求文档「本期不包含」对齐）

- 不改 views/error-pages/403.tsx（不补文案、不做申请链接兜底）
- 不改 hooks/useVerify.ts 的 urlParams 覆盖写入
- 不改平台视角 agent_assistant
- 不改后端权限模型

### 验收映射

| AC | 编码侧如何覆盖 |
|---|---|
| AC-001 | 无 chatbot 有 biz_access → `MENU_BUSINESS_HOST_MANAGEMENT` + 原 query |
| AC-002 | 非冷启动也传 `to`，与 AC-001 同逻辑 |
| AC-003 | 无 chatbot 无 biz_access → `403` + `params.id=biz_access` |
| AC-004 | `hasAuth` 直接 `next()`，不变 |
| AC-005 / AC-S01 | 守卫内同步 `next`，不先渲染 chatbot；全流程不再出现 `biz_agent_assistant` 申请入口 |
| AC-006 | 有 chatbot 权限冷启动直链 `/business/host?bizs=…` → 留在主机页，不跳 chatbot |

### 状态

- [x] 用户确认本 coding.md 可作为唯一执行依据
- [x] 编码完成（无权限去向 + 非冷启动传 `to` + host 直链不再分流）
- [x] `bkdevbuddy_lint` 通过
- [x] freshness seat
