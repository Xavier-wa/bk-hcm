---
name: wf-docs-closeout
description: 文档收口：先 docs_status 算 gaps（touched 且 stub/stale），有缺口才更新 modules 并 deepen；无缺口可跳过正文。按触发器补漏询问是否 docs_promote（只进 pending）；待评审仅在用户确认后走 promotions-review。test.post required（存在 docs 底座且 docs 开启时）。当 pending hcm-docs-closeout 或 closeout 指向文档收口时使用。
---

# 文档收口 Skill（wf-docs-closeout）

## 使用时机

`test.md` 结论确认后、`wf next` 进入 `done` 之前，`stageNodes.post` 含 pending 的 `hcm-docs-closeout` 时执行。仅当存在 docs 底座且 `config.docs.enabled !== false` 时出现。

## 目标

**核对本 workflow 是否还有文档缺口并补齐**，刷新漂移基线。热写入（读后即写 / 改后同批）才是主沉淀路径；本 skill 是安全网，不是「第一次才写文档」的舞台。

## 操作步骤（AI 必须按序执行）

1. **取命中模块并算 gaps**
   - 收集本次分支 `changedFiles`（相对基线的 git diff）。
   - 调 `bkdevbuddy_docs_status({ changedFiles })`，读 `touchedModules`、各模块 `status` / `hasDrift`（或 `staleModules`）。
   - 定义 **gaps** = touched 模块中 `status == stub` **或** `hasDrift == true`（仍 stale）的集合。
   - 无 blueprint → 提示 `bkdevbuddy docs init` 后可 `workflow_node_skip`。

2. **按 gaps 决定是否写正文**
   - **gaps 为空**：跳过正文更新；准备 `note`：「中途已沉淀，收口无缺口」→ 跳到步骤 4（仍可做 promote 回顾）。
   - **gaps 非空**：**必须**更新每个 gap 模块的 `<dataDir>/docs/modules/<id>.md`（stub 补职责/关键流/接缝；已有则增量补结构变化）。需要独立明细时按约定写/更新 `modules/<id>/<topic>.md`，父文件只留 TOC 一行，禁止复制正文。
   - **禁止** gaps 非空时只 `node_complete` 不写不 deepen。
   - **禁止** gaps 为空时整篇重写「本迭代总结」。
   - **命中过窄**：hooks/components 等只落到 `base` → 在消费仓扩该业务模块 `globs`，再重跑 `docs_status`。例：`front/src/views/chatbot/**` 下的改动若命中的 hooks/components 只落进 `base`，应把这些路径补进 `chatbot` 模块的 `globs` 而非留在 `base`。

3. **刷新基线（仅当步骤 2 写过或仍有 gap 需 deepen）**
   - `bkdevbuddy_docs_deepen({ changedFiles })` 或按 gap id 逐个 deepen。
   - gaps 为空且中途已 deepen 过 → 本步可跳过。
   - **禁止**在未实际更新模块文档正文的情况下调用 `docs_deepen` / `bkdevbuddy_docs_deepen`（deepen 只用于刚写完之后刷基线，不能用来"消掉" stale/stub 状态而不写内容）。

4. **（补漏）经验捕获**
   - 只回看本迭代是否还有未问过的触发器命中：用户纠正 / 换打法 / 新约定 / 意外根因。
   - 命中 → 先问用户是否记下；同意 → `bkdevbuddy_docs_promote({ title, summary, why, kind })`，只回「已记下，之后可评审。」
   - 未命中 → 不问、不写。不要无差别记录每次改动。
   - **禁止**在本步开启 `promotions-review`。

5. **强提示评审（不阻塞 done）**
   - `promotionQueue.totalOpen > 0` → 向用户确认是否现在 `promotions-review`。
   - 禁止未确认 accept/ship。

6. **收口登记**
   - `bkdevbuddy_workflow_node_complete({ nodeId: 'hcm-docs-closeout', note })`；note 写明 gaps 为空或已补哪些 id。

## 分层职责（防复制）

| 位置 | 写什么 |
|------|--------|
| `modules/<id>.md` | 模块摘要、TOC、跨 topic 边界 |
| `modules/<id>/<topic>.md` | 该明细唯一正文 |
| `coding.md` | 仅本迭代改动/文件列表，不重述模块百科 |

## 例外与解闸

- 确无文档影响 → 用户同意后 `workflow_node_skip`（note 必填）。
- docs 关闭 / 无底座 → 节点不出现。

## 注意事项

- 只处理本 workflow 相关模块；遵守自检三问（见 docs-navigation）。
- 高信号指针，不贴大段源码；敏感信息用占位符。
