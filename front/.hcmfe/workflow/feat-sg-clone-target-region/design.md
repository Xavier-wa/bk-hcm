# Design：安全组-克隆支持选择目标地域（无设计稿）

> **本迭代无独立设计稿**。豁免原因：TAPD 需求仅附现有克隆弹窗截图，无新视觉稿；本需求为在现有弹窗内新增一个下拉控件 + 提交参数透传，沿用现网 UI 风格。

## 0. 本子需求范围

| 在本迭代内 | 不在本迭代内 |
|------------|--------------|
| 克隆安全组弹窗新增「目标地域」下拉（默认源地域，可选业务可用地域全集） | 批量克隆多地域 |
| 提交时携带 `target_region`（含默认源地域场景） | 跨业务复制 |
| 负责人/备份负责人默认回填源值、可修改（现有交互核对确认） | 克隆后自动跳转目标地域列表 |
| 成功提示文案带目标地域；失败走现有错误提示框架 | 菜单/入口改造 |

## 1. 设计稿引用

| 状态 | 说明 |
|------|------|
| N/A | 无新 Figma 稿 |

## 2. UI 参照

- **现网**：`front/src/views/resource/resource-manage/children/dialog/clone-security/index.vue`（克隆安全组弹窗，Dialog 960 宽）
  - 现有结构：管理业务/使用业务展示区 → bk-form（安全组名称，vertical）→ `SecurityGroupManagerSelector`（负责人/备份负责人）→ 安全组规则区（radio 切换入站/出站 + bk-table）
  - 关键现状：
    - 负责人选择器已接收源值：`:manager="props?.data?.manager"` `:bak-manager="props?.data?.bak_manager"`（index.vue L491–495），F-002 的「默认回填」由该组件内部实现，本需求只需核对不回退。
    - 提交链路：`handleConfirm` → `useBusiness.cloneSecurity({ id, name, bak_manager, manager })`（index.vue L422–442），需在此增加 `target_region`。
    - 成功提示现文案「克隆成功！」，本需求改为携带目标地域的文案。
- **PRD 文字约束**：
  - 「目标地域」下拉位于弹窗表单字段内，建议顺序：安全组名称 → 目标地域 → 负责人 → 备份负责人。
  - 默认值 = 源安全组所在地域；选项 = 当前业务可用地域全集（复用现有地域枚举）。
  - 必选：默认即有值，不允许为空。

## 3. 与 PRD 验收映射

| PRD 验收项 | Design 结论 |
|------------|-------------|
| AC-001 弹窗含目标地域下拉、默认源地域、选项=业务可用地域全集 | 新增 bk-select 下拉，数据源复用项目现有地域枚举接口/字典（coding 阶段从现有代码确定，如 CVM 申领已有的地域数据源） |
| AC-002 跨地域克隆归属源业务、字段随提交 | 归属由后端保证（前端不改 biz）；提交参数增加 `target_region` |
| AC-003 默认源地域时行为与改造前一致 | 默认值=源地域，提交 `target_region` 恒有值（向后兼容，与后端契约对齐） |
| AC-004 成功提示「已克隆至 X 地域」并停留当前列表 | 现有 Message 成功提示文案改为携带目标地域显示名；不新增跳转逻辑 |
| AC-005 失败展示后端 message | 现有 cloneSecurity 调用链已走全局错误提示框架，不改动 |

## 3.x 关键图标语义（icon-intake 结论）

| 语义描述 | 项目候选（类名 / 组件，可空） |
|----------|------------------------------|
| N/A | 本需求无新增图标语义：UI 变更仅为克隆弹窗内新增 bk-select 下拉控件，不涉及任何图标（iconfont / bkui icon 均不引入） |

## 3.y 组件候选表（通识）

| 稿面区域/语义 | 组件候选 | 体系 | 文档确认 | 复用层级 | 落码入口 | 项目路径 |
|--------------|----------|------|----------|----------|----------|----------|
| 目标地域下拉 | bk-select | bkui-vue | 待 coding 阶段按 fe-bkui-usage 规则经 MCP 查询 prop 细节 | `adjacent`（现有克隆弹窗 Dialog 内新增控件，非标准 page/comp 模式） | 无（直接改现有组件） | `front/src/views/resource/resource-manage/children/dialog/clone-security/index.vue` |
| 负责人/备份负责人 | SecurityGroupManagerSelector（现有） | 项目组件 | 沿用现网 | `adjacent`（现有组件沿用，仅核对回填行为） | 无 | `front/src/views/resource/resource-manage/children/components/security/manager-selector/index.vue` |

## 8. 实现边界纪要（无稿版精简）

* 仅改 `front/` 前端代码；后端 `/clone` 接口已支持 `target_region`，无需后端改动。
* 目标地域下拉数据源必须复用项目现有地域枚举（禁止新建字典、禁止硬编码地域清单）。
* 修改共享组件（若涉及 SecurityGroupManagerSelector）前必须 lsp findReferences 排查全部使用方。
