# 多类型字段模式（多云/多资源）

与 `page-list` skill 中的 Factory 机制完全一致。

## 何时需要 Factory

当同一资源在不同云厂商（`VendorEnum`）或不同资源类型（`ResourceTypeEnum`）下，**字段列表存在差异**时，使用 Factory 模式。

如果所有类型的字段完全一致，直接写静态装饰器类即可，无需 Factory。

## 目录结构

```
children/details/
├── details.vue
├── field-factory.ts       # 工厂：根据 vendor 返回对应字段模型
└── field-tcloud.ts        # 腾讯云字段定义
```

## Factory 实现

```typescript
// field-factory.ts
import { VendorEnum } from '@/common/constant';
import { getModel } from '@/model/manager';
import { DetailsFieldTcloud } from './field-tcloud';

export class FieldFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD:
        return getModel(DetailsFieldTcloud);
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
export class DetailsFieldTcloud {
  @Column('string', { name: '模板名称', group: '基本信息' })
  name: string;

  @Column('string', {
    name: '模板类型',
    group: '基本信息',
    meta: {
      display: {
        render: (value) => h(Tag, { theme }, label),
      },
    },
  })
  'extension.cloud_type': number;

  @Column('user', { name: '创建人', group: '基本信息' })
  creator: string;

  @Column('datetime', { name: '创建时间', group: '基本信息' })
  created_at: string;

  @Column('json', { name: '', group: '权限模板' })
  policy_document: string;
}
```

## 继承公共字段（推荐）

当多种类型之间存在大量公共字段时，使用**继承**抽取基类：

```typescript
// field.ts（基类：定义所有公共字段）
@Model('module/details-field')
export class DetailsField {
  @Column('string', { name: '名称', group: '基本信息' })
  name: string;

  @Column('user', { name: '创建人', group: '基本信息' })
  creator: string;

  @Column('datetime', { name: '创建时间', group: '基本信息' })
  created_at: string;
}

// field-tcloud.ts（继承：添加特有字段）
@Model('module/details-field-tcloud')
export class DetailsFieldTcloud extends DetailsField {
  @Column('string', { name: '腾讯云特有字段', group: '扩展信息' })
  tcloud_specific: string;
}

// field-aws.ts（继承：添加特有字段）
@Model('module/details-field-aws')
export class DetailsFieldAws extends DetailsField {
  @Column('string', { name: 'AWS 特有字段', group: '扩展信息' })
  aws_specific: string;
}
```

## 单类型简化

若无需多类型支持，直接去掉 Factory，在 `details.vue` 中：

```typescript
// 不需要 inject currentVendor
// 不需要 field-factory.ts

import { getModel } from '@/model/manager';
import { DetailsFields } from './fields'; // 单个装饰器类

const model = getModel(DetailsFields);
const properties = model.getPropertiesByGroup<ModelPropertyDisplay>();
```
