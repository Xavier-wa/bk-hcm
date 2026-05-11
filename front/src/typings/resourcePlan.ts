import { IPageQuery, IQueryResData, IListResData } from '@/typings';
import { AdjustType } from './plan';

export interface IBizResourcesTicketsParam {
  ticket_ids?: string[];
  statuses?: string[];
  ticket_types?: string[];
  applicants?: string[];
  submit_time_range?: {
    start?: string;
    end?: string;
  };
  page?: IPageQuery;
}

export interface IResourcesTicketItem {
  id: string;
  bk_biz_id: number;
  bk_biz_name: string;
  op_product_id: number;
  op_product_name: string;
  plan_product_id: number;
  plan_product_name: string;
  demand_class: string;
  status: string;
  status_name: string;
  ticket_type: string;
  ticket_type_name: string;
  original_info: {
    cvm: {
      cpu_core: number;
      memory: number;
    };
    cbs: {
      disk_size: number;
    };
  };
  updated_info: {
    cvm: {
      cpu_core: number;
      memory: number;
    };
    cbs: {
      disk_size: number;
    };
  };
  applicant: string;
  remark: string;
  submitted_at: string;
  completed_at: string;
  created_at: string;
  updated_at: string;
}

export type IBizResourcesTicketsResult = IListResData<IResourcesTicketItem>;

export interface IOpResourcesTicketsParam {
  bk_biz_ids?: number[];
  op_product_ids?: number[];
  plan_product_ids?: number[];
  ticket_ids?: string[];
  statuses?: string[];
  ticket_types?: string[];
  applicants?: string[];
  submit_time_range?: {
    start?: string;
    end?: string;
  };
  page?: IPageQuery;
}

export type IOpResourcesTicketsResult = IListResData<IResourcesTicketItem>;

export type ResourcePlanTicketByIdResult = IQueryResData<TicketByIdResult>;

export type TicketStatus =
  | 'init'
  | 'auditing'
  | 'rejected'
  | 'partial_rejected'
  | 'done'
  | 'failed'
  | 'partial_failed'
  | 'revoked'
  | 'terminated';

export interface TicketByIdResult {
  id: string;
  base_info: TicketBaseInfo;
  status_info: {
    status: TicketStatus;
    status_name: string;
    message: string;
    itsm_sn: string;
    itsm_url: string;
    crp_sn: string;
    crp_url: string;
  };
  demands: {
    original_info: TicketDemands;
    updated_info: TicketDemands;
    demand_class: string;
  }[];
}

export interface TicketDemands {
  obs_project: string;
  expect_time: string;
  region_id: string;
  zone_id: string;
  demand_res_types: string[];
  cvm: {
    res_mode: string;
    device_family: string;
    device_type: string;
    device_class: string;
    cpu_core: number;
    memory: number;
    res_pool: string;
    core_type: string;
  };
  cbs: {
    disk_type: string;
    disk_type_name: string;
    disk_io: number;
    disk_size: number;
  };
}

export interface TicketBaseInfo {
  type: string;
  type_name: string;
  applicant: string;
  bk_biz_id: number;
  bk_biz_name: string;
  op_product_id: number;
  op_product_name: string;
  plan_product_id: number;
  plan_product_name: string;
  virtual_dept_id: number;
  virtual_dept_name: string;
  remark: string;
  submitted_at: string;
}

export interface IPlanTicketAudit {
  ticket_id: string;
  itsm_audit: IPlanTicketItsmAudit;
  crp_audit: IPlanTicketCrpAudit;
}
export interface IPlanTicketItsmAudit {
  itsm_sn: string;
  itsm_url: string;
  status: string;
  status_name: string;
  message: string;
  current_steps: IPlanTicketAuditCurrentStep[];
  logs: IPlanTicketAuditLog[];
}
export interface IPlanTicketCrpAudit {
  crp_sn: string;
  crp_url: string;
  status: string;
  status_name: string;
  message: string;
  current_steps: IPlanTicketAuditCurrentStep[];
  logs: IPlanTicketAuditLog[];
}
export interface IPlanTicketAuditCurrentStep {
  state_id: number | string;
  name: string;
  processors: string[];
  processors_auth: {
    [key: string]: boolean;
  };
}
export interface IPlanTicketAuditLog {
  operator: string;
  operate_at: string;
  message: string;
  name?: string; // Optional because it's not present in every log
}
export type ResourcePlanTicketAuditResData = IQueryResData<IPlanTicketAudit>;

export interface IPlanTicket {
  bk_biz_id: number;
  demand_class: string;
  demands: IPlanTicketDemand[];
  remark: string;
}

export interface IPlanTicketDemand {
  obs_project: string;
  expect_time: string;
  region_id: string;
  region_name: string;
  zone_id: string;
  zone_name: string;
  demand_source: string;
  demand_class: string;
  remark?: string;
  demand_res_types: string[];
  demand_res_type: string;
  cvm?: {
    res_mode: string;
    device_class: string;
    device_type: string;
    os: string | number;
    cpu_core: number;
    memory: number;
    [key: string]: any;
  };
  cbs?: {
    disk_type: string;
    disk_type_name: string;
    disk_io: number;
    disk_size: number;
    disk_num: number;
    disk_per_size: number;
    [key: string]: any;
  };
  adjustType: AdjustType;
  demand_id: string;
  [key: string]: any;
}

export interface IBizOrgRelation {
  bk_biz_id: number;
  bk_biz_name: string;
  bk_product_id: number;
  bk_product_name: string;
  plan_product_id: number;
  plan_product_name: string;
  virtual_dept_id: number;
  virtual_dept_name: string;
}

export interface IRegion {
  region_id: string;
  region_name: string;
}

export interface IZone {
  zone_id: string;
  zone_name: string;
}

export interface IDeviceType {
  device_type: string;
  core_type: string;
  cpu_core: number;
  memory: number;
  device_class: string;
  device_family: string;
  gpu_amount: number;
}

export interface IDiskType {
  disk_type: string;
  disk_type_name: string;
}

export interface IPlanProducts {
  plan_product_id: number;
  plan_product_name: string;
}

interface StatusListResult {
  details: {
    status: 'init' | 'auditing' | 'rejected' | 'done' | 'failed';
    status_name: string;
  }[];
}

export type IResPlanTicketStatusListResult = IQueryResData<StatusListResult>;

export enum ResourcesDemandsStatus {
  CAN_APPLY = 'can_apply',
  NOT_READY = 'not_ready',
  EXPIRED = 'expired',
  SPENT_ALL = 'spent_all',
  LOCKED = 'locked',
}

export interface IListResourcesDemandsParam {
  bk_biz_ids?: number[];
  op_product_ids?: string[];
  plan_product_ids?: string[];
  demand_ids?: string[];
  obs_projects?: string[];
  demand_classes?: string[];
  device_classes?: string[];
  device_types?: string[];
  region_ids?: string[];
  zone_ids?: string[];
  plan_types?: string[];
  expiring_only?: boolean;
  expect_time_range?: {
    start: string;
    end: string;
  };
  statuses?: ResourcesDemandsStatus[];
  page: IPageQuery;
}

export interface IListResourcesDemandsResult {
  count?: number;
  overview: {
    total_cpu_core: number;
    total_applied_core: number;
    in_plan_cpu_core: number;
    in_plan_applied_cpu_core: number;
    out_plan_cpu_core: number;
    out_plan_applied_cpu_core: number;
    expiring_cpu_core: number;
  };
  details: {
    demand_id: string;
    bk_biz_id: number;
    bk_biz_name: string;
    op_product_id: number;
    op_product_name: string;
    status: ResourcesDemandsStatus;
    status_name: string;
    demand_class: string;
    available_year_month: string;
    expect_time: string;
    device_class: string;
    device_type: string;
    total_os: string;
    applied_os: string;
    remained_os: string;
    total_cpu_core: number;
    applied_cpu_core: number;
    remained_cpu_core: number;
    total_memory: number;
    applied_memory: number;
    remained_memory: number;
    total_disk_size: number;
    applied_disk_size: number;
    remained_disk_size: number;
    region_id: string;
    region_name: string;
    zone_id: string;
    zone_name: string;
    plan_type: string;
    obs_project: string;
    generation_type: string;
    device_family: string;
    core_type: string;
    disk_type: string;
    disk_type_name: string;
    disk_io: number;
  }[];
}

export type IListResourcesDemandsItem = IListResourcesDemandsResult['details'][number];

interface PlanDemandResult {
  demand_id: string;
  year_month_week: string;
  expect_start_date: string;
  expect_end_date: string;
  expect_time: string;
  bk_biz_id: number;
  bk_biz_name: string;
  bg_id: number;
  bg_name: string;
  dept_id: number;
  dept_name: string;
  plan_product_id: number;
  plan_product_name: string;
  op_product_id: number;
  op_product_name: string;
  obs_project: string;
  area_id: string;
  area_name: string;
  region_id: string;
  region_name: string;
  zone_id: string;
  zone_name: string;
  plan_type: string;
  plan_advance_week: number;
  expedited_postponed: string;
  core_type_id: number;
  core_type: string;
  device_family: string;
  device_class: string;
  device_type: string;
  os: number;
  memory: number;
  cpu_core: number;
  disk_size: number;
  disk_io: number;
  disk_type: string;
  disk_type_name: string;
  demand_week: string;
  res_pool_type: number;
  res_pool: string;
  res_mode: string;
  generation_type: string;
}

export type IPlanDemandResult = IQueryResData<PlanDemandResult>;

export interface IListChangeLogsParam {
  demand_id: string;
  page: IPageQuery;
}

export interface IListChangeLogsResult {
  details: {
    demand_id: string;
    expect_time: string;
    bg_name: string;
    dept_name: string;
    plan_product_name: string;
    op_product_name: string;
    obs_project: string;
    region_name: string;
    zone_name: string;
    demand_week: string;
    res_pool_type: number;
    res_pool: string;
    device_class: string;
    device_type: string;
    change_cvm_amount: number;
    after_cvm_amount: number;
    change_core_amount: number;
    after_core_amount: number;
    change_ram_amount: number;
    after_ram_amount: number;
    changed_disk_amount: number;
    after_disk_amount: number;
    disk_type: string;
    disk_io: number;
    demand_source: string;
    crp_sn: string;
    create_time: string;
    remark: string;
  }[];
}

interface TicketTypesResult {
  details: {
    ticket_type: string;
    ticket_type_name: string;
  }[];
}

export type ITicketTypesResult = IQueryResData<TicketTypesResult>;

interface OpProductsResult {
  details: {
    op_product_id: number;
    op_product_name: string;
  }[];
}

export type IOpProductsResult = IQueryResData<OpProductsResult>;

interface PlanProductsResult {
  details: {
    plan_product_id: number;
    plan_product_name: string;
  }[];
}

export type IPlanProductsResult = IQueryResData<PlanProductsResult>;

interface BizsByOpProductResult {
  details: Array<{
    bk_biz_id: number;
    bk_biz_name: string;
  }>;
}

export type IBizsByOpProductResult = IQueryResData<BizsByOpProductResult>;

export enum ResourceDemandResultStatusCode {
  Default,
  BGNone,
  BGHas,
  BIZNone,
  BIZHas,
}

export const ResourceDemandResultStatus = {
  [ResourceDemandResultStatusCode.Default]: '默认预测',
  [ResourceDemandResultStatusCode.BGNone]: 'BG无预测',
  [ResourceDemandResultStatusCode.BGHas]: 'BG有预测',
  [ResourceDemandResultStatusCode.BIZNone]: '本业务有预测',
  [ResourceDemandResultStatusCode.BIZHas]: '本业务无预测',
};
