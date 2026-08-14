# iter-biz-index-chatbot-route-layout 测试清单

> 工作流: `iter-biz-index-chatbot-route-layout`
> 关联文档: PRD / Design / API / Coding
> 负责人: 开发自测

## 验证范围

- 功能边界: 业务视角 chatbot 路由（`/business/chatbot`）、默认首页分流、顶栏/左侧菜单权限表现、chatbot 页三栏布局（业务侧栏 + 会话侧栏 + 主对话区）
- 不在范围: Agent API 变更、`bizs` 路径前缀改造、旧 `/chatbot` 重定向、新权限体系 `meta.auth.view`

## 测试环境

- 前端入口: HCM 前端测试环境（Hash 路由），由执行人在测试时确认具体域名
- 测试账号:
  - **账号 A**：具备 `chatbot_access`（agent_assistant）
  - **账号 B**：不具备 `chatbot_access`
- 数据准备:
  - 账号 A/B 均在权限中心配置好对应动作
  - 账号 A 建议预置至少 1 条历史会话（便于验证 `sessionCode` URL 同步）
  - 403 场景：账号 B 直访 `/business/chatbot`（无需 mock）

## 用例清单

### P0 - 主流程（必测）

| ID | 场景 | 前置 | 操作 | 期望 | 回滚 |
|----|------|------|------|------|------|
| P0-01 | 有权限默认首页 | 账号 A，清空缓存后首次打开 `#/` | 等待权限加载完成 | 最终落在 `#/business/chatbot`（带 `bizs` query 若业务有选） | — |
| P0-02 | 无权限默认首页 | 账号 B，首次打开 `#/` | 同上 | 落在 `#/business/host` | — |
| P0-03 | 顶栏资源管理（有权限） | 账号 A | 点击顶栏「资源管理」 | 进入 chatbot；左侧业务菜单可见；二级菜单有「首页」 | — |
| P0-04 | 顶栏资源管理（无权限） | 账号 B | 点击顶栏「资源管理」 | 进入主机列表；顶栏仍显示「资源管理」；左侧**无**「首页」 | — |
| P0-05 | 会话 URL 同步 | 账号 A，在 chatbot 页 | 切换/新建会话 | 地址栏变为 `#/business/chatbot/<sessionCode>`（有 code 时） | — |
| P0-06 | 平台管理侧栏 | 任意有平台权限账号 | 访问 `#/platform/rolling-server` | 左侧平台管理菜单正常展示（非空白） | — |

### P1 - 异常分支（必测）

| ID | 场景 | 前置 | 操作 | 期望 |
|----|------|------|------|------|
| P1-01 | 无权限直访 chatbot | 账号 B | 地址栏输入 `#/business/chatbot` | 403 或无权限页（`toCurrentPage`） |
| P1-02 | 无顶层旧路由 | 任意账号 | 访问 `#/chatbot` | 无匹配路由或 404（不应进入旧首页） |
| P1-03 | 业务子页不受影响 | 账号 B | 从 host 进入磁盘/VPC 等二级菜单 | 正常打开对应资源页 |

### P2 - UI 细节（按需）

- [ ] chatbot 页：左侧业务资源树始终可见（Figma 2006-6056）
- [ ] 会话侧栏展开/收起（Figma 2031-829），主区域宽度自适应
- [ ] `.chatbot-page` 在存在业务侧栏时纵向填满，无双重滚动条
- [ ] 面包屑在 chatbot 页隐藏

## 验证结论

> 每条用例填写：PASS / FAIL / Skipped + 简要说明（FAIL 必须附复现步骤）

| ID | 结果 | 备注 |
|----|------|------|
| P0-01 | PASS | 用户确认 |
| P0-02 | PASS | 用户确认 |
| P0-03 | PASS | 用户确认 |
| P0-04 | PASS | 用户确认 |
| P0-05 | PASS | 用户确认 |
| P0-06 | PASS | 用户确认 |
| P1-01 | PASS | 用户确认 |
| P1-02 | PASS | 用户确认 |
| P1-03 | PASS | 用户确认 |

- 执行人: 开发自测
- 执行日期: 2026-05-28
- 总体结论: PASS
- 后续行动: 可合入 `feat-biz-index-chatbot`
