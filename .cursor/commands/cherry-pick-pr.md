---
description: 从 PR 链接 cherry-pick 所有提交到新分支
---

从 PR `$ARGUMENTS` 中 cherry-pick 所有提交到新分支。

## 执行步骤

### 1. 解析 PR 信息

从提供的 PR 链接中获取：
- PR 的所有 commit 列表（按提交顺序）
- PR 的来源分支名称（用作新分支默认名称）
- PR 的目标分支（用作新分支的基础分支）

使用 `gh pr view <pr-url> --json commits,headRefName,baseRefName` 获取信息。

### 2. 创建新分支

使用**选择器**让用户确认（而非手动输入）：
- **新分支名称**：提供选项 - 默认名称（PR 来源分支）或自定义
- **基于分支**：提供选项 - origin/bcc/v1.8.x（推荐）、origin/bcc/master 或其他

如果分支已存在，提供选项：使用其他名称 / 删除重建 / 切换到现有分支

### 3. 按顺序 Cherry-pick 提交

依次执行 `git cherry-pick <commit-sha>` 对每个提交进行 cherry-pick。

**冲突处理**：
- 如果遇到冲突，暂停并列出冲突文件及冲突内容
- 使用**选择器**让用户选择解决方式：保留两边 / 只保留 HEAD / 只保留 PR / 手动解决
- 用户解决后执行 `git add <冲突文件>` 和 `git cherry-pick --continue`（不使用 `git add .` 避免添加未跟踪文件）
- 继续处理剩余提交

### 4. 推送结果

所有 cherry-pick 完成后，调用 commit-push 命令逻辑：
- 自动识别个人远程仓库（me / local / personal / mine）
- 使用 `git push -u <remote> <branch>` 推送
- 推送成功后打开 PR 创建链接

## 注意事项

- 需要安装 GitHub CLI (`gh`) 并已登录
- PR 链接格式支持：`https://github.com/owner/repo/pull/123` 或简写 `owner/repo#123`
