---
description: 更新目标环境分支的代码（通过 cherry-pick 合入或直接强制推送）
---

更新目标环境分支。参数: `$ARGUMENTS`

## 参数说明

- 第一个参数：目标环境分支（如 `origin/env/dev`）
- 第二个参数：源分支（cherry-pick 时用于选取 commit；force-push 时直接推送到目标分支）
- `--force-push`（可选）：指定时使用源分支直接强制推送到目标分支，跳过 cherry-pick

## 执行步骤

### 1. 解析参数

使用**选择器**让用户确认：

- **目标环境分支**：通过 `git branch -r | grep env` 获取包含 "env" 的远程分支列表
- **源分支**：通过 `git branch --sort=-committerdate` 按最近提交时间排序获取本地分支列表
- **模式**：通过选择器让用户确认操作模式：
  - `cherry-pick`（默认）：选取源分支的 commit 合入目标分支
  - `force-push`：直接用源分支强制覆盖目标分支

### 2. Fetch 更新远程分支

执行 `git fetch <remote> <branch>` 更新目标远程分支的最新代码。

### 模式一：Cherry-pick 合入

#### 3. 切换到目标分支（detached HEAD）

使用 detached HEAD 方式检出，不创建本地分支：

- `git checkout <remote/branch> --detach`

**优点**：不会累积本地分支，每次执行不会有分支已存在的问题

#### 4. 获取源分支的 commits

获取需要合入的分支的 commit 列表：

- `git log <base>...<source-branch> --oneline`
- 使用**选择器**让用户确认要 cherry-pick 的 commits（可多选）

#### 5. Cherry-pick 提交

依次执行 `git cherry-pick <commit-sha>` 对每个选中的提交进行 cherry-pick。

**冲突处理**：

- 如果遇到冲突，暂停并列出冲突文件及冲突内容
- 使用**选择器**让用户选择解决方式：保留两边 / 只保留 HEAD / 只保留 PR / 手动解决
- 用户解决后执行 `git add <冲突文件>` 和 `git cherry-pick --continue`
- 继续处理剩余提交

#### 6. 强制推送到目标分支

使用 `git push --force-with-lease <remote> HEAD:<branch>` 强制推送到目标分支。

**说明**：因为使用 detached HEAD，需要用 `HEAD:<branch>` 格式指定推送目标。

**安全提示**：使用 `--force-with-lease` 而非 `--force`，避免覆盖他人的推送。

### 模式二：直接强制推送（Force Push）

#### 3. 确认推送

向用户展示即将执行的操作：

- 源分支：`git rev-parse <source-branch>` 获取源分支的 HEAD commit
- 目标分支：`git rev-parse <remote>/<target-branch>` 获取目标分支当前 HEAD
- 提示用户该操作会直接覆盖目标分支历史

#### 4. 执行强制推送

使用 `git push --force-with-lease <remote> <source-branch>:<target-branch>` 将源分支强制推送到目标分支。

**说明**：此操作会用源分支的完整历史直接替换目标分支。

**安全提示**：使用 `--force-with-lease` 而非 `--force`，避免覆盖他人的推送。

## 注意事项

- 强制推送会覆盖目标分支的历史，请确认操作
- 建议在推送前确认 commit 列表是否正确
- `force-push` 模式会直接替换目标分支，请确保源分支状态正确
