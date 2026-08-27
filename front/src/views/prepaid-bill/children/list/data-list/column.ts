import { Model, Column } from '@/decorator';
import { BILL_VENDORS_MAP } from '@/views/bill/account/account-manage/constants';
import {
  ACCOUNTING_STATES,
  ACCOUNTING_STATE_MAP,
  SETTLE_STATES,
  SETTLE_STATE_MAP,
} from '@/views/prepaid-bill/constants';
import { formatAmount, formatOrderMonthFromAt } from '@/views/prepaid-bill/utils';
import type { IPrepaidBillItem } from '@/views/prepaid-bill/typings';

const enumFilter = (option: Record<string, string>) => ({
  list: Object.entries(option).map(([value, label]) => ({ value, label, text: label })),
});

@Model('bill-prepaid/table-column')
export class TableColumn {
  @Column('string', {
    name: '预付费 ID',
    index: 1,
    minWidth: 140,
    fixed: 'left',
    meta: {
      display: { appearance: 'link' },
    },
  })
  id: string;

  @Column('string', {
    name: '订单月份',
    index: 2,
    minWidth: 110,
    fixed: 'left',
    meta: {
      display: {
        render: (data: IPrepaidBillItem) => formatOrderMonthFromAt(data.order_at),
      },
    },
  })
  order_month: number;

  @Column('string', { name: '资源 ID', index: 3, minWidth: 140 })
  resource_id: string;

  @Column('string', { name: '运营产品', index: 4, minWidth: 140 })
  product_id: number;

  @Column('string', { name: '二级账号名称', index: 5, minWidth: 160 })
  main_account_id: string;

  @Column('string', { name: '一级账号名称', index: 6, minWidth: 160 })
  root_account_id: string;

  @Column('string', { name: 'GPU 型号', index: 7, minWidth: 120 })
  gpu_type: string;

  @Column('number', { name: '数量(台)', index: 8, sort: true, align: 'right', minWidth: 100 })
  device_num: number;

  @Column('number', { name: '数量(卡)', index: 9, sort: true, align: 'right', minWidth: 100 })
  card_num: number;

  @Column('string', {
    name: '优惠后总价(不含税)',
    index: 10,
    sort: true,
    align: 'right',
    minWidth: 160,
    meta: {
      display: { format: formatAmount },
    },
  })
  cost: string;

  @Column('string', {
    name: '累计核算金额',
    index: 11,
    sort: true,
    align: 'right',
    minWidth: 140,
    meta: {
      display: { format: formatAmount },
    },
  })
  accounted_cost: string;

  @Column('enum', {
    name: '核算状态',
    index: 12,
    option: ACCOUNTING_STATE_MAP,
    minWidth: 120,
    filter: enumFilter(ACCOUNTING_STATE_MAP),
    meta: {
      display: {
        appearance: 'dynamic-status',
        appearanceProps: {
          statusObject: {
            success: [ACCOUNTING_STATES.ACCOUNTED],
            fail: [],
            wait: [],
            ing: [ACCOUNTING_STATES.ACCOUNTING],
            stop: [ACCOUNTING_STATES.PENDING],
          },
        },
      },
    },
  })
  accounting_state: string;

  @Column('enum', {
    name: '币种',
    index: 13,
    sort: true,
    minWidth: 90,
    option: { RMB: 'RMB', USD: 'USD' },
  })
  currency: string;

  @Column('string', { name: '产品名称', index: 14, minWidth: 140 })
  product_name: string;

  @Column('string', { name: '产品规格', index: 15, minWidth: 140 })
  product_spec: string;

  @Column('string', { name: 'Region', index: 16, minWidth: 120 })
  region: string;

  @Column('datetime', { name: '使用开始时间', index: 17, minWidth: 170 })
  usage_start_at: string;

  @Column('datetime', { name: '使用结束时间', index: 18, sort: true, minWidth: 170 })
  usage_end_at: string;

  @Column('datetime', { name: '订单时间', index: 19, sort: true, minWidth: 170 })
  order_at: string;

  @Column('string', { name: '发票 ID', index: 20, minWidth: 140 })
  invoice_id: string;

  @Column('enum', {
    name: '云厂商',
    index: 21,
    minWidth: 110,
    fixed: 'right',
    option: BILL_VENDORS_MAP,
  })
  vendor: string;

  @Column('enum', {
    name: '单据状态',
    index: 22,
    option: SETTLE_STATE_MAP,
    minWidth: 160,
    fixed: 'right',
    filter: enumFilter(SETTLE_STATE_MAP),
    meta: {
      display: {
        appearance: 'dynamic-tag-status',
        appearanceProps: {
          themeObject: {
            success: [SETTLE_STATES.SETTLED],
            default: [SETTLE_STATES.UNSETTLED],
          },
        },
      },
    },
  })
  settle_state: string;
}
