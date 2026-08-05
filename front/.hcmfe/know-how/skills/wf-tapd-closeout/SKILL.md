---
name: wf-tapd-closeout
description: 工作流到达 done 时把已绑定的 TAPD 单据按 tapdSync **逐单按 objectType** 验收收口：story 追到 done（tested→done 的最后一跳）；bug 最高停在 for_test（禁止强推 done），有 ownerAction 则回交提单人。是 done.pre 的 required 收口节点执行体。遇 Smart Mode 拦截 stories_update/bugs_update 时必须 requestSmartModeApproval 重试拿审批，禁止改口“用户手动改”。当 workflow_status / workflow_next 返回 closeout 未完成、或存在 pending 的 hcm-tapd-sync-done 节点时使用。
---

# TAPD 收口 Skill（wf-tapd-closeout）

## 使用时机

工作流推进到终态 `done` 后，`workflow_status` / `workflow_next` 返回：

- `stageNodes.pre` 含 pending 的 `hcm-tapd-sync-done`（或同类 `tapdLinked` required 收口节点），或
- `closeout.complete === false` 且 `blockingActions` 指向 TAPD 收口节点。

此时**必须**先做完本 skill，再把工作流当作收工完。`stop` hook 也会在此处**硬阻断**结束本轮，直到该节点 `workflow_node_complete` / `workflow_node_skip`。

## 前提

- 工作流已通过 `bkdevbuddy_tapd_link` 绑定单据（`state.tapd.itemId` 存在）。
- 已 `bkdevbuddy_tapd_status_sync` 同步过状态模型（否则 `tapdSync.configured=false`，先补同步）。

## 核心原则

- **按 objectType 分型验收，禁止一刀切推到 done**：
  - **story**：done 是工作流完成态，不以真实上线为前提。TAPD 真实标签可能是「已上线」，但本工作流里 `done` 只代表研发流程走完。**不得**以「代码未提交 / 未部署 / 未上线」为由停在 `tested`。
  - **bug**：语义目标最高 `for_test`（`childTargets` 通常已被引擎剔除 `tested`/`done`），留给测试验证后人工关单。**禁止**为了让本收口节点 complete 而把 bug 强推到 TAPD `tested`/`done`。若 `ownerAction.type === 'reassign_to_reporter'`，必须完成回交提单人（`bugs_get` 取 reporter → `bugs_update(current_owner=reporter)`）。
- **单调只进不退**：语义链 `backlog < todo < doing < for_test < tested < done`，只在**严格前进**时更新；工作流向后 `set_stage` 时**绝不**回退 TAPD。
- **追平不跳级**：story 逐级推进，`tested` 里程碑要显式经过再到 `done`。

## 操作步骤（AI 必须按序执行）

1. **取目标**
   - 调 `bkdevbuddy_workflow_status`（或用刚才 `workflow_next` 的返回），读 `tapdSync`（含多单场景下的 `tapdSync.items`）。
   - 若 `tapdSync.configured === false` → 先 `bkdevbuddy_tapd_status_sync`，再重取。

2. **对每一张单（遍历 `tapdSync.items`，单绑定视为一张单）分型处理**
   - **`objectType = story`**：读 `resolvedTargets`；`stories_get`(with_v_status=1) 读当前状态；对**严格大于**当前状态的每个目标 forward-only 追到 `done`（通常经 `tested → done`）。
   - **`objectType = bug`**：`childTargets` 通常为空（无需再推状态）；`bugs_get`(with_v_status=1) 核对当前状态仍为 `for_test`（若已由测试推到更高态，forward-only 跳过，不回退不重推）。**禁止**只为了让本节点 complete 而写入 `tested`/`done`。若该单的 `ownerAction.type === 'reassign_to_reporter'`（少见的 catch-up 场景，通常在进入 `for_test` 时已处理），先 `bugs_get` 取 reporter 再 `bugs_update(current_owner=reporter)` 完成回交。
   - **遇 Smart Mode 拦截**（仓外共享状态写入被拦是**预期行为**）：
     - **必须** `requestSmartModeApproval` 重试，走审批通道完成写入。
     - **严禁**把拦截当硬失败、也**严禁**改口让「用户手动去 TAPD 改」——那等于丢掉 tapdSync 收口。
   - 每跳成功后再进行下一跳，保持 forward-only。

3. **父需求 roll-up（若有 parentId，逻辑不变）**
   - 子单据更新后，拉取父需求的全部子项状态，调 `bkdevbuddy_tapd_rollup`。
   - `shouldAdvance === true` 才更新父需求状态（同样 forward-only + Smart Mode 审批）。

4. **收口登记**
   - 确认每张单已按其 `objectType` 验收到位（story 到 `done`；bug 仍在 `for_test` 或更高）。
   - 调 `bkdevbuddy_workflow_node_complete({ nodeId: 'hcm-tapd-sync-done', note })`，**note 回填每张单最终 status key，以及是否改过 owner**（bug 回交提单人的情况）作留痕。
   - 完成后 `closeout.complete` 变 true、`stop` hook 不再拦。

## 例外与解闸

- **确无法推进 / 用户明确不追平**：经用户同意后 `bkdevbuddy_workflow_node_skip({ nodeId: 'hcm-tapd-sync-done', note })`，note 写清理由（如「单据由他人流转 / 本次不涉及该单据」）。skip 后节点解闸。
- **未绑定单据**：该节点 `when.tapdLinked` 不满足，本就不会出现，无需处理。

## 注意事项

- 不要把真实 TAPD 域名 / 账号写进任何仓库产物；引用时用占位符（如 `<TAPD_HOST>`）。
- 引擎无 TAPD 凭证，**不会**替你改状态，也无法校验真实状态——收口是否真的到位，取决于你按本 skill 执行并核对。
