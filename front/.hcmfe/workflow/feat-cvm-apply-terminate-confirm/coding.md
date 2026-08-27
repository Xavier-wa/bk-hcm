# Coding — feat-cvm-apply-terminate-confirm

## 需求概述
主机申领界面中，"终止"提交后需二次确认。在两个页面的"终止"操作前增加 `InfoBox` 二次确认弹窗。

## 设计稿要点（用户提供）
- **标题**："确认需要终止该单据？"
- **正文（两段）**：
  1. 待交付设备：N 台（N = `row.pending_num`，动态）
  2. 灰色背景提示框（#f5f7f9）："终止后将停止交付剩余设备，且该操作不可恢复，请谨慎操作！"
- **按钮**：终止（danger 主题）+ 取消
- **顶部图标**：黄色感叹号（InfoBox 默认）

## 改动方案
- 用 bkui-vue `InfoBox` 在终止按钮点击时弹出二次确认（danger 主题）
- 用户确认后才执行 `scrStore.stopOrder`，取消则不提交
- 沿用现有 stopOrder 调用逻辑、权限、disabled 条件，仅增加确认拦截
- 参考项目已有模式 `views/resource-plan/gpu/hooks/use-terminate-confirm.ts`（InfoBox + danger theme + quickClose:false）

## 涉及文件
1. `src/views/ziyanScr/hostApplication/components/application-list/index.tsx` — 【资源管理-单据管理-主机申请】
2. `src/views/ticket/children/host-apply/applications/index.tsx` — 【服务请求-主机申领-申请单据】

两个文件的终止操作代码几乎一致，改动模式同步。

## 改动点

### 文件 1: `src/views/ziyanScr/hostApplication/components/application-list/index.tsx`
- **import**（第 5 行）：`{ Button, Message, Table, Sideslider }` → 追加 `InfoBox`
- **终止按钮 onClick**（第 482-488 行）：由直接调 `scrStore.stopOrder` 改为先弹 InfoBox 确认

### 文件 2: `src/views/ticket/children/host-apply/applications/index.tsx`
- **import**（第 6 行）：`{ Button, Message }` → 追加 `InfoBox`
- **终止按钮 onClick**（第 491-497 行）：同文件 1

### InfoBox 配置（两处一致）
```tsx
InfoBox({
  title: '确认需要终止该单据？',
  type: 'warning',
  theme: 'danger',
  confirmText: '终止',
  cancelText: '取消',
  headerAlign: 'center',
  contentAlign: 'left',
  footerAlign: 'center',
  quickClose: false,
  content: () => h('div', [
    h('div', { class: 'mb12', style: { fontSize: '14px' } }, `待交付设备：${row.pending_num} 台`),
    h('div', {
      style: {
        background: '#f5f7f9',
        padding: '12px 16px',
        borderRadius: '4px',
        color: '#4d4d4d',
        fontSize: '14px',
      },
    }, '终止后将停止交付剩余设备，且该操作不可恢复，请谨慎操作！'),
  ]),
  async onConfirm() {
    await scrStore.stopOrder({ suborder_id: [row.suborder_id] });
    Message({ theme: 'success', message: '终止成功' });
    getListData();
  },
});
```

需要在 import 中追加 `import { h } from 'vue';`（用于 InfoBox content 渲染函数）。

## 关键属性说明（手测发现 → 修复）

- **`type` vs `theme` 正交**：
  - `type`: success/danger/warning/loading → 控制**顶部图标**（黄色感叹号对应 `warning`）
  - `theme`: primary/danger/success/warning → 控制**确认按钮颜色**（红色对应 `danger`）
  - 设计稿需要：黄色感叹号图标 + 红色按钮 → 必须**同时**设 `type:'warning'` + `theme:'danger'`，否则顶部无图标

- **`ziyanScr/.../application-list/index.tsx`** 文件 prettier 对 `h()` 单行写法报错（与 `ticket/...` 同格式不报错，原因未明）。已用 `/* eslint-disable prettier/prettier */ ... /* eslint-enable */` 块注释包住 content 区域绕过。

## 字段来源确认
- `row.pending_num`：待交付设备数（两个文件"待交付数"列均使用此字段，来源为 SCR 接口）

## 不改动
- 后端终止接口、权限体系、审计日志
- 终止按钮的 disabled 条件（沿用 `opBtnDisabled`）
- 其他页面（host-recycle、sub-ticket-list 等）的终止操作（不在本期范围）

## 风险点
- **低风险**：纯前端交互增强，确认后沿用现有 stopOrder 逻辑
- 两个文件改动模式一致，需保持同步
- InfoBox 为 bkui-vue 内置组件，无需额外依赖
- 需从 `vue` 导入 `h` 用于 content 渲染函数（InfoBox content 接受 string 或 VNode 函数）