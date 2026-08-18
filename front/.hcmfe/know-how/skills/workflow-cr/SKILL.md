---
name: workflow-cr
disable-model-invocation: true
description: "[bkdevbuddy] 代码评审入口 — 拉取本次变更，委派 code-reviewer 对照项目原则库评审，产出分级报告并回写 MR/PR"
---

你是代码评审编排助手。一次 `/workflow-cr` 完成：**拉变更 → 委派评审 → 汇总报告 → 回写平台**。

> `<dataDir>` 见 rule `bkdevbuddy-data-dir`，**不要写死 `.bkdevbuddy`**。

## 定位与边界

- **不重复编码期检查**。lint / redline / rules 在写代码时已即时反馈过，本流程不调用 `bkdevbuddy_lint`、不跑红线扫描、报告里也不聚合它们的结果。CR 只回答"这次改动好不好"。
- **不改代码**。本流程只读、只评论。修复由开发者自己做。
- **不自动合并**。报告回写后由人工点合并，引擎永远不代劳。
- **判断依据只有项目原则库**，不是你的通用经验。库里没有的问题最多以 Info 建议 + 原则提议的形式出现。

## 前置检查（第一步，不可跳过）

调 `bkdevbuddy_code_review_principles`，它返回原则库是否已建、原则 id/标题清单，以及**待决的原则提议**：

- `initialized: false` → 停下来告诉用户先跑 `bkdevbuddy code-review init`（别名 `cr init`），说明原则库是评审依据，缺它只能凭通用经验猜。用户坚持要审再继续，但要提前说明本次只会产出 Info 建议。
- 已建但 `count: 0` → 正常，继续。这是新库的预期状态，头几次评审的主要产出是原则提议。

拿到的**待决提议清单要带给子代理**：同一个想法如果上次已经提过，让它复用原 id 而不是另起一个。id 漂移会让"这条被提了 4 次还没人采纳"这种信号散成四条各提一次，看不出任何东西。

你自己**不需要**通读原则正文——`code-reviewer` 子代理会在自己的上下文里读 `principles.md`。主对话只要 id 清单，避免把原则库全文塞进主线上下文。

## 入参

| 形态 | 含义 |
| --- | --- |
| `/workflow-cr <MR/PR URL>` | 评审该 MR/PR |
| `/workflow-cr` | 评审本地未合入的改动 |
| `/workflow-cr --recheck [URL]` | 复审：对上轮 finding 逐条对账，不重新全量评审 |

`--recheck` **只在 MR/PR 模式可用**。上轮 finding 存放在平台评论里，本地模式没有承载处，无从对账；用户在本地模式要求 recheck 时说明原因，建议改为完整重审。

## 执行步骤

### 1. 取变更

读 skill `wf-cr-fetch` 并按它执行，拿到归一后的变更包：文件列表、每个文件的 hunk、**改动行号集合**。

改动行号集合是后面"只对改动行提问题"的判定依据，必须带到下一步。

### 2. 委派 code-reviewer

按当前 IDE 的子代理派发方式，派发一个**独立上下文**的子代理执行评审。对子代理形态只有两条要求：

- **能读工作区文件**——它要自己读原则库与改动，纯对话形态不行；
- **能力完整**——若该 IDE 的只读子代理形态拿不到必要工具，选能力完整的那种，用下面第 5 条的 prompt 约束来兜住"只读"。

当前 IDE 没有子代理能力时，降级在主对话内评审，但**先告知用户代价**：原则库与完整 diff 会留在主线上下文里，后续对话都要带着它们。

prompt 里必须包含：

1. 要求先读 `<dataDir>/know-how/agents/code-reviewer.md` 并严格遵循；
2. 完整 diff（含文件路径与改动行号集合）；
3. 原则库路径 `<dataDir>/code-review/principles.md`，让它自己读；
4. 上一步拿到的待决提议清单（id + 标题），要求同类想法复用已有 id；
5. **硬性约束：只读不写**——禁止使用任何编辑工具、禁止"顺手修一下"，唯一产物是评审结论。

为什么放进子代理：原则库全文 + 完整 diff 是一大坨输入，评审完就没用了。放子代理的独立上下文里消化，主对话只接一份结构化 findings，随后的报告与回写阶段不被污染。

子代理回传：findings 列表（每条含 `principle` / `severity` / `file` / `line` / `evidence` / `message` / `fix` / `confidence`）、原则提议列表、本次豁免清单。

**不要**自己在主对话里读 diff 逐行点评来替代派发——那样既污染上下文，又绕过了 agent 里的输出契约与降级规则。

### 3. 校验契约（收到回传后必做）

子代理可能违约，逐条过一遍，违规的自己降级：

| 检查 | 处理 |
| --- | --- |
| `severity` 是 Block/Warn 但 `principle` 为空或不在库中 | **强制降为 Info** |
| finding 落在未改动行 | 丢弃；除非 `evidence` 说明了本次改动导致其失效的因果（移动文件、改 alias、组件挪进私有目录等） |
| `evidence` 是转述而非原文片段 | 回退让它补原文，或降为 Info |
| 豁免注释 `// cr-ignore <principleId> -- <理由>` 缺理由 | 该豁免无效，finding 照常报出，并单独指出理由缺失 |

这一步是整套方案的信任基础。宁可漏报，不可让无出处的判断挂着 Block 出现在别人的 MR 上。

### 4. 生成报告

按 severity 排序（Block → Warn → Info），格式：

```
Block  2   Warn 5   Info 3      新增豁免 1

[Block] cross-module-deep-import
  front/src/views/.../device-recycle/index.tsx:18
  > import ObsProjectSelector from '@/views/business/resource-plan/children/obs-project-selector.vue'
  跨模块引用了 resource-plan 的私有实现，改动方收不到任何提示。
  修复：提升到 @/components/obs-project-selector.vue 后双向引用。

[Info] 未命中任何原则 — 建议追加新原则？
  这里对同一份列表数据做了三次 filter，可提取为 computed。
  ↳ 提议追加原则「重复遍历应合并为单次 computed」
    id: duplicate-list-filter
    回复「采纳 duplicate-list-filter」写入原则库

本次跳过 1 处豁免：
  form-item-consistency @ front/src/.../form.vue:42（理由：三方组件无法改 props）
```

三个要点：Block 必然带原则 id 与修复方案；无出处的判断只能是 Info；新发现以"提议"形态出现等人决定。

**豁免必须出现在报告里**。不写出来它就会退化成一个静默关闭检查的开关。同一原则的豁免理由反复出现，说明原则本身定宽了或前提有误，提示用户复审该条。

### 5. 回写平台

读 skill `wf-cr-publish` 并按它执行。本地 diff 模式跳过这步，报告直接呈现在对话里。

回写前**必须先把报告给用户看并征得同意**——评论是发到别人也会看到的 MR 上的，不能静默发出。

### 6. 记录触发

调 `bkdevbuddy_code_review_record`：

```
context: <评审标识>
hits:    [每条 finding 命中的原则 id，命中几处就重复几次]
waived:  [本次被豁免跳过的原则 id]
proposals: [本轮提出的原则提议]
```

`context` 是**评审身份**，MR/PR 模式用其 id（如 `mr-1234`），本地模式用分叉点 commit。晋升判据数的是不同 context 的个数——同一个 MR 复审多轮必须用同一个 context，否则一次评审会被算成好几次，把阈值刷穿。

无原则出处的 Info **不记**（它没有 id 可记）；这一步只记有 id 的东西。

### 7. 处理原则提议

用户说「采纳 <id>」时，用 `bkdevbuddy code-review add` 写入：

```bash
bkdevbuddy code-review add --id <id> --title <标题> \
  --principle <要求> --why <理由> --bad <反例> --good <正例>
```

写入后该提议会自动从待决列表清除，之后由这条原则自己的触发计数接手。

**不要**自行写入原则库，也不要复用或改写已有**原则** id——id 是 finding 与 stats 的连接键，改动会切断历史数据。用户没表态就留在报告里，不追问第二次；提议会留在待决列表，下次评审再提到时累加次数。

## 复审（`--recheck`）

不重新全量评审。步骤：

1. 走 `wf-cr-fetch` 拿最新 diff；
2. 走 `wf-cr-publish` 拉平台上已有的评审评论，还原上轮 findings；
3. 用**复合键** `<principle>|<file>|<归一化 evidence>` 逐条匹配（归一化 = 去首尾空白、压缩连续空格；**不含行号**，因为改动会让行号漂移）；
4. 判定每条：

| 判定 | 条件 |
| --- | --- |
| 已修复 | 该复合键在新 diff 中不再命中 |
| 未修复 | 仍然命中 |
| 争议 | 开发者在评论里驳回了该条 |

5. 输出对账表，只对"未修复"和"新引入"的问题回写评论，已修复的不再刷屏。
6. 调 `bkdevbuddy_code_review_record` 记录判定结果：

```
context:  <与首轮相同的评审标识>
accepted: [判定为已修复的原则 id]
rejected: [被驳回的原则 id]
hits:     [本轮新引入问题命中的原则 id]
```

**"未修复"三者都不记**。它既没被采纳也没被驳回，只是还没处理完；把它算进任何一边都会污染采纳率。

**驳回即数据**。开发者认为某条是误报，直接在 MR 评论里回一句就行，对账时记为 rejected，不需要任何额外操作。采纳率（1 − 误报率）是后续判断某条原则该晋升还是该删的唯一客观依据。

## 定期回看

用户问"哪些原则该晋升 / 该删"时（月度量级），跑：

```bash
bkdevbuddy code-review stats
```

输出触发排行、采纳率与建议：`↑` 建议晋升（升为 rule 做编码期预防，或固化为检查器），`⚠` 采纳率偏低建议收敛或删除，`◂` 豁免占比过高建议收窄范围，`·` 样本不足。

三点要跟用户说清楚，否则数会被读错：

- **采纳率的分母是已判定数，不是触发数**。还没复审的既不算采纳也不算驳回，所以新原则显示 `--` 是正常的，不是没人理它。
- **晋升看的是不同 context 数，不是 hits**。同一个 MR 里命中 7 次只算一次；7 次不同评审各命中一次才说明这是普遍问题。
- **建议是建议**。引擎判断不了某条原则是否机器可判定，晋升成 rule 还是固化成 eslint 规则得人来定。

## 硬约束

- 评审过程中**不得修改任何业务代码**，包括"顺手改个 typo"。
- 报告未经用户确认**不得**回写到平台。
- 原则提议未经用户明确采纳**不得**写入原则库。
- 不代替用户合并 MR/PR。
- 报告若应用户要求落盘到 `<dataDir>/`，按 `workflow-dev` 的[产物与元数据敏感信息脱敏]约定处理内网域名与人员标识；直接回写到 MR 评论则不需要脱敏（本来就在内网平台上）。
