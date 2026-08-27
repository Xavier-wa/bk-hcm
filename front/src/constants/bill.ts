import { VendorEnum } from '@/common/constant';

// 账单下共用的业务key
export const BILL_BIZS_KEY = 'bill_bizs';
// 账单下共有的二级账号key
export const BILL_MAIN_ACCOUNTS_KEY = 'main_accounts';

// 账单类型 - 华为
export const BILL_TYPE__MAP_HW = {
  1: '消费-新购',
  2: '消费-续订',
  3: '消费-变更',
  4: '退款-退订',
  5: '消费-使用',
  8: '消费-自动续订',
  9: '调账-补偿',
  14: '消费-服务支持计划月末扣费',
  15: '消费-税金',
  16: '调账-扣费',
  17: '消费-保底差额',
  20: '退款-变更',
  23: '消费-节省计划抵扣',
  24: '退款-包年/包月转按需',
  100: '退款-退订税金',
  101: '调账-补偿税金',
  102: '调账-扣费税金',
};

// 调账状态
export const BILL_ADJUSTMENT_STATE__MAP = {
  unconfirmed: '未确认',
  confirmed: '已确认',
};

// 调账推送状态（list.push_status）
export const BILL_ADJUSTMENT_PUSH_STATUS_MAP = {
  unpushed: '未推送',
  pushing: '推送中',
  pushed: '已推送',
  failed: '失败',
};

export const BILL_ADJUSTMENT_PUSH_STATUS_THEME: Record<string, 'info' | 'success' | 'danger' | undefined> = {
  unpushed: undefined,
  pushing: 'info',
  pushed: 'success',
  failed: 'danger',
};

// 调账定账状态（list.settle_state）
export const BILL_ADJUSTMENT_SETTLE_STATE_MAP = {
  unsettled: '未定账',
  settled: '已定账',
};

export const BILL_ADJUSTMENT_SETTLE_STATE_THEME: Record<string, 'success' | undefined> = {
  unsettled: undefined,
  settled: 'success',
};

// 调账类型
export const BILL_ADJUSTMENT_TYPE__MAP = {
  increase: '增加',
  decrease: '减少',
};

// 资源类别
export enum ResClassEnum {
  Cpu = 'cpu',
  GpuCard = 'gpu_card',
  GpuApi = 'gpu_api',
  GpuOther = 'gpu_other',
}

export const ResClassList = [
  { label: 'CPU', value: ResClassEnum.Cpu },
  { label: 'GPU卡', value: ResClassEnum.GpuCard },
  { label: 'GPU API', value: ResClassEnum.GpuApi },
  { label: 'GPU其他', value: ResClassEnum.GpuOther },
];

export const RES_CLASS_MAP = {
  [ResClassEnum.Cpu]: 'CPU',
  [ResClassEnum.GpuCard]: 'GPU卡',
  [ResClassEnum.GpuApi]: 'GPU API',
  [ResClassEnum.GpuOther]: 'GPU其他',
};

export const BILL_ADJUSTMENT_SUPPORTED_VENDORS = [VendorEnum.AWS, VendorEnum.GCP, VendorEnum.HUAWEI];

// 币种
export const CURRENCY_ALIAS_MAP = {
  USD: 'USD',
  RMB: 'RMB',
  CNY: 'CNY',
};

// 币种
export const CURRENCY_MAP = {
  [CURRENCY_ALIAS_MAP.USD]: '美元',
  [CURRENCY_ALIAS_MAP.RMB]: '人民币',
  [CURRENCY_ALIAS_MAP.CNY]: '人民币',
};

// 币种符号
export const CURRENCY_SYMBOL_MAP = {
  [CURRENCY_ALIAS_MAP.USD]: '$',
  [CURRENCY_ALIAS_MAP.CNY]: '¥',
  [CURRENCY_ALIAS_MAP.RMB]: '¥',
};

// 一级账号账单汇总状态
export const BILLS_ROOT_ACCOUNT_SUMMARY_STATE_MAP = {
  accounting: '核算中',
  accounted: '已核算',
  confirmed: '已确认',
  syncing: '同步中',
  synced: '已同步',
  stopped: '停止中',
};

// 币种
export const BILLS_CURRENCY = [
  {
    id: 'USD',
    name: '美元',
  },
  {
    id: 'RMB',
    name: '人民币',
  },
];
