import { Model, Column } from '@/decorator';
import {
  SPLIT_ADJUST_TYPES,
  SPLIT_ADJUST_TYPE_MAP,
  SPLIT_ROW_ACCOUNTING,
  SPLIT_ROW_ACCOUNTING_MAP,
} from '@/views/prepaid-bill/constants';
import { formatAmount } from '@/views/prepaid-bill/utils';

@Model('bill-prepaid/split-column')
export class SplitColumn {
  @Column('string', { name: '调账编号', minWidth: 160 })
  adjustment_id: string;

  @Column('string', { name: '账期', sort: true, minWidth: 120 })
  bill_period: string;

  @Column('enum', {
    name: '核算状态',
    option: SPLIT_ROW_ACCOUNTING_MAP,
    minWidth: 120,
    meta: {
      display: {
        appearance: 'dynamic-status',
        appearanceProps: {
          statusObject: {
            success: [SPLIT_ROW_ACCOUNTING.YES],
            fail: [],
            wait: [],
            ing: [],
            stop: [SPLIT_ROW_ACCOUNTING.NO],
          },
        },
      },
    },
  })
  accounted: string;

  @Column('enum', {
    name: '调账类型',
    option: SPLIT_ADJUST_TYPE_MAP,
    minWidth: 120,
    meta: {
      display: {
        appearance: 'dynamic-tag-status',
        appearanceProps: {
          themeObject: {
            success: [SPLIT_ADJUST_TYPES.INCREASE],
            danger: [SPLIT_ADJUST_TYPES.DECREASE],
          },
        },
      },
    },
  })
  type: string;

  @Column('string', {
    name: '调账金额',
    sort: true,
    align: 'right',
    minWidth: 140,
    meta: {
      display: { format: formatAmount },
    },
  })
  cost: string;

  @Column('string', { name: '备注', minWidth: 240 })
  memo: string;
}
