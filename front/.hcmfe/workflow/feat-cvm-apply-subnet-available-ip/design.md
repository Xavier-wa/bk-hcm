# Design：海垒申领界面及修改界面增加子网网段"可用IP"数量显示（无设计稿）

> **本迭代无独立设计稿**。豁免原因：展示形态已在 TAPD 澄清记录中用文字明确（内联追加 `可用IP: X`），且是在现有 `CvmSubnetSelector` 组件上做内联追加展示，沿用现网 UI，无新增视觉稿。

## 0. 本子需求范围

| 在本迭代内 | 不在本迭代内 |
|------------|--------------|
| 子网下拉选项内联展示"可用IP"数量 | 扩容 / 变配等其他界面 |
| 异常 / 无数据显示"查询失败"（与可用IP=0 可区分） | IP 预警、不足时拦截提交、自动推荐网段 |
| 子网选择器附近注明口径 | 公有云 / 非自研云子网 |

## 1. 设计稿引用

| 状态 | 说明 |
|------|------|
| N/A | 无新 Figma 稿；TAPD 单据内为线上系统截图，仅作现状参考 |

## 2. UI 参照

- **现网组件**：`front/src/views/ziyanScr/components/cvm-subnet-selector/index.vue`（`CvmSubnetSelector`）
- **使用场景**：
  - 资源申领界面：`front/src/views/ziyanScr/hostApplication/components/network-info-collapse-panel/index.vue`（网络信息折叠面板的子网表单项）
  - 未生产需求调整（修改界面）：`front/src/views/ziyanScr/hostApplication/components/application-modify/index.vue`（复用同一 `network-info-collapse-panel`）

- **PRD 文字约束**（界面相关验收）：
  - F-001：选项内联展示形如 `subnet-xxx | cvm_use_199 | 可用IP: 200`
  - F-002：异常/无数据显示 `可用IP: 0`
  - F-003：口径「可用IP 为云厂商未分配可用 IP 数」
  - R-003：内联追加，不做选中后单列
  - R-004：只展示，不因可用IP不足禁用/拦截，可用IP=0 仍可选可提交

## 3. UI 交互设计

### 3.1 子网下拉选项内联展示

- 现状：选项名称为 `${subnetId} | ${subnetName}`（如 `subnet-nji0c0hp | cvm_use_199`）
- 目标：在末尾追加 ` | 可用IP: X`，最终形态 `subnet-nji0c0hp | cvm_use_199 | 可用IP: 200`
- 仅自研云场景生效（组件本身即自研云专用，无需额外判断）

### 3.2 异常 / 无数据降级

- 后端 `available_ip_count` 为 `uint64`，查询失败 / 未取到 / 剩余 0 时均返回 `0`，JSON 层不区分
- 前端统一显示 `可用IP: 0`（不区分"查询失败"与"剩余 0"，与后端现状一致）
- 不因可用 IP 不足禁用选项或拦截提交

### 3.3 口径说明

- 在子网选择器附近（子网表单项下方，与现有 `bcs-select-tips` 同级位置）注明口径文案：`可用IP 为云厂商未分配可用 IP 数`
- 使用与现有提示一致的小字号弱化样式，不干扰表单主流程

## 4. 状态流转

| 状态 | 展示 |
|------|------|
| 正常（available_ip_count 为数字 ≥ 0） | `subnetId | subnetName | 可用IP: X` |
| 查询失败 / 无数据 / 剩余 0（字段缺失或为 0） | `subnetId | subnetName | 可用IP: 0` |

- 无新增交互状态；选择、提交流程不变（AC-002）

## 5. 与 PRD 验收映射

| PRD 验收项 | Design 结论 |
|------------|-------------|
| AC-001 选项内联显示 `可用IP: X` | §3.1，Option name 追加 `可用IP: X` |
| AC-002 不影响选择与提交流程 | 仅追加展示文本，不改 v-model 与提交逻辑 |
| AC-003 查询失败/无权限/无数据显示 `可用IP: 0` | §3.2 降级 |
| AC-004 仅自研云子网显示 | 组件即自研云专用 |
| AC-005 可见口径说明 | §3.3，子网表单项下方提示 |
| AC-006 可用IP=0 显示 `可用IP: 0` 且仍可选 | §3.2，与后端现状一致（0 与失败不区分） |
| AC-P01 3 秒内渲染（P95） | 字段来自现有接口返回，无额外请求 |
