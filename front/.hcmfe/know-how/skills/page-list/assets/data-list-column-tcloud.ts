import { h } from 'vue';
import { Tag } from 'bkui-vue';
import { Model, Column } from '@/decorator';

/**
 * 腾讯云表格列定义
 *
 * 使用 @Model() + @Column() 装饰器定义，通过 getModel() 转换为 ModelPropertyColumn[]。
 * 每个 @Column 参数：
 *   第1个参数：列数据类型（string / number / datetime 等）
 *   第2个参数：配置对象
 *     - name: 列头显示名
 *     - sort: 是否支持排序
 *     - render: 自定义渲染函数（接收 { row } 参数，返回 VNode）
 */
@Model('xxx/table-column-tcloud')
export class TableColumnTcloud {
  @Column('string', { name: '名称' })
  name: string;

  @Column('string', {
    name: '类型',
    render: ({ row }: { row: any }) => {
      // TODO: 替换为实际的状态映射逻辑
      const label = row.type === 1 ? '类型A' : '类型B';
      const theme = row.type === 1 ? 'success' : 'warning';
      return h(Tag, { radius: '4px', theme }, label);
    },
  })
  type: number;

  @Column('string', { name: '数量', sort: true })
  count: number;

  @Column('user', { name: '创建人' })
  creator: string;

  @Column('datetime', { name: '创建时间', sort: true })
  created_at: string;

  @Column('user', { name: '更新人' })
  reviser: string;

  @Column('datetime', { name: '更新时间', sort: true })
  updated_at: string;
}
