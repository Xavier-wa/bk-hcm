# GridContainer / GridItem 布局模式

## 组件来源

```typescript
import GridContainer from '@/components/layout/grid-container/grid-container.vue';
import GridItem from '@/components/layout/grid-container/grid-item.vue';
```

## GridContainer Props

| Prop | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `layout` | `'horizontal' \| 'vertical'` | `'horizontal'` | 布局方向 |
| `column` | `number` | `4` | 列数 |
| `labelWidth` | `number \| string` | `160` | 标签宽度（horizontal 时生效） |
| `labelAlign` | `'left' \| 'center' \| 'right'` | `'right'` | 标签对齐 |
| `bordered` | `boolean` | `false` | 边框模式（表格态） |
| `fixed` | `boolean` | `false` | 固定列宽（根据内容） |
| `gap` | `(number\|string)[] \| number \| string` | — | 行列间距 |

## GridItem Props

| Prop | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `label` | `string \| (() => string \| VNode)` | — | 标签文本或渲染函数 |
| `span` | `number` | `1` | 占用的列数 |

## 详情页典型用法

详情页通常采用**单列 horizontal 布局**：

```vue
<template>
  <div v-for="(fields, group) in properties" :key="group" class="details-panel">
    <div class="panel-title">{{ group }}</div>
    <grid-container :column="1" :label-width="120">
      <grid-item v-for="field in fields" :key="field.id" :label="field.name">
        <display-value :property="field" :value="data[field.id]" />
      </grid-item>
    </grid-container>
  </div>
</template>
```

## 样式规范

每个分组面板统一使用以下样式：

```scss
.details-panel {
  background: #fff;
  border-radius: 2px;
  box-shadow: 0 2px 4px 0 #1919290d;
  padding: 16px 24px;

  .panel-title {
    font-size: 14px;
    font-weight: 700;
    color: #313238;
    line-height: 22px;
    margin-bottom: 8px;
  }
}
```

面板之间保持 `gap: 12px`。
