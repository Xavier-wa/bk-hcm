---
name: workflow-test-checklist
description: 在 test 阶段为新功能产出可执行的手测验证清单 (P0/P1/P2 用例 + 数据准备 + 期望结果), 落到 .hcmfe/workflow/<id>/ 下。当 hcmfe wf 进入 test 阶段或需要为某个 feature 设计验证用例时使用。
---

# Test Checklist Skill

## 使用时机

工作流进入 `test` 阶段时（或用户明确要求“补一份验证清单 / 列一下要测哪些用例”时），按本 skill 完成下面这件事：

- 产出**人可读、可执行**的手测验证清单 `test.md`，覆盖本次改动的 P0 / P1 / P2 场景，留出"验证结论"小节让自测人 / QA 写回结果。

> 历史上这个 skill 还会产出 Playwright 脚本骨架并尝试自动跑 E2E，由于内部环境 SSO + 域名鉴权的限制可实施性差，**已下线 E2E 能力**。如果将来要补回自动化，建议独立到 verify-e2e skill，不在 test 阶段强制要求。

## 输入

调用本 skill 前应已读到当前 workflow 的：

- 用户故事（来自 `.hcmfe/workflow/<id>/prd.md`）
- 关键 UI 元素 / 交互（来自 `.hcmfe/workflow/<id>/design.md`）
- 涉及的接口（来自 `.hcmfe/workflow/<id>/api.md`）
- 已实施方案（来自 `.hcmfe/workflow/<id>/coding.md`，如果存在）

如果不确定，先调 `hcmfe_workflow_status` 拿到 `state.artifacts`，再读对应文件。缺失就回到对应阶段补。

## 产物路径（强制）

| 文件 | 路径 | 是否必需 |
|---|---|---|
| 手测清单 | `.hcmfe/workflow/<id>/test.md` | ✅ 必需，是 test 阶段的硬性产物 |

> 不要把这份文件放到 `tests/`、`docs/` 或 `e2e/` 目录。

## 模板

- `./assets/test-checklist-template.md` —— `test.md` 模板

> 这份模板在 know-how 同步后位于 `<projectRoot>/.hcmfe/know-how/skills/workflow-test-checklist/assets/`，可直接读出来作为初始内容。

## 操作步骤（AI 必须按序执行）

1. **读上下文**
   - 调 `hcmfe_workflow_status` 拿当前 `id` 与已登记产物
   - 读 `.hcmfe/workflow/<id>/{prd,design,api,coding}.md`（缺失的可跳过），列出本次改动的核心交互、接口和已实施范围
   - 若 `design.md` 与 `coding.md` 冲突，以实际已实施的 `coding.md` 为准，并在 `test.md` 备注中指出设计文档需要修正
   - 验证项只能覆盖本次实际落地的行为，**不要**把 PRD 中未实施的设想写成验收项

2. **写 `test.md`**
   - 用 Cursor 的 Write 工具创建 `.hcmfe/workflow/<id>/test.md`
   - 初始内容用 `assets/test-checklist-template.md`，把 `<workflow-id>` 替换成当前 `id`
   - 用上一步的上下文填 P0 / P1 / P2 用例（描述用动宾短语，断言针对业务语义）
   - "测试环境"小节中的前端入口写占位说明，**不要**把具体测试域名 / 个人开发地址写入仓库
   - 调 `hcmfe_workflow_artifact_add` 把它登记为 test 阶段产物：
     ```
     stage: test
     ref:   .hcmfe/workflow/<id>/test.md
     ```

3. **执行 / 分配**
   - 询问用户本次是**自测**还是**交给 QA**
   - 自测情境下，AI **不替代用户操作浏览器**，让用户按 P0 / P1 用例操作；用户可让 AI 准备测试数据 / 提供 mock 接口建议
   - 如果是 QA 测试，把 `.hcmfe/workflow/<id>/test.md` 链接 / 内容贴给 QA 即可

4. **回写结论**
   - 用户验证完成后，把每条用例的结果（PASS / FAIL / Skipped + 备注）写回 `test.md` 的“验证结论”小节
   - 失败用例需要在 coding 阶段修复后重测，可由用户决定是否回退到 coding 阶段（`hcmfe wf set coding`）

5. **推进**
   - 验证结论补齐后，明确询问用户是否确认本次测试结果可作为发布 / 合入依据
   - 用户确认后调用 `hcmfe_workflow_approve`（stage=test），再 `hcmfe_workflow_next` 进入 done
   - 被拒就按 `reasons` 补齐再试

## 注意事项

- 用例标题写动宾短语：✅ "新建 VPC 后回到列表并出现新条目" ❌ "test1"
- 断言要表达业务语义（如"列表新增 1 行"），而不是 DOM 结构（如"出现一个 div.row"）
- 接口失败场景在数据准备里写明 mock 方式（`page.route` / 后端预置错误账号 / 自行构造异常数据），不要依赖真实后端造异常
- 验证范围只覆盖**本 workflow 改动**，不要回归整个模块
- 不要把测试域名、个人 dev 地址写到 `test.md` 里；前端入口用占位描述（"对应业务的测试环境地址"），实际地址在执行时由用户口头提供
- 数据敏感字段（账号 / 密码 / token）一律用占位符 `<test_account>`，绝不要把真实敏感信息提交到仓库
