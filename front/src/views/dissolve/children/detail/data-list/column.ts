import { Model, Column } from '@/decorator';
import { useBusinessGlobalStore } from '@/store/business-global';
import { useRegionsStore } from '@/store/useRegionsStore';
import { VendorEnum } from '@/common/constant';
import { UNKNOWN_BIZ_ID, UNKNOWN_BIZ_NAME } from '@/views/dissolve/common/unknown-biz';

const DISSOLVE_STATUS_MAP: Record<string, string> = {
  complete: '已裁撤',
  incomplete: '未裁撤',
};

// businessGlobalStore 的全量业务列表在 preload 时已获取（businessMapStore 在本模块未初始化，不可用）
const businessGlobalStore = useBusinessGlobalStore();
const regionsStore = useRegionsStore();

@Model('dissolve-detail/table-column')
export class TableColumn {
  @Column('string', { name: '裁撤截止时间', minWidth: 140, index: 0 })
  expect_abolish_time: string;

  @Column('string', { name: '内网IP', minWidth: 120, index: 1 })
  inner_ip: string;

  @Column('string', { name: '固资号', minWidth: 120, index: 2 })
  asset_id: string;

  @Column('user', { name: '负责人', minWidth: 100, index: 3 })
  operators: string[];

  @Column('business', {
    name: '业务名称',
    minWidth: 100,
    index: 4,
    meta: {
      display: {
        // bk_biz_id === 0 为"未知"业务；其他未匹配的 id 保持 '--'
        render: (value: number) => {
          if (value === UNKNOWN_BIZ_ID) return UNKNOWN_BIZ_NAME;
          return businessGlobalStore.businessFullList.find((item) => item.id === value)?.name || '--';
        },
      },
    },
    exportFormatter: (row: Record<string, any>) => {
      if (row.bk_biz_id === UNKNOWN_BIZ_ID) return UNKNOWN_BIZ_NAME;
      return (
        businessGlobalStore.businessFullList.find((item) => item.id === row.bk_biz_id)?.name || row.bk_biz_id || '--'
      );
    },
  })
  bk_biz_id: number;

  @Column('region', {
    name: '地域',
    minWidth: 100,
    index: 5,
    props: { vendor: VendorEnum.ZIYAN },
    exportFormatter: (row: Record<string, any>) => {
      return regionsStore.getRegionName(VendorEnum.ZIYAN, row.region) || row.region || '--';
    },
  })
  region: string;

  @Column('string', { name: '所属机房模块 ', minWidth: 120, index: 6 })
  module: string;

  @Column('string', { name: '机型', minWidth: 120, index: 7 })
  device_type: string;

  @Column('string', { name: '项目类型', minWidth: 120, index: 8 })
  project_name: string;

  @Column('enum', {
    name: '裁撤状态',
    option: DISSOLVE_STATUS_MAP,
    minWidth: 140,
    index: 9,
    meta: {
      display: {
        appearance: 'status',
      },
    },
    exportFormatter: (row: Record<string, any>) => {
      const val = row.status;
      return DISSOLVE_STATUS_MAP[val] ?? val ?? '--';
    },
  })
  status: string;
}
