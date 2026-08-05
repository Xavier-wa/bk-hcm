---
name: promotions-review
description: 评审并落成经验晋升候选（docs promotions）：起草/更新 rule 或 skill 到工具仓 know-how，人确认后 ship（工蜂 MR → 本地直写 → 仅草稿）。在 closeout 强提示用户确认后、或用户要求「评审 promotions / 经验晋升」时使用。
---

# 经验晋升评审

## 何时用

- closeout / `docs_status` 显示 `promotionQueue.totalOpen > 0`，且**用户确认**现在评审
- 用户主动要求「评审 promotions / 经验晋升」

## 步骤

1. `bkdevbuddy_docs_promotions_list`（可滤 pending / drafted / accepted）查看待处理项
2. 对每条：读 `summary` / `why` / `provenance` → **先搜**工具仓 `know-how/projects/<project>` 与 `know-how/public` 是否已有可改对象 → 决定 `create` 或 `update` + `kind` + `relativePath`
3. 写正文 → `bkdevbuddy_docs_promotions_draft`（update 时带 `baseDigest`）
4. **等人明确 accept 或 reject**（禁止自动 accept）→ `…_accept` / `…_reject`
5. 对 accepted：`bkdevbuddy_docs_promotions_ship({ via: 'auto' })`
   - 若本地已写入 → 告知在工具仓自行 commit/PR（或再问是否代开 MR）
   - 若 `needsGongfeng` → 用工蜂 MCP：建分支 → 写/改 `know-how/projects/...` → 开 MR → `ship` 传 `mrUrl`
   - 都失败 → 给出 sidecar `DRAFT.md` 拷贝指引
6. 汇总本轮：shipped / rejected / 仍 open

## 禁区

- 不阻塞 workflow `done`
- `docs.enabled=false` 时不要强行写入
- 不够格进 know-how 的直接 reject，不要硬塞
- 能 update 已有 rule/skill 时不要无故 create
