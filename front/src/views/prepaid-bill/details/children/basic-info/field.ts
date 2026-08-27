import { Model, Column } from '@/decorator';
import { BILL_VENDORS_MAP } from '@/views/bill/account/account-manage/constants';
import { formatAmount, formatOrderMonthFromAt } from '@/views/prepaid-bill/utils';
import type { IPrepaidBillItem } from '@/views/prepaid-bill/typings';

@Model('bill-prepaid/details-field')
export class DetailsField {
  @Column('string', { name: '预付费 ID', group: '基本信息', index: 1 })
  id: string;

  @Column('string', {
    name: '订单月份',
    group: '基本信息',
    index: 2,
    meta: {
      display: {
        render: (data: IPrepaidBillItem) => formatOrderMonthFromAt(data.order_at),
      },
    },
  })
  order_month: number;

  @Column('string', { name: '资源 ID', group: '基本信息', index: 3 })
  resource_id: string;

  @Column('string', { name: '运营产品', group: '基本信息', index: 4 })
  product_id: number;

  @Column('string', { name: '一级账号名称', group: '基本信息', index: 5 })
  root_account_id: string;

  @Column('string', { name: '二级账号名称', group: '基本信息', index: 6 })
  main_account_id: string;

  @Column('string', { name: 'GPU 型号', group: '基本信息', index: 7 })
  gpu_type: string;

  @Column('number', { name: '数量(台)', group: '基本信息', index: 8 })
  device_num: number;

  @Column('number', { name: '数量(卡)', group: '基本信息', index: 9 })
  card_num: number;

  @Column('string', {
    name: '优惠后总价(不含税)',
    group: '基本信息',
    index: 10,
    meta: {
      display: {
        render: (data: IPrepaidBillItem) => {
          const amount = formatAmount(data.cost);
          if (amount === '--') return amount;
          return data.currency ? `${amount} ${data.currency}` : amount;
        },
      },
    },
  })
  cost: string;

  @Column('string', { name: '产品名称', group: '基本信息', index: 11 })
  product_name: string;

  @Column('string', { name: '产品规格', group: '基本信息', index: 12 })
  product_spec: string;

  @Column('string', { name: 'Region', group: '基本信息', index: 13 })
  region: string;

  @Column('datetime', { name: '订单时间', group: '基本信息', index: 14 })
  order_at: string;

  @Column('datetime', { name: '使用开始时间', group: '基本信息', index: 15 })
  usage_start_at: string;

  @Column('datetime', { name: '使用结束时间', group: '基本信息', index: 16 })
  usage_end_at: string;

  @Column('string', { name: '发票 ID', group: '基本信息', index: 17 })
  invoice_id: string;

  @Column('enum', {
    name: '云厂商',
    group: '基本信息',
    index: 18,
    option: BILL_VENDORS_MAP,
  })
  vendor: string;
}
