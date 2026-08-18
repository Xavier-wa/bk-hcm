---
name: wf-cr-fetch
description: 为代码评审拉取本次变更，把本地 diff / 工蜂 MR / GitHub PR 三种来源归一成同一份变更包（文件 + hunk + 改动行号集合）。由 workflow-cr 在评审第一步读取执行。
---

# CR 变更获取

被 `workflow-cr` 读取执行。职责单一：**拿到本次要评审的变更，归一成一份结构，屏蔽平台差异**。

不做评审判断，不做任何写操作。

## 平台识别

按入参形态与 git remote 判定，不要问用户"你用的是哪个平台"：

| 线索 | 平台 |
| --- | --- |
| URL 含 `/merge_requests/` 或 remote host 是工蜂 | 工蜂（gongfeng MCP） |
| URL 含 `/pull/` 或 remote host 是 `github.com` | GitHub（`gh` CLI） |
| 无 URL | 本地 diff |

工蜂 MCP 不可用或 `gh` 未登录时，**降级为本地 diff 并明确告知用户**——评审内容一样，只是结果没法回写到平台。不要卡在这里反复重试。

## 三种来源

### 本地 diff

评审"本地未合入的改动"，包含工作区未提交的部分：

```bash
git merge-base HEAD origin/master   # 或 origin/main / 项目主干分支
git diff --unified=3 <merge-base>   # 相对分叉点，含未提交改动
```

用 `merge-base` 而不是 `origin/master` 直接比，避免把主干上别人的提交算成本次改动。主干分支名先探测（`git symbolic-ref refs/remotes/origin/HEAD`），拿不到再依次试 `master` / `main`。

新增但未 `git add` 的文件不会出现在 `git diff` 里，用 `git status --porcelain` 补一遍，逐个当作全新增文件纳入。

### 工蜂 MR

从 URL 解析出项目路径与 MR iid（形如 `https://<GIT_HOST>/<org>/<repo>/-/merge_requests/<iid>`），调 gongfeng MCP：

- `get_merge_request_changes` — 拿 diff
- `get_project_detail` / `search_projects` — URL 里只有项目路径、需要项目 id 时用

用户只给了 iid 没给完整 URL 时，从 git remote 推项目路径。

### GitHub PR

```bash
gh pr diff <PR号或URL> --patch
gh pr view <PR号或URL> --json files,baseRefName,headRefName
```

## 归一后的变更包

三种来源都产出同一份结构，交回 `workflow-cr`：

```
变更包
├─ source: local | gongfeng | github
├─ ref: MR/PR 标识（本地模式为分叉点 commit）
└─ files[]
   ├─ path: 文件路径（仓库根相对）
   ├─ status: added | modified | deleted | renamed
   ├─ hunks[]: 带上下文的 diff 片段原文
   └─ changedLines: 本次新增/修改的行号集合
```

`changedLines` 是**必须带上的**，不能省成"让评审自己从 diff 里看"。它是"只对改动行提问题"这条约束的判定依据；没有它，评审会顺手点评上下文里的老代码，一次就能刷屏，然后整个 CR 会被当噪音关掉。

删除的行不进 `changedLines`——代码已经没了，没有可评审的对象。

## 过滤

以下文件不送去评审，在变更包里注明"已跳过 N 个文件"：

- 锁文件（`package-lock.json`、`yarn.lock`、`pnpm-lock.yaml`）
- 构建产物、`dist/`、`node_modules/`
- 二进制与媒体文件
- 快照文件（`__snapshots__/`）

单个文件 diff 超过约 1500 行时，只取前 1500 行并标注截断——超大 diff 往往是格式化或搬运，逐行评审的收益极低。

## 规模保护

变更包总量超过约 2000 行改动时，先告诉用户规模并询问怎么办，不要闷头把巨量 diff 塞进子代理：

- 按目录/模块拆成几次评审（推荐）
- 只评审其中指定的若干文件
- 坚持整体评审（说明结论质量会下降）

## 注意

- 全程只读。**不要** checkout 分支、不要 fetch 后改变工作区状态、不要 stash 用户的未提交改动。
- 拿不到 diff 就如实回报（MR 已合并/已关闭、无权限、PR 号不存在），不要用空变更包冒充成功。
- 变更为空时直接告诉用户"本次无可评审改动"，不要派发子代理空跑一趟。
