import type { TicketByIdResult, TicketDemands, IPlanTicket, IPlanTicketDemand } from '@/typings/resourcePlan';
import { AdjustType } from '@/typings/plan';

// 从接口 disk_size + os 反推表单「云磁盘容量/实例」
function deriveDiskPerSize(diskSize: number, os: string | number | undefined): number {
  const osNum = Number(os) || 0;
  if (diskSize > 0 && osNum > 0) {
    return Math.floor(diskSize / osNum);
  }
  return 0;
}

// CBS 单资源时用 disk_num；CVM 场景由 cvm.os 参与计算，此处仅兜底
function deriveDiskNum(diskSize: number, diskPerSize: number, os: string | number | undefined): number {
  const osNum = Number(os) || 0;
  if (osNum > 0) {
    return 0;
  }
  if (diskPerSize > 0 && diskSize > 0) {
    return Math.floor(diskSize / diskPerSize);
  }
  return diskSize > 0 ? 1 : 0;
}

/** updated_info 未带 demand_res_types 时，按 cvm/cbs 是否有量推断 */
function deriveDemandResTypes(u: TicketDemands): string[] {
  if (u.demand_res_types?.length) {
    return u.demand_res_types;
  }
  const types: string[] = [];
  const { cvm } = u;
  const { cbs } = u;
  const hasCvm = !!cvm && !!(cvm.device_type || Number(cvm.os) > 0 || (cvm.cpu_core ?? 0) > 0 || (cvm.memory ?? 0) > 0);
  const hasCbs = !!cbs && !!((cbs.disk_size ?? 0) > 0 || cbs.disk_type);
  if (hasCvm) {
    types.push('CVM');
  }
  if (hasCbs) {
    types.push('CBS');
  }
  return types.length ? types : ['CVM', 'CBS'];
}

function deriveDemandResType(demandResTypes: string[]): string {
  if (demandResTypes.length === 1) {
    return demandResTypes[0];
  }
  return 'CVM';
}

// 单个 demand 详情 → 表单结构
function mapDemand(d: TicketByIdResult['demands'][number]): IPlanTicketDemand {
  const u = d.updated_info ?? ({} as TicketDemands);
  const uCvm = u.cvm ?? {};
  const uCbs = u.cbs ?? {};
  const demandResTypes = deriveDemandResTypes(u);
  const diskSize = uCbs.disk_size ?? 0;
  const { os } = uCvm;
  const diskPerSize = deriveDiskPerSize(diskSize, os);

  return {
    obs_project: u.obs_project || '',
    expect_time: u.expect_time || '',
    return_plan_time: u.return_plan_time || '',
    region_id: u.region_id || '',
    region_name: u.region_name || '',
    zone_id: u.zone_id || '',
    zone_name: u.zone_name || '',
    demand_source: u.demand_source || '指标变化',
    demand_class: d.demand_class || 'CVM',
    remark: u.remark || '',
    demand_res_types: demandResTypes,
    demand_res_type: deriveDemandResType(demandResTypes),
    cvm: {
      res_mode: uCvm.res_mode || '按机型',
      device_family: uCvm.device_family || '',
      device_class: uCvm.device_class || '',
      device_type: uCvm.device_type || '',
      os: Number(os) || 0,
      cpu_core: uCvm.cpu_core ?? 0,
      memory: uCvm.memory ?? 0,
    },
    cbs: {
      disk_type: uCbs.disk_type || '',
      disk_type_name: uCbs.disk_type_name || '',
      disk_io: uCbs.disk_io ?? 15,
      disk_size: diskSize,
      disk_num: deriveDiskNum(diskSize, diskPerSize, os),
      disk_per_size: diskPerSize,
    },
    adjustType: AdjustType.none,
    demand_id: '',
  };
}

// 单据详情 → 表单初始值
export function mapTicketDetailToPlanTicket(detail: TicketByIdResult, bizId: number): IPlanTicket {
  return {
    bk_biz_id: bizId,
    demand_class: detail.demands?.[0]?.demand_class || detail.base_info?.demand_class || 'CVM',
    remark: detail.base_info?.remark || '',
    demands: (detail.demands || []).map(mapDemand),
  };
}
