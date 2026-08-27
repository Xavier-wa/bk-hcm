export const ACCOUNTING_STATES = {
  PENDING: 'pending',
  ACCOUNTING: 'accounting',
  ACCOUNTED: 'accounted',
} as const;

export const ACCOUNTING_STATE_MAP = {
  [ACCOUNTING_STATES.PENDING]: '待核算',
  [ACCOUNTING_STATES.ACCOUNTING]: '核算中',
  [ACCOUNTING_STATES.ACCOUNTED]: '已完成',
};

export const SETTLE_STATES = {
  UNSETTLED: 'unsettled',
  SETTLED: 'settled',
} as const;

export const SETTLE_STATE_MAP = {
  [SETTLE_STATES.UNSETTLED]: '未定账',
  [SETTLE_STATES.SETTLED]: '已定账',
};

export const SPLIT_ADJUST_TYPES = {
  INCREASE: 'increase',
  DECREASE: 'decrease',
} as const;

export const SPLIT_ADJUST_TYPE_MAP = {
  [SPLIT_ADJUST_TYPES.INCREASE]: '增加',
  [SPLIT_ADJUST_TYPES.DECREASE]: '减少',
};

export const SPLIT_ROW_ACCOUNTING = {
  YES: 'yes',
  NO: 'no',
} as const;

export const SPLIT_ROW_ACCOUNTING_MAP = {
  [SPLIT_ROW_ACCOUNTING.YES]: '已核算',
  [SPLIT_ROW_ACCOUNTING.NO]: '未核算',
};
