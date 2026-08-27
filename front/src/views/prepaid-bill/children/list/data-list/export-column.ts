import type { ExportColumn } from '@/utils/common';
import { usePrepaidBillStore } from '@/store/prepaid-bill';
import { BILL_VENDORS_MAP } from '@/views/bill/account/account-manage/constants';
import { ACCOUNTING_STATE_MAP, SETTLE_STATE_MAP } from '@/views/prepaid-bill/constants';
import { formatAmount, formatOrderMonthFromAt } from '@/views/prepaid-bill/utils';
import type { IPrepaidBillItem } from '@/views/prepaid-bill/typings';

const displayText = (value: unknown) => {
  if (value === null || value === undefined || value === '') return '--';
  return String(value);
};

const mapLabel = (map: Record<string, string>, value: unknown) => {
  if (value === null || value === undefined || value === '') return '--';
  return map[String(value)] || String(value);
};

export const createPrepaidBillExportColumns = (): ExportColumn[] => {
  const store = usePrepaidBillStore();

  const formatMainAccount = (row: IPrepaidBillItem) => {
    const id = row.main_account_id;
    if (!id) return '--';
    const item = store.mainCache.get(id);
    return item?.name || item?.cloud_id || id;
  };

  const formatRootAccount = (row: IPrepaidBillItem) => {
    const id = row.root_account_id;
    if (!id) return '--';
    return store.parentCache.get(id)?.name || id;
  };

  const formatProduct = (row: IPrepaidBillItem) => {
    const id = row.product_id;
    if (id === null || id === undefined) return '--';
    return store.productCache.get(Number(id))?.op_product_name || String(id);
  };

  return [
    { label: '预付费 ID', field: 'id', exportFormatter: (row) => displayText(row.id) },
    {
      label: '订单月份',
      field: 'order_month',
      exportFormatter: (row) => formatOrderMonthFromAt(row.order_at),
    },
    { label: '资源 ID', field: 'resource_id', exportFormatter: (row) => displayText(row.resource_id) },
    { label: '运营产品', field: 'product_id', exportFormatter: (row) => formatProduct(row as IPrepaidBillItem) },
    {
      label: '二级账号名称',
      field: 'main_account_id',
      exportFormatter: (row) => formatMainAccount(row as IPrepaidBillItem),
    },
    {
      label: '一级账号名称',
      field: 'root_account_id',
      exportFormatter: (row) => formatRootAccount(row as IPrepaidBillItem),
    },
    { label: 'GPU 型号', field: 'gpu_type', exportFormatter: (row) => displayText(row.gpu_type) },
    { label: '数量(台)', field: 'device_num', exportFormatter: (row) => displayText(row.device_num) },
    { label: '数量(卡)', field: 'card_num', exportFormatter: (row) => displayText(row.card_num) },
    {
      label: '优惠后总价(不含税)',
      field: 'cost',
      exportFormatter: (row) => formatAmount(row.cost),
    },
    {
      label: '累计核算金额',
      field: 'accounted_cost',
      exportFormatter: (row) => formatAmount(row.accounted_cost),
    },
    {
      label: '核算状态',
      field: 'accounting_state',
      exportFormatter: (row) => mapLabel(ACCOUNTING_STATE_MAP, row.accounting_state),
    },
    { label: '币种', field: 'currency', exportFormatter: (row) => displayText(row.currency) },
    { label: '产品名称', field: 'product_name', exportFormatter: (row) => displayText(row.product_name) },
    { label: '产品规格', field: 'product_spec', exportFormatter: (row) => displayText(row.product_spec) },
    { label: 'Region', field: 'region', exportFormatter: (row) => displayText(row.region) },
    { label: '使用开始时间', field: 'usage_start_at', exportFormatter: (row) => displayText(row.usage_start_at) },
    { label: '使用结束时间', field: 'usage_end_at', exportFormatter: (row) => displayText(row.usage_end_at) },
    { label: '订单时间', field: 'order_at', exportFormatter: (row) => displayText(row.order_at) },
    { label: '发票 ID', field: 'invoice_id', exportFormatter: (row) => displayText(row.invoice_id) },
    { label: '云厂商', field: 'vendor', exportFormatter: (row) => mapLabel(BILL_VENDORS_MAP, row.vendor) },
    {
      label: '单据状态',
      field: 'settle_state',
      exportFormatter: (row) => mapLabel(SETTLE_STATE_MAP, row.settle_state),
    },
  ];
};
