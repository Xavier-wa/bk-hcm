# Coding 方案 — 资源预测-调整-复制后没有回填变更原因

- TAPD: https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137606579
- 分支: `fix-resplan-adjust-copy-reason`
- Requirement: `req-feat-resplan-time-quantity-adjust`

## 一、问题定位

调整页行内「复制」走 `render-row` → `cloneRowAsNew` → `data-list.copyRow`。`createEmptyAdjustRow` 虽默认 `demand_source = 指标变化`，但 `cloneRowAsNew` 把源行 `...rest` 铺在默认值之后，存量行 list 常不带变更原因（空字符串），把默认值盖掉。复制行是 `is_new`，应展示可编辑下拉，结果变成空。

CVM / CBS 同行共用这一条复制路径。

## 二、修复

`src/views/resource-plan/cvm/adjust/adjust-payload.ts` 的 `cloneRowAsNew`：在铺完源行字段后显式

`demand_source: rest.demand_source || DEFAULT_DEMAND_SOURCE`

- 源行已有值：沿用
- 源行为空：与「新增」一致，默认「指标变化」
- 仍清空期望到货日 / 退回日（原复制语义不变）

`data-list.copyRow` 只把已克隆行推进表，无需再改。

选项尚未加载时 SelectColumn 假清空：现有 `selectModel` 对非 clearable 列（含 `demand_source`）已拦截「有值→空」，覆盖 AC 中「不得把已回填的值清掉」。

## 三、不在范围

- 新建 / 修改等其他入口的复制
- 存量行只读变更原因（list 未返回时继续 "-"）
- 变更原因枚举增删改

## 四、验证

- 复制存量为空变更原因的行 → 新行为「指标变化」且可编辑
- 复制已改过变更原因的新增行 → 新行与源行相同
- 复制 CBS 行 → 与 CVM 同一规则
- 仅点「新增」不点复制 → 仍默认「指标变化」
