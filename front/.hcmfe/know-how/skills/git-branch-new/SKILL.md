---
name: git-branch-new
disable-model-invocation: true
description: >-
  [bkdevbuddy] 基于指定远端分支创建本地开发分支：先 fetch 基线，用选择器确认基线与分支名，展示计划确认后才 checkout。
  只在本地创建，不 push、不建 upstream。分支名同时是 workflow / requirement id 的来源（引擎按 slugify(branch) 推导），
  故必须 ASCII kebab-case。仅显式调用。
---

# Git 新建分支（基于远端基线 / bkdevbuddy）

基于**指定远端分支**创建本地开发分支。典型时机：拿到 TAPD 单、准备开工，且**还没** `bkdevbuddy wf init` —— 因为工作流 id 默认从当前分支名推导，分支要先于工作流存在。

## 硬性约束

1. **禁止自动调用**：仅用户显式调用本 skill（或工作流按下方 [与工作流的关系] 引用）时执行。
2. **只本地创建**：**禁止** `git push` / 不建 upstream / 不 `git remote add` / 不改 git config。推送由用户后续自行决定（需要时走 `git-commit` skill 的推送门禁）。
3. **基线必须是远端分支**：形如 `<remote>/<branch>`，且**创建前必须 fetch**；禁止基于本地可能过期的同名分支拉新分支。
4. **默认 `--no-track`**：基线通常是共享集成分支（`master` / `develop` / `release/*` / 长驻 `feature-*`），把 upstream 落在它上面会让后续推送误直推共享分支（见 `git-commit` C2 门禁）。仅用户明确要求跟踪时才去掉 `--no-track`。
5. **不动工作区（默认）**：不擅自 `git stash` / `git checkout -f` / 丢弃未提交改动；有改动时按 [步骤 5] 让用户选。仅当用户明确选择「由 agent stash」时，才按该选项执行 stash（含后续是否 pop）。
6. **默认停在计划确认**：展示「基线 + 分支名 + 将执行的命令 + 推导出的 workflow id」，用户明确确认后才执行。

## 与工作流的关系

分支名不只是分支名，它是 workflow / requirement 的**身份来源**：

- `wf init` 未显式传 id 时，引擎用当前分支推导：`getBranchSlug()` = `slugify(branch)`，即**转小写 + 非 `[a-z0-9]` 字符段替换为 `-` + 去首尾与重复 `-`**，全部字符都不合法时退化为字面量 `workflow`。
- `resolveWorkflowId()` 优先级：**显式 id > 当前分支 slug > 仓库内唯一工作流**。分支切走后，靠分支名解析的工作流就找不到了。

由此得到的执行顺序与规则：

1. **先建分支，再 `bkdevbuddy wf init`**（本 skill 不代调 `wf init`，只在结束时提示）。
2. 分支名必须满足 [分支名规范]，让 `slugify(branch) === branch`——这样分支名与工作流目录名 `<dataDir>/workflow/<id>/` 完全一致，人和引擎看到的是同一个标识。
3. **中文分支名一律拒绝**：`slugify` 会把中文清空，多条需求都会退化成同一个 `workflow` id 并互相覆盖。
4. 若仓库已有工作流且用户是**在已有需求中途**切新分支：先提示「原工作流 `<id>` 之后需要显式传 id 才能解析」，让用户确认再建。

**当前不挂 `workflow-nodes.json` 节点**：本 skill 由用户或 agent 在 `wf init` 前显式调用。后续若要与工作流结合，倾向做成**软闸**（如 `workflow_status` 检测到当前处于共享分支、或分支名不适合当 id 时，提示调用本 skill），而不是加 pre 节点强制拦截。

## 执行步骤

### 1. 采集事实（只读，一次跑完）

```bash
git rev-parse --abbrev-ref HEAD                 # 当前分支
git status --porcelain                          # 未提交改动
git remote -v                                   # 远端清单
git branch -r                                   # 远端分支候选
git symbolic-ref -q refs/remotes/origin/HEAD    # origin 默认主干（可能为空）
git branch --sort=-committerdate                # 本地分支（查同名冲突）
```

### 2. 选基线远端分支（选择器）

从 `git branch -r` 生成候选，**不要**让用户裸手输：

- 排序：`origin/HEAD` 指向的主干 → `master` / `main` / `develop` → `release/*` → 其余按名称；末尾附「自定义输入」。
- 默认高亮 `origin` 的主干分支；多远端时基线**默认取 `origin`**（个人 fork 通常滞后）。
- 校验：必须是 `<remote>/<branch>` 形态且存在于 `git branch -r`；不满足则重新询问，**不猜**。
- 用户已在调用时点名基线（如 `基于 origin/develop`）→ 跳过选择器，仅做存在性校验。

### 3. Fetch 基线

```bash
git fetch <remote> <branch>
```

失败（网络 / 无权限）→ 报告原文并停止，**不要**退化成基于本地旧引用创建。

### 4. 定分支名（选择器）

候选来源，按可得性给出：

| 来源 | 说明 |
|------|------|
| TAPD 单 | 已绑定 / 用户给了单据时，用**标题的英文语义化**结果，如 `feat-issue-status-webhook`；**不要**塞入 TAPD 短 ID。标题为中文时自行译成语义英文短语，不要拼音、不要机器直译长句 |
| 用户描述 | 用户口述的任务语义，如「修 profiling tooltip」→ `fix-profiling-tooltip` |
| 已有 workflow id | 用户是给既有工作流补分支时，直接沿用其 id，保证两者一致 |
| 自定义 | 用户直接给名字，仅做 [分支名规范] 校验 |

**冲突处理**（本地已有同名分支，或基线远端已有同名分支）：给选择器——换个名字 / 切到现有分支（不新建）/ 删除重建（**仅本地分支**可删，且需用户明确确认；远端同名分支一律不删）。

### 5. 未提交改动处置

`git status --porcelain` 非空时，先列出改动文件，再让用户四选一，**不擅自决定**：

1. 带着改动切到新分支（默认；`git checkout -b` 会保留工作区改动，若与基线冲突会报错，此时如实报告让用户处理）；
2. **由 agent stash 后再建分支**：`git stash push -u -m "git-branch-new: before <new-branch>"` → 按计划建分支 → 再问用户是否 `git stash pop` 到新分支（同意才 pop；冲突则停下来列冲突文件，不 `--force` / 不丢弃）。stash 失败则中止，不建分支。
3. 先在当前分支提交 / 处理完再回来建分支（本 skill 中止）；
4. 取消。

选了选项 2 时，步骤 6 的计划里必须写明 stash message 与「建完后是否 pop（待二次确认）」。

### 6. 展示计划（默认停在这里）

````text
新建分支计划（确认后执行）

基线：origin/master（已 fetch，HEAD=abc1234）
分支名：feat-issue-status-webhook
workflow id（slugify 推导）：feat-issue-status-webhook  ← 与分支名一致 ✓
工作区：干净
命令：git checkout -b feat-issue-status-webhook --no-track origin/master

回复「确认」执行；回复「取消」中止。
````

推导出的 id 与分支名不一致时（如名字里有 `/` 或大写），必须显式列出两者并建议改名。

### 7. 执行与校验

```bash
git checkout -b <new-branch> --no-track <remote>/<base>
git status -sb
git log --oneline -1
```

完成后一句话汇报：新分支名、基线与其 commit、upstream 未设置、**下一步建议** `bkdevbuddy wf init`（附推导出的 id）——但**不要**自动调 `wf init`。

## 分支名规范

- 正则：`^[a-z0-9]+(-[a-z0-9]+)*$`（纯小写 + 数字 + 单连字符分隔）。
- 建议 `<type>-<语义短语>`，`type` 取 `feat` / `fix` / `refactor` / `chore` 等（与 `git-commit` 的 type 一致）。**不要**塞入 TAPD 短 ID / 长 ID。
- 语义短语用完整、可理解的英文词；**除常见缩写**（如 `api` / `ui` / `url` / `http` / `oauth` / `jwt` / `css` / `html` / `rpc` / `mq`）外，**不主动**用不直观的缩写（如 `iss-sts-wh`、`prf-ttp`）。
- 长度：整名（含 `type-`）优先 ≤ 36 字符。仅当完整语义短语导致整名 **> 36** 时，才考虑缩短——优先删次要修饰词，其次才用仍可辨认的缩写；缩短后仍须让人一眼看懂主题。
- **不要** `/`（`slugify` 会转成 `-`，分支名与 workflow id 不再一致）、不要大写、不要下划线、不要中文、不要 `#` 等 TAPD ID 前缀符号。

## 反例（禁止）

- 未展示计划就 `git checkout -b`
- 创建后顺手 `git push` / `git push -u`（本 skill 只本地创建）
- 省略 `--no-track`，把 upstream 落在 `origin/master`、`origin/develop`、`release/*`、长驻 `feature-*` 等共享分支
- 跳过 `git fetch`，基于本地过期引用创建
- 用中文或带 `/`、大写的分支名（会破坏 `slugify` 推导出的 workflow id）
- 分支名塞入 TAPD 短 ID / 长 ID（如 `feat-135926589-...`）
- 未超长却主动用不直观缩写（如 `feat-iss-sts-wh`）
- 有未提交改动时擅自 `git stash` / `git checkout -f`（未选「由 agent stash」）
- 选了 stash 却在建分支前 stash 失败仍继续 checkout，或未经二次确认就 `stash pop` / 冲突时强制丢弃
- 同名冲突时默认删除重建，或删除远端同名分支
- 代替用户调 `bkdevbuddy wf init`
