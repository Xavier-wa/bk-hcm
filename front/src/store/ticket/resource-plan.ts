import { ref } from 'vue';
import { defineStore } from 'pinia';
import http from '@/http';
import { IListResData, QueryParamsType } from '@/typings';
import { enableCount, resolveBizApiPath } from '@/utils/search';

export interface IResourcePlanTicketItem {
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
  ticket_type: 'add' | 'adjust' | 'delete' | 'budget_declare';
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
  audited_original_info: {
    cvm: {
      cpu_core: number;
      memory: number;
    };
  };
  audited_updated_info: {
    cvm: {
      cpu_core: number;
      memory: number;
    };
  };
  applicant: string;
  remark: string;
  submitted_at: string;
  completed_at: string;
  created_at: string;
  updated_at: string;
}

export interface IResourcePlanTicketStatusItem {
  status: string;
  status_name: string;
}
export interface IResourcePlanTicketTypeItem {
  ticket_type: string;
  ticket_type_name: string;
}

export const useResourcePlanTicketStore = defineStore('ticket/resource-plan', () => {
  const ticketListLoading = ref(false);
  const ticketStatusList = ref<IResourcePlanTicketStatusItem[]>();
  const ticketTypeList = ref<IResourcePlanTicketTypeItem[]>();

  const getTicketList = async (params: QueryParamsType, bizId?: number) => {
    ticketListLoading.value = true;
    const api = `/api/v1/woa/${resolveBizApiPath(bizId)}plans/resources/tickets/list`;
    try {
      const [listRes, countRes] = await Promise.all<
        [Promise<IListResData<IResourcePlanTicketItem[]>>, Promise<IListResData<IResourcePlanTicketItem[]>>]
      >([http.post(api, enableCount(params, false)), http.post(api, enableCount(params, true))]);
      const [{ details: list = [] }, { count = 0 }] = [listRes?.data ?? {}, countRes?.data ?? {}];
      return { list: list || [], count };
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    } finally {
      ticketListLoading.value = false;
    }
  };

  const getTicketStatusList = async () => {
    if (ticketStatusList.value) {
      return ticketStatusList.value;
    }
    try {
      const res: IListResData<IResourcePlanTicketStatusItem[]> = await http.get(
        '/api/v1/woa/plan/res_plan_ticket_status/list',
      );
      ticketStatusList.value = res.data.details ?? [];
      return ticketStatusList.value;
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    }
  };

  const getTicketTypeList = async () => {
    if (ticketTypeList.value) {
      return ticketTypeList.value;
    }
    try {
      const res: IListResData<IResourcePlanTicketTypeItem[]> = await http.post('/api/v1/woa/metas/ticket_types/list');
      ticketTypeList.value = res.data.details ?? [];
      return ticketTypeList.value;
    } catch (error) {
      console.error(error);
      return Promise.reject(error);
    }
  };

  return {
    ticketListLoading,
    getTicketList,
    getTicketStatusList,
    getTicketTypeList,
  };
});
