import { Model, Column } from '@/decorator';

@Model('dissolve-overview/table-column')
export class TableColumn {
  @Column('business', { name: '业务名称', fixed: 'left', minWidth: 120, index: 0 })
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
