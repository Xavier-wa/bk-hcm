# feat-resplan-cvm-reqmodify 测试清单

> 工作流: `feat-resplan-cvm-reqmodify`
> 关联文档: [prd.md](./prd.md) / [design.md](./design.md) / [api.md](./api.md) / [coding.md](./coding.md)
> 负责人: <开发自测人 / QA>

## 验证范围

- **功能边界**:
  1. 资源预测申请单「修改需求」入口（详情页 Approval 面板 + 子单标题右侧）+ 修改页（复用 add 结构, 复写提交）+ `POST /overwrite` 接口对接
  2. 子单列表「终止」按钮可用状态扩展（`failed` / `partial_failed` / `rejected` / `partial_rejected`）
  3. 非本年（≥`NON_CURRENT_YEAR_START_DATE`，当前默认 `2027-01-01`）预测的截止期限制：评审期内禁用「新增」表单的非本年日期 + 禁用列表行内 / 批量操作
- **不在范围**:
  - 资源视角 / 工作台视角的列表行为（本次仅业务视角）
  - 非 CVM 类型单据的修改流程兜底（后端拦截，前端不做拒绝）
  - 历史「申请单详情页」其他模块（资源详情、操作记录等）回归
  - 「修改需求」按钮以外的 Approval 面板原有交互

## 测试环境

- 前端入口: `<对应业务的测试环境地址，由执行人在测试时确认>`
- 测试账号: `<test_account>`
- 后端配合 / 数据准备:
  - 准备至少 1 条**可修改**的资源预测申请单（主单状态 ∈ {`rejected`/`partial_failed`/`revoked`}，且子单中无 `done` 状态）
  - 准备至少 1 条**不可修改**的申请单（主单 `done` / `processing`，或任一子单 `done`）
  - 后端 `report_deadline` 配置：
    - 场景 A（申报期）: `data.deadline = ''` 或 `data.deadline > 当前时刻`
    - 场景 B（评审期）: `data.deadline < 当前时刻`（精确到秒）
  - 准备 1 条「期望到货日期」≥`2027-01-01` 的列表数据，1 条 <`2027-01-01` 的列表数据

## 用例清单

### P0 - 主流程（必测）

| ID | 场景 | 前置 | 操作 | 期望 | 回滚 |
|----|------|------|------|------|------|
| P0-01 | 可修改状态展示「修改需求」入口 | 主单 `rejected`，无 `done` 子单 | 进入业务侧申请单详情页 | Approval 面板右侧 + 子单标题右侧均出现「修改需求」按钮 | - |
| P0-02 | 不可修改状态不展示入口 | 主单 `done`，或任一子单 `done` | 进入业务侧申请单详情页 | 两处入口均不展示，详情页其他交互不受影响 | - |
| P0-03 | 点击「修改需求」跳转修改页并回填 | P0-01 数据 | 点击按钮 | 跳转到 `/business/<bizId>/resource-plan/modify/<ticketId>`；Header 文案为「修改资源预测」；基本信息 / 需求列表 / 备注根据原单回填 | 返回 |
| P0-04 | 修改页提交成功跳回详情页 | 已进入修改页 | 调整一项需求数量 → 点击「提交」 | 调用 `POST /overwrite` 成功 → Toast 成功 → 跳转回原详情页；新数据在详情页上展示 | 重新发起预测复原 |
| P0-05 | 修改页 Header「返回」回到原详情页 | 已进入修改页 | 点击 Header 返回 | 回到原申请单详情页（非列表页） | - |
| P0-06 | 申报期：新增页日期可选非本年 | `deadline=''` 或 `deadline > 当前时刻` | 进入业务侧「新建资源预测」→ 期望到货日期 picker | `≥2027-01-01` 的日期可选；无评审期提示文案 | - |
| P0-07 | 评审期：新增页禁选非本年日期 + 提示 | `deadline < 当前时刻`（精确到秒） | 进入业务侧「新建资源预测」→ 期望到货日期 picker | `≥2027-01-01` 的日期 disabled 不可选；显示「评审期内不可提交 2027 及之后年份预测」类提示文案 | - |
| P0-08 | 评审期：列表非本年行禁用行内操作 | 评审期 + 列表内含 `expect_time ≥ 2027-01-01` 行 | 业务侧资源预测管理列表 | 「一键申领」「更多操作」对该行 disabled；hover 有 tooltip 说明原因 | - |
| P0-09 | 评审期：列表勾选含非本年禁用批量操作 | 评审期 + 多选含 ≥2027 与 <2027 数据 | 勾选包含 ≥2027 的行 | 「批量调整」「批量取消」disabled，tooltip 说明；仅选 <2027 行时按钮恢复可用 | - |
| P0-10 | 修改页不受截止期限制 | 评审期 + 原单 `expect_time ≥ 2027-01-01` 且处于可修改状态 | P0-01 入口 → 进入修改页 | 修改页 picker 仍可选 ≥2027 日期；无评审期提示；可正常提交 overwrite 成功 | - |
| P0-11 | 「终止」按钮在 4 个白名单状态下可点 | 分别准备主单 `failed` / `partial_failed` / `rejected` / `partial_rejected` 各 1 条 | 进入子单列表 | 「终止」按钮均可点击；点击后确认弹窗、终止成功后单据状态刷新与现有逻辑一致 | - |
| P0-12 | 「终止」按钮在非白名单状态下 disabled | 主单 `init` / `auditing` / `processing` / `done` / `terminated` / `revoked` | 进入子单列表 | 「终止」按钮均 disabled, 不可点击 | - |
| P0-13 | 修改页跨 `device_class` 编辑回填 (核心防退化) | 原单含 2 条需求, 分别属于不同 `device_class` (A / B); 至少其中一条 `device_type` 非空 | 进入修改页 → 编辑需求 1 (device_class=A, 不改动) → 关闭 → 编辑需求 2 (device_class=B) | 两次打开 sideslider 时，「机型类型」「机型规格」「实例数量」「CPU 总核数」「内存总量」均按原单值正确展示, 不出现空白或被清零 | - |
| P0-14 | 修改页同 demand 反复打开编辑 | 同一条需求 | 打开 → 关闭 → 再打开 → ... 5 次 | 每次打开字段都按原单值显示, 不出现 cpu_core/memory 被算成 0 的瞬态残留 | - |

### P1 - 异常分支（必测）

| ID | 场景 | 前置 | 操作 | 期望 |
|----|------|------|------|------|
| P1-01 | overwrite 接口失败回显 | 修改页, mock `POST /overwrite` 返回非 0 | 点击「提交」 | Toast 错误信息；停留在修改页；按钮恢复可点 |
| P1-02 | 详情接口失败时入口表现 | mock `getBizResourcesTicketsById` 失败 | 进入详情页 | 不渲染「修改需求」入口（不报 JS 错）；详情页有错误兜底 |
| P1-03 | 子单接口未返回时入口表现 | mock 子单接口 pending / 失败 | 进入详情页 | `subTickets === undefined` → 不展示「修改需求」入口（避免误判可修改） |
| P1-04 | 子单为空数组时仍可修改 | 主单可修改 + 子单接口返回 `[]` | 进入详情页 | 「修改需求」入口正常展示，可走 P0-03 / P0-04 |
| P1-05 | `report_deadline` 接口失败兜底 | mock `GET /report_deadline` 返回 500 / 网络异常 | 进入新建页 / 列表页 | 控制台 warn；当作申报期处理（无任何限制）；不打扰用户 |
| P1-06 | `report_deadline` 返回空字符串 | mock `data.deadline = ''` | 进入新建页 / 列表页 | 当作无截止期；非本年日期 / 行操作均不受限 |
| P1-07 | 评审期临界点（精确到秒） | 设 `deadline = T`，分别在 `T` 前 1s / 后 1s 进入 | 刷新页面进入新建页 | 前 1s 仍属申报期，picker 不禁用；后 1s 进入评审期，picker 禁用 |
| P1-08 | 修改页表单校验 | 修改页，清空必填字段 | 提交 | 表单红字提示，不发请求 |
| P1-09 | URL 直接命中修改页（无权限或单据不存在） | 手动访问 `/business/<bizId>/resource-plan/modify/<bad-id>` | 直接访问 | 详情拉取失败兜底（错误提示 / 跳转），不出现白屏或 JS 错误 |

### P2 - UI 细节（按需）

- [ ] 修改页 Header 文案为「修改资源预测」（非「新建资源预测」）
- [ ] Approval 面板内「修改需求」按钮与原有按钮在同一行右侧对齐，间距与设计一致
- [ ] 子单标题右侧「修改需求」按钮在 `#title-extra` 槽位最右
- [ ] 评审期 picker 下方提示文案样式正常（颜色 / 字号与同类提示一致），中文标点
- [ ] 列表行内 disabled 按钮 hover 有 tooltip，tooltip 文案语义明确（不是"undefined"或空）
- [ ] 批量操作按钮 disabled 时 tooltip 文案正确
- [ ] 1280 / 1440 / 1920 三种宽度下修改页 / 列表无横向滚动
- [ ] 权限受限账号下「修改需求」入口仍按 `useTicketModifiable` 规则展示，操作受统一权限拦截

## 验证结论

> 每条用例填写：PASS / FAIL / Skipped + 简要说明（FAIL 必须附复现步骤、截图链接或日志片段）

| ID | 结果 | 备注 |
|----|------|------|
| P0-01 | <PASS / FAIL / Skipped> | |
| P0-02 | <PASS / FAIL / Skipped> | |
| P0-03 | <PASS / FAIL / Skipped> | |
| P0-04 | <PASS / FAIL / Skipped> | |
| P0-05 | <PASS / FAIL / Skipped> | |
| P0-06 | <PASS / FAIL / Skipped> | |
| P0-07 | <PASS / FAIL / Skipped> | |
| P0-08 | <PASS / FAIL / Skipped> | |
| P0-09 | <PASS / FAIL / Skipped> | |
| P0-10 | <PASS / FAIL / Skipped> | |
| P0-11 | <PASS / FAIL / Skipped> | |
| P0-12 | <PASS / FAIL / Skipped> | |
| P0-13 | <PASS / FAIL / Skipped> | |
| P0-14 | <PASS / FAIL / Skipped> | |
| P1-01 | <PASS / FAIL / Skipped> | |
| P1-02 | <PASS / FAIL / Skipped> | |
| P1-03 | <PASS / FAIL / Skipped> | |
| P1-04 | <PASS / FAIL / Skipped> | |
| P1-05 | <PASS / FAIL / Skipped> | |
| P1-06 | <PASS / FAIL / Skipped> | |
| P1-07 | <PASS / FAIL / Skipped> | |
| P1-08 | <PASS / FAIL / Skipped> | |
| P1-09 | <PASS / FAIL / Skipped> | |

- 执行人: <name>
- 执行日期: <YYYY-MM-DD>
- 总体结论: PASS / FAIL（FAIL 时需在 Issue / TAPD 单据中跟进）
- 后续行动: <例如：回退到 coding 阶段修复 P0-xx / 直接合入 / 等待 QA 复测>

## 备注

- 「修改」入口豁免截止期限制是本期的明确决策（见 prd.md §3 / use-deadline-restrict 的 `enabled=false` 分支），P0-10 是这一豁免的核心防退化用例
- `report_deadline` 后端约定返回 `YYYY-MM-DD HH:MM:SS`（精确到秒），前端 `now() > deadline` 字典序比较；如果后端实际只下发 `YYYY-MM-DD`，会导致 P1-07 误判，需要回到 coding 阶段补一层格式归一
