# 表单字段定义模式

## @Column 参数详解

表单字段通过 `@Column(type, options)` 定义，`options` 支持以下属性：

| 属性 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `name` | `string` | 字段显示名称（label） | `'模板名称'` |
| `required` | `boolean` | 是否必填 | `true` |
| `rules` | `IRule[]` | 验证规则数组 | `[{ validator, message, trigger }]` |
| `option` | `Record<string, OptionItem>` | 枚举选项（radio/select） | `{ '1': { label: '引用', disabled: false } }` |
| `apiOnly` | `boolean` | 仅用于 API，不在表单展示 | `true` |
| `meta.display.appearance` | `string` | 组件外观类型 | `'radio'` |
| `meta.display.props` | `Record<string, any>` | 组件额外 props | `{ type: 'textarea', rows: 3 }` |

## 验证规则

```typescript
interface IRule {
  validator?: (value: any) => boolean;   // 同步验证函数
  message?: string;                       // 错误提示
  trigger?: 'blur' | 'change';            // 触发时机
}
```

```typescript
@Column('string', {
  name: '模板名称',
  required: true,
  rules: [
    {
      validator: (value: string) => /^[a-zA-Z0-9_-]{1,128}$/.test(value),
      message: '长度为1~128个字符，可包含英文字母、数字和-_',
      trigger: 'blur',
    },
  ],
})
name: string;
```

## 枚举选项

用于 `radio`、`select` 等选择类组件：

```typescript
@Column('string', {
  name: '权限模板类型',
  required: true,
  option: {
    '1': { label: '引用策略库', disabled: false },
    '2': { label: '自定义', disabled: true },
  },
  meta: {
    display: { appearance: 'radio' },
  },
})
type: string;
```

## 组件 Props

通过 `meta.display.props` 向表单组件传递额外属性：

```typescript
// 文本域
@Column('string', {
  name: '模板描述',
  meta: {
    display: {
      props: { type: 'textarea', rows: 3, maxlength: 100 },
    },
  },
})
memo: string;

// 只读文本域
@Column('string', {
  name: '模板预览',
  meta: {
    display: {
      props: {
        type: 'textarea',
        readonly: true,
        rows: 10,
        placeholder: '内容由策略库自动生成，不可手动编辑',
      },
    },
  },
})
policy_document: string;

// 下拉框配置
@Column('list', {
  name: '权限策略库',
  required: true,
  meta: {
    display: {
      props: { idKey: 'id', displayKey: 'name' },
    },
  },
})
policy_library_id: string;
```

## apiOnly 字段

`apiOnly: true` 表示该字段只在 API 提交时使用，不在表单 UI 中展示：

```typescript
@Column('string', { apiOnly: true })
id: string;  // 编辑时由后端返回，新建时为空
```

这类字段会被包含在 `formData` 中，但被 `fields.filter(f => !f.apiOnly)` 过滤掉。

## 字段类型与表单组件映射

> 项目内部表单组件位于 `@/components/form/`。以下列出所有已实现的 `hcm-form-*` 组件，具体用法可直接查看对应组件源码。

| 字段类型 | 表单组件 | 源码路径 | 典型配置 |
|---------|---------|---------|---------|
| `string` | `hcm-form-string` | `@/components/form/string.vue` | `type: 'text' \| 'textarea'`, `maxlength`, `placeholder` |
| `number` | `hcm-form-number` | `@/components/form/number.vue` | `min`, `max`, `precision` |
| `bool` | `hcm-form-bool` | `@/components/form/bool.vue` | — |
| `enum` | `hcm-form-enum` | `@/components/form/enum.vue` | `option`, `appearance: 'radio' \| 'select'` |
| `list` | `hcm-form-list` | `@/components/form/list.vue` | `listGenerator`, `idKey`, `displayKey` |
| `user` | `hcm-form-user` | `@/components/form/user.vue` | — |
| `datetime` | `hcm-form-datetime` | `@/components/form/datetime.vue` | — |
| `business` | `hcm-form-business` | `@/components/form/business.vue` | — |
| `array` | `hcm-form-array` | `@/components/form/array.vue` | — |
| `cert` | `hcm-form-cert` | `@/components/form/cert.vue` | `appearance: 'cert' \| 'select'` |
| `ca` | `hcm-form-ca` | `@/components/form/ca.vue` | `appearance: 'ca' \| 'select'` |

### 选项数量与组件选择

| 选项数量 | 推荐方式 | 说明 |
|---------|---------|------|
| ≤ 3 个 | `string` + `appearance: 'radio'` | 直接平铺展示，无需下拉 |
| > 3 个 或 需远程加载 | `enum` / `list` | 使用下拉选择器，节省空间 |

**3 个以下使用 radio**：

```typescript
@Column('string', {
  name: '权限模板类型',
  required: true,
  option: {
    '1': { label: '引用策略库', disabled: false },
    '2': { label: '自定义', disabled: true },
  },
  meta: {
    display: { appearance: 'radio' },
  },
})
type: string;
```

### option 常量引用

使用 `enum` 或 `list` 时，`option` 通常引用模块内定义的常量，而非内联写死：

```typescript
// 从模块 constants 导入
import { OPERATION_LOG_SOURCE_NAME } from '@/views/operation-log/constants';

@Column('enum', {
  name: '操作来源',
  option: OPERATION_LOG_SOURCE_NAME,
})
source: OperationLogSource;
```

### enum 与 list 的区别

| 维度 | `enum`（`hcm-form-enum`） | `list`（`hcm-form-list`） |
|------|--------------------------|--------------------------|
| 数据源 | 静态 `option` 配置 | 支持远程数据（`listGenerator`） |
| 顺序 | 不保序（对象 key 无序） | 保序（数组顺序） |
| number 值 | 需要特殊处理（key 为 string） | 直接支持 |
| 典型场景 | 固定选项少（如类型、状态） | 选项多或需远程加载（如策略库、资源列表） |

**enum 的 number 值处理**：

```typescript
// enum 的 option key 必须是 string，number 值需要在外层转换
@Column('enum', {
  name: '状态',
  option: { '0': { label: '禁用' }, '1': { label: '启用' } },
})
status: number;  // 提交前需将 string '0'/'1' 转为 number
```

**list 的远程数据**：

```typescript
@Column('list', {
  name: '策略库',
  meta: {
    display: {
      props: { idKey: 'id', displayKey: 'name' },
    },
  },
})
policy_library_id: string;

// form.vue 中注入数据生成器
const getFormCompProps = (field) => {
  if (field.id === 'policy_library_id') {
    return { listGenerator: async () => store.getPolicyList() };
  }
};
```
