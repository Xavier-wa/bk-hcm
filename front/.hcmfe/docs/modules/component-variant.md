# 组件级多云与内外版差异化（.plugin 模式）

> status: drafted · kind: module
> globs: `src/**/*.plugin.ts`、`src/**/*-internal.plugin.ts`
> 系统基础特色能力。规范单个组件如何同时应对「多云厂商」与「内外版（外部版 / 内部版 bcc）」两条正交的差异化。典范实现：`src/components/account-selector/`。

## 两条正交的差异轴

| 差异轴 | 生效时机 | 机制 |
|---|---|---|
| 多云厂商（VendorEnum） | 运行时 | Factory + 属性映射（`data-factory.ts` / `vendor-property.ts`） |
| 内外版（外部版 / 内部版 bcc） | 构建时 | `.plugin.ts` ↔ `-internal.plugin.ts` 模块替换 |

## 一、内外版：构建时模块替换（核心）

`front/bk.config.js` 用 `webpack.NormalModuleReplacementPlugin` 匹配 `/\.plugin(\.\w+)?$/`：当 `process.env.TARGET_MODE === 'bcc'`（内部版）时，把 import 请求 `xxx.plugin` 自动改写为 `xxx-internal.plugin`。

```js
// bk.config.js（节选）
config.plugin('moduleReplacement').use(webpack.NormalModuleReplacementPlugin, [
  /\.plugin(\.\w+)?$/,
  (resource) => {
    if (process.env.TARGET_MODE === 'bcc') {
      resource.request = resource.request.replace(/\.plugin/, '-internal.plugin');
    }
  },
]);
```

因此约定：

- **`xxx.plugin.ts`** = 外部版（默认 / 兜底）实现。
- **`xxx-internal.plugin.ts`** = 内部版（bcc 构建）实现，构建时自动顶替 `xxx.plugin.ts`。
- **组件里永远只 `import './xxx.plugin'`**，严禁直接 import `-internal.plugin`——内外版切换完全交给构建，业务代码无感知、无分支。
- 内部版实现通常在外部版基础上**扩展**（spread 默认值再增补），而非另起炉灶。

### 抽象基类 + 插件子类

差异点用「抽象类定公共逻辑 + 抽象方法留差异」的方式收口（account-selector 的 filter）：

```typescript
// filter.class.ts —— 公共逻辑（何时过滤）+ 差异点（过滤规则）
export default abstract class {
  accountFilter(list, { route, whereAmI, resourceType }) {
    // 仅在 CLB/证书/参数模板等场景才过滤
    if (/* ...场景判断... */) return list.filter(this.filterfn);
    return list;
  }
  abstract filterfn(value: IAccountItem): unknown;
}

// filter.plugin.ts（外部版）：只认腾讯云
class FilterPlugin extends Filter { filterfn = (v) => v.vendor === VendorEnum.TCLOUD; }

// filter-internal.plugin.ts（内部版）：腾讯云 + 自研云 ZIYAN
class FilterPlugin extends Filter { filterfn = (v) => v.vendor === VendorEnum.TCLOUD || v.vendor === VendorEnum.ZIYAN; }
```

## 二、多云：运行时 Factory + 属性映射

- `data-factory.ts` 的 `optionFactory(vendor)` 返回一个 `FactoryType`（如 `{ useList, vendorProperty }`）；`optionMap` 未命中的厂商回落到 `dataCommon`。这是**按厂商替换数据源/行为**的扩展点。
  ```typescript
  export default function optionFactory(vendor?: VendorEnum): FactoryType {
    const optionMap: { [K in VendorEnum]?: FactoryType } = {};
    return optionMap[vendor] ?? dataCommon;   // 默认走通用实现
  }
  ```
- `vendor-property.ts` 维护每个厂商的视觉属性（icon / style）映射，组件据此渲染厂商标签/图标；组件从数据里动态得到厂商列表，无需硬编码。
- 多云与内外版**叠加**：`vendor.plugin.ts` 仅 re-export 默认属性；`vendor-internal.plugin.ts` 在默认基础上加内部版专属厂商（如 `ZIYAN` 自研云，复用腾讯云图标）——即"内部版比外部版多支持一朵云"。

## 典范文件布局（account-selector）

```
components/account-selector/
├── index-new.vue            # 组件本体，只 import './xxx.plugin'
├── data-common.ts           # 多云通用数据源（useList + vendorProperty）
├── data-factory.ts          # 多云运行时工厂（按 vendor 选实现）
├── vendor-property.ts       # 厂商视觉属性映射（icon/style）
├── vendor.plugin.ts         # 外部版：re-export 默认属性
├── vendor-internal.plugin.ts# 内部版：默认 + ZIYAN
├── filter.class.ts          # 抽象基类：公共过滤逻辑 + 抽象 filterfn
├── filter.plugin.ts         # 外部版过滤规则（TCLOUD）
└── filter-internal.plugin.ts# 内部版过滤规则（TCLOUD + ZIYAN）
```

## 约定与边界

- **只 import `.plugin`**：任何需要内外版差异的能力，抽成 `xxx.plugin.ts`（外部版）+ `xxx-internal.plugin.ts`（内部版），两者导出同一接口/默认导出；调用方引用 `.plugin`。
- **外部版是默认与兜底**：无 `-internal.plugin.ts` 时外部版对内外版都生效。
- **分工**：组件"能力/行为"差异 → 本模式（factory / property / filter 插件）；页面"字段列表"差异 → [model](model.md) 的字段模型 Factory。
- **粒度边界**：本模块管**行为/数据级**替换（`.plugin.ts`）；若要按版本替换**整块 UI 片段**（Vue SFC / TSX 渲染函数，`.plugin.vue` / `.plugin.tsx`），见 [page-variant](page-variant.md)——同机制、异粒度。
- **与 plugin-handler 的关系**：`@pluginHandler` 别名（`src/plugin-handler`，`TARGET_MODE=bcc` 时指向 `/bcc`）是**目录级**的同源内外版机制；[plugin-handler](plugin-handler.md) 模块已标退场中，**新代码优先用本模式的文件级 `.plugin` 替换**。
- 现有使用点：account-selector、`views/resource/.../security-group/*.plugin.ts`、`views/service/service-apply/*.plugin.ts` 等。
