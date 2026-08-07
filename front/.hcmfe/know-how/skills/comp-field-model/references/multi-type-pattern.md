# 多类型字段模式（多云 / 多资源）

## 何时使用 Factory

当 `vendor`（云厂商）或 `resourceType`（资源类型）变化会导致字段集合、选项或展示元数据变化时，使用 Factory + `field-<type>.ts`。该判断同时适用于：

- form：输入字段与校验规则
- search：筛选条件
- table-column：表格列
- detail：详情分组与渲染

如果各类型字段完全一致，直接使用单个 `@Model` 类和 `getModel`，不要为未来可能出现的差异预建 Factory。

## Factory 模式

```typescript
import { VendorEnum } from '@/common/constant';
import { getModel } from '@/model/manager';
import { FieldAws } from './field-aws';
import { FieldTcloud } from './field-tcloud';

export class FieldFactory {
  static createModel(vendor: VendorEnum) {
    switch (vendor) {
      case VendorEnum.TCLOUD:
        return getModel(FieldTcloud);
      case VendorEnum.AWS:
        return getModel(FieldAws);
      default:
        throw new Error(`Unsupported vendor: ${vendor}`);
    }
  }
}
```

按资源类型切换时，参数改为 `ResourceTypeEnum`；需要“全部资源”时可显式支持 `'all'`，不要用隐式 fallback 吞掉未知类型：

```typescript
static createModel(resourceType: ResourceTypeEnum | 'all') {
  switch (resourceType) {
    case ResourceTypeEnum.CLB:
      return getModel(FieldClb);
    case 'all':
      return getModel(FieldAll);
    default:
      throw new Error(`Unsupported resource type: ${resourceType}`);
  }
}
```

调用方从当前页面上下文取得 `vendor` / `resourceType`，再调用 `FieldFactory.createModel(type)`。类型变化时应重新取得模型；具体 `provide` / `inject`、computed 和组件渲染由对应 `comp-*` skill 负责。

## 字段定义与公共基类

每个类型使用独立的 `@Model()` 类。公共字段较多时抽取基类，子类只追加或覆盖差异：

```typescript
import { Column, Model } from '@/decorator';

@Model('module/field')
export class Field {
  @Column('string', { name: '名称' })
  name: string;
}

@Model('module/field-tcloud')
export class FieldTcloud extends Field {
  @Column('string', { name: '腾讯云地域' })
  region: string;
}
```

模型命名应体现用途，例如 `FormFieldTcloud`、`SearchConditionTcloud`、`TableColumnTcloud`、`DetailsFieldTcloud`；对应 Factory 也按用途命名，避免一个 Factory 混用不同消费协议。

## 单类型简化

```typescript
import { getModel } from '@/model/manager';
import { FormFields } from './fields';

const model = getModel(FormFields);
```

单类型场景：

1. 不创建 `field-factory.ts`。
2. 不创建只有一个实现的 `field-<type>.ts` 层级。
3. 直接在用途明确的字段文件中定义 `@Model` 类。
4. 后续出现真实类型差异时，再迁移到 Factory。

## 决策速查

| 条件 | 选择 |
|------|------|
| 字段集合或元数据随 vendor 变化 | Factory，参数使用 `VendorEnum` |
| 字段集合或元数据随资源变化 | Factory，参数使用 `ResourceTypeEnum` |
| 各类型公共字段多 | 基类 + 类型子类 |
| 字段完全一致 | 单类 + `getModel` |
| form/list/detail 用途协议不同 | 按用途拆模型与 Factory |
