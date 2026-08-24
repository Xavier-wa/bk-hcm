## 1. 校验中枢

- [x] 1.1 将 `ValidateResPlan()` 改为只校已知形态（固定五类 + `YYYY春节保障` / `YYYY机房裁撤`）
- [x] 1.2 确认 `GetObsProjectMembersForResPlan()` / `ListObsProject` 仍按当前时间窗口返回选项
- [x] 1.3 补充 `ValidateResPlan` 与下拉窗口单测

## 2. 退回计划对齐

- [x] 2.1 覆盖追加 filter / 明细改调 `ValidateResPlan()`
- [x] 2.2 `ListReturnReasonClassReq` 改调 `ValidateResPlan()`
- [x] 2.3 更新退回计划覆盖追加与原因大类单测

## 3. CRP 回包

- [x] 3.1 删除 `apply_change.go` 对 `ProjectName` 的校验
- [x] 3.2 补充回包未来春保不失败的单测

## 4. 回归

- [x] 4.1 覆盖追加 / 页面提单 / 简易提单形态校验单测
- [x] 4.2 跑 `enumor`、`types/plan`、`types/return-plan`、dispatcher 包测试
