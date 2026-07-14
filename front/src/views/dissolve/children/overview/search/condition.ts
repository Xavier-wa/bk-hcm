import { Model, Column } from '@/decorator';
import { QueryRuleOPEnum } from '@/typings';
import { VendorEnum } from '@/common/constant';
import { useDissolveQuotaStore } from '@/store/dissolve/quota';

@Model('dissolve-overview/search-condition')
export class SearchCondition {
  @Column('enum', {
    name: '裁撤截止时间',
    index: 0,
    option: () => useDissolveQuotaStore().getExpectAbolishTimeOptions(),
    props: {
      showAll: true,
      allOptionId: ['all'],
    },
    meta: {
      search: {
        op: QueryRuleOPEnum.IN,
      },
    },
  })
  expect_abolish_times: string[];

  @Column('number', {
    name: '项目类型',
    index: 1,
    meta: {
      search: {
        op: QueryRuleOPEnum.IN,
        // 保留 'all' 标记，不强制转数字（由父组件在 API 调用前展开为真实 ID）
        format: (value: any) => {
          const arr = Array.isArray(value) ? value : [value];
          return arr.map((v) => (v === 'all' ? v : Number(v)));
        },
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
