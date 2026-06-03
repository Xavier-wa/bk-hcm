import { Model, Column } from '@/decorator';
import { toArray } from '@/common/util';

/**
 * 腾讯云搜索条件定义
 *
 * 使用 @Model() + @Column() 装饰器定义，通过 getModel() 转换为 ModelPropertySearch[]。
 * 每个 @Column 参数：
 *   第1个参数：搜索组件类型（string / enum / user / datetime / business 等）
 *   第2个参数：配置对象
 *     - name: 字段显示名
 *     - option: enum 类型的选项
 *     - format: 值格式化函数
 *     - meta.search.filterRules: 自定义搜索规则转换（复杂查询时使用）
 *     - index: 字段排序权重
 */
@Model('xxx/search-condition-tcloud')
export class SearchConditionTcloud {
  @Column('string', {
    name: 'ID',
    format: (value: string | string[]) => toArray(value).map((val) => String(val)),
  })
  ids: string[];

  @Column('string', {
    name: '名称',
    format: (value: string | string[]) => toArray(value).map((val) => String(val)),
  })
  names: string[];

  @Column('enum', {
    name: '状态',
    option: { active: '启用', disabled: '禁用' },
  })
  status: string;

  @Column('user', { name: '创建人' })
  creator: string;

  @Column('user', { name: '更新人' })
  reviser: string;
}
