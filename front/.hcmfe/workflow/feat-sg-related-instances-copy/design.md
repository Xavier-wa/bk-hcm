# Design：安全组-关联实例-增加复制按钮（无设计稿）

> **本迭代无独立 Figma 设计稿**。豁免原因：功能为在已有页面的操作栏中新增复制按钮，UI 布局沿用现网，TAPD 父需求中附有设计参考截图。

## 0. 本子需求范围

| 在本迭代内 | 不在本迭代内 |
|------------|--------------|
| `platform.vue` — 账号下/资源运营 `.operate-btn-wrap` 中新增「复制」下拉按钮 | 其他页面或组件 |
| `collapse-data-list.vue` — 业务下当前业务 `.tools` 中新增「复制内网IP」「复制主机ID」按钮 | 业务下非当前业务的折叠面板 |
| 跨页选择能力（如当前不支持，需实现） | — |

## 1. 设计稿引用

| 状态 | 说明 |
|------|------|
| 无 Figma 稿 | 本迭代无 Figma 设计稿 |
| TAPD 参考图1 | 业务下-管理业务 — `https://<TAPD_HOST>/tfl/captures/2026-01/tapd_69995598_base64_1767489984_748.png` |
| TAPD 参考图2 | 业务下-使用业务 — `https://<TAPD_HOST>/tfl/captures/2026-01/tapd_69995598_base64_1767490187_990.png` |
| TAPD 参考图3 | 账号下 — `https://<TAPD_HOST>/tfl/captures/2026-01/tapd_69995598_base64_1767489825_186.png` |

## 2. UI 参照

### 2.1 功能1：账号下 / 资源运营 — 复制下拉按钮（platform.vue）

**现网页面**：`front/src/views/resource/resource-manage/children/components/security/security-relate/platform.vue`

**当前 `.tools-bar` 布局**：
```
[Tab 切换]   [新增绑定] [批量解绑]              [搜索框]
              └─ .operate-btn-wrap ─┘
```

**新增后 `.operate-btn-wrap` 布局**：
```
[新增绑定] [批量解绑] [复制 ▾]                   [搜索框]
                       └─ 新增 ─┘
                            ├── 复制内网IP
                            └── 复制实例ID
```

- 「复制」按钮位于「批量解绑」按钮右侧，同一行
- 使用 `HcmDropdown` 组件包裹，按钮类型为普通按钮（非 primary），右侧带 angle-down 图标
- 下拉菜单包含两个 `CopyToClipboard type="dropdown-item"` 项
- 按钮状态与「批量解绑」一致：默认置灰，勾选后点亮

**参考实现**：
- `front/src/views/load-balancer/clb/children/batch-copy.vue`
- `front/src/views/load-balancer/device/main-content/children/batch-copy.vue`

### 2.2 功能2：业务下 — 一键复制按钮（collapse-data-list.vue）

**现网页面**：`front/src/views/resource/resource-manage/children/components/security/security-relate/data-list/collapse-data-list.vue`

**当前 `.tools` 栏布局**（当前业务）：
```
[▼] 业务名称  [当前业务]  [+ 新增绑定]              [批量解绑]
```

**新增后 `.tools` 栏布局**（当前业务）：
```
[▼] 业务名称  [当前业务]  [+ 新增绑定]  [复制内网IP] [复制主机ID]  [批量解绑]
```

- 「复制内网IP」和「复制主机ID」两个独立按钮位于「新增绑定」和「批量解绑」之间
- 使用 `CopyToClipboard` 组件，`type="icon"`（默认模式，显示为图标+文本按钮）
- 仅当前业务（`isCurrentBusiness = true`）面板显示
- 数据为空时按钮置灰

**非当前业务 `.tools` 栏布局**：不新增任何按钮，保持现网不变。

### 2.3 跨页选择能力（功能1 依赖）

- 当前 `platform.vue` 的 `data-list` 已有 `@select` 事件驱动 `selected` ref
- 需确认当前是否已支持跨页选择；若未支持：
  - 参照项目已支持跨页选择的列表页实现
  - 保证对现有「批量解绑」功能无影响
  - 跨页勾选的数据在翻页后保持

## 3. 与 PRD 验收映射

| PRD 验收项 | Design 结论 |
|------------|-------------|
| 未勾选时复制按钮置灰 | 与「批量解绑」按钮状态逻辑对齐 |
| 勾选后按钮点亮 | 与「批量解绑」按钮状态逻辑对齐 |
| 下拉菜单包含"复制内网IP"和"复制实例ID" | HcmDropdown 包裹两个 CopyToClipboard dropdown-item |
| 复制成功弹出提示 | CopyToClipboard 组件内置 Message 提示 |
| 复制后下拉关闭 | CopyToClipboard dropdown-item 默认行为 |
| 业务下当前业务显示复制按钮 | 在 `.tools` 模板 `isCurrentBusiness` 分支内新增 |
| 非当前业务不显示 | 现网非当前业务模板不变 |
| 无数据时置灰 | `relResList.length === 0` 时 disabled |
| 功能2 全部数据复制 | 需调用 rollRequest 拉取全部页数据后复制 |
| 功能1 跨页勾选复制 | 需确保跨页选择已支持并兼容批量解绑 |

## 4. 组件与交互要点

### 4.1 功能1 组件选型

| 元素 | 使用组件 | 说明 |
|------|----------|------|
| 下拉按钮容器 | `HcmDropdown`（`@/components/hcm-dropdown/index.vue`） | 项目已有，提供 hover 展开下拉 |
| 复制内网IP 菜单项 | `CopyToClipboard` `type="dropdown-item"` | 项目已有 `@/components/copy-to-clipboard/index.vue` |
| 复制实例ID 菜单项 | `CopyToClipboard` `type="dropdown-item"` | 同上 |
| IP 提取工具 | `getPrivateIPs`（`@/utils/common.ts`） | 拼接 IPv4 + IPv6 地址 |

### 4.2 功能2 组件选型

| 元素 | 使用组件 | 说明 |
|------|----------|------|
| 复制内网IP 按钮 | `CopyToClipboard` 默认 type（icon 模式） | 与项目其他独立复制按钮一致 |
| 复制主机ID 按钮 | `CopyToClipboard` 默认 type | 同上 |
| IP 提取工具 | `getPrivateIPs`（`@/utils/common.ts`） | 拼接 IPv4 + IPv6 地址 |

### 4.3 按钮风格

| 位置 | 按钮类型 | 风格 |
|------|----------|------|
| platform.vue `.operate-btn-wrap` | `HcmDropdown` 包裹普通按钮 | 默认按钮 + angle-down 图标，与「批量解绑」同风格 |
| collapse-data-list.vue `.tools` | `CopyToClipboard` 默认 type | icon+文本 text 按钮，与「新增绑定」同风格 |

## 5. 关键图标语义

本功能不涉及独立图标选型：
- 复制按钮图标由 `CopyToClipboard` 组件内置的 copy 图标提供
- 下拉展开图标由 `HcmDropdown` 组件内置的 angle-down 图标提供
- 无自定义 iconfont 或 bkui 图标需求
