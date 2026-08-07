import { Model, Column } from '@/decorator';
import { QueryRuleOPEnum } from '@/typings';
import { useDissolveQuotaStore } from '@/store/dissolve/quota';

const DISSOLVE_STATUS_MAP: Record<string, string> = {
  complete: '已裁撤',
  incomplete: '未裁撤',
};

@Model('dissolve-detail/search-condition')
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

  @Column('enum', {
    name: '裁撤状态',
    option: DISSOLVE_STATUS_MAP,
    index: 2,
    props: {
      multiple: false,
    },
    meta: {
      search: {
        op: QueryRuleOPEnum.EQ,
      },
    },
  })
  status: string;

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

  @Column('business', { name: '业务名称', index: 4, props: { scope: 'auth' } })
  bk_biz_ids: number[];

  @Column('user', { name: '负责人', index: 5 })
  operators: string[];

  @Column('string', {
    name: '内网IP',
    index: 6,
    meta: {
      search: {
        op: QueryRuleOPEnum.IN,
      },
    },
    props: {
      pasteFn: (value: string) =>
        value
          .split(/[\r\n,;\s]+/)
          .filter(Boolean)
          .map((tag: string) => ({ id: tag.trim(), name: tag.trim() })),
    },
  })
  inner_ips: string;

  @Column('string', {
    name: '固资号',
    index: 7,
    meta: {
      search: {
        op: QueryRuleOPEnum.IN,
      },
    },
    props: {
      pasteFn: (value: string) =>
        value
          .split(/[\r\n,;\s]+/)
          .filter(Boolean)
          .map((tag: string) => ({ id: tag.trim(), name: tag.trim() })),
    },
  })
  asset_ids: string;
}
