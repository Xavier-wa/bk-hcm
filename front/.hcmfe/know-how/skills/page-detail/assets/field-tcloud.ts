import { h } from 'vue';
import { Tag } from 'bkui-vue';
import { Model, Column } from '@/decorator';

@Model()
export class DetailsFieldTcloud {
  @Column('string', {
    name: '名称',
    group: '基本信息',
  })
  name: string;

  @Column('string', {
    name: '类型',
    group: '基本信息',
    meta: {
      display: {
        render: (data: any) => {
          // render 依赖整个 data 对象时，模板中需传 data 作为 value
          const label = String(data.type);
          const theme = 'success';
          return h(Tag, { radius: '4px', theme }, label);
        },
      },
    },
  })
  'extension.type': string;

  @Column('enum', {
    name: '状态',
    group: '基本信息',
    meta: {
      display: {
        appearance: 'status',
        appearanceProps: {
          statusMap: { 0: '未启用', 1: '启用中', 2: '已停用' },
          themeMap: { 0: 'default', 1: 'success', 2: 'danger' },
        },
      },
    },
  })
  status: number;

  @Column('string', {
    name: '所属账号',
    group: '基本信息',
  })
  account_id: string;

  @Column('number', {
    name: '配额',
    group: '基本信息',
  })
  quota: number;

  @Column('bool', {
    name: '是否启用',
    group: '基本信息',
  })
  enabled: boolean;

  @Column('user', {
    name: '创建人',
    group: '基本信息',
  })
  creator: string;

  @Column('datetime', {
    name: '创建时间',
    group: '基本信息',
  })
  created_at: string;

  @Column('user', {
    name: '更新人',
    group: '基本信息',
  })
  reviser: string;

  @Column('datetime', {
    name: '更新时间',
    group: '基本信息',
  })
  updated_at: string;

  @Column('string', {
    name: '描述',
    group: '基本信息',
  })
  memo: string;

  @Column('list', {
    name: '标签',
    group: '基本信息',
  })
  tags: string[];

  @Column('region', {
    name: '地域',
    group: '基本信息',
  })
  region: string;

  @Column('json', {
    name: '扩展配置',
    group: '高级信息',
  })
  policy_document: string;
}
