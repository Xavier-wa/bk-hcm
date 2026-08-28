import type { IAdjustInfoPayload, IAdjustItemPayload } from '@/store/resource-plan/cvm-adjust';
import type { IAdjustBaseline, IAdjustRow } from './typings';
import { DEFAULT_DISK_IO, DEFAULT_RES_MODE, resolveResMode, SHORT_LEASE_OBS_PROJECT } from './typings';

const DEFAULT_DEMAND_SOURCE = '指标变化';

/** 数值型配置字段：InputColumn 可能写回字符串，需按数值比较 */
const NUMERIC_CONFIG_KEYS: (keyof IAdjustBaseline)[] = [
  'remained_os',
  'remained_cpu_core',
  'remained_memory',
  'disk_io',
  'disk_per_size',
  'remained_disk_size',
];

/** 字符串型配置字段（不含 demand_class：顶层单独传递，不参与行内配置 diff） */
const STRING_CONFIG_KEYS: (keyof IAdjustBaseline)[] = [
  'obs_project',
  'return_plan_time',
  'region_id',
  'zone_id',
  'demand_res_type',
  'res_mode',
  'device_class',
  'device_type',
  'disk_type',
];

/** 用于判断配置是否变更的字段（不含 expect_time） */
const CONFIG_KEYS: (keyof IAdjustBaseline)[] = [...STRING_CONFIG_KEYS, ...NUMERIC_CONFIG_KEYS];

function normalizeString(value: unknown): string {
  if (value === undefined || value === null) return '';
  return String(value);
}

function normalizeNumber(value: unknown): number {
  if (value === undefined || value === null || value === '') return 0;
  const num = Number(value);
  return Number.isNaN(num) ? 0 : num;
}

/** 按字段类型比较，避免 "10" 与 10 造成假 diff */
export function isSameFieldValue(field: keyof IAdjustBaseline, left: unknown, right: unknown): boolean {
  if (NUMERIC_CONFIG_KEYS.includes(field)) {
    return normalizeNumber(left) === normalizeNumber(right);
  }
  return normalizeString(left) === normalizeString(right);
}

export function isConfigEqual(row: IAdjustBaseline, baseline: IAdjustBaseline): boolean {
  return CONFIG_KEYS.every((key) => isSameFieldValue(key, row[key], baseline[key]));
}

export function isTimeChanged(row: IAdjustBaseline, baseline: IAdjustBaseline): boolean {
  return normalizeString(row.expect_time) !== normalizeString(baseline.expect_time);
}

export function isRowUnchanged(row: IAdjustRow): boolean {
  if (row.is_new) return false;
  if (!row.baseline) return true;
  return !isTimeChanged(row, row.baseline) && isConfigEqual(row, row.baseline);
}

export function isFieldChanged(row: IAdjustRow, field: keyof IAdjustBaseline): boolean {
  if (row.is_new || !row.baseline) return false;
  return !isSameFieldValue(field, row[field], row.baseline[field]);
}

export function isShortLeaseProject(obsProject: string): boolean {
  return obsProject === SHORT_LEASE_OBS_PROJECT;
}

/** 仅申报 CBS 的行：现网整个「CVM云主机信息」面板不渲染，表格里对应列显示 "-" */
export function isCbsOnlyRow(row: IAdjustBaseline): boolean {
  return row.demand_res_type === 'CBS';
}

/** 按行资源类型推导 API demand_res_types */
export function resolveDemandResTypes(row: IAdjustBaseline): ('CVM' | 'CBS')[] {
  if (isCbsOnlyRow(row)) {
    return ['CBS'];
  }
  // CVM 行始终同时申报 CVM 与 CBS，与现网调整映射保持一致（CBS 可为 0）
  return ['CVM', 'CBS'];
}

/** 收集需要做可用时间校验的日期：新增行与到货时间发生变更的行 */
export function collectExpectTimesToValidate(rows: IAdjustRow[]): string[] {
  const times = rows
    .filter((row) => {
      if (row.is_new) return Boolean(row.expect_time);
      if (!row.baseline) return false;
      return isTimeChanged(row, row.baseline);
    })
    .map((row) => row.expect_time)
    .filter(Boolean);
  return Array.from(new Set(times));
}

export function mapRowToAdjustInfo(row: IAdjustBaseline): IAdjustInfoPayload {
  const demandResTypes = resolveDemandResTypes(row);
  const os = Number(row.remained_os) || 0;
  const cpu = Number(row.remained_cpu_core) || 0;
  const memory = Number(row.remained_memory) || 0;
  const diskSize = Number(row.remained_disk_size) || 0;

  const info: IAdjustInfoPayload = {
    obs_project: row.obs_project,
    expect_time: row.expect_time,
    region_id: row.region_id,
    zone_id: row.zone_id || undefined,
    demand_source: row.demand_source || DEFAULT_DEMAND_SOURCE,
    remark: '',
    demand_res_types: demandResTypes,
  };

  if (isShortLeaseProject(row.obs_project) || row.return_plan_time) {
    info.return_plan_time = row.return_plan_time || undefined;
  }

  if (demandResTypes.includes('CVM')) {
    info.cvm = {
      res_mode: resolveResMode(row.res_mode),
      device_type: row.device_type,
      os,
      cpu_core: cpu,
      memory,
    };
  }

  if (demandResTypes.includes('CBS')) {
    info.cbs = {
      disk_type: row.disk_type,
      disk_io: Number(row.disk_io) || 0,
      disk_size: diskSize,
    };
  }

  return info;
}

/**
 * 按行 diff 推导 adjust 载荷，契约依据 MR !3290（取代 !3267）。
 * - 新增行 → add：没有 demand_id / original_info 可依
 * - 仅到货时间变 → delay：裸载荷（demand_id + expect_time），改前信息由后端按 demand_id 回查
 * - 含配置变更（含「时间 + 配置」组合）→ update：到货时间走 updated_info.expect_time，不发顶层字段
 * - 无变更 → null，不入 adjusts
 * 同机型改期改量同样走 update（delay 仅为整单改期）。
 */
export function convertRowToAdjust(row: IAdjustRow): IAdjustItemPayload | null {
  if (row.is_new) {
    return {
      adjust_type: 'add',
      demand_source: row.demand_source || DEFAULT_DEMAND_SOURCE,
      updated_info: mapRowToAdjustInfo(row),
    };
  }

  const { baseline } = row;
  if (!baseline) return null;

  const timeChanged = isTimeChanged(row, baseline);
  const configChanged = !isConfigEqual(row, baseline);

  if (!timeChanged && !configChanged) return null;

  if (timeChanged && !configChanged) {
    return {
      demand_id: row.demand_id,
      adjust_type: 'delay',
      expect_time: row.expect_time,
    };
  }

  return {
    demand_id: row.demand_id,
    adjust_type: 'update',
    demand_source: row.demand_source || DEFAULT_DEMAND_SOURCE,
    original_info: mapRowToAdjustInfo(baseline),
    updated_info: mapRowToAdjustInfo(row),
  };
}

/** 解析本批统一的 demand_class：优先历史行，其次任意已填行 */
export function resolveDemandClass(rows: IAdjustRow[]): string {
  const fromHistorical = rows.find((row) => !row.is_new && row.demand_class)?.demand_class;
  if (fromHistorical) return fromHistorical;
  return rows.find((row) => row.demand_class)?.demand_class || '';
}

/** 是否存在多种非空 demand_class（如手动改 planIds 混入不同用途） */
export function hasMixedDemandClass(rows: Array<{ demand_class?: string }>): boolean {
  const classes = new Set(rows.map((row) => row.demand_class).filter((item): item is string => Boolean(item)));
  return classes.size > 1;
}

/**
 * 组装提交体：
 * - adjusts：按行 diff → update / delay / add
 * - 顶层 demand_class：从当前表行解析（优先历史行）；有值才带上
 * - demand_class 不进 updated_info / original_info
 */
export function buildAdjustPayload(rows: IAdjustRow[]): {
  adjusts: IAdjustItemPayload[];
  demand_class?: string;
} {
  const adjusts = rows.map(convertRowToAdjust).filter((item): item is IAdjustItemPayload => Boolean(item));
  const resolved = resolveDemandClass(rows).trim();
  const payload: { adjusts: IAdjustItemPayload[]; demand_class?: string } = { adjusts };
  if (resolved) {
    payload.demand_class = resolved;
  }
  return payload;
}

export function createEmptyAdjustRow(partial?: Partial<IAdjustRow>): IAdjustRow {
  return {
    row_key: `new-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    is_new: true,
    demand_id: '',
    obs_project: '',
    expect_time: '',
    return_plan_time: '',
    region_id: '',
    region_name: '',
    zone_id: '',
    zone_name: '',
    demand_class: '',
    res_mode: DEFAULT_RES_MODE,
    device_class: '',
    device_type: '',
    remained_os: 0,
    remained_cpu_core: 0,
    remained_memory: 0,
    disk_type: '',
    disk_type_name: '',
    disk_io: DEFAULT_DISK_IO,
    disk_per_size: 0,
    remained_disk_size: 0,
    demand_source: DEFAULT_DEMAND_SOURCE,
    demand_res_type: 'CVM',
    ...partial,
    is_new: true,
    demand_id: '',
    baseline: undefined,
  };
}

export function cloneRowAsNew(row: IAdjustRow): IAdjustRow {
  // 原样拷贝配置；去掉 baseline，作为新增行提交
  const { baseline: _baseline, ...rest } = row;
  return createEmptyAdjustRow({
    ...rest,
    row_key: `copy-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    // 源行到货日常已过可申领窗；连带清退回日（相对到货日）
    expect_time: '',
    return_plan_time: '',
    // 存量行 list 常不带变更原因；复制后按新增行默认，已有值则沿用
    demand_source: rest.demand_source || DEFAULT_DEMAND_SOURCE,
  });
}

export function demandDetailToAdjustRow(detail: Record<string, any>): IAdjustRow {
  const os = Number(detail.remained_os) || 0;
  const diskSize = Number(detail.remained_disk_size) || 0;
  const diskPerSize = os > 0 ? Math.floor(diskSize / os) : 0;

  const baseline: IAdjustBaseline = {
    obs_project: detail.obs_project || '',
    expect_time: detail.expect_time || '',
    return_plan_time: detail.return_plan_time || '',
    region_id: detail.region_id || '',
    region_name: detail.region_name || '',
    zone_id: detail.zone_id || '',
    zone_name: detail.zone_name || '',
    demand_class: detail.demand_class || '',
    res_mode: resolveResMode(detail.res_mode),
    device_class: detail.device_class || '',
    device_type: detail.device_type || '',
    remained_os: os,
    remained_cpu_core: Number(detail.remained_cpu_core) || 0,
    remained_memory: Number(detail.remained_memory) || 0,
    disk_type: detail.disk_type || '',
    disk_type_name: detail.disk_type_name || '',
    disk_io: Number(detail.disk_io) || 0,
    disk_per_size: diskPerSize,
    remained_disk_size: diskSize,
    // 不在此处兜底：list 目前不返回该字段，兜底会把「无历史值」伪装成「变更原因是指标变化」。
    // 存量行只读展示，空值显示 "-"；提交时各 payload 分支自带 DEFAULT_DEMAND_SOURCE 兜底。
    // 后端补上字段后此处即自动展示真值。
    demand_source: detail.demand_source || '',
    demand_res_type: detail.demand_res_type || 'CVM',
  };

  return {
    ...baseline,
    row_key: detail.demand_id || `row-${Date.now()}`,
    is_new: false,
    demand_id: detail.demand_id || '',
    baseline: { ...baseline },
  };
}
