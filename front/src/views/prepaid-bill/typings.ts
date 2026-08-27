export interface ISearchCondition {
  [key: string]: any;
}

export interface IPrepaidBillItem {
  id: string;
  uuid?: string;
  order_year: number;
  order_month: number;
  vendor: string;
  root_account_id: string;
  main_account_id: string;
  root_account_cloud_id?: string;
  main_account_cloud_id?: string;
  product_id: number;
  resource_id?: string;
  invoice_id?: string;
  gpu_type?: string;
  device_num?: number;
  card_num?: number;
  product_name?: string;
  product_spec?: string;
  region?: string;
  usage_start_at?: string;
  usage_end_at?: string;
  order_at?: string;
  currency?: string;
  cost?: string;
  rmb_cost?: string;
  settle_state?: string;
  accounting_state?: string;
  accounted_cost?: string;
  accounted_rmb_cost?: string;
  creator?: string;
  reviser?: string;
  created_at?: string;
  updated_at?: string;
}

export interface IMainAccountItem {
  id: string;
  name?: string;
  cloud_id?: string;
  vendor?: string;
  parent_account_id?: string;
  parent_account_name?: string;
}

export interface IRootAccountOption {
  id: string;
  name: string;
}

export interface IOperationProductItem {
  op_product_id: number;
  op_product_name: string;
}

export interface IPrepaidSplitItem {
  adjustment_id: string;
  bill_year: number;
  bill_month: number;
  accounted: boolean;
  type: string;
  cost: string;
  rmb_cost?: string;
  currency?: string;
  res_class?: string;
  res_sub_class?: string;
  push_status?: string;
  settle_state?: string;
  memo?: string | null;
  bill_period?: string;
}
