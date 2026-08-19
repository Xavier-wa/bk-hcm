---
name: git-commit
disable-model-invocation: true
description: >-
  [bkdevbuddy] 按 Conventional Commits + TAPD 约定起草并确认后执行 git commit。
  需求 feat: <TAPD标题> --story=<短ID>；缺陷 fix: <TAPD标题> --bug=<短ID>；一单一 commit。
  仅显式调用；默认只列可编辑待提交清单，确认后才 commit，成功后再问是否 push。
---

# Git Commit（约定式 + TAPD / bkdevbuddy）

配合 `workflow-dev`：从当前工作流已绑定的 TAPD 单据取标题与 ID，按**一单一 commit**起草提交；无 TAPD 时退回纯约定式提交。

## 硬性约束

1. **禁止自动调用**：仅用户显式调用本 skill 时执行。
2. **禁止默认 commit / push**：先展示可编辑待提交清单；用户明确「确认提交」后才 `git commit`；成功后再问是否 push，明确同意才推。
3. **一单一 commit（重点）**：工作流绑定了多张 TAPD 单时，**必须**按单拆成多条 commit；禁止一条 message 带多个 `--story` / `--bug`，禁止把多张单的改动混进同一 commit（跨单公共文件见下方拆分规则）。
4. **默认不加 scope**：subject 用 `feat: ...` / `fix: ...`，**不要**写 `feat(scope):`；仅当用户在清单里手改加上 `(scope)` 时才保留。
5. **安全**：不改 git config；不用 `-i`；不 `--no-verify`（除非用户明确要求）；不 force push main/master；不提交疑似密钥（`.env` 等）——用户点名则警告。

## 提交信息格式

遵循 [约定式提交 1.0.0](https://www.conventionalcommits.org/zh-hans/v1.0.0/)：

```text
<type>: <description>
```

默认**无** `[optional scope]`。

### TAPD 需求单（story）

```text
feat: <TAPD需求标题> --story=<短ID>
```

示例：`feat: webhook方式同步issue状态 --story=135926589`

- `description` **默认**取 TAPD `stories_get` 返回的 `name`（标题）；用户可在清单中改写。
- `--story=` 后必须是 **短 ID**（见 [短 ID]），不是 19 位长 ID。

### TAPD 缺陷单（bug）

```text
fix: <TAPD缺陷标题> --bug=<短ID>
```

示例：`fix: Profiling 火焰图滚动时关闭 tooltip --bug=160034346`

- `description` **默认**取 TAPD `bugs_get` 返回的 `title`；用户可改写。
- `--bug=` 后必须是 **短 ID**。

### 非需求 / 非缺陷

无 TAPD 关联时，按 diff 推荐 `type`，**不加** `--story` / `--bug`，仍默认不加 scope：

| type | 何时 |
|------|------|
| `feat` | 新功能 |
| `fix` | 修 bug（无单） |
| `docs` / `style` / `refactor` / `perf` / `test` / `build` / `ci` / `chore` / `revert` | 见约定式提交常用类型 |

## TAPD 关联（bkdevbuddy）

### 1. 解析当前绑定单据

优先从工作流读取（与 `workflow-dev` / `workflow-contract` 一致）：

1. 调 `bkdevbuddy_workflow_status`（或等价 CLI），读取 `tapd` / `getTapdItems()`：
   - `workspaceId`
   - `objectType`：`story` | `bug`
   - `itemId`：**19 位长 ID**（`tapd_link` 写入的是长 ID）
2. 多单：`tapd.items[]`（或 status 返回的多条目）→ **每个 item 一条拟提交**。
3. 若未绑定 TAPD：再查分支名 / 用户本轮给出的 ID / 对话上下文；仍无则走「非需求/非缺陷」。

### 2. 拉标题

对每张单用 TAPD MCP（直连或 `lookup_*` + `proxy_execute_tool`）：

| objectType | 工具 | 标题字段 | 查询 |
|------------|------|----------|------|
| story | `stories_get` | `name` | `workspace_id` + `id`（19 位长 ID）+ 建议 `fields=id,name` |
| bug | `bugs_get` | `title` | `workspace_id` + `id` + 建议 `fields=id,title` |

标题过长时：保留语义，可截到约 50～72 字并在清单备注「已截断」；**不要**擅自加 scope 或改 type。

### 3. 短 ID（`--story` / `--bug` 必用）

TAPD MCP 只有 **短→长**：`tapd_id_get(short_id, type=story|bug|task)`。commit 需要 **短 ID**，按下面推导（**待你实测校准**；技能内勿写死未验证的魔法数到其它模块）。

#### 判定

- 长度 **≤ 9** 且为纯数字 → 已是短 ID，直接用。
- 长度 **= 19** 且为纯数字 → 长 ID，需转短。
- 其它 → 清单标「待确认」，询问用户。

#### 长 → 短（候选算法，需回环校验）

业界常见编码（与 MCP 文档示例形态一致：`1` + `workspace_id` 补齐 9 位 + `short_id` 补齐 9 位 = 19 位）：

```text
long_id = "1" + workspace_id.padStart(9, "0") + short_id.padStart(9, "0")
short_id = String(Number(long_id.slice(-9)))   // 去掉短 ID 段前导 0
```

**回环校验（推荐每次对长 ID 做一次）**：

1. 用上式得到 `candidateShort`。
2. 调 `tapd_id_get({ short_id: candidateShort, type: story|bug })` 得 `resolvedLong`。
3. `resolvedLong === 原长 ID` → 采用 `candidateShort`。
4. 不一致或 MCP 失败 → **不要瞎用长 ID 写进 `--story/--bug`**；在清单里写出长 ID + 候选短 ID，标「短 ID 待确认」，等用户改或确认后再提交。

> 用户实测后若算法有修正，只改本 skill 本节即可。

#### 短 → 长（仅查 TAPD 时）

已有短 ID、需 `stories_get` / `bugs_get` 时：先 `tapd_id_get`，再用返回的 19 位 ID 查询。

## 文件归属与一单一 commit

1. 读 `git status` / `git diff` / `git diff --staged` / `git log -8 --oneline`。
2. 调 `bkdevbuddy_workflow_status`，若返回 `codingItemStructure.applicable === true`：
   - **`ok === false`**（缺 `## 单据` 节或节内无匹配 `**TAPD**`）：停止起草按单 commit；展示 `structureProblems` + `remediation`，先按 workflow-dev 模板补单据节再重跑。
   - **`ok === true`**：按单一条拟提交。`files:` 优先级：
     1. 该单 `items[].files` 非空（来自 coding.md `**文件**`）→ **优先**用这些路径（与工作区实际变更取交集，避免提交已不存在的路径）；
     2. 该单无路径或明显过时 → **按 git diff + 单据标题/`**改动点**` 描述**建议分组；
     3. 最终以用户确认的可编辑清单为准（唯一保证）。
   - 有 `uncoveredCodeFiles` / 缺 `**文件**` 的 `warnings`：清单顶部列出并附 `remediation`；请用户归入某单、单独 chore、或勾掉——**禁止静默丢弃或默认塞进第一张单**。若 coding.md 已过时，先更新对应单据节的 `**文件**`/`**改动点**`（必要时 `artifact_seat`）再起草。
3. 单绑 / `applicable === false`：无单据结构闸门；按 diff 逻辑拆分或整批一条；仍禁止把无关文件塞进带 `--story`/`--bug` 的 commit。
4. 跨单公共文件：以 coding.md「共享改动」说明为准；默认挂到动机所属单的 `**文件**`，或拆无 TAPD 的 `refactor`/`chore`；由用户勾选确认。

## 工作流

### A. 起草清单（默认停在这里）

每条拟提交：

- `[x]` / `[ ]` 勾选
- 完整 subject（可编辑；默认无 scope；TAPD 单默认用标题 + 短 ID）
- `files:` 路径列表（可编辑）

展示格式：

````text
待提交清单（编辑后回复「确认提交」才会执行）

依据：workflow 绑定 N 张 TAPD；一单一 commit；标题来自 TAPD；ID 为短 ID（已回环校验 / 待确认）

1. [x] feat: webhook方式同步issue状态 --story=135926589
   files:
   - path/to/a.ts
   - path/to/b.ts
   notes: long=1000...5824 → short=135926589 (tapd_id_get 回环 OK)

2. [x] fix: Profiling 火焰图滚动时关闭 tooltip --bug=160034346
   files:
   - path/to/c.ts

3. [x] docs: 补充说明
   files:
   - docs/xxx.md

回复「确认提交」执行 [x] 项；或贴回编辑后的清单并写「确认提交」。
回复「取消」则中止。
````

### B. 确认后 commit

1. 解析清单：跳过 `[ ]`；采用用户改过的 message / files；若用户手写了 `(scope)` 则保留。
2. 校验：带 `--story`/`--bug` 的 ID 必须是短 ID（≤9 位数字）；若仍是 19 位 → 拒绝该条并提示先修正。
3. 逐条：`git add -- <files...>`，再 commit。**禁止用 `git commit -m "中文"`**：
   - Windows PowerShell 5.1 会把无 BOM 的 UTF-8 脚本/命令文本按系统 ANSI 码页（GBK/CP936）解码，或经 cmd.exe 的 GBK 控制台转码：中文在传给 git 前就已损坏，git 只把错误字节原样存盘 → commit message 乱码。不同 shell/通道行为不一致，因此不能依赖 `-m` 的编码路径。
   - bash heredoc `<<'EOF' ... EOF` 在 PowerShell 也不支持（PowerShell 只有 here-string `@'...'@`，没有 bash heredoc 语法，旧写法会直接解析报错）。

   **改用临时文件 + `-F`（跨平台、无乱码）**：

   1. 用 Write 工具把 commit message 写到仓库内临时文件（如 `.git/COMMIT_MSG_TMP.txt`），**UTF-8 无 BOM** 编码（Write 工具默认即 UTF-8）。放在 `.git/` 下不被跟踪、也不受 `core.autocrlf` 和 clean/smudge 过滤影响，字节在多平台保持一致；常规仓库可直接用，worktree/submodule 场景改用 `git rev-parse --git-path COMMIT_MSG_TMP.txt` 解析真实路径。
   2. 执行 `git commit -F <临时文件路径>`（`-F` 直接读文件内容作为 message，绕开 shell 的编码转换）。
   3. 无论成功失败都用 Delete 工具清理临时文件（固定文件名，下次写入会覆盖，风险低）。

   临时文件内容示例（UTF-8 无 BOM）：

   ```text
   feat: webhook方式同步issue状态 --story=135926589
   ```

4. 失败（含 hook）：报告错误，不 amend；是否继续后续条先问用户。
5. 成功后 `git status`，列出新 commit。

### C. 询问 push

> 提交已完成。是否执行 `git push`？（回复「push」/「是」才执行；默认不推）

用户同意后**禁止立刻 `git push`**：有 upstream 也不代表能直推。按 C1 → C4 逐步判定推送目标（C5 硬约束、C6 示例），**先用一句话说明最终目标再执行**。

#### C1. 先采集事实（只读，一次跑完）

```bash
git rev-parse --abbrev-ref HEAD                                   # LOCAL：本地分支名
git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null   # UPSTREAM：<UP_REMOTE>/<UP_BRANCH>，无则为空
git remote -v                                                     # 远端清单
git rev-list --left-right --count @{u}...HEAD 2>/dev/null          # 输出「BEHIND<TAB>AHEAD」
git config user.name; git config user.email                        # 用于个人远端匹配
```

注意 `UP_BRANCH` 可能与 `LOCAL` 不同名（从共享集成分支 checkout 时 git 会自动把 upstream 设成那条共享分支），这**不是**「个人跟踪分支」。

#### C2. upstream 直推门禁（四条**全部**满足才允许 `git push` 直推）

1. `UPSTREAM` 非空；**且**
2. `UP_BRANCH === LOCAL`（分支名完全一致）；**且**
3. `UP_BRANCH` **不**命中共享/集成分支黑名单（见下）；**且**
4. 未发散：`AHEAD` 与 `BEHIND` **不同时**非零（同时非零 → 转 C4）。

任一条不满足 → **转 C3 个人远端识别**，`UPSTREAM` 只作为候选之一。

共享/集成分支黑名单（默认值；仓库另有明确约定时以仓库约定为准）。只看 `UP_BRANCH`，与远端名无关：

| 规则 | 命中示例 |
|------|----------|
| 前缀 `release/`、`hotfix/`、`feature/`、`feat/`（多人共享的长驻分支） | `feature/agent-server`、`release/1.2` |
| 分支名含 `feature-`、`integration`、`shared` 的长驻集成分支 | `feature-agent-server`、`integration/main` |
| 分支名等于 `main`、`master`、`develop`、`dev`、`test`、`staging`、`pre`、`prod` | `origin/main`、`origin/develop` |

#### C3. 个人远端识别（**有 upstream 也要走这里**）

按 `git remote -v`：

- **无任何远端** → 停止，提示用户先 `git remote add`，**不擅自添加**。
- **只有一个远端** → `git push -u <remote> HEAD`；若目标分支命中 C2 黑名单，先说明「本仓库无个人远端，目标是共享分支 `<...>`」并等用户确认。
- **多个远端** → 按优先级从高到低匹配个人远端：
  1. 远端名（不区分大小写）等于 `me` / `mine` / `local` / `fork`；
  2. 远端名等于或包含当前 git 用户名（`user.name`，或 `user.email` 的 `@` 前缀）；
  3. 远端 URL 路径段含当前用户名（个人 fork 特征，如 `git@<GIT_HOST>:<DEVELOPER_NAME>/hcm.git`）。
  - **唯一命中** → `git push -u <personal-remote> HEAD`，且推送前**必须**一句话说明目标与原因，例如：
    > 识别到个人远端 `me`，未直推共享 upstream `origin/feature/agent-server`（原因：upstream 是共享集成分支，且分支名与本地 `feat-xxx-auth` 不一致）。将执行 `git push -u me HEAD`。
  - **命中多个 / 都不命中** → 列出所有候选让用户选（把当前 `UPSTREAM` 也列为候选，并标注它「共享分支」/「名字不匹配」的原因），**不默认 `origin`**。

#### C4. 发散处置（`AHEAD > 0` 且 `BEHIND > 0`）

先报告差异（`git rev-list --left-right --count` 的数字 + `git log --oneline @{u}..HEAD` 摘要），再让用户三选一，**不擅自决定**：

1. 先 `git pull --rebase`（或 `merge`）再推 `UPSTREAM`；
2. 改推个人远端（若 C3 已识别到，推荐此项）；
3. 取消。

不 `--force` / `--force-with-lease`（除非用户明确要求）；main/master 一律不 force。

#### C5. 硬约束

- **禁止用 `HEAD:<branch>` 去「凑」共享分支名**：`LOCAL !== UP_BRANCH` 时不得默认执行 `git push <remote> HEAD:<UP_BRANCH>`；只有用户在对话里**明确点名**要推那条共享分支时才可执行。
- **禁止 silent redirect**：用户已回「push / 是」后，若目标因门禁从 `UPSTREAM` 改判为个人远端（或反之），必须在执行前告知新目标。
- 不改 git config（不自动 `git remote add`、不改 `branch.*.remote`）；不默认 `--force`；不 force push main/master。
- 「不直推 origin」不是绝对禁令：用户明确指定 `origin` 上的分支时照做。

#### C6. 示例

**正例**：`LOCAL=feat-xxx-auth`，`UPSTREAM=origin/feature/agent-server`，`BEHIND=1 AHEAD=1`，远端有 `origin` / `me` / `github` / `githubme`。

- C2 门禁：分支名不一致 ✗、`feature/` 命中黑名单 ✗、ahead+behind 同时非零 ✗ → 不直推。
- C3：`me` 命中优先级 1 且唯一 → 说明原因后 `git push -u me HEAD`。

**反例**：同上场景里执行 `git push`（失败：本地分支名与 upstream 不一致），或改执行 `git push origin HEAD:feature/agent-server`（non-fast-forward 被拒，且把改动推向共享集成分支）。

## 解析用户回贴

- `[x]`/`[X]` 执行；`[ ]` 跳过
- subject 行作为 commit message（允许用户手加 `type(scope):`）
- `files:` 下 `- ` 路径；files 被清空则跳过该条并说明
- 只说「确认提交」未贴回 → 按最近一次展示的清单执行

## 反例（禁止）

- 未确认就 commit / push
- `feat(api): ...`（除非用户手改加了 scope）
- `--story=<19位长ID>` / 一条 message 多个 TAPD ID
- 多单改动打成一个 commit
- `git add -A` 绕过清单文件列表
- **「有 upstream 就直推、跳过个人远端识别」**（必须先过 C2 四条门禁；不满足一律进 C3）
- 把 upstream 落在共享/集成分支（`feature/*`、`origin/main`、长驻 `feature-*` 等）时仍直推
- 本地分支名与 upstream 分支名不一致时，用 `git push <remote> HEAD:<upstream-branch>` 去凑共享分支（用户未点名时禁止）
- ahead 与 behind 同时非零时擅自 `pull` / `--force` / 改推，未先报告差异并让用户选
- 多远端时不作识别、默认推 `origin`（应按 C3 个人远端优先级；命中不唯一才让用户确认）
- 目标从 upstream 改判为个人远端却不告知（silent redirect）
