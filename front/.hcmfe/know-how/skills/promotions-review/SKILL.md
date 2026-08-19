---
name: promotions-review
description: 评审并落成经验晋升候选（docs promotions）：起草/更新 rule 或 skill 到工具仓 know-how，人确认后 ship，再默认分支+commit（可 push），开 MR 前必须再确认一次。在 closeout 强提示用户确认后、或用户要求「评审 promotions / 经验晋升」时使用。
---

# 经验晋升评审

## 何时用

- **用户明确要求**「评审 promotions / 经验晋升」，或 closeout 里用户确认现在评审
- `docs_status` / stop hook 出现待评审计数 **不是**启动条件；捕获成功（`docs_promote` 返回 notice）**不是**启动条件
- **禁止**在刚 `docs_promote` 之后自动进入本 skill

## 步骤

1. `bkdevbuddy_docs_promotions_list`（可滤 pending / drafted / accepted）查看待处理项
2. 对每条：读 `summary` / `why` / `provenance` → **先搜**工具仓 `know-how/projects/<project>` 与 `know-how/public` 是否已有可改对象 → 决定 `create` 或 `update` + `kind` + `relativePath`
3. 写正文 → `bkdevbuddy_docs_promotions_draft`（update 时带 `baseDigest`）
   - **create**：`draftMarkdown` = 完整 know-how 正文（可进 `DRAFT.md`）
   - **update**：`draftMarkdown` = **短增量意图**（改哪、加什么）；完整修订稿用 `shipContent`（落 gitignored `SHIP_BODY.md`），**禁止**把目标文件全文塞进 `DRAFT.md`
4. **等人明确 accept 或 reject**（禁止自动 accept）→ `…_accept` / `…_reject`
5. 对 accepted：`bkdevbuddy_docs_promotions_ship({ via: 'auto' })`（update 若无 `SHIP_BODY` 则传 `content` 全量正文；**勿**回写膨胀版 `DRAFT.md`）
   - 本地已写入，或 `needsGongfeng`（用工蜂写入同一批文件）：写入方式不同，**后续落仓 + 确认门 + MR 相同**（见下节）
   - ship / 写权限 / 工蜂均失败 → 降级给出 sidecar 手工步骤（意图在 `DRAFT.md`），并标明卡点；不要假装已落仓
6. **工具仓落仓与 MR**（accept 且 ship/写入成功后默认连续执行，勿停在「请自行 commit」）
7. 汇总本轮：shipped / rejected / 仍 open；若停在确认门则标明「待用户确认是否开 MR」

## Sidecar 约定

- 持久化 `ship`：只写可移植字段（`via` / `at` / `knowHowPath` / `mrUrl`）。**禁止**把本机绝对路径写入 `ship.json` / `meta` / `promotions.jsonl`（不得进消费仓 git）。
- update：`DRAFT.md` 长期只保留增量意图；全量正文仅 ship 瞬时输入（`shipContent` / `content` / 内存），写入工具仓后不必也不应回写膨胀版 `DRAFT.md`。
- create：可用全文 `DRAFT.md`。

## 工具仓落仓与 MR

用户确认 accept（或「都 accept」）且写入成功后，Agent **默认**连续做完下列清单，无需再问「是否直提 master / 用什么分支名」：

| 步骤 | 行为 |
|---|---|
| 切到/新建正确分支 → commit（只含本轮 promotion 改动）→（推荐）`push -u` | **默认执行**，无需再问 |
| 创建 MR | **必须再确认一次** 后再执行 |
| hard reset 本地默认分支（如 master） | **永远**需用户明确授权 |

### 默认清单

1. 在工具仓切到/新建分支（**禁止**直提 `master` / 默认分支）
2. commit（仅本轮 promotion 相关文件；可用 `docs(know-how): …`，与分支前缀无关）
3. （可选但推荐）`push -u` 推远程分支
4. **确认门**：停住，展示拟开 MR 摘要后询问（见下）
5. 仅当用户明确同意后，用工蜂创建 MR，并回复 MR URL

`needsGongfeng` 时：先用工蜂在正确分支上写/改 `know-how/projects/...`（或等价本地写入），再进入同一套「确认门 → 用户同意后开 MR」；不要在未确认时把「写文件」和「开 MR」绑成静默一步。

### 确认门话术

展示并明确询问「是否现在创建 MR？」，至少包含：

- 分支名
- target（默认分支，如 `master`）
- 拟用标题
- 改动文件列表

**视为已确认、可直接开 MR**的说法举例：「开 MR」「确认」「提交并创建 mr」。  
用户若只说「帮我 commit」：做完分支 + commit（可 push）后仍停在确认门，**不要**擅自开 MR。

### 分支命名

格式：`asset/<project>-<concrete-change-slug>`

- `asset/` = 消费项目 know-how 资产（非引擎功能、非引擎文档）
- slug 落到本轮具体规则/主题；同一次多条 promo 可合并一条语义 slug
- **禁止**：`feat/`、`docs/`、泛化名（如 `know-how-promotions`）

正例：`asset/hcm-fe-api-docs-boundary-handle-naming`  
反例：`feat/hcm-fe-know-how-promotions`、`docs/...`、`asset/hcm-know-how-promotions`

### 降级

ship 失败 / 无写权限 / 工蜂不可用 → 给出可执行的手工步骤（目标路径、分支建议、拟 MR 摘要），并标明卡在哪一步；不要空泛说「请自行提交」。

## 禁区

- 不阻塞 workflow `done`
- `docs.enabled=false` 时不要强行写入
- 不够格进 know-how 的直接 reject，不要硬塞
- 能 update 已有 rule/skill 时不要无故 create
- 禁止直提工具仓默认分支；禁止泛化分支名
- 禁止 accept/ship 成功后只甩「请自行 commit/PR」而不做默认落仓
- 禁止未过确认门就创建 MR
- 禁止未经明确授权 hard reset 本地默认分支
- 禁止 update 把完整目标文件写入消费仓 `DRAFT.md`；禁止 sidecar 持久化本机绝对路径
