# fix-biz-chatbot-403-apply 测试清单

> 工作流: `fix-biz-chatbot-403-apply`
> 关联文档: Coding / 需求文档 `docs/reqs/AI助手无权限回退.md`
> TAPD: [#1069995598136566294](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598136566294)

## 验证范围

- 功能边界: 业务视角 AI 助手直链访问时，无 `biz_agent_assistant` 权限的去向修正（回退主机页或改走业务访问申请页）；冷启动与站内跳转行为一致
- 不在范围: 403 页通用兜底、平台视角 agent_assistant、鉴权链路公共重构（urlParams 覆盖 / 跳转丢参）、后端权限模型

## 测试环境

- 前端入口: `<对应业务的测试环境地址，由执行人在测试时确认；不要把具体域名写入仓库>`
- 测试账号:
  - `<account_with_biz_no_chatbot>`：有业务访问（biz_access）、无 AI 助手（biz_agent_assistant）
  - `<account_no_biz>`：无业务访问、无 AI 助手
  - `<account_with_chatbot>`：有 AI 助手权限（回归）
- 数据准备:
  - 准备一个可用的业务 id `<biz_id>`（用于 URL query `bizs=<biz_id>`）
  - 站内跳转用例：先用有权限账号登录进入业务任意页，再手动改 hash 到 `/business/chatbot?bizs=<biz_id>`（或用无 chatbot 权限账号从其他业务页改地址）

## 用例清单

### P0 - 主流程（必测）

| ID | 场景 | 前置 | 操作 | 期望 | 回滚 |
|----|------|------|------|------|------|
| P0-01 | 无 AI 助手、有业务访问：冷启动直链回退主机页 | 账号 `<account_with_biz_no_chatbot>`；浏览器新开/刷新 | 直接访问 `/#/business/chatbot?bizs=<biz_id>` | 落在业务下主机页；URL 仍含 `bizs=<biz_id>`；不出现 AI 助手无权限申请页；不出现「申请权限」且说明为空的死页 | 无 |
| P0-02 | 无 AI 助手、有业务访问：站内跳转回退主机页 | 同上账号，已在海垒业务侧任意页 | 将地址改为 `/#/business/chatbot?bizs=<biz_id>` 或站内导航至该路径 | 与 P0-01 一致：回退主机页、保留业务 id、无 AI 助手申请页 | 无 |
| P0-03 | 既无 AI 助手也无业务访问：展示业务访问申请页 | 账号 `<account_no_biz>` | 直接访问 `/#/business/chatbot?bizs=<biz_id>` | 进入无权限申请页；权限标识为业务访问（biz_access）；「权限申请说明」「功能说明」非空；点击「申请权限」可跳转权限中心 | 无 |
| P0-04 | 有 AI 助手权限：正常进入 | 账号 `<account_with_chatbot>` | 直接访问 `/#/business/chatbot?bizs=<biz_id>` | 正常进入 AI 助手页；会话列表/深链行为不变 | 无 |
| P0-05 | 有 AI 助手权限：直链主机页不跳 chatbot | 账号 `<account_with_chatbot>`；浏览器新开/刷新 | 直接访问 `/#/business/host?bizs=<biz_id>` | 落在业务下主机页；URL 仍含 `bizs=<biz_id>`；**不**跳转到 chatbot | 无 |

### P1 - 异常 / 一致性（必测）

| ID | 场景 | 前置 | 操作 | 期望 |
|----|------|------|------|------|
| P1-01 | 无路由循环、无页面闪现 | P0-01 / P0-02 前置 | 完成直链或站内跳转 | 不出现 chatbot 页内容闪现后被踢回；不反复跳转 |
| P1-02 | 全流程无 biz_agent_assistant 申请入口 | P0-01～P0-03 | 观察落地页 | 任何路径下都不出现以 biz_agent_assistant 为权限点的「申请权限」页 |

### P2 - 细节（按需）

- [ ] 直链 URL 未带 `bizs` 时，回退主机页后业务 id 符合系统既有解析规则（localStorage / 默认业务）
- [ ] 有权限用户从 `/` 或 `/business` 进入时，默认进 chatbot 的既有逻辑不受影响
- [ ] 有权限用户冷启动直链 `/business/host` 留在主机页（见 P0-05）

## 验证结论

> 每条用例填写：PASS / FAIL / Skipped + 简要说明（FAIL 必须附复现步骤、错误截图链接或日志片段）

| ID | 结果 | 备注 |
|----|------|------|
| P0-01 | Pending | 待执行人验证：需「有业务访问、无 AI 助手权限」账号 |
| P0-02 | Pending | 同 P0-01 账号，站内跳转路径 |
| P0-03 | Pending | 待执行人验证：需「无业务访问」账号 |
| P0-04 | Pending | 可用当前开发账号（有 AI 助手权限）验证 |
| P0-05 | Pending | 可用当前开发账号验证：直链 `/business/host?bizs=<biz_id>` 应留在主机页 |
| P1-01 | Pending | 随 P0-01 / P0-02 一并观察 |
| P1-02 | Pending | 随 P0-01～P0-03 一并观察 |

- 执行日期: 2026-07-28（清单交付日；运行时验证待执行人补充）
- 总体结论: **Pending** —— 代码改动与 lint 已完成，运行时验证依赖无 `biz_agent_assistant` / 无 `biz_access` 的测试账号，开发环境暂不具备，用户确认先推进工作流收口，验证结果由执行人（自测或 QA）后续回填本表。
- 后续行动: 拿到测试账号后按 P0-01～P0-05 执行；若出现 FAIL，回退到 coding 阶段修复后重测（`bkdevbuddy wf set coding`）。
