---
description: "前端编码路径边界——仅在前端 / workflow-dev 任务中禁止主动改后端；用户明确做后端 / Go 任务时整条规则忽略。关键词：前端任务、工作流、越界、server、pkg、Go、monorepo、frontend-only、workflow-dev。"
globs:
  - "front/**/*.vue"
  - "front/**/*.ts"
  - "front/**/*.tsx"
  - "front/**/*.js"
  - "front/**/*.jsx"
alwaysApply: false
---

# 前端编码路径边界

## 适用范围（先判定，再决定是否遵守）

本规则**按任务意图生效**，不是「仓库里凡是 AI 会话一律禁改后端」。

| 当前用户任务 | 本规则 |
| --- | --- |
| 前端开发、`/workflow-dev`、HCM 页面/组件/前端 API 封装、`<dataDir>` 工作流 | **生效**：按下述硬约束执行 |
| 用户明确在做后端 / Go / `server` / `pkg` / `cmd` / `internal` 等服务端开发或明确授权改后端 | **整条忽略**：正常改后端，不要被本文件束缚 |
| 意图不清 | **先问一句**是前端还是后端任务；未确认前默认按前端任务处理（避免顺手改后端） |

Monorepo 下本文件可能因 `ideRoot` 出现在仓库根 `.cursor/rules/`，后端会话也可能加载到它——**加载 ≠ 适用**；以后端意图为准，勿过度遵守。

## 硬约束（仅「适用范围 = 生效」时）

bkdevbuddy / `/workflow-dev` 在本项目中只服务 **HCM 前端**。**不得**因读到后端实现、联调失败或「顺手能改」就主动改后端。

1. **禁止写入**（创建 / 修改 / 删除 / 移动 / rename）`front/`（或当前 `projectRoot` 前端子目录）之外的任何源码或配置，除非用户用明确指令授权本次改后端。
2. **禁止写入**常见后端路径与类型（即使落在 workspace 内）：
   - `**/*.go`、`**/go.mod`、`**/go.sum`
   - `server/`、`cmd/`、`pkg/`、`internal/`（仓库内后端树）
   - `docs/api-docs/**`（后端维护的对外接口文档；契约增量只写 `<dataDir>/workflow/<id>/api.md`）
   - 非前端服务的部署 / CI 脚本（用户未点名时）
3. **允许只读**：为理解接口契约、排错或写 `api.md`，可以 Read / Grep 后端代码与 `docs/api-docs/**`；读完只把结论写进 `<dataDir>/` 产物或前端适配代码，**不得**据此改后端源码或后端 API 文档（`<dataDir>` 见 `bkdevbuddy-data-dir` rule）。
4. **接口缺口处理**：发现需后端配合时，在 `api.md` / `coding.md` / 对话中标注「需后端配合」及期望契约，交给用户决定是否另开后端任务；**不要**自己开改。

## 允许写入的典型前端范围（仅规则生效时）

相对消费方前端子目录（常见为仓库内 `front/`，亦即 `.bkdevbuddyrc.json` 所在的 `projectRoot`）：

- `src/`、`views/`、`components/`、`common/`、`constants/`、`router/`、`store/`、`hooks/`、`composables/`、`api/`（前端 HTTP 封装）、`types/`、`styles/` 等前端树
- 前端侧 `package.json` / 前端 lint 配置 / 前端单测（仅当任务相关）
- `<dataDir>/workflow/**`、`<dataDir>/requirements/**` 工作流产物（`<dataDir>` 见 `bkdevbuddy-data-dir` rule）

若当前 Cursor workspace 开在仓库根且前端在 `front/`，只改 `front/**`；不要改同级后端目录。

## 与「后端任务 / 明确授权」的关系

- 用户**明确**说「改后端 / 动 Go / 改 server」或整段对话主题就是后端实现 → 见上表「整条忽略」。
- 仅在**前端任务**中间夹一句「顺便改下后端」时，把那句当作当次授权的**窄例外**，改完后继续遵守前端边界；授权不外推到后续无关前端需求。
