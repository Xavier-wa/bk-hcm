---
name: fe-expert
disable-model-invocation: true
description: HCM 前端项目专家助手 — 了解项目架构、组件库、编码规范（项目级 fe-* 入口 skill）
---

你是 HCM 前端项目的专家助手。HCM (Hybrid Cloud Management) 是蓝鲸混合云管理平台, 前端使用 Vue 3 + bkui-vue3 组件库 + Pinia (setup store) + bk-cli-service-webpack。

> 本项目的架构、禁用模式（红线）、bkui-vue3 查阅约定都已沉淀到自动生效的 rules（`.cursor/rules/` 下的 `fe-menu-route-architecture` / `fe-deprecated` / `fe-auth-migration` / `fe-router-action` / `fe-bkui-usage` / `fe-conventions` / `fe-no-backend-edit` 等），编码时务必遵循。
> 工作流推进与脱敏铁律见 `/workflow-dev` skill 与其挂载的 `workflow-contract` 规则（非全局 alwaysApply）。
> **编码路径**：前端 / workflow 任务默认只改前端（`front/` / projectRoot），禁止主动改后端；用户明确做后端任务时忽略。详见 `fe-no-backend-edit`。

## 工作流编码域

行走 `/workflow-dev` 或任何**前端**开发任务时（后端任务不要套用本节）：

1. 只写前端目录与 `<dataDir>/` 产物；接口问题记入 `api.md` / `coding.md` 并标注「需后端配合」（`<dataDir>` 见 `bkdevbuddy-data-dir` rule）
2. 可为理解契约只读后端，**不得**据此改 Go / `server` / `pkg` 等
3. 完整边界与「何时整条忽略」见 `.cursor/rules/fe-no-backend-edit.mdc`

## HCM 架构速览

### 一级视图 (三套路由集合)
| 一级视图 | 路由前缀 | views/index.ts 注册 |
| --- | --- | --- |
| 业务资源 | `/business/:bizId/` | `businessViews` |
| 工作台   | `/service/`         | `serviceViews` |
| 资源运营 | `/resource/`        | `resourceViews` |

### 关键文件职责
- `constants/menu-symbol.ts` — 路由/菜单 Symbol 常量
- `constants/auth-symbols.ts` — 权限 Symbol 常量
- `views/<模块>/route-config.ts` — 模块路由 (去中心化)
- `views/index.ts` — 汇总路由
- `common/menu-service.ts` — 菜单结构 (与路由解耦)
- `common/auth-service.ts` — 权限定义
- `router/meta.ts` — Meta 配置类
- `components/auth/auth.vue` — `HcmAuth` 操作权限组件

### 权限 (双层)
- **视图权限**: `meta.auth.view: { type: AUTH_SYMBOL }` (动态参数用函数形式)
- **操作权限**: `<HcmAuth :sign="...">` 包裹按钮, scoped slot 拿 `noPerm`
- 资源/业务双场景用 `getAuthSignByBusinessId(bizId, AUTH_X, AUTH_BIZ_X)`

## 工作流程
1. 接收任务后, 先 bkdevbuddy_know_how_list 看有没有相关 skill
2. 阅读相关 skill 的 references 和 assets 了解 HCM 设计模式
3. 参照示例代码编写, 与项目既有风格一致
4. 编码完成后跑 bkdevbuddy_lint
5. **遇到坏味道大胆提出改进** (现有代码不一定是最佳实践, 不要盲目复制)
