---
description: 从 PR/MR 链接 cherry-pick 所有提交到新分支
---

从 PR/MR `$ARGUMENTS` 中 cherry-pick 所有提交到新分支。

## 执行步骤

### 1. 解析 PR/MR 信息

首先判断链接类型：

#### GitHub PR

链接格式：`https://github.com/owner/repo/pull/123` 或简写 `owner/repo#123`

使用 `gh pr view <pr-url> --json commits,headRefName,baseRefName` 获取：

- PR 的所有 commit 列表（按提交顺序）
- PR 的来源分支名称（`headRefName`，用作新分支默认名称）
- PR 的目标分支（`baseRefName`，用作新分支的基础分支）

#### 其它 MR (GitLab)

链接格式：`https://<gitlab-host>/owner/repo/merge_requests/123` 或 `https://<gitlab-host>/owner/repo/-/merge_requests/123`

从链接中提取 `project_id`（如 `owner/repo`）和 `iid`（如 `123`），然后调用 **GitLab MCP** 获取信息：

1. **获取 MR 详情**：调用 `search_merge_request` 工具
   - 参数：`project_id`（MR 链接中的项目路径）、`iid`（MR 编号）
   - 从返回结果中提取：
     - `source_branch`：来源分支（用作新分支默认名称）
     - `target_branch`：目标分支（用作新分支的基础分支）

2. **获取 commit 列表**：调用 `compare` 工具
   - 参数：`project_id`（同上）、`from`（`target_branch`）、`to`（`source_branch`）
   - 从返回结果中的 `commits` 数组按顺序提取每个 commit 的 `id`（即 SHA），作为 cherry-pick 的提交列表

> 注意：`compare` 返回的 `commits` 已按从旧到新排序，可直接用于顺序 cherry-pick。

### 2. 创建新分支

使用**选择器**让用户确认（而非手动输入）：

- **新分支名称**：提供选项 - 默认名称（PR 来源分支）或自定义
- **基于分支**：提供选项 - origin/bcc/v1.8.x（推荐）、origin/bcc/master 或其他

若基于分支的远程（如 `origin`）上已存在同名分支（不同远程下允许同名），提供选项：使用其他名称 / 删除重建 / 切换到现有分支

### 3. 按顺序 Cherry-pick 提交

依次执行 `git cherry-pick <commit-sha>` 对每个提交进行 cherry-pick。

**冲突处理**：

- 如果遇到冲突，暂停并列出冲突文件及冲突内容
- 使用**选择器**让用户选择解决方式：保留两边 / 只保留 HEAD / 只保留 PR / 手动解决
- 用户解决后执行 `git add <冲突文件>` 和 `git cherry-pick --continue`（不使用 `git add .` 避免添加未跟踪文件）
- 继续处理剩余提交

### 4. 确认并推送

所有 cherry-pick 完成后：

1. **列出变更**：执行 `git log --oneline <基础分支>..HEAD` 和/或 `git diff --stat <基础分支>..HEAD`，展示本次 cherry-pick 引入的提交列表及变更统计，供用户确认。

2. **Author 修正（跨平台场景）**：
   如果 pick 来源是**其它 MR**，且要推送的目标远程是 **GitHub**（远程 URL 包含 `github.com`），则在推送前提示用户是否需要修正 commit author：
   - 默认读取本地 git config 的 `user.name` 和 `user.email` 作为推荐值
   - 使用**选择器**让用户选择：
     - 使用默认配置（`git config user.name` / `git config user.email`）
     - 自定义 author（让用户输入 Name 和 Email）
     - 跳过不修改
   - 用户确认后执行修正：
     - **单条提交**：`git commit --amend --author="Name <email>" --no-edit`
     - **多条提交**：`git rebase -i <基础分支> --exec 'git commit --amend --author="Name <email>" --no-edit'`

3. **用户确认**：等待用户确认无误后再继续推送；若用户发现有问题，提供选项：直接推送 / 修改后再推送 / 放弃并退出。

4. **执行推送**：自动识别个人远程仓库（me / local / personal / mine），使用 `git push -u <remote> <branch>` 推送。

5. **打开创建页面**：推送成功后，读取 push 返回信息中的 PR/MR 链接地址，并使用系统默认浏览器自动打开该链接。

## 注意事项

- **GitHub PR**：需要安装 GitHub CLI (`gh`) 并已登录
- **其它 MR**：需要配置 GitLab MCP，并确保已设置对应的 OAuth Token 或 Access Token
- 链接格式支持：
  - GitHub：`https://github.com/owner/repo/pull/123` 或简写 `owner/repo#123`
  - 其它：`https://<gitlab-host>/owner/repo/merge_requests/123` 或 `https://<gitlab-host>/owner/repo/-/merge_requests/123`
