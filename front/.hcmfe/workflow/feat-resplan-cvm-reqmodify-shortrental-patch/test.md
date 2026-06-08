# feat-resplan-cvm-reqmodify-shortrental-patch 测试清单

> 工作流: `feat-resplan-cvm-reqmodify-shortrental-patch`  
> 关联文档: `coding.md`  
> 负责人: 开发自测

## 验证范围

- 功能边界: 资源预测「修改需求」页加载单据详情后，侧栏「修改预测需求」表单字段与 `updated_info` 一致（短租退回日期、云磁盘容量/实例、资源类型推断）
- 不在范围: 覆盖提交 overwrite、首迭代列表/新增、非短租项目全量回归

## 测试环境

- 前端入口: 业务视角 → 资源预测 → 可修改状态的 CVM 预测单详情 →「修改需求」
- 测试账号: `<TEST_ACCOUNT>`
- 数据准备:
  - 主单状态为 `rejected` / `partial_rejected` / `failed` / `partial_failed` / `revoked` 之一，且无 `done` 子单
  - 至少一条 demand：`obs_project=短租项目`，`updated_info` 含 `return_plan_time`、`cvm.os`、`cbs.disk_size`（可参考单据 `000000do` 结构）

## 用例清单

### P0 - 主流程（必测）

| ID | 场景 | 前置 | 操作 | 期望 | 回滚 |
|----|------|------|------|------|------|
| P0-01 | 短租退回日期回填 | 短租项目 demand，详情含 `return_plan_time` | 进入修改页 → 列表点「修改」 | 侧栏「短租退回日期」与接口一致（如 2026-08-15） | 关闭侧栏 |
| P0-02 | 云磁盘容量/实例回填 | CVM+CBS，`disk_size` 与 `os` 均 >0 | 同上 | 「云磁盘容量/实例」= floor(disk_size/os)（如 1/1=1） | 关闭侧栏 |
| P0-03 | 无 demand_res_types 推断 | `updated_info` 无 `demand_res_types` 但有 cvm+cbs | 打开修改侧栏 | 展示 CVM/CBS 相关表单项，资源类型为 cvm | 关闭侧栏 |

### P1 - 异常分支（必测）

| ID | 场景 | 前置 | 操作 | 期望 |
|----|------|------|------|------|
| P1-01 | os 为字符串 | 详情 `cvm.os` 为 `"1"` | P0-02 | 推算仍为数值 1，不为 0 |
| P1-02 | original_info 为 null | 新增型驳回单 `original_info: null` | 进入修改页 | 页面不报错，列表与侧栏可打开 |

### P2 - UI 细节（按需）

- [ ] 期望到货日期已填时，短租退回日期日期选择器可点
- [ ] 云盘总量展示与 disk_size 一致

## 验证结论

| ID | 结果 | 备注 |
|----|------|------|
| P0-01 | PASS | 开发自测确认 |
| P0-02 | PASS | 开发自测确认 |
| P0-03 | PASS | 样例单 000000do |
| P1-01 | PASS | os 字符串 "1" 场景 |
| P1-02 | PASS | original_info null |
| P2 | Skipped | 未单独测 UI 细节 |

- 执行人: `<DEVELOPER_NAME>`
- 执行日期: 2026-05-21
- 总体结论: PASS
- 后续行动: 可合入 `feat-resplan-cvm-reqmodify-shortrental-patch`
