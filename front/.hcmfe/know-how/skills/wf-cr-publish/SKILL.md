---
name: wf-cr-publish
description: 把代码评审报告回写到工蜂 MR / GitHub PR（行级评论 + 汇总结论），并在复审时拉取已有评论与上轮 finding 逐条对账。由 workflow-cr 在评审末步读取执行。
---

# CR 报告回写与复审对账

被 `workflow-cr` 读取执行。两个职责：**把报告发到 MR/PR 上**，以及**复审时还原上轮结果并对账**。

不做评审判断。发出去的内容以入口传来的报告为准，不要在这一步自行增删 finding。

## 前置闸口（硬性）

**未经用户明确同意，不得发出任何评论。** 评论发到的是团队其他人也会看到的 MR 上，且工蜂/GitHub 的评论删除成本不低。入口已经把报告展示给用户，你要确认拿到的是"用户已同意回写"的状态；不确定就再问一次。

批量发送前再报一次数量："将发出 N 条行级评论 + 1 条汇总"，让用户对规模有预期。

## 平台工具对照

| 动作 | 工蜂（gongfeng MCP） | GitHub（`gh` CLI） |
| --- | --- | --- |
| 行级评论 | `create_merge_request_note` | `gh api repos/{owner}/{repo}/pulls/{n}/comments` |
| 汇总结论 | `submit_mr_review_summary` | `gh pr review --comment` |
| 拉已有评论 | `search_merge_request_notes` | `gh api repos/{owner}/{repo}/pulls/{n}/comments` |
| 回复既有讨论 | `reply_merge_request_note` | `gh api repos/{owner}/{repo}/pulls/comments/{id}/replies` |

## 回写

### 行级评论

Block 与 Warn 逐条发行级评论，锚定到 `file:line`。单条格式：

```
**[Block] cross-module-deep-import**

跨模块引用了 resource-plan 的私有实现，改动方收不到任何提示。

修复：提升到 `@/components/obs-project-selector.vue` 后双向引用。

<!-- cr-fp: cross-module-deep-import|front/src/views/.../device-recycle/index.tsx|import ObsProjectSelector from '@/views/business/resource-plan/children/obs-project-selector.vue' -->
```

末尾的 HTML 注释是**复审对账的锚点**，必须带上，格式 `<!-- cr-fp: <principle>|<file>|<归一化 evidence> -->`。归一化 = 去首尾空白、连续空格压成一个。它**不含行号**——改动会让行号漂移，带行号的键在复审时必然对不上。

Info 不发行级评论，只进汇总。Info 是建议不是问题，逐条挂到代码行上会让 MR 页面变成噪音场。

行号锚定失败时（平台拒绝、行不在 diff 范围内）降级并入汇总，不要放弃该条。

### 汇总结论

一条汇总评论，含计数、Info 列表、原则提议、豁免清单：

```
## bkdevbuddy 代码评审

Block 2 · Warn 5 · Info 3 · 新增豁免 1

Block 与 Warn 已作为行级评论标注。

### 建议（Info，不阻断合入）
- `front/src/.../list.vue:88` 同一份数据做了三次 filter，可提取为 computed

### 原则提议
- 「重复遍历应合并为单次 computed」（id: `duplicate-list-filter`）— 待项目确认

### 本次豁免
- `form-item-consistency` @ `front/src/.../form.vue:42` — 三方组件无法改 props

---
认为某条是误报？直接回复该评论说明理由即可，复审时会记为驳回。
```

末尾那句不是客套。它是采纳率数据的**唯一采集入口**——开发者不需要学任何新操作，正常回复评论就完成了反馈。

工蜂上若同时要提交评审结论，用 `submit_mr_review_summary`；**不要**代替用户做"通过/不通过"的表决动作，除非用户明确要求。

## 复审对账

`--recheck` 时执行，顺序：

1. **拉已有评论**，筛出本引擎发出的（含 `cr-fp:` 锚点的），解析出上轮 findings 的复合键。
2. **与本轮 findings 按复合键匹配**：

| 判定 | 条件 | 动作 |
| --- | --- | --- |
| 已修复 | 上轮有、本轮无 | 回复原评论"已修复"，不再新发 |
| 未修复 | 两轮都有 | 回复原评论提醒，不新发重复评论 |
| 争议 | 开发者在该评论下回复了驳回意见 | 记为 rejected，**不再重复报出** |
| 新引入 | 本轮有、上轮无 | 正常新发行级评论 |

3. **输出对账表**给用户：

```
对账结果（较上轮）
已修复 4 · 未修复 1 · 争议 1 · 新引入 2

争议：cross-module-deep-import @ .../index.tsx
  开发者意见：该组件确实是共享性质，只是暂时放在 resource-plan 下
  → 记为驳回。同类驳回累积后应复审该原则的措辞或适用范围。
```

**用回复而不是新发**是这一步的关键。同一个问题在 MR 上出现两次，第二次开始就是噪音，几次之后整份报告都不会有人再看。

### 判断驳回

开发者的回复表达"不认同该 finding"就算驳回（"这里是有意为之"、"这个组件本来就是共享的"、"误报"）；表达"已改"或提问不算。

拿不准就当作**未驳回**，并在对账表里标出"回复语义不明确，请确认"。把有歧义的回复算成驳回，会让误报率数据虚高，进而误导后续该不该删掉某条原则。

## 注意

- 全程不改代码、不合并、不 approve MR。
- 平台调用失败（无权限、MR 已关闭、网络问题）如实回报并把完整报告呈现在对话里兜底，不要假装发送成功。
- 一次评审内避免对同一 `file:line` 发多条评论，同位置多个 finding 合并成一条，按最高 severity 标注。
- 对账判定结果交回入口，由入口统一调 `bkdevbuddy_code_review_record` 落盘；这一步**不要**自己写 `stats.json`，避免同一轮被记两次。
