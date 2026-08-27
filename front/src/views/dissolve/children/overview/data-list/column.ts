import { Model, Column } from '@/decorator';
import { useBusinessGlobalStore } from '@/store/business-global';
import { UNKNOWN_BIZ_ID, UNKNOWN_BIZ_NAME } from '@/views/dissolve/common/unknown-biz';

const businessGlobalStore = useBusinessGlobalStore();

@Model('dissolve-overview/table-column')
export class TableColumn {
  @Column('business', {
    name: '业务名称',
    fixed: 'left',
    minWidth: 120,
    index: 0,
    meta: {
      display: {
        // bk_biz_id === 0 为"未知"业务；其他未匹配的 id 保持 '--'
        render: (value: number) => {
          if (value === UNKNOWN_BIZ_ID) return UNKNOWN_BIZ_NAME;
          return businessGlobalStore.businessFullList.find((item) => item.id === value)?.name || '--';
        },
      },
    },
  })
  bk_biz_id: number;

  @Column('string', { name: '裁撤进度', fixed: 'left', minWidth: 120, sort: true, index: 1 })
  progress: string;

  @Column('number', { name: '裁撤设备数', group: '待裁撤', minWidth: 120, sort: true, index: 2 })
  current_host_count: number;

  @Column('number', { name: '裁撤CPU总核数', group: '待裁撤', minWidth: 140, sort: true, index: 3 })
  current_cpu_core: number;

  @Column('number', {
    name: '已申领CPU总核数',
    minWidth: 160,
    sort: true,
    index: 4,
  })
  delivered_cpu_core: number;

  @Column('number', { name: '裁撤设备数', group: '原计划', minWidth: 120, sort: true, index: 5 })
  origin_host_count: number;

  @Column('number', { name: '裁撤CPU总核数', group: '原计划', minWidth: 220, sort: true, index: 6 })
  origin_cpu_core: number;
}
