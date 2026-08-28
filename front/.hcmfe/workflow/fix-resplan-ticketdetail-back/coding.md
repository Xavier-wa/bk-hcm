# Coding 方案 — 资源预测-修改预测提交后-单据详情返回按钮没有响应

- TAPD: https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137606214
- 分支: `feat-resplan-time-quantity-adjust`
- Requirement: `req-feat-resplan-time-quantity-adjust`

## 一、问题定位

资源预测申请单详情用旧 `DetailHeader` 页内箭头，把「返回单据列表」配在 `fromConfig` 上。`DetailHeader` 只要有 `fromConfig` 就走 `handleBack(fromConfig)`。

`handleBack` 原先只把 `fromConfig` 当成 merge 补参（业务 ID），真正目的地是 `from`：

- 有 `_f`：HistoryStorage.peek
- 否则：`menu.relative`

调整提交进详情时既没有 `_f`，详情路由也没有 `relative`，`from` 为空 → `if (!target) return` → 空点无响应。

从单据列表进详情会带 `{ history: true }`（`_f`），所以列表路径正常。

## 二、修复

`src/router/hooks/use-back.ts` 的 `handleBack`：`from` 为空且 `fromConfig?.name || fromConfig?.path` 时，把 `fromConfig` 当作目的地。

- 提交后详情：无 history → 跳 `fromConfig`（单据列表 + 资源预测 tab + 当前业务）
- 列表进详情：有 `_f` → 仍 pop 自定义栈，行为不变
- 新面包屑仍只在 `from` 有值时显示箭头，不受影响
- 仅带 `query` 的 `fromConfig`（购买页补业务 ID）仍只作补参，不单独跳转

详情页 `fromConfig`、提交 redirect（不带 `history: true`）不改。

## 三、不在范围

- 迁新面包屑 / 给提交加 `_f`
- 返回调整页或预测资源列表
- 浏览器后退键、普通申请单详情视觉改动

## 四、验证

- 调整提交成功进详情 → 页内返回 → 单据列表（资源预测、当前业务）
- 单据列表打开同一张详情 → 返回仍回列表
- 工作台视角同类详情 → 返回工作台单据列表
