# 页面级内外版差异化（UI 片段替换）

> status: drafted · kind: module
> globs: `src/**/*.plugin.vue`、`src/**/*-internal.plugin.vue`、`src/**/*.plugin.tsx`、`src/**/*-internal.plugin.tsx`
> 系统基础特色能力。与 [component-variant](component-variant.md) 是**同一套构建替换机制**，区别只在替换粒度：这里替换的是**整块 UI 片段**（Vue SFC / TSX 渲染函数），而非行为/数据模块（`.ts`）。

## 与组件级的关系（同机制、异粒度）

构建替换机制完全复用 `.plugin` 模式（`front/bk.config.js` 的 `NormalModuleReplacementPlugin`，正则 `/\.plugin(\.\w+)?$/` 同样命中 `.plugin.vue` / `.plugin.tsx`；`TARGET_MODE=bcc` 时 `xxx.plugin` → `xxx-internal.plugin`）。机制细节见 [component-variant](component-variant.md)。

| 粒度 | 替换单位 | 后缀 | 模块 |
|---|---|---|---|
| 行为/数据级 | filter / vendor / data 逻辑 | `.plugin.ts` | [component-variant](component-variant.md) |
| 页面/UI 片段级 | 整块组件 / 渲染函数 | `.plugin.vue` / `.plugin.tsx` | 本模块 |

**选择依据**：UI / 交互流程本身按版本不同 → 页面级（本模块）；仅数据/规则不同而 UI 不变 → 组件级（`.ts`）。

## 形态 A：SFC 组件替换（.plugin.vue）

父组件 import `./xxx.plugin.vue`，构建时按版本换成外/内版实现；两版保持**相同 props/emits 契约**，父组件无感知。

推荐三层结构（安全组删除按钮为例）：

```
single-delete.vue                       # 父：import './single-delete-button.plugin.vue'
single-delete-button.vue                # 纯展示基件（一个 danger 按钮，无版本逻辑）
single-delete-button.plugin.vue         # 外部版：直接调 deleteBatch 删除
single-delete-button-internal.plugin.vue# 内部版：删除前插入 MOA 校验（moa-verify-btn），失败/过期分支处理
```

- 外/内版都复用纯展示基件 `single-delete-button.vue`，只在其外层包装各自的**行为/流程**。
- 内部版 = 外部版 + 增量流程（这里是 MOA 二次校验），对外 props（`id`/`disabled`/`loading` model）与 `success` emit 完全一致。

## 形态 B：渲染函数替换（.plugin.tsx）

导出**同名渲染函数**，构建时按版本替换；内版通常加分支/额外入参（工单申请详情为例）：

```tsx
// apply-content-render.plugin.tsx（外部版）
export const applyContentRender = (currentApplyData, curApplyKey, applyDetailProps) => {
  if (ACCOUNT_TYPES.includes(currentApplyData.value.operation)) return <AccountApplyDetail ... />;
  return <CommonApplyDetail ... />;
};

// apply-content-render-internal.plugin.tsx（内部版）：多一个 bpaas 来源分支 + bpaasProps 入参
export const applyContentRender = (currentApplyData, curApplyKey, applyDetailProps, bpaasProps) => {
  if (currentApplyData.value.source === 'bpaas') return <BpassApplyDetail ... />;  // 内部版专属
  if (ACCOUNT_TYPES.includes(currentApplyData.value.operation)) return <AccountApplyDetail ... />;
  return <CommonApplyDetail ... />;
};
```

消费方 `import './apply-content-render.plugin'` 并调用该函数。

## 约定与边界

- **只 import `.plugin`**：需要按版本换 UI 的地方，抽成 `xxx.plugin.vue|tsx`（外部版）+ `xxx-internal.plugin.vue|tsx`（内部版）；调用方引用 `.plugin`，**严禁直接 import `-internal.plugin`**（交给构建替换）。
- **对外契约一致**：SFC 两版的 props / emits / expose 必须一致；渲染函数两版的导出名与签名保持一致（内版可追加可选入参），保证消费方无感。
- **内版 = 外版 + 增量**：内部版通常在外部版基础上加校验步骤 / 分支 / 专属子视图（MOA 校验、bpaas 来源等），不是完全另写一套。
- **外版是默认与兜底**：无 `-internal.plugin.*` 时外部版对内外版都生效。
- **与 plugin-handler 的关系**：`@pluginHandler` 目录级机制已标退场中（见 [plugin-handler](plugin-handler.md)）；新代码优先用本模式的文件级片段替换。
- 现有使用点：安全组删除按钮（`resource-manage/.../security-group/single-delete-button*.plugin.vue`，MOA 校验）、工单申请详情渲染（`ticket/.../apply-content-render*.plugin.tsx`，bpaas 来源）等。
