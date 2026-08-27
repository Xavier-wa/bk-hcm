import dayjs from 'dayjs';
import { Model, Column } from '@/decorator';
import { toArray, toFilterDateTime } from '@/common/util';
import { QueryRuleOPEnum } from '@/typings';
import { buildFilterRulesWithSearchSelect } from '@/utils/search';
import { BILL_VENDORS_MAP } from '@/views/bill/account/account-manage/constants';
import { isInvalidDateRange } from '@/views/prepaid-bill/utils';

const formatUsageTimeRules = (value: unknown) => {
  if (!Array.isArray(value) || !value[0] || !value[1] || isInvalidDateRange(value)) {
    return null;
  }
  return {
    op: QueryRuleOPEnum.AND,
    rules: [
      {
        field: 'usage_end_at',
        op: QueryRuleOPEnum.GTE,
        value: toFilterDateTime(dayjs(value[0]).startOf('day')),
      },
      {
        field: 'usage_start_at',
        op: QueryRuleOPEnum.LTE,
        value: toFilterDateTime(dayjs(value[1]).endOf('day')),
      },
    ],
  };
};

const formatOrderMonthRules = (value: unknown) => {
  if (!Array.isArray(value) || (!value[0] && !value[1])) {
    return null;
  }
  const start = value[0] || value[1];
  const end = value[1] || value[0];
  if (isInvalidDateRange([start, end])) {
    return null;
  }
  return {
    op: QueryRuleOPEnum.AND,
    rules: [
      {
        field: 'order_at',
        op: QueryRuleOPEnum.GTE,
        value: toFilterDateTime(dayjs(start).startOf('month')),
      },
      {
        field: 'order_at',
        op: QueryRuleOPEnum.LTE,
        value: toFilterDateTime(dayjs(end).endOf('month')),
      },
    ],
  };
};

@Model('bill-prepaid/search-condition')
export class SearchCondition {
  @Column('datetime', {
    name: '使用时间',
    index: 0,
    props: {
      type: 'daterange',
      format: 'yyyy-MM-dd',
      placeholder: '请选择日期',
    },
    meta: {
      search: {
        filterRules: formatUsageTimeRules,
      },
    },
  })
  usage_time: string[];

  @Column('list', {
    name: '运营产品',
    index: 1,
    format: (value: string | number | (string | number)[]) => toArray(value).map((val) => Number(val)),
    props: {
      idKey: 'op_product_id',
      displayKey: 'op_product_name',
      multiple: true,
      clearable: true,
    },
  })
  product_id: number[];

  @Column('list', {
    name: '二级账号',
    index: 2,
    format: (value: string | string[]) => toArray(value).map((val) => String(val)),
    props: {
      idKey: 'id',
      displayKey: 'name',
      multiple: true,
      clearable: true,
    },
  })
  main_account_id: string[];

  @Column('list', {
    name: '一级账号',
    index: 3,
    format: (value: string | string[]) => toArray(value).map((val) => String(val)),
    props: {
      idKey: 'id',
      displayKey: 'name',
      multiple: true,
      clearable: true,
    },
  })
  root_account_id: string[];

  @Column('string', {
    name: 'GPU 型号',
    index: 4,
    meta: {
      search: {
        filterRules: (value: string | string[]) =>
          buildFilterRulesWithSearchSelect(value, 'gpu_type', QueryRuleOPEnum.CS),
      },
    },
  })
  gpu_type: string;

  @Column('string', {
    name: '产品名称',
    index: 5,
    meta: {
      search: {
        filterRules: (value: string | string[]) =>
          buildFilterRulesWithSearchSelect(value, 'product_name', QueryRuleOPEnum.CS),
      },
    },
  })
  product_name: string;

  @Column('enum', {
    name: '云厂商',
    index: 6,
    option: BILL_VENDORS_MAP,
  })
  vendor: string;

  @Column('datetime', {
    name: '订单月份',
    index: 7,
    props: {
      type: 'monthrange',
      format: 'yyyy-MM',
      placeholder: '请选择月份',
    },
    meta: {
      search: {
        filterRules: formatOrderMonthRules,
      },
    },
  })
  order_month_range: string[];
}
