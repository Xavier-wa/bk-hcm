import type { TicketByIdResult, IPlanTicket, IPlanTicketDemand } from '@/typings/resourcePlan';
import { AdjustType } from '@/typings/plan';

// 单个 demand 详情 → 表单结构
// 详情接口未返回的表单字段 (demand_source / demand_res_type / disk_num / disk_per_size / adjustType / demand_id)
// 用默认值兜底, 由用户在修改页交互时补全
function mapDemand(d: TicketByIdResult['demands'][number]): IPlanTicketDemand {
  const u = (d.updated_info || {}) as TicketByIdResult['demands'][number]['updated_info'];
  const uCvm = (u.cvm || {}) as TicketByIdResult['demands'][number]['updated_info']['cvm'];
  const uCbs = (u.cbs || {}) as TicketByIdResult['demands'][number]['updated_info']['cbs'];

  return {
    obs_project: u.obs_project || '',
    expect_time: u.expect_time || '',
    region_id: u.region_id || '',
    region_name: u.region_name || '',
    zone_id: u.zone_id || '',
    zone_name: u.zone_name || '',
    demand_source: '指标变化',
    demand_class: d.demand_class || 'CVM',
    remark: u.remark || '',
    demand_res_types: u.demand_res_types || ['CVM', 'CBS'],
    demand_res_type: '',
    cvm: {
      res_mode: uCvm.res_mode || '按机型',
      device_family: uCvm.device_family || '',
      device_class: uCvm.device_class || '',
      device_type: uCvm.device_type || '',
      os: uCvm.os || '',
      cpu_core: uCvm.cpu_core || 0,
      memory: uCvm.memory || 0,
    },
    cbs: {
      disk_type: uCbs.disk_type || '',
      disk_type_name: uCbs.disk_type_name || '',
      disk_io: uCbs.disk_io || 15,
      disk_size: uCbs.disk_size || 0,
      disk_num: 0,
      disk_per_size: 0,
    },
    adjustType: AdjustType.none,
    demand_id: '',
  };
}

// 单据详情 → 表单初始值
export function mapTicketDetailToPlanTicket(detail: TicketByIdResult, bizId: number): IPlanTicket {
  return {
    bk_biz_id: bizId,
    demand_class: detail.demands?.[0]?.demand_class || 'CVM',
    remark: detail.base_info?.remark || '',
    demands: (detail.demands || []).map(mapDemand),
  };
}
