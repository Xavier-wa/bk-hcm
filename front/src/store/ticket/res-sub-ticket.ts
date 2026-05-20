import http from '@/http';
import { IPageQuery, IQueryResData } from '@/typings';
import { IPlanTicketAudit, IPlanTicketCrpAudit } from '@/typings/resourcePlan';
import { getEntirePath } from '@/utils';
import { resolveBizApiPath } from '@/utils/search';
import { defineStore } from 'pinia';

export const useResSubTicketStore = defineStore('resSubTicketStore', () => {
  const getList = (params: SubTicketParam, bizId?: number): Promise<SubTicketsResult> => {
    return http.post(`/api/v1/woa/${resolveBizApiPath(bizId)}plans/resources/sub_tickets/list`, params);
  };
  const getAudit = (subTicketId: string, bizId?: number): Promise<SubTicketAuditResult> => {
    return http.get(`/api/v1/woa/${resolveBizApiPath(bizId)}plans/resources/sub_tickets/${subTicketId}/audit`);
  };
  const getDetail = (subTicketId: string, bizId?: number): Promise<SubTicketDetailResult> => {
    return http.get(`/api/v1/woa/${resolveBizApiPath(bizId)}plans/resources/sub_tickets/${subTicketId}`);
  };
  const retryTickets = (ticketId: string, bizId?: number): Promise<ActionResult> => {
    return http.post(`/api/v1/woa/${resolveBizApiPath(bizId)}plans/resources/tickets/${ticketId}/retry`);
  };
  const approveAdminNode = (
    subTicketId: string,
    params: ApproveAdminNodeParams,
    bizId?: number,
  ): Promise<ActionResult> => {
    return http.post(
      `/api/v1/woa/${resolveBizApiPath(bizId)}plans/resources/sub_tickets/${subTicketId}/approve_admin_node`,
      params,
    );
  };

  // 批量部门审批
  const batchApproveAdminNode = (params: BatchApproveAdminNodeParams, bizId?: number): Promise<ActionResult> => {
    return http.post(
      `/api/v1/woa/${resolveBizApiPath(bizId)}plans/resources/sub_tickets/approve_admin_node/batch`,
      params,
    );
  };

  // 获取审批额度
  const getTransferQuotaConfigs = (): Promise<TransferQuotasConfigsResult> => {
    return http.get(getEntirePath(`plans/resources/transfer_quotas/configs`));
  };
  // 获取剩余额度
  const getTransferQuotaSummary = (params: TransferQuotasParams, bizId?: number): Promise<TransferQuotasResult> => {
    return http.post(`/api/v1/woa/${resolveBizApiPath(bizId)}plans/resources/transfer_quotas/summary`, params);
  };
  // 终止资源预测单据
  const terminateTicket = (ticketId: string, bizId?: number): Promise<ActionResult> => {
    return http.post(`/api/v1/woa/${resolveBizApiPath(bizId)}plans/resources/tickets/${ticketId}/terminate`);
  };

  return {
    getTransferQuotaSummary,
    getList,
    getAudit,
    getDetail,
    retryTickets,
    approveAdminNode,
    batchApproveAdminNode,
    getTransferQuotaConfigs,
    terminateTicket,
  };
});

// 子单列表
export interface SubTicketParam {
  ticket_id?: string;
  statuses?: string[];
  sub_ticket_types?: string[];
  page?: IPageQuery;
}

export const STATUS_ENUM: Record<string, string> = {
  init: '待审批',
  auditing: '审批中',
  rejected: '审批驳回',
  failed: '失败',
  done: '成功',
  invalid: '已失效',
};
export const STAGE_ENUM: Record<string, string> = {
  admin_audit: '部门审批',
  crp_audit: '公司审批',
};
export interface SubTicketItem {
  id: string;
  // 获取STATUS_ENUM的key
  status: keyof typeof STATUS_ENUM;
  status_name: string;
  stage: keyof typeof STAGE_ENUM;
  sub_ticket_type: string;
  ticket_type_name: string;
  crp_sn: string;
  crp_url: string;
  original_info: {
    cvm: {
      cpu_core: number | null;
      memory: number | null;
    };
  };
  updated_info: {
    cvm: {
      cpu_core: number;
      memory: number;
    };
  };
  submitted_at: string;
  created_at: string;
  updated_at: string;
  message: string;
}
export type SubTicketsResult = { details: SubTicketItem[]; data: { details: SubTicketItem[] } };

// 部门审核
export interface ApproveAdminNodeParams {
  approval: boolean;
  use_transfer_pool: boolean;
  operate_info?: string; // 审批意见，最多100字
}

// 批量部门审核
export interface BatchApproveAdminNodeParams {
  sub_ticket_ids: string[];
  approval: boolean;
  use_transfer_pool: boolean;
  operate_info?: string; // 审批意见，最多100字
}

// 审批流
export interface AdminAudit {
  status: string;
  status_name?: string;
  current_steps: {
    name: string;
    processors: string[];
    processors_auth: Record<string, boolean>;
  }[];
  logs: {
    name: string;
    operator: string;
    operate_at: string;
    message: string;
    operate_info?: string; // 审批意见
  }[];
}

export interface Log {
  operator: string;
  operate_at: string;
  message: string;
  name: string;
}

export interface SubTicketAudit extends IPlanTicketAudit {
  id: string;
  admin_audit: AdminAudit;
  crp_audit: IPlanTicketCrpAudit;
}

export type SubTicketAuditResult = IQueryResData<SubTicketAudit>;

// 子单详情类型
export interface CvmInfo {
  res_mode: string;
  device_type: string;
  device_class: string;
  device_family: string;
  core_type: string;
  os?: number;
  cpu_core: number;
  memory: number;
  technical_class?: string;
}

export interface CbsInfo {
  disk_type: string;
  disk_io: number;
  disk_size: number;
}

export interface TicketInfo {
  obs_project: string;
  expect_time: string;
  region_id: string;
  zone_id: string;
  demand_res_types: string[];
  cvm: CvmInfo;
  cbs: CbsInfo;
}

export interface SubTicketDemand {
  demand_class: string;
  original_info: TicketInfo;
  updated_info: TicketInfo;
}

export interface BaseInfo {
  type: string;
  type_name: string;
  bk_biz_id: number;
  op_product_id: number;
  plan_product_id: number;
  virtual_dept_id: number;
  submitted_at: string;
}

export interface StatusInfo {
  status: string;
  status_name: string;
  stage: string;
  admin_audit_status: string;
  crp_sn: string;
  crp_url: string;
  message: string;
}

export interface SubTicketDetail {
  id: string;
  base_info: BaseInfo;
  status_info: StatusInfo;
  demands: SubTicketDemand[];
}
export type SubTicketDetailResult = IQueryResData<SubTicketDetail>;

// 操作类请求统一回调（重试、审批）
export type ActionResult = IQueryResData<null>;

export interface TransferQuotasParams {
  bk_biz_id?: number[];
  year?: number;
  applied_type?: string[];
  sub_ticket_id?: string[];
  technical_class?: string[];
  obs_project: string[];
}

export interface TransferQuotas {
  used_quota: number;
  remain_quota: number;
}

export type TransferQuotasResult = IQueryResData<TransferQuotas>;

export interface TransferQuotasConfigs {
  quota: number;
  audit_quota: number;
}

export type TransferQuotasConfigsResult = IQueryResData<TransferQuotasConfigs>;
