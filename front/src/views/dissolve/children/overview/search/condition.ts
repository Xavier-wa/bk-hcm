import { Model, Column } from '@/decorator';
import { QueryRuleOPEnum } from '@/typings';
import { VendorEnum } from '@/common/constant';

@Model('dissolve-overview/search-condition')
export class SearchCondition {
  @Column('string', {
    name: '裁撤时间段',
    index: 0,
    meta: {
      search: {
        op: QueryRuleOPEnum.IN,
      },
    },
  })
  time_periods: string[];

  @Column('number', {
    name: '项目类型',
    index: 1,
    meta: {
      search: {
        op: QueryRuleOPEnum.IN,
      },
    },
  })
  project_ids: number[];

  @Column('number', {
    name: '组织',
    index: 2,
    meta: {
      search: {
        op: QueryRuleOPEnum.IN,
      },
    },
  })
  group_ids: number[];

  @Column('business', { name: '业务名称', index: 3, props: { scope: 'auth' } })
  bk_biz_ids: number[];

  @Column('user', { name: '负责人', index: 4 })
  operators: string[];

  @Column('region', { name: '地域', index: 5, props: { vendor: VendorEnum.ZIYAN, multiple: true } })
  regions: string[];
}
