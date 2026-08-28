/** 调整页行基线快照（仅已有预测行） */
export interface IAdjustBaseline {
  obs_project: string;
  expect_time: string;
  return_plan_time?: string;
  region_id: string;
  region_name: string;
  zone_id: string;
  zone_name: string;
  demand_class: string;
  res_mode: string;
  device_class: string;
  device_type: string;
  remained_os: number;
  remained_cpu_core: number;
  remained_memory: number;
  disk_type: string;
  disk_type_name: string;
  disk_io: number;
  disk_per_size: number;
  remained_disk_size: number;
  demand_source: string;
  /** 资源类型：CVM / CBS */
  demand_res_type: string;
}

/** 调整页可编辑行模型 */
export interface IAdjustRow extends IAdjustBaseline {
  /** 行唯一键（前端） */
  row_key: string;
  /** 是否为调整单内新增/复制行 */
  is_new: boolean;
  /** 预测 ID；新增行为空 */
  demand_id: string;
  /** 原始行基线；新增行为 undefined */
  baseline?: IAdjustBaseline;
}

/** 提交前整表校验结果 */
export interface IAdjustValidateResult {
  valid: boolean;
  /** 校验未通过的行在当前表格中的下标 */
  invalidRowIndexes: number[];
}

export const SHORT_LEASE_OBS_PROJECT = '短租项目';

export const ROLLING_SERVER_OBS_PROJECT = '滚服项目';

/** 滚服项目只有 931 业务可选，口径与新建/修改表单一致 */
export const ROLLING_SERVER_BIZ_ID = 931;

/** 选中这些项目类型时提示需先咨询，与新建/修改表单一致 */
export const SPECIAL_OBS_PROJECTS = ['改造复用', '轻量云徙'];
export const SPECIAL_OBS_PROJECT_TIP = '注意：所选项目为特殊类型，如需使用该项目类型，请咨询ICR助手';

/** 磁盘 IO 默认值：新增行初始值、切换云盘类型后的回落值，与现网 handleUpdateDiskType 一致 */
export const DEFAULT_DISK_IO = 15;

/** 高性能云盘：磁盘 IO 上限 150，其余盘型 260 */
export const PREMIUM_DISK_TYPE = 'CLOUD_PREMIUM';

/** 短租退回日期的快捷月数，与现网日期面板 footer 一致 */
export const RETURN_TIME_SHORTCUT_MONTHS = [1, 2, 3];

/** 资源类型：存量行只读，新增行可在 CVM / CBS 间选择，与现网调整态/新建态一致 */
export const DEMAND_RES_TYPE_OPTIONS = [
  { value: 'CVM', label: 'CVM' },
  { value: 'CBS', label: 'CBS' },
];

/**
 * 资源模式在两个接口里是两套表示：列表返回 code（device_type / device_family），
 * 调整接口只认中文名（按机型 / 按机型族），直接透传会报 `unsupported res mode: device_type`。
 * 现网旧调整页是装载时硬写 '按机型'，这里改为按 code 映射，语义等价且兼容后续放开机型族。
 */
export const DEFAULT_RES_MODE = '按机型';

const RES_MODE_BY_CODE: Record<string, string> = {
  device_type: DEFAULT_RES_MODE,
  device_family: '按机型族',
};

/** 兼容已是中文名的入参（如工单侧数据）原样返回 */
export function resolveResMode(value?: string): string {
  if (!value) return DEFAULT_RES_MODE;
  if (Object.values(RES_MODE_BY_CODE).includes(value)) return value;
  return RES_MODE_BY_CODE[value] ?? DEFAULT_RES_MODE;
}
