# DisplayValue 组件使用模式

## 组件来源

```typescript
import DisplayValue from '@/components/display-value/display-value.vue';
```

## Props

| Prop | 类型 | 说明 |
|------|------|------|
| `property` | `ModelPropertyDisplay` | 字段定义（含 name、type、meta 等） |
| `value` | `any` | 字段值 |
| `display` | `object` | 显示配置，覆盖 property.meta.display |

## Display 配置

```typescript
interface IDisplayConfig {
  on?: 'form' | 'info' | 'table';     // 应用场景
  appearance?: string;                // 外观类型
  appearanceProps?: Record<string, any>; // 外观附加属性
  render?: (value: any) => VNode;     // 自定义渲染函数
}
```

## Value 绑定规则

```vue
<!-- 规则：render 函数是否需要依赖本字段之外的其它字段值？ -->
<!-- 不需要 → 传本字段值 -->
<display-value :property="field" :value="data[field.id]" />

<!-- 需要 → 传整个 data 对象 -->
<display-value :property="field" :value="data" />
```

## 常见用法

### 1. 默认展示（根据字段类型自动渲染）

```vue
<display-value
  :property="field"
  :value="data[field.id]"
  :display="{ on: 'info' }"
/>
```

### 2. 自定义渲染（render）

当 render 需要访问整个数据对象时，字段 ID 通常以 `extension.` 为前缀：

```typescript
@Column('string', {
  name: '模板类型',
  group: '基本信息',
  meta: {
    display: {
      render: (data: IPermissionTemplateItem) => {
        const { label, theme } = getTypeData(data);
        return h(Tag, { radius: '4px', theme }, label);
      },
    },
  },
})
extension.cloud_type: number;
```

对应模板中需要传整个 `data`：

```vue
<display-value :property="field" :value="data" />
```

### 3. Link-Popover 外观（关联资源悬浮展示）

```vue
<display-value
  :property="field"
  :value="data.associated_sub_account_count"
  :display="{
    appearance: 'link-popover',
    appearanceProps: {
      loadFn: async () => [{ id: '1', label: 'account-1' }],
      onLinkClick: (item) => routerAction.open({ name: MENU_XXX, query: { id: item.id } }),
      emptyText: '未查询到关联资源',
    },
  }"
/>
```

### 4. 特殊字段的模板覆盖

当 `display-value` 无法满足需求时，直接用 `<template>` 覆盖：

```vue
<grid-item label="所属二级账号">
  <template v-if="field.id === 'account_id'">
    <SecondaryAccountValue
      :value="data.cloud_account_id"
      :biz-id="getBizsId()"
      :vendor="currentVendor"
    />
    <Share class="icon" @click="handleGoToAccount(data)" />
  </template>
</grid-item>
```

## 字段类型映射

> 以下类型会持续扩展，最新完整列表参考 `@/model/typings.ts` 中的 `ModelPropertyType`。

| 字段类型 | 说明 | display-value 渲染 |
|---------|------|-------------------|
| `string` | 字符串 | 纯文本 |
| `number` | 数字 | 纯文本 |
| `bool` | 布尔 | 是/否 |
| `datetime` | 日期时间 | 格式化时间 |
| `enum` | 枚举 | 根据枚举值映射文本 |
| `list` | 列表 | 标签列表 |
| `array` | 数组 | 逗号分隔文本 |
| `json` | JSON | 代码块 / 折叠展示 |
| `user` | 用户 | 用户名 + 头像 |
| `account` | 账号 | 账号名称 |
| `region` | 地域 | 地域名称 |
| `business` | 业务 | 业务名称 |
| `cloud-area` | 云区域 | 云区域名称 |
| `cert` | 证书 | 证书信息 |
| `ca` | CA 证书 | CA 证书信息 |

## Appearance 类型

> 以下类型会持续扩展，最新完整列表参考 `@/model/typings.ts` 中的 `AppearanceType`。

| appearance | 说明 |
|-----------|------|
| `status` | 状态标签（带颜色） |
| `dynamic-status` | 动态状态（支持状态流） |
| `tag` | 标签展示 |
| `link` | 链接跳转 |
| `link-button` | 带图标的链接按钮 |
| `wxwork-link` | 企业微信链接 |
| `cvm-status` | CVM 专用状态 |
| `clb-status` | CLB 专用状态 |
| `business-assign-tag` | 业务分配标签 |

## 路由跳转规范

详情页中涉及的路由跳转必须使用 `routerAction`：

```typescript
import routerAction from '@/router/utils/action';

// 在当前窗口跳转（带历史记录）
routerAction.redirect(
  { name: MENU_BUSINESS_XXX, query: { id } },
  { history: true },
);

// 新开窗口
routerAction.open({ name: MENU_BUSINESS_XXX, query: { id } });

// 返回上一页
routerAction.back();
```

禁止直接使用 `router.push` 或 `window.open`。
