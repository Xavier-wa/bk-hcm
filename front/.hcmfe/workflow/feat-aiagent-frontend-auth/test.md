# feat-aiagent-frontend-auth 测试清单

> 工作流: `feat-aiagent-frontend-auth`
> 关联文档: PRD / Design / API / Coding

## 验证范围

- 功能边界: 验证业务首页菜单、chatbot 直访、默认业务落地、智能体浮窗统一改用 IAM `agent_assistant`，并验证业务访问优先级与 403 主动授权提示。
- 不在范围: 后端会话接口鉴权、旧权限迁移、IAM 删除旧权限点、`biz_access` 按当前业务精确鉴权。
- 实现差异备注:
  - Design / PRD 原描述为通过通用页自助申请「平台-智能体助手」；实际实现按最新确认改为主动授权：`/403/agent_assistant` 隐藏「申请权限」按钮，使用 `w-name` 展示「联系管理员申请」并拉起企微。
  - `biz_access` 目前未携带 `bk_biz_id`，后端执行 `AuthorizeAny`。因此“无当前业务访问”不能作为 P0 断言；P0-06 必须使用对任何业务都无访问权限的账号。该历史问题另开单处理。

## 测试环境

- 前端入口: `<对应业务的测试环境地址，由执行人在测试时确认；不要把具体域名写入仓库>`
- 测试账号:
  - `<platform_and_biz_account>`：有 `agent_assistant`，且至少有一个业务访问权限。
  - `<biz_only_account>`：有业务访问权限，无 `agent_assistant`。
  - `<legacy_only_account>`：有业务访问权限，仅有旧 `biz_agent_assistant`，无 `agent_assistant`。
  - `<no_biz_account>`：对任何业务都无 `biz_access`；是否有 `agent_assistant` 均可。
- 数据准备:
  - 准备一个上述账号可访问的业务 ID，记为 `<biz_id>`。
  - 清理或记录浏览器 localStorage 中已有的 `bizs`，避免与 URL 中 `<biz_id>` 混淆。
  - 打开浏览器 Network，按用例记录 `/api/v1/web/auth/verify` 请求次数及响应中的 `permissionAction`。
  - 确认 `ASSISTANT_CONTACT.name` 对应的企微联系人可被测试环境客户端拉起。

## 用例清单

### P0 - 主流程（必测）

| ID | 场景 | 前置 | 操作 | 期望 | 回滚 |
|----|------|------|------|------|------|
| P0-01 | 使用双权限账号进入业务首页 | 登录 `<platform_and_biz_account>` | 打开 `/#/?bizs=<biz_id>` | 默认进入 `/business/chatbot`；业务菜单显示“首页”；业务视图显示智能体浮窗 | 无 |
| P0-02 | 使用双权限账号直访 chatbot | 登录 `<platform_and_biz_account>` | 打开 `/#/business/chatbot?bizs=<biz_id>` | 正常进入 chatbot 页面，不跳 403；URL 保留 `bizs` | 无 |
| P0-03 | 隐藏无平台权限账号的入口 | 登录 `<biz_only_account>` | 打开业务视图并检查菜单、浮窗 | 不显示“首页”菜单；不渲染智能体浮窗；不弹权限申请弹窗 | 无 |
| P0-04 | 无平台权限直访主动授权页 | 登录 `<biz_only_account>` | 打开 `/#/business/chatbot?bizs=<biz_id>` | 停留在 `/403/agent_assistant?bizs=<biz_id>`；不回退主机页；文案显示无「平台-智能体助手」权限及“请联系管理员申请权限” | 无 |
| P0-05 | 联系管理员申请平台权限 | 承接 P0-04 | 点击“联系管理员申请” | 不显示 IAM “申请权限”按钮；通过 `w-name` 拉起 `ASSISTANT_CONTACT.name` 对应的企微联系人 | 关闭企微窗口 |
| P0-06 | 无任何业务访问时优先申请业务访问 | 登录 `<no_biz_account>` | 分别打开 `/#/business/host?bizs=<biz_id>` 与 `/#/business/chatbot?bizs=<biz_id>` | 两条路径均进入 `/403/biz_access?bizs=<biz_id>`，不会先展示平台权限提示 | 无 |
| P0-07 | 不兼容旧业务智能体权限 | 登录 `<legacy_only_account>` | 进入业务视图并直访 chatbot | 无“首页”菜单、无浮窗；直访进入 `/403/agent_assistant`，不把旧权限视为平台权限 | 无 |
| P0-08 | 根据平台权限选择默认落地页 | 分别登录 `<platform_and_biz_account>`、`<biz_only_account>` | 打开 `/#/business?bizs=<biz_id>` | 有平台权限进入 chatbot；无平台权限进入 `/business/host`；两者均保留 `bizs` | 无 |

### P1 - 异常与回归分支（必测）

| ID | 场景 | 前置 | 操作 | 期望 |
|----|------|------|------|------|
| P1-01 | 处理带尾斜杠的业务入口 | 分别登录双权限账号与仅业务权限账号 | 打开 `/#/business/?bizs=<biz_id>` | 双权限账号进入 chatbot，仅业务权限账号进入 host；页面不空白 |
| P1-02 | 保留 403 失效链接的业务参数 | 登录 `<platform_and_biz_account>` | 打开 `/#/403/biz_access?bizs=<biz_id>` | 因已持有该权限而弹回默认首页，最终 URL 仍保留 `bizs=<biz_id>` |
| P1-03 | 允许真实缺失权限停留在 403 | 登录 `<biz_only_account>` | 打开 `/#/403/agent_assistant?bizs=<biz_id>` | 继续停留在该申请说明页，不被旧逻辑弹回 `/` |
| P1-04 | 回归其它权限申请页 | 登录有 `biz_access`、缺目标权限的账号 | 访问至少一个现网其它 `/403/:id`，如 `/403/rolling_server_manage` | 真实缺少目标权限时仍停留在对应 403 页，不因持有 `biz_access` 被弹回首页 |
| P1-05 | verify 失败时避免白屏与无限请求 | DevTools 临时阻断 `/api/v1/web/auth/verify` 或使其返回 500 | 冷启动打开业务路径，观察页面与 Network；恢复接口后再次导航 | 失败时导航可结束并按无权限落 403，不出现空白页或无限请求；恢复后下一次导航会重试 verify 并可正常进入 |
| P1-06 | 验证 verify 请求未因入口切换额外增加 | 正常网络，清空 Network | 冷启动打开业务视图并等待落地 | 入口鉴权复用启动批量 verify；重定向链不会重复发起独立 verify 请求 |
| P1-07 | 保持方案页历史直通行为 | 登录任意账号 | 分别打开 `/scheme/recommendation`、`/scheme/deployment/list` | 两条既有路由不被本次业务鉴权逻辑额外拦截 |

### P2 - UI 与文案细节（按需）

- [ ] `/403/agent_assistant` 的权限说明、功能说明和“联系管理员申请”文案无“灰度”描述。
- [ ] `agent_assistant` 场景不展示底部 IAM “申请权限”按钮。
- [ ] “联系管理员申请”使用 `w-name` 的主色文字按钮样式，点击区域可用。
- [ ] 菜单隐藏后布局无空占位；浮窗不渲染时业务页面无残留遮罩或弹层。
- [ ] 1280 / 1440 / 1920 三种宽度下 403 页文案与联系按钮无溢出。

## 验证结论

> 每条用例填写：PASS / FAIL / Skipped + 简要说明（FAIL 必须附复现步骤、错误截图链接或日志片段）

| ID | 结果 | 备注 |
|----|------|------|
| P0-01 | PASS | 用户确认通过 |
| P0-02 | PASS | 用户确认通过 |
| P0-03 | PASS | 用户确认通过 |
| P0-04 | PASS | 用户确认通过 |
| P0-05 | PASS | 用户确认通过 |
| P0-06 | PASS | 用户确认通过 |
| P0-07 | PASS | 用户确认通过 |
| P0-08 | PASS | 用户确认通过 |
| P1-01 | PASS | 用户确认通过 |
| P1-02 | PASS | 用户确认通过 |
| P1-03 | PASS | 用户确认通过 |
| P1-04 | PASS | 用户确认通过 |
| P1-05 | PASS | 用户确认通过 |
| P1-06 | PASS | 用户确认通过 |
| P1-07 | PASS | 用户确认通过 |

- 执行日期: 2026-08-18
- 总体结论: PASS
- 后续行动: 可作为发布 / 合入依据，推进工作流至 done。
