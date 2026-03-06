import { ref } from 'vue';
import http from '@/http';
import { CreateRecallTaskModal } from '@/typings/scr';
import { defineStore } from 'pinia';

import type { IPageQuery, IQueryResData } from '@/typings/common';
import type {
  IRecycleArea,
  IQueryDissolveList,
  IDissolveList,
  IDissolveHostCurrentListResult,
  IDissolveHostCurrentListParam,
  IDissolveHostOriginListResult,
  IDissolveHostOriginListParam,
  IDissolveRecycledModuleListParam,
  IApplyCrpTicketAudit,
  ITaskApplyRecordInitItem,
  IApplyOrderItem,
  ICloudInstanceConfigItem,
} from '@/typings/ziyanScr';
import { useWhereAmI } from '@/hooks/useWhereAmI';
import rollRequest from '@blueking/roll-request';
import { VendorEnum } from '@/common/constant';

const { BK_HCM_AJAX_URL_PREFIX } = window.PROJECT_CONFIG;

export const useZiyanScrStore = defineStore('ziyanScr', () => {
  const { getBusinessApiPath } = useWhereAmI();

  const listVpc = (region: any) => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/mov/cvm/manage/describevpcs`, { region });
  };
  const listSubnet = ({ region, zone, vpcId }: { region: string; zone: string; vpcId: string }) => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/mov/cvm/manage/describesubnets`, { region, zone, vpcId });
  };

  /**
   * @returns 资源上下架 - 获取单据状态list
   */
  const getTaskStatusList = () => {
    return http.get(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/pool/find/config/task/status`);
  };

  /**
   * @returns 机型配置信息列表
   */
  const getDeviceTypeList = () => {
    return http.get(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/pool/find/config/devicetype`);
  };

  /**
   * @returns IDC物理机操作系统列表
   */
  const getIdcpmOsTypeList = () => {
    return http.get(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/config/find/config/idcpm/ostype`);
  };

  /**
   * @returns IDC地域列表
   */
  const getIdcRegionList = () => {
    return http.get(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/config/find/config/idc/region`);
  };

  /**
   * @param data.cmdb_region_name 地域列表。若列表非空，则返回地域列表下的区域信息；若列表为空，则返回所有地域下的区域信息
   * @returns IDC可用区配置信息列表
   */
  const queryIdcZoneList = (data: { cmdb_region_name: string[] }) => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/config/findmany/config/idc/zone`, data);
  };

  const getRecycleAreas = (data: {
    page: IPageQuery;
    filter?: IDissolveRecycledModuleListParam;
  }): Promise<{ data: { details: IRecycleArea[] } }> => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/dissolve/recycled_module/list`, data);
  };

  const getDissolveList = (data: IQueryDissolveList): Promise<IDissolveList> => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/dissolve/table/list`, data);
  };

  /**
   * 资源申请单据执行接口
   * @returns {Promise}
   */
  const retryOrder = (data: any) => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/${getBusinessApiPath()}task/start/apply`, data);
  };

  /**
   * 资源申请单据终止接口
   * @returns {Promise}
   */
  const stopOrder = (data: any) => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/${getBusinessApiPath()}task/terminate/apply`, data);
  };

  /**
   * 资源从资源池下架
   */
  const createRecallTask = (data: CreateRecallTaskModal) => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/pool/create/recall/task`, data);
  };

  /**
   * 资源上架到资源池
   * @param data 要上架的CC主机ID，数量最大500
   */
  const createOnlineTask = (data: { bk_host_ids: number[] }) => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/pool/create/launch/task`, data);
  };

  const buildStatusFilter = (status: any) => ({
    op: 'and',
    rules: status || status === 0 ? [{ field: 'status', op: 'eq', value: status }] : [],
  });

  // 资源生产详情
  const getProductionDetails = (subOrderId: any, page: any, status: any) =>
    http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/${getBusinessApiPath()}task/find/apply/record/generate`, {
      suborder_id: subOrderId,
      page,
      filter: buildStatusFilter(status),
    });

  // 资源初始化详情
  const getInitializationDetails = (subOrderId: any, page: any, status: any) =>
    http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/${getBusinessApiPath()}task/find/apply/record/init`, {
      suborder_id: subOrderId,
      page,
      filter: buildStatusFilter(status),
    });

  // 资源初始化详情全部ip
  const getInitializationDetailsIps = async (subOrderId: string, status: number) => {
    const list = await rollRequest({
      httpClient: http,
      pageEnableCountKey: 'count',
    }).rollReq<ITaskApplyRecordInitItem>(
      `/api/v1/woa/${getBusinessApiPath()}task/find/apply/record/init`,
      {
        suborder_id: subOrderId,
        filter: buildStatusFilter(status),
      },
      { limit: 500, countGetter: (res) => res.data.count, listGetter: (res) => res.data.info },
    );
    return list.map((item) => item.ip).join('\n');
  };

  /** @deprecated 本地盘性能压测，已废弃不再使用 */
  const getDiskCheckDetails = (subOrderId: any, page: any, status: any) =>
    http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/task/find/apply/record/disk_check`, {
      suborder_id: subOrderId,
      page,
      filter:
        status || status === 0 ? { op: 'and', rules: [{ field: 'status', op: 'equal', value: status }] } : undefined,
    });

  // 资源交付详情
  const getDeliveryDetails = (subOrderId: any, page: any, status: any) =>
    http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/${getBusinessApiPath()}task/find/apply/record/deliver`, {
      suborder_id: subOrderId,
      page,
      filter: buildStatusFilter(status),
    });

  // 获取资源申请CRP单据审核信息：docs/api-docs/web-server/docs/biz/scr/get_ticket_crp_audit_by_suborderid.md
  const getApplyCrpTicketAudit = async (data: { crp_ticket_id: string; suborder_id: string }) => {
    const res: IQueryResData<IApplyCrpTicketAudit> = await http.post(
      `/api/v1/woa/${getBusinessApiPath()}task/apply/crp_ticket/audit/get`,
      data,
    );
    return res.data;
  };

  // 取消主机申请子订单的所有可取消CRP单据
  const cancelApplyCrpTicket = async (data: { suborder_id: string }) => {
    return http.post(`/api/v1/woa/${getBusinessApiPath()}task/apply/ticket/crp_audit/cancel`, data);
  };

  // 查询裁撤数据中当前主机信息。
  const dissolveHostCurrentList = (data: IDissolveHostCurrentListParam): Promise<IDissolveHostCurrentListResult> => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/dissolve/host/current/list`, data);
  };

  // 查询裁撤数据中原始主机信息。
  const dissolveHostOriginList = (data: IDissolveHostOriginListParam): Promise<IDissolveHostOriginListResult> => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/dissolve/host/origin/list`, data);
  };

  /**
   * @returns 主机申请-需求类型列表
   */
  const getRequirementList = () => {
    return http.get(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/config/find/config/requirement`);
  };

  /**
   * @returns 主机申请-单据状态列表
   */
  const getApplyStageList = () => {
    return http.get(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/config/find/config/apply/stage`);
  };

  // 资源可回收状态检查
  const getRecyclableHosts = (data: { ips?: string[]; asset_ids?: string[]; bk_host_ids?: number[] }) => {
    return http.post(
      `${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/${getBusinessApiPath()}task/findmany/recycle/recyclability`,
      data,
    );
  };

  // 回收单据预览
  const getPreRecycleList = (data: {
    ips: string[];
    remark?: string;
    return_plan: { cvm: string; pm: string };
    skip_confirm: boolean;
    recycle_type_sequence?: string[];
  }) => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/${getBusinessApiPath()}task/preview/recycle/order`, data);
  };

  // 获取回收单据中的主机
  const getRecycleOrderHost = (data: {
    suborder_id: string[];
    page: { limit?: number; start?: number; enable_count: boolean };
    bk_biz_id: number[];
  }) => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/${getBusinessApiPath()}task/findmany/recycle/host`, data);
  };

  // 资源回收单据执行接口
  const startRecycleOrder = (data: {
    suborder_id: string[];
    return_forecast_configs: {
      suborder_id: string;
      return_forecast: string;
      return_forecast_time: string;
    }[];
  }) => {
    return http.post(`${BK_HCM_AJAX_URL_PREFIX}/api/v1/woa/${getBusinessApiPath()}task/start/recycle/order`, data);
  };

  // 按指定项目类型进行资源回收
  const startRecycleOrderByRecycleType = async (data: {
    suborder_id_types: { suborder_id: string; recycle_type: string }[];
  }) => {
    return http.post('/api/v1/woa/task/start/recycle/order/by/recycle_type', data);
  };

  const applyOrdersLoading = ref(false);
  const getApplyOrderList = async (params: {
    bk_biz_id?: number[];
    suborder_id?: string[];
    page?: { start?: number; limit: number }; // 不传的时候，默认拉取全部
    get_product?: boolean; // 是否获取CVM生产数据，用于修改生产需求
  }) => {
    applyOrdersLoading.value = true;
    try {
      const res = await http.post(`/api/v1/woa/${getBusinessApiPath()}task/findmany/apply`, params);
      return (res.data?.info || []) as IApplyOrderItem[];
    } catch (error) {
      return Promise.reject(error);
    } finally {
      applyOrdersLoading.value = false;
    }
  };

  const modifyApplyOrderLoading = ref(false);
  const modifyApplyOrder = async (params: {
    suborder_id: string;
    bk_username: string;
    replicas: number;
    remark?: string;
    spec: IApplyOrderItem['spec'];
  }) => {
    modifyApplyOrderLoading.value = true;
    try {
      const res = await http.post(`/api/v1/woa/${getBusinessApiPath()}task/modify/apply`, params);
      return res;
    } catch (error) {
      return Promise.reject(error);
    } finally {
      modifyApplyOrderLoading.value = false;
    }
  };

  const queryCloudInstanceConfigLoading = ref(false);
  const queryCloudInstanceConfig = async (params: {
    account_id: string;
    region: string;
    filters?: {
      name: 'zone' | 'instance-family' | 'instance-type' | 'instance-charge-type' | 'sort-keys';
      values: string[];
    }[];
  }) => {
    queryCloudInstanceConfigLoading.value = true;
    try {
      const res = await http.post(
        `/api/v1/cloud/${getBusinessApiPath()}vendors/${VendorEnum.ZIYAN}/instances/config/query_from_cloud`,
        params,
      );
      return (res.data?.details ?? []) as ICloudInstanceConfigItem[];
    } catch (error) {
    } finally {
      queryCloudInstanceConfigLoading.value = false;
    }
  };

  return {
    listVpc,
    listSubnet,
    getTaskStatusList,
    getDeviceTypeList,
    getIdcpmOsTypeList,
    getIdcRegionList,
    queryIdcZoneList,
    getRecycleAreas,
    getDissolveList,
    retryOrder,
    stopOrder,
    createRecallTask,
    createOnlineTask,
    getProductionDetails,
    getInitializationDetails,
    getInitializationDetailsIps,
    getDiskCheckDetails,
    getDeliveryDetails,
    getApplyCrpTicketAudit,
    cancelApplyCrpTicket,
    dissolveHostCurrentList,
    dissolveHostOriginList,
    getRequirementList,
    getApplyStageList,
    getRecyclableHosts,
    getPreRecycleList,
    getRecycleOrderHost,
    startRecycleOrder,
    startRecycleOrderByRecycleType,
    applyOrdersLoading,
    getApplyOrderList,
    modifyApplyOrderLoading,
    modifyApplyOrder,
    queryCloudInstanceConfigLoading,
    queryCloudInstanceConfig,
  };
});
