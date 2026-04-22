---
description: 更新目标环境分支的代码（通过 cherry-pick 合入）
---

更新目标环境分支。参数: `$ARGUMENTS`

## 参数说明

- 第一个参数：目标环境分支（如 `origin/env/dev`）
- 第二个参数：需要合入的代码分支（用于 cherry-pick 其中的 commit）

## 执行步骤

### 1. 解析参数

使用**选择器**让用户确认：

- **目标环境分支**：通过 `git branch -r | grep env` 获取包含 "env" 的远程分支列表
- **合入代码分支**：通过 `git branch --sort=-committerdate` 按最近提交时间排序获取本地分支列表

### 2. Fetch 更新远程分支

执行 `git fetch <remote> <branch>` 更新目标远程分支的最新代码。

### 3. 切换到目标分支（detached HEAD）

使用 detached HEAD 方式检出，不创建本地分支：

- `git checkout <remote/branch> --detach`

**优点**：不会累积本地分支，每次执行不会有分支已存在的问题

### 4. 获取合入分支的 commits

获取需要合入的分支的 commit 列表：

- `git log <base>...<source-branch> --oneline`
- 使用**选择器**让用户确认要 cherry-pick 的 commits（可多选）

### 5. Cherry-pick 提交

依次执行 `git cherry-pick <commit-sha>` 对每个选中的提交进行 cherry-pick。

**冲突处理**：

- 如果遇到冲突，暂停并列出冲突文件及冲突内容
- 使用**选择器**让用户选择解决方式：保留两边 / 只保留 HEAD / 只保留 PR / 手动解决
- 用户解决后执行 `git add <冲突文件>` 和 `git cherry-pick --continue`
- 继续处理剩余提交

### 6. 强制推送到目标分支

使用 `git push --force-with-lease <remote> HEAD:<branch>` 强制推送到目标分支。

**说明**：因为使用 detached HEAD，需要用 `HEAD:<branch>` 格式指定推送目标。

**安全提示**：使用 `--force-with-lease` 而非 `--force`，避免覆盖他人的推送。

## 注意事项

- 强制推送会覆盖目标分支的历史，请确认操作
- 建议在推送前确认 commit 列表是否正确
