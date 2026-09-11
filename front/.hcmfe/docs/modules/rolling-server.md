# 滚服

> status: drafted · kind: module
> globs: `src/views/rolling-server/**`、`src/components/device-type-selector/cvm-apply/children/asset-match.vue`、`src/components/device-type-selector/cvm-apply/children/device-type-dialog.vue`

滚服涵盖账单、配额、用量与主机申领流程。功能物理分散在顶层、business、ziyanScr 及共享机型选择组件中，目标收敛为单一原子模块。

## 职责

- 承载滚服项目的资源用量、配额和账单展示。
- 在主机申领中复用 CVM 机型选择弹窗，并在推荐模式下处理继承主机固资号与推荐机型的联动。
- 固资号是否合法由既有校验接口决定；前端负责保证“当前输入值”与展示、确认门禁一致，不改变后端校验规则。

## 固资号校验接缝

入口位于 `src/components/device-type-selector/cvm-apply/children/asset-match.vue`，确认门禁位于同目录 `device-type-dialog.vue`。

- 固资号自由输入的自动校验属于共享行为，滚服推荐与普通/裁撤模式均启用；`enableRecommend` 只控制推荐候选、下拉与机型族联动。
- 自由输入变化时立即清空旧结果并进入校验中状态，停顿 300ms 后请求，避免逐字符放大后端流量。
- 推荐列表选择、机型族联动、清空与手动重试属于确定性操作，保持即时处理，并取消尚未发送的防抖任务。
- 每次输入变化都会使旧请求序号失效；异步响应只有同时匹配最新序号和当前固资号时才能更新界面。
- 父弹窗在等待、失败或空值状态禁用确认；只有最新固资号校验成功后才能恢复确认。
- 不要用共享 store 的普通布尔 loading 作为弹窗确认门禁；并发请求下它无法表达“当前输入对应的校验是否完成”。

## 关键验证

- 连续输入期间旧成功态立即消失，停止输入后只请求最新值。
- 旧请求晚返回不得覆盖新值结果，也不得提前解除新值的等待态。
- 推荐选择即时且只请求一次；清空不请求空值。
- 普通/裁撤模式自由输入同样在停止 300ms 后自动请求，并复用过期响应丢弃和确认门禁；手动按钮仍可即时重试。
