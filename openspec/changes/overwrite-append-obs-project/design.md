## Context

项目类型合法集合集中在 `pkg/criteria/enumor/woa_ziyan.go`。原 `ValidateResPlan()` 按当前时间滚动窗口拦截，覆盖追加无法写入预算第 3 年春保（如 `2029春节保障`）。退回计划还在用更严的 `Validate()`。

约束：实现必须收敛，禁止沿 ticket / dispatch / split 堆 skip。

## Goals / Non-Goals

**Goals:**

- resPlan 校验只认已知形态，不卡当前年份，覆盖追加能写入未来春保 / 裁撤。
- 页面下拉仍按 `GetObsProjectMembersForResPlan()` 给出当前窗口选项。
- 退回计划覆盖追加与退回原因大类与预测校验对齐。
- CRP 回包 `ProjectName` 不再校验。
- 非法形态拒绝，整单不落库。

**Non-Goals:**

- 按期望到货时间动态算窗口。
- 改 meta 下拉枚举内容。
- 任意字符串放行。
- 改造 CRP 项目类型主数据。
- 调整 `IsDissolveObsProjectForResPlan` 的裁撤消耗语义。

## Decisions

### D-1：选择严、校验宽

- **选择**：`ListObsProject` 继续返回 `GetObsProjectMembersForResPlan()`（当前时间窗口）。
- **校验**：`ValidateResPlan()` 只认形态（固定五类 + `^\d{4}春节保障$` / `^\d{4}机房裁撤$`）。页面用户选不到未来年；覆盖追加或直调 API 传入已知形态可通过。

不再拆 `ValidateResPlanForm` / `ValidateForOverwriteAppend`。所有 resPlan 写入路径已经在调 `ValidateResPlan()`，改中枢即可打通 persist，无需改 DAL 调用点。

**替代方案：**

- 仅覆盖追加入口 skip、DAL 仍走窗口。否决：persist 二次失败。
- 沿链路各打开关。否决：违反 R-005。
- 校验与下拉都放开。否决：页面会露出过远年份。

### D-2：退回计划改用 `ValidateResPlan()`

覆盖追加 filter / 明细、`ListReturnReasonClass` 不再用更严的 `Validate()`。

### D-3：CRP 回包项目类型信任

删除 `apply_change.go` 中 `item.ProjectName.ValidateResPlan()`。其它回包校验保持。CRP 拒未来年份时按 R-005 停评。

## Risks / Trade-offs

- [页面 / 简易提单 API 直调也可传入未来年] → 下拉仍卡窗口，日常用户选不到；属「选择严、校验宽」的明确取舍。
- [CRP 不认未来年份] → 本期不在 HCM 兜底。
- [未来年裁撤不被 `IsDissolveObsProjectForResPlan` 视为裁撤] → 本期不改。
- [形态不设年份上限] → `2099春节保障` 也会被校验接受。

## Migration Plan

- 纯逻辑变更，无表结构、无数据迁移。
- 回滚：回退二进制即可恢复窗口拦截。

## Open Questions

- Q-001（非阻塞）：是否加年份上限。默认不加。
- Q-002（非阻塞）：前端 / meta 是否露出未来年份。默认不改。
