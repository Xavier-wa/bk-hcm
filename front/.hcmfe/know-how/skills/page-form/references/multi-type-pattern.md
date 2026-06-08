# 多类型字段模式（多云/多资源）

与 `page-list`、`page-detail` skill 中的 Factory 机制完全一致。

## 何时需要 Factory

当同一资源在不同云厂商（`VendorEnum`）或不同资源类型（`ResourceTypeEnum`）下，**表单字段列表存在差异**时，使用 Factory 模式。

如果所有类型的字段完全一致，直接写静态装饰器类即可，无需 Factory。

## 目录结构

```
children/form/
├── form.vue
├── create.vue
├── edit.vue
├── field-factory.ts       # 工厂：根据 vendor 返回对应字段模型
└── field-tcloud.ts        # 腾讯云字段定义
```

## Factory 实现

```typescript
// field-factory.ts
import { VendorEnum } from '@/common/constant';
import { getModel } from '@/model/manager';
import { FieldTcloud } from './field-tcloud';

export class FieldFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD:
        return getModel(FieldTcloud);
      default:
        throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}
```

## 字段定义

```typescript
// field-tcloud.ts
import { Model, Column } from '@/decorator';

@Model()
export class FieldTcloud {
  @Column('string', { apiOnly: true })
  id: string;

  @Column('string', { name: '名称', required: true })
  name: string;

  @Column('enum', {
    name: '类型',
    required: true,
    option: { '1': { label: '类型A' }, '2': { label: '类型B' } },
  })
  type: string;
}
```

## 继承公共字段（推荐）

当多种类型之间存在大量公共字段时，使用**继承**抽取基类：

```typescript
// field.ts（基类：定义所有公共字段）
@Model('module/form-field')
export class FormField {
  @Column('string', { apiOnly: true })
  id: string;

  @Column('string', { name: '名称', required: true })
  name: string;

  @Column('string', { name: '描述', meta: { display: { props: { type: 'textarea', rows: 3 } } } })
  memo: string;
}

// field-tcloud.ts（继承：添加特有字段）
@Model('module/form-field-tcloud')
export class FormFieldTcloud extends FormField {
  @Column('string', { name: '腾讯云地域', required: true })
  region: string;
}

// field-aws.ts（继承：添加特有字段）
@Model('module/form-field-aws')
export class FormFieldAws extends FormField {
  @Column('string', { name: 'AWS 区域', required: true })
  aws_region: string;
}
```

## 单类型简化

若无需多类型支持，直接去掉 Factory：

```typescript
// form.vue 中
import { getModel } from '@/model/manager';
import { FormFields } from './fields'; // 单个装饰器类

const fieldModel = computed(() => getModel(FormFields));
```
