<script setup lang="ts">
import { computed, nextTick, ref, watch, onMounted, useTemplateRef } from 'vue';
import dayjs from 'dayjs';
import isBetween from 'dayjs/plugin/isBetween';
import isoWeek from 'dayjs/plugin/isoWeek';
import { SelectColumn, InputColumn, DateTimePickerColumn, TextPlainColumn } from '@blueking/ediatable';
import { InfoLine } from 'bkui-vue/lib/icon';
import { useCvmAdjustStore } from '@/store/resource-plan/cvm-adjust';
import type { IDeviceType, IDiskType, IRegion, IZone } from '@/typings/resourcePlan';
import type { IExceptTimeRange } from '@/typings/plan';
import AdjustOperationColumn from './operation-column.vue';
import {
  cloneRowAsNew,
  createEmptyAdjustRow,
  isCbsOnlyRow,
  isFieldChanged,
  isShortLeaseProject,
} from './adjust-payload';
import type { IAdjustBaseline, IAdjustRow } from './typings';
import {
  DEFAULT_DISK_IO,
  DEMAND_RES_TYPE_OPTIONS,
  PREMIUM_DISK_TYPE,
  RETURN_TIME_SHORTCUT_MONTHS,
  ROLLING_SERVER_OBS_PROJECT,
  SHORT_LEASE_OBS_PROJECT,
  SPECIAL_OBS_PROJECTS,
  SPECIAL_OBS_PROJECT_TIP,
} from './typings';

const localData = defineModel<IAdjustRow>({ required: true });
const props = withDefaults(
  defineProps<{
    index: number;
    removeable?: boolean;
    obsProjectOptions: { value: string; label: string }[];
    /** 滚服项目仅 931 业务可选，由 data-list 统一下发 */
    showRollingServerProject: boolean;
    regionOptions: IRegion[];
    deviceClassOptions: string[];
    diskTypeOptions: IDiskType[];
    demandSourceOptions: { value: string; label: string }[];
    demandClassOptions: { value: string; label: string }[];
    /** 仅纯新增表可编辑预测用途；混编/历史行只读 */
    demandClassEditable?: boolean;
    /** 上述字典在 data-list 并行拉取，未回来前下拉置 loading */
    optionsLoading?: boolean;
    /** 评审期禁选（非本年），由 data-list 统一下发；与「非本周过去日」组合成完整日历禁选 */
    isDateDisabled: (date: Date) => boolean;
    /** 评审期文案，非评审期为空串 */
    reviewPhaseTip: string;
  }>(),
  {
    removeable: true,
    demandClassEditable: false,
  },
);
const emit = defineEmits<{
  (e: 'add'): void;
  (e: 'row-copy', row: IAdjustRow): void;
  (e: 'remove', index: number): void;
}>();

dayjs.extend(isBetween);
dayjs.extend(isoWeek);

const adjustStore = useCvmAdjustStore();

/** 首屏异步拉选项期间禁止回写行字段，避免假 diff */
const isHydrating = ref(true);
/**
 * SelectColumn 源码写死了 auto-focus + filterable：挂载时聚焦搜索框，空搜索会 emitChange('')，
 * 先改掉内部 localValue；必填校验失败时还不回写父级——于是「行数据还在、格子已空」。
 * 上一版只挡 v-model 没用（父级根本没被改）。正确做法：选项就绪前不挂 Select，挂时关掉 auto-focus。
 */
const selectsReady = ref(false);

const expectTimeRange = ref<IExceptTimeRange | null>(null);

/** 与新建预测页一致的可申领周期提示，关键日期单独成段以便标红 */
const expectTimeTipSegments = computed(() => {
  const {
    year_month_week: week,
    date_range_in_week: inWeek,
    date_range_in_month: inMonth,
  } = expectTimeRange.value || {};
  if (!week || !inWeek?.start || !inWeek?.end) return [];

  const segments = [
    { text: '注意：日期落在' },
    { text: `${week.year}年${week.month}月W${week.week}`, strong: true },
    { text: '，需要在' },
    { text: `${inWeek.start}~${inWeek.end}`, strong: true },
    { text: '之间申领' },
  ];
  if (inMonth?.end) {
    segments.push({ text: '，超过' }, { text: inMonth.end, strong: true }, { text: '将无法申领' });
  }
  return segments;
});

const expectTimeTip = computed(() => expectTimeTipSegments.value.map((item) => item.text).join(''));

/** 面板 footer 常驻：未选日期时先给引导文案 */
const expectTimeFooterSegments = computed(() =>
  localData.value.expect_time ? expectTimeTipSegments.value : [{ text: '请先选择期望到货日期', strong: true }],
);

const isCbsOnly = computed(() => isCbsOnlyRow(localData.value));

/**
 * CBS 的云盘填法在现网分两态，整表把两态并到了一起：
 * - 新增行对应新建态：填「容量/块」与块数，总量派生；块数复用实例数量列
 * - 存量行对应调整态：容量与块数都不出现，直接填总量
 */
const isCbsAdjustRow = computed(() => isCbsOnly.value && !localData.value.is_new);

/**
 * 现网调整态先过 convertToPlanTicketDemand，云盘剩余量为 0 时 cbs 为 undefined，
 * 整个「CBS云磁盘信息」面板不渲染，四个云盘字段一起消失。表格四列恒在，
 * 这里按同样的口径置只读，避免给一个本来没有云盘的预测补上云盘。
 * 判定取 baseline 而非当前值，否则用户改动后条件会自己翻转。
 */
const hasNoDisk = computed(() => !localData.value.is_new && Number(localData.value.baseline?.remained_disk_size) === 0);

const obsProjectSelectList = computed(() => {
  // 对齐现网 ObsProjectSelector：滚服项目限 931 业务，短租项目不适用于 CBS。
  // 非 931 时存量/新增都不出现滚服，也不把当前值补回选项（避免下拉里又能选到）。
  // 其它已下线的项目类型仍补回，保证回显。
  const hidden = [
    props.showRollingServerProject ? '' : ROLLING_SERVER_OBS_PROJECT,
    isCbsOnly.value ? SHORT_LEASE_OBS_PROJECT : '',
  ].filter(Boolean);
  const list = props.obsProjectOptions.filter((item) => !hidden.includes(item.value));
  const current = localData.value.obs_project;
  if (!current || list.some((item) => item.value === current)) return list;
  if (!props.showRollingServerProject && current === ROLLING_SERVER_OBS_PROJECT) return list;
  return [...list, { value: current, label: current }];
});

const isSpecialObsProject = computed(() => SPECIAL_OBS_PROJECTS.includes(localData.value.obs_project));

/**
 * ediatable SelectColumn 把 modelValue 声明成 [Boolean, Number, String, Array]，Boolean 排在 String 前面，
 * Vue 的布尔转型规则会把空串当作「布尔属性简写」转成 true，单元格于是显示 "true"。
 * 空值统一改用 null 下发绕开这条规则，行数据本身仍是空串。
 */
type SelectField =
  | 'obs_project'
  | 'region_id'
  | 'zone_id'
  | 'demand_res_type'
  | 'device_class'
  | 'device_type'
  | 'disk_type'
  | 'demand_source';

/** 允许用户清空的下拉；其余列 clearable=false，控件空回写一律视为假回写 */
const CLEARABLE_SELECT_FIELDS: ReadonlySet<SelectField> = new Set(['zone_id', 'disk_type']);

const selectModel = (field: SelectField) =>
  computed<string | null>({
    get: () => localData.value[field] || null,
    set: (val) => {
      const next = val ?? '';
      const prev = localData.value[field];
      // 复制/新增行挂载或选项列表刷新时，SelectColumn 常会把已有值打成空。
      // 这与是否清 expect_time 无关，但复制行最容易踩中，看起来像「整行被清空」。
      // 非 clearable 列：永远不接受「有值→空」；clearable 列：仅 hydrating 期间挡。
      if (!next && prev && (isHydrating.value || !CLEARABLE_SELECT_FIELDS.has(field))) return;
      (localData.value as Record<SelectField, string>)[field] = next;
    },
  });

const obsProjectModel = selectModel('obs_project');
const regionModel = selectModel('region_id');
const zoneModel = selectModel('zone_id');
const demandResTypeModel = selectModel('demand_res_type');
const deviceClassModel = selectModel('device_class');
const deviceTypeModel = selectModel('device_type');
const diskTypeModel = selectModel('disk_type');
const demandSourceModel = selectModel('demand_source');

const zoneOptions = ref<IZone[]>([]);
const isZoneLoading = ref(false);
const isDeviceTypeLoading = ref(false);
const deviceTypes = ref<IDeviceType[]>([]);
const deviceTypeTip = ref('');

const expectTimeRef = useTemplateRef<InstanceType<typeof DateTimePickerColumn>>('expectTimeRef');
const returnTimeRef = useTemplateRef<InstanceType<typeof DateTimePickerColumn>>('returnTimeRef');
const obsProjectRef = useTemplateRef<any>('obsProjectRef');
const demandResTypeRef = useTemplateRef<any>('demandResTypeRef');
const regionRef = useTemplateRef<any>('regionRef');
const deviceClassRef = useTemplateRef<any>('deviceClassRef');
const deviceTypeRef = useTemplateRef<any>('deviceTypeRef');
const osRef = useTemplateRef<any>('osRef');
const diskPerSizeRef = useTemplateRef<any>('diskPerSizeRef');
const diskSizeRef = useTemplateRef<any>('diskSizeRef');
const diskIoRef = useTemplateRef<any>('diskIoRef');
const diskTypeRef = useTemplateRef<any>('diskTypeRef');
const demandSourceRef = useTemplateRef<any>('demandSourceRef');
const demandClassRef = useTemplateRef<any>('demandClassRef');

/**
 * 预测用途：可编辑时写回本行；一致性由提交按钮侧校验（勿整批同步）。
 * clearable=false：不接受「有值→空」的假回写。
 */
const demandClassModel = computed<string | null>({
  get: () => localData.value.demand_class || null,
  set: (val) => {
    if (!props.demandClassEditable) return;
    const next = val ?? '';
    const prev = localData.value.demand_class;
    if (!next && prev) return;
    localData.value.demand_class = next;
  },
});

const demandClassSelectList = computed(() => {
  const list = props.demandClassOptions;
  const current = localData.value.demand_class;
  if (!current || list.some((item) => item.value === current)) return list;
  return [...list, { value: current, label: current }];
});

/** 三个列组件的最小交集：expose 出来的 getValue()，与用于读校验态的根元素 */
type IEditableCell = { $el?: unknown; getValue?: () => Promise<unknown> };

/**
 * ediatable 的校验错误态存在各列组件内部（useValidtor 的 message ref，非空即渲染 .is-error），
 * 只有再跑一次 validator 才会被清掉，而 validator 的入口只有三个：控件自身的 change、日期面板收起、
 * expose 出来的 getValue()。程序化写值绕过了前两个，错误态会一直滞留到下次 blur 或提交前的整表校验，
 * 所以凡是代码直接改值的地方都要手动补一次重校验。
 * 列组件的值走 props 下发，必须等一帧新值到位再校验，否则校验的还是旧值。
 */
const revalidateCell = async (cell?: IEditableCell | null) => {
  await nextTick();
  // getValue 校验失败会 reject，这里只为刷新错误态，吞掉避免 unhandled rejection
  await cell?.getValue?.().catch(() => false);
};

/**
 * 「只降不升」的重校验：仅当该格当前已是错误态时才重跑规则，用于键入过程中即时消错。
 * 未出错的格子一律不参与——无差别地随输入校验会把中间态立刻标红（如把 62 全选删除的瞬间），
 * 比错误态滞留更聒噪。因为 validator 进门先清错误再逐条重判，这个方向上只可能消错或维持原错。
 * 校验态没有 expose，只能从根元素的 .is-error 读——与 data-list 定位出错单元格用的是同一个信号。
 */
const clearResolvedError = (cell?: IEditableCell | null) => {
  const el = cell?.$el;
  if (!(el instanceof HTMLElement) || !el.classList.contains('is-error')) return;
  revalidateCell(cell);
};

// 与原表单一致：短租退回日期仅短租项目出现且必填，且不得早于期望到货日期
const requireReturnPlanTime = computed(() => isShortLeaseProject(localData.value.obs_project));
/**
 * ediatable 的 useValidtor 只在 setup 时读一次 props.rules，之后换 rules 不再生效，
 * 所以「是否短租项目」必须写进 validator 由闭包实时判定，不能靠切换 rules 数组——
 * 否则行内改项目类型后规则仍是首帧结果：改成短租项目不校验、改走短租项目反而卡住提交。
 */
const returnPlanTimeRules = [
  {
    validator: (val: string) => !requireReturnPlanTime.value || Boolean(val),
    message: '短租项目请填写短租退回日期',
  },
  {
    validator: (val: string) =>
      !requireReturnPlanTime.value || !dayjs(val).isBefore(dayjs(localData.value.expect_time), 'day'),
    message: '短租退回日期不能早于期望到货日期',
  },
];

/**
 * 同一列在 CVM / CBS 下措辞不同，而 rules 定格在首帧，写成三元会让新增行切换资源类型后
 * 报错文案停在旧措辞。useValidtor 的 getRuleMessage 支持函数式 message，改到校验时才求值。
 */
const osRules = [
  {
    validator: (val: number) => Number(val) > 0,
    message: () => (isCbsOnly.value ? '所需数量应大于0' : '实例数量应大于0'),
  },
  // 包一层而非直接引用：isCpuCoreInteger 声明在下方，直接引用会在 setup 求值时踩 TDZ
  { validator: (val: number) => isCpuCoreInteger(val), message: '请调整实例数为整数' },
];

const diskPerSizeRules = [
  {
    validator: (val: number) => (Number(val) || 0) >= 0,
    message: () => (isCbsOnly.value ? '云盘容量/块应大于等于0' : '云盘容量/实例应大于等于0'),
  },
];

/**
 * 现网 `<bk-form-item property='disk_type' required>` 无条件必填，新建/调整都校验；
 * 唯一不校验的是没有云盘的存量行——那时整个 CBS 面板不渲染。表格里对应 hasNoDisk 渲染纯文本、
 * 拿不到 ref，天然不参与校验，所以这里直接无条件必填即可。
 */
const diskTypeRules = [{ validator: (val: string) => Boolean(val), message: '请选择云盘类型' }];
/**
 * 只读的唯一判定源：既驱动组件的 disabled，也驱动单元格灰底。
 * 此前灰底分别来自 SelectColumn 的 .is-disable、TimePicker 的 .is-disabled 和
 * TextPlainColumn 的 .default-display，后者表达的其实是「没传 data 的占位态」而非只读。
 */
const readonlyFields = computed<Partial<Record<keyof IAdjustBaseline, boolean>>>(() => ({
  // 混编继承历史用途只读；纯新增表内可按行编辑（一致性由提交侧校验）
  demand_class: !props.demandClassEditable,
  // 现网调整态资源类型是禁用的 radio（disabled={type !== AdjustType.none}），只有新建态可选
  demand_res_type: !localData.value.is_new,
  // 已有预测的变更原因跟着原单据走，只有本次新增的行才由用户选
  demand_source: !localData.value.is_new,
  // CBS 行现网整个「CVM云主机信息」面板不渲染，这几列不适用
  device_class: isCbsOnly.value,
  device_type: isCbsOnly.value,
  remained_os: isCbsAdjustRow.value,
  // 机型 × 实例数量派生
  remained_cpu_core: true,
  remained_memory: true,
  // 无云盘的存量行现网整个「CBS云磁盘信息」面板不渲染，这几列不适用
  disk_type: hasNoDisk.value,
  // 现网 disabled={!disk_type}：IO 上限取决于盘型，未选盘型时无从校验
  disk_io: hasNoDisk.value || !localData.value.disk_type,
  disk_per_size: isCbsAdjustRow.value || hasNoDisk.value,
  // 存量 CBS 行直接填总量，其余由「容量/实例 × 实例数量」派生
  remained_disk_size: !isCbsAdjustRow.value || hasNoDisk.value,
  // 现网 disabled={!expect_time}：退回日期的可选范围以期望到货日期为下界
  return_plan_time: !requireReturnPlanTime.value || !localData.value.expect_time,
}));
const isReadonly = (field: keyof IAdjustBaseline) => Boolean(readonlyFields.value[field]);

/**
 * 对齐现网 getReturnDisabledDate：退回日期不早于「今天」与「期望到货日期」中较晚的一个。
 * 校验规则只比到货日期，这里补上今天的下界，避免选出一个已经过期的退回日期。
 */
const isReturnDateDisabled = (date: Date) => {
  const today = dayjs();
  const expectTime = dayjs(localData.value.expect_time);
  return dayjs(date).isBefore(today.isAfter(expectTime) ? today : expectTime, 'day');
};

/**
 * 现网把「请先选择期望到货日期」写在字段下方，表格里没有这个位置；
 * 禁用态面板不会展开，footer 也提示不到，故回退到单元格 tooltip——
 * 仅在没有「调整前为」可显示时占用，两者不冲突。
 */
const returnTimeTip = computed(() => {
  const origin = originTip('return_plan_time');
  if (!origin.disabled) return origin;
  if (requireReturnPlanTime.value && !localData.value.expect_time) {
    return { content: '请先选择期望到货日期', disabled: false };
  }
  return origin;
});

/** 对齐现网 handleReturnTimeWithMonth：按月数从期望到货日期起算，1 个月记 30 天 */
const setReturnTimeByMonth = async (month: number) => {
  if (!localData.value.expect_time) return;
  localData.value.return_plan_time = dayjs(localData.value.expect_time)
    .add(month * 30, 'day')
    .format('YYYY-MM-DD');
  // 面板 footer 的快捷按钮不经过 DatePicker 的 change，报错态要靠这次重校验才即时消除
  await revalidateCell(returnTimeRef.value);
};

/** 现网：高性能云盘上限 150，其余（SSD 云硬盘）上限 260 */
const diskIoMax = computed(() => (localData.value.disk_type === PREMIUM_DISK_TYPE ? 150 : 260));

const regionSelectList = computed(() =>
  props.regionOptions.map((item) => ({ value: item.region_id, label: item.region_name })),
);
const zoneSelectList = computed(() =>
  zoneOptions.value.map((item) => ({ value: item.zone_id, label: item.zone_name })),
);
const deviceClassSelectList = computed(() => props.deviceClassOptions.map((item) => ({ value: item, label: item })));
const deviceTypeSelectList = computed(() =>
  deviceTypes.value.map((item) => ({ value: item.device_type, label: item.device_type })),
);
const diskTypeSelectList = computed(() =>
  props.diskTypeOptions.map((item) => ({ value: item.disk_type, label: item.disk_type_name })),
);

const cellClass = (field: keyof IAdjustBaseline) => ({
  'adjust-cell': true,
  'is-readonly': isReadonly(field),
  'is-new': localData.value.is_new,
  'is-changed': !localData.value.is_new && isFieldChanged(localData.value, field),
});

const originTip = (field: keyof IAdjustBaseline) => {
  if (localData.value.is_new || !localData.value.baseline) return { disabled: true };
  if (!isFieldChanged(localData.value, field)) return { disabled: true };
  const origin = localData.value.baseline[field];
  return {
    content: `调整前为：${origin === undefined || origin === null || origin === '' ? '-' : origin}`,
    disabled: false,
  };
};

/** 与现网对齐：字典里已下线的存量可用区，在选项加载完后清掉，避免提交一个不存在的可用区 */
const dropOrphanZone = () => {
  const zoneId = localData.value.zone_id;
  if (!zoneId) return;
  if (zoneOptions.value.some((item) => item.zone_id === zoneId)) return;
  localData.value.zone_id = '';
  localData.value.zone_name = '';
};

const loadZones = async (regionId: string) => {
  if (!regionId) {
    zoneOptions.value = [];
    return;
  }
  isZoneLoading.value = true;
  try {
    zoneOptions.value = await adjustStore.getZones(regionId);
    // 只在拉取成功后校正，请求失败时选项为空并不代表存量值失效
    dropOrphanZone();
  } catch {
    zoneOptions.value = [];
  } finally {
    isZoneLoading.value = false;
  }
};

const loadDeviceTypes = async (deviceClass: string) => {
  if (!deviceClass) {
    deviceTypes.value = [];
    return;
  }
  isDeviceTypeLoading.value = true;
  try {
    deviceTypes.value = await adjustStore.getDeviceTypes(deviceClass);
  } catch {
    deviceTypes.value = [];
  } finally {
    isDeviceTypeLoading.value = false;
  }
};

const updateDeviceTip = () => {
  const device = deviceTypes.value.find((item) => item.device_type === localData.value.device_type);
  // 沿用现网 calcCpuAndMemory 的文案；表格里没有常显位置，改由单元格右侧 ⓘ 承接
  deviceTypeTip.value = device
    ? `所选机型为${device.core_type}，CPU为${device.cpu_core}核，内存为${device.memory}G${
        device.gpu_amount > 0 ? `，GPU卡为${device.gpu_amount}张` : ''
      }`
    : '';
  return device;
};

/**
 * 与现网 os 的第二条规则一致：单机核数为小数的机型乘上实例数后总核数可能不是整数。
 * 这里直接用「单机核数 × 实例数」而非派生出的 remained_cpu_core——派生走 watch，
 * change 触发校验时派生值可能还没落库。
 */
const isCpuCoreInteger = (os: number) => {
  const device = deviceTypes.value.find((item) => item.device_type === localData.value.device_type);
  if (!device) return true;
  return (device.cpu_core * Number(os)) % 1 === 0;
};

/** 对齐现网 calcCpuAndMemory：机型规格被清空时派生值归零，不能留着上一个机型的旧数 */
const syncCpuAndMemory = () => {
  const device = updateDeviceTip();
  if (isCbsOnly.value) return;
  const os = Number(localData.value.remained_os) || 0;
  localData.value.remained_cpu_core = (device?.cpu_core ?? 0) * os;
  localData.value.remained_memory = (device?.memory ?? 0) * os;
};

/** 对齐现网 calcDiskSize：单盘容量 × 数量，CVM 取实例数、新增 CBS 取块数，本表同用一列 */
const syncDiskSize = () => {
  // 存量 CBS 行没有块数可依，云盘总量由用户直接填写，不能被派生值覆盖
  if (isCbsAdjustRow.value || hasNoDisk.value) return;
  const num = Number(localData.value.remained_os) || 0;
  localData.value.remained_disk_size = (Number(localData.value.disk_per_size) || 0) * num;
};

// 对齐现网 handleUpdateResourceType：切换资源类型时清空项目类型（可选集随之变化）
watch(
  () => localData.value.demand_res_type,
  (_type, oldType) => {
    if (isHydrating.value || oldType === undefined) return;
    localData.value.obs_project = '';
  },
);

watch(
  () => localData.value.region_id,
  async (regionId, oldRegionId) => {
    await loadZones(regionId);
    if (isHydrating.value) return;
    const region = props.regionOptions.find((item) => item.region_id === regionId);
    if (region) localData.value.region_name = region.region_name;
    // 仅用户切换城市时清空可用区，避免首屏加载误清
    if (oldRegionId !== undefined && oldRegionId !== regionId) {
      localData.value.zone_id = '';
      localData.value.zone_name = '';
    }
  },
);

watch(
  () => localData.value.zone_id,
  (zoneId) => {
    if (isHydrating.value) return;
    // 清空可用区时 find 落空，zone_name 必须一并清掉，否则残留旧值会让「调整前为」提示失效
    const zone = zoneOptions.value.find((item) => item.zone_id === zoneId);
    localData.value.zone_name = zone?.zone_name ?? '';
  },
);

watch(
  () => localData.value.device_class,
  async (deviceClass, oldClass) => {
    await loadDeviceTypes(deviceClass);
    if (isHydrating.value) {
      updateDeviceTip();
      return;
    }
    updateDeviceTip();
    if (oldClass !== undefined && oldClass !== deviceClass) {
      localData.value.device_type = '';
      deviceTypeTip.value = '';
    }
  },
);

// 两条派生分开跟随各自的输入，与现网 calcCpuAndMemory / calcDiskSize 的依赖一致。
// 合成一个 watch 会让「只改机型规格」也重算云盘总量，把 floor 反推丢掉的余数静默抹平。
watch(
  () => [localData.value.device_type, localData.value.remained_os] as const,
  () => {
    if (isHydrating.value) return;
    syncCpuAndMemory();
  },
);

watch(
  () => [localData.value.disk_per_size, localData.value.remained_os] as const,
  () => {
    if (isHydrating.value) return;
    syncDiskSize();
  },
);

watch(
  () => localData.value.disk_type,
  async (diskType) => {
    if (isHydrating.value) return;
    // 清空云盘类型时 find 落空，名称必须一并清掉，否则残留旧值会让「调整前为」提示失真
    const found = props.diskTypeOptions.find((item) => item.disk_type === diskType);
    localData.value.disk_type_name = found?.disk_type_name ?? '';
    // 对齐现网 handleUpdateDiskType：盘型不同 IO 上限不同，切换后回落到默认值，避免留下超限值
    localData.value.disk_io = DEFAULT_DISK_IO;
    // 回落到的默认值一定满足规则，原有的 IO 报错已不成立，需重校验才会消除
    await revalidateCell(diskIoRef.value);
  },
);

/**
 * 四个带校验规则的数字输入列（实例数量 / 云盘容量/实例 / 云盘总量 / 单实例磁盘IO）都由 InputColumn 渲染，
 * 而 InputColumn 在键入阶段只写值不校验：它绑了 bk-input 的 input 事件，但 handleInput 只做
 * `modelValue.value = value`；validator 挂在 change / blur / Enter / clear / getValue() 上，
 * 而 bk-input 的 change 是原生 change 语义（失焦或回车才触发）。于是格子一旦报错——最常见来源是提交前的
 * 整表校验，新增行 remained_os 初始为 0，点一次「提交调整」就会标红——键入合法值也不会消错，红框挂到失焦为止。
 * 这里统一在值变化后补一次「只降不升」的重校验。四列共用一个 watch：clearResolvedError 只消错不造错，
 * 顺带扫到没变的那几格也是安全的，比逐字段各写一个 watch 清楚。
 */
watch(
  () => [
    localData.value.remained_os,
    localData.value.disk_per_size,
    localData.value.remained_disk_size,
    localData.value.disk_io,
  ],
  () => {
    [osRef.value, diskPerSizeRef.value, diskSizeRef.value, diskIoRef.value].forEach((cell) => clearResolvedError(cell));
  },
);

watch(
  () => localData.value.expect_time,
  async (time) => {
    if (!time) {
      expectTimeRange.value = null;
      return;
    }
    const expectTime = dayjs(time).format('YYYY-MM-DD');
    try {
      const range = await adjustStore.getAvailableTime(expectTime);
      // 请求期间用户可能又改了日期，丢弃过期结果
      if (dayjs(localData.value.expect_time).format('YYYY-MM-DD') !== expectTime) return;
      expectTimeRange.value = range;
    } catch {
      expectTimeRange.value = null;
    }
  },
  { immediate: true },
);

onMounted(async () => {
  isHydrating.value = true;
  try {
    await Promise.all([loadZones(localData.value.region_id), loadDeviceTypes(localData.value.device_class)]);
    updateDeviceTip();
    // 选项就绪后再挂 Select，避免首挂空 list + auto-focus 把 localValue 打成空
    selectsReady.value = true;
    await nextTick();
  } finally {
    isHydrating.value = false;
  }
});

/**
 * 对齐现网 add/basic getDisabledDate：
 * 1) 评审期内非本年禁选（props.isDateDisabled）
 * 2) 本周（ISO）内可选，含今天之前
 * 3) 非本周且早于今天禁选
 */
const isExpectTimeDisabled = (date: Date) => {
  if (props.isDateDisabled(date)) return true;
  const currentDate = dayjs(date);
  const startOfWeek = dayjs().startOf('isoWeek');
  const endOfWeek = dayjs().endOf('isoWeek');
  if (currentDate.isBetween(startOfWeek, endOfWeek, 'day', '[]')) return false;
  return currentDate.isBefore(dayjs(), 'day');
};

/** 对齐现网「13周后」快捷；写值后重校验，与短租退回快捷同一套路 */
const setExpectTimeInThirteenWeeks = async () => {
  localData.value.expect_time = dayjs().add(13, 'week').format('YYYY-MM-DD');
  await revalidateCell(expectTimeRef.value);
};

const handleAdd = () => {
  emit('add');
};

const handleCopy = () => {
  emit('row-copy', cloneRowAsNew(localData.value));
};

const handleRemove = () => {
  emit('remove', props.index);
};

const getValue = async () => {
  // Select 延后挂载：提交时若选项还没就绪，先等到挂上再校验，避免漏检必填下拉
  if (!selectsReady.value) {
    await new Promise<void>((resolve) => {
      const stop = watch(selectsReady, (ready) => {
        if (!ready) return;
        stop();
        resolve();
      });
    });
    await nextTick();
  }
  const refs = [
    expectTimeRef.value,
    demandClassRef.value,
    returnTimeRef.value,
    obsProjectRef.value,
    demandResTypeRef.value,
    regionRef.value,
    deviceClassRef.value,
    deviceTypeRef.value,
    osRef.value,
    diskPerSizeRef.value,
    diskSizeRef.value,
    diskIoRef.value,
    diskTypeRef.value,
    demandSourceRef.value,
  ].filter(Boolean);
  await Promise.all(refs.map((item) => (item as any).getValue?.()));
  return { ...localData.value };
};

defineExpose({
  getValue,
  createEmptyAdjustRow,
});
</script>

<template>
  <tr :class="{ 'is-new-row': localData.is_new }">
    <td :class="cellClass('expect_time')">
      <div class="expect-time-cell" :class="{ 'has-tip': Boolean(expectTimeTip) }">
        <div v-bk-tooltips="originTip('expect_time')">
          <!-- type=datetime 只为拿到「确定」确认栏：面板的确认栏由 isConfirm 控制，
               date 类型选完即关，用户来不及看 footer 提示。值仍由 format 约束为日期，
               datetime 与 date 的 formatter/parser 实现一致，故进出值不变。 -->
          <DateTimePickerColumn
            ref="expectTimeRef"
            v-model="localData.expect_time"
            type="datetime"
            format="yyyy-MM-dd"
            ext-popover-cls="expect-time-picker-dropdown"
            :clearable="false"
            :disabled-date="isExpectTimeDisabled"
            :rules="[{ validator: (val: string) => Boolean(val), message: '请选择期望到货时间' }]"
          >
            <!-- 插槽必须常驻：bk-date-picker 的 hasFooter 是无响应式依赖的 computed，只会求值一次 -->
            <template #footer>
              <!--
                footer 是 panel 的兄弟节点，不在 pickerPanel.$el 内。
                bk-date-picker 的 clickoutside 在 document mouseup 里关面板，且只把
                pickerPanel.contains(target) 当「内部」——点 footer 会被当成外部点击。
                必须在 footer 上 stop 掉 mouseup（仅 mousedown.prevent 不够）。
              -->
              <div class="expect-time-picker-footer" @mousedown.prevent.stop @mouseup.prevent.stop @click.stop>
                <!-- 视觉上绝对定位到「确定」行居中，见 .expect-time-picker-dropdown -->
                <div class="expect-time-thirteen-weeks" @click="setExpectTimeInThirteenWeeks">13周后</div>
                <p v-if="expectTimeFooterSegments.length">
                  <span
                    v-for="(seg, i) in expectTimeFooterSegments"
                    :key="i"
                    :class="{ 'time-txt': seg.strong }"
                    v-text="seg.text"
                  />
                </p>
                <p v-if="reviewPhaseTip" class="review-phase-tip" v-text="reviewPhaseTip" />
              </div>
            </template>
          </DateTimePickerColumn>
        </div>
        <InfoLine v-if="expectTimeTip" v-bk-tooltips="{ content: expectTimeTip }" class="expect-time-cell__tip" />
      </div>
    </td>

    <td :class="cellClass('demand_class')">
      <TextPlainColumn v-if="!selectsReady || !props.demandClassEditable">
        {{ localData.demand_class || '-' }}
      </TextPlainColumn>
      <SelectColumn
        v-else
        ref="demandClassRef"
        v-model="demandClassModel"
        :auto-focus="false"
        :list="demandClassSelectList"
        :loading="optionsLoading"
        filterable
        :clearable="false"
        :rules="[{ validator: (val: string) => Boolean(val), message: '请选择预测用途' }]"
      />
    </td>

    <td :class="cellClass('obs_project')">
      <div class="select-tip-cell" :class="{ 'has-tip': isSpecialObsProject }">
        <TextPlainColumn v-if="!selectsReady">{{ localData.obs_project || '-' }}</TextPlainColumn>
        <SelectColumn
          v-else
          ref="obsProjectRef"
          v-model="obsProjectModel"
          v-bk-tooltips="originTip('obs_project')"
          :auto-focus="false"
          :list="obsProjectSelectList"
          :loading="optionsLoading"
          :clearable="false"
          :rules="[{ validator: (val: string) => Boolean(val), message: '请选择项目类型' }]"
        />
        <InfoLine
          v-if="isSpecialObsProject"
          v-bk-tooltips="{ content: SPECIAL_OBS_PROJECT_TIP }"
          class="select-tip-cell__tip"
        />
      </div>
    </td>

    <td :class="cellClass('region_id')" v-bk-tooltips="originTip('region_name')">
      <TextPlainColumn v-if="!selectsReady">{{ localData.region_name || localData.region_id || '-' }}</TextPlainColumn>
      <SelectColumn
        v-else
        ref="regionRef"
        v-model="regionModel"
        :auto-focus="false"
        :list="regionSelectList"
        :loading="optionsLoading"
        filterable
        :clearable="false"
        :rules="[{ validator: (val: string) => Boolean(val), message: '请选择城市' }]"
      />
    </td>

    <td :class="cellClass('zone_id')" v-bk-tooltips="originTip('zone_name')">
      <TextPlainColumn v-if="!selectsReady">{{ localData.zone_name || localData.zone_id || '-' }}</TextPlainColumn>
      <SelectColumn
        v-else
        v-model="zoneModel"
        :auto-focus="false"
        :list="zoneSelectList"
        :loading="isZoneLoading"
        filterable
        clearable
      >
        <!-- 见下方 optionRender 说明：不给这个插槽，选项列表变过之后重选会回显 zone_id -->
        <template #optionRender="{ item }">
          <span class="bk-select-option-item" :title="item.label" v-text="item.label" />
        </template>
      </SelectColumn>
    </td>

    <td :class="cellClass('demand_res_type')" v-bk-tooltips="originTip('demand_res_type')">
      <TextPlainColumn v-if="!selectsReady">{{ localData.demand_res_type || '-' }}</TextPlainColumn>
      <SelectColumn
        v-else
        ref="demandResTypeRef"
        v-model="demandResTypeModel"
        :auto-focus="false"
        :list="DEMAND_RES_TYPE_OPTIONS"
        :clearable="false"
        :disabled="isReadonly('demand_res_type')"
        :rules="[{ validator: (val: string) => Boolean(val), message: '请选择资源类型' }]"
      />
    </td>

    <!-- CBS 行：现网整个「CVM云主机信息」面板不渲染，这里对应显示 "-" -->
    <td :class="cellClass('device_class')" v-bk-tooltips="originTip('device_class')">
      <TextPlainColumn v-if="isCbsOnly || !selectsReady">
        {{ isCbsOnly ? '-' : localData.device_class || '-' }}
      </TextPlainColumn>
      <SelectColumn
        v-else
        ref="deviceClassRef"
        v-model="deviceClassModel"
        :auto-focus="false"
        :list="deviceClassSelectList"
        :loading="optionsLoading"
        filterable
        :clearable="false"
        :rules="[{ validator: (val: string) => Boolean(val), message: '请选择机型类型' }]"
      />
    </td>

    <!-- originTip 挂在 SelectColumn、ⓘ 作兄弟，对齐期望到货时间：hover 目标互斥，同时只出一个 tip -->
    <td :class="cellClass('device_type')">
      <div class="select-tip-cell" :class="{ 'has-tip': Boolean(deviceTypeTip) }">
        <TextPlainColumn v-if="isCbsOnly || !selectsReady">
          {{ isCbsOnly ? '-' : localData.device_type || '-' }}
        </TextPlainColumn>
        <SelectColumn
          v-else
          ref="deviceTypeRef"
          v-model="deviceTypeModel"
          v-bk-tooltips="originTip('device_type')"
          :auto-focus="false"
          :list="deviceTypeSelectList"
          :loading="isDeviceTypeLoading"
          filterable
          :clearable="false"
          :rules="[{ validator: (val: string) => Boolean(val), message: '请选择机型规格' }]"
        >
          <!--
            bk-select 解析回显文案的顺序是「已挂载选项 → list 属性 → 已解析过的缓存 → 值本身」，
            最后一档就是直接显示 id。而 ediatable 的 SelectColumn 只在检测到 optionRender 插槽时
            才把 list 透传给 bk-select（否则传空数组、改用默认插槽渲染 Option），
            于是「已挂载选项」成了唯一可依的一档。可用区/机型规格的选项列表会随城市/机型类型重新拉取，
            清空后重选时那一档取不到，就回落到显示 id。给出插槽即恢复 list 这一档，与挂载时机无关。
            代价是选项内的搜索关键字高亮（.is-keyword）没了，故这里自行套上原有的 class 保留省略号样式。
          -->
          <template #optionRender="{ item }">
            <span class="bk-select-option-item" :title="item.label" v-text="item.label" />
          </template>
        </SelectColumn>
        <InfoLine v-if="deviceTypeTip" v-bk-tooltips="{ content: deviceTypeTip }" class="select-tip-cell__tip" />
      </div>
    </td>

    <td
      :class="cellClass('remained_os')"
      v-bk-tooltips="
        isCbsOnly && !isCbsAdjustRow ? { content: 'CBS 预测在此填写所需云盘块数' } : originTip('remained_os')
      "
    >
      <TextPlainColumn v-if="isCbsAdjustRow">-</TextPlainColumn>
      <InputColumn v-else ref="osRef" v-model="localData.remained_os" type="number" :min="1" :rules="osRules" />
    </td>

    <!-- 派生列：CBS 行不适用显示 "-"，未算出结果时按设计稿显示占位色的 0 -->
    <td :class="cellClass('remained_cpu_core')" v-bk-tooltips="originTip('remained_cpu_core')">
      <TextPlainColumn>
        <span v-if="isCbsOnly">-</span>
        <span v-else :class="{ 'is-empty': !localData.remained_cpu_core }">{{ localData.remained_cpu_core || 0 }}</span>
      </TextPlainColumn>
    </td>

    <td :class="cellClass('remained_memory')" v-bk-tooltips="originTip('remained_memory')">
      <TextPlainColumn>
        <span v-if="isCbsOnly">-</span>
        <span v-else :class="{ 'is-empty': !localData.remained_memory }">{{ localData.remained_memory || 0 }}</span>
      </TextPlainColumn>
    </td>

    <td :class="cellClass('disk_type')" v-bk-tooltips="originTip('disk_type_name')">
      <TextPlainColumn v-if="hasNoDisk || !selectsReady">
        {{ hasNoDisk ? '-' : localData.disk_type_name || localData.disk_type || '-' }}
      </TextPlainColumn>
      <SelectColumn
        v-else
        ref="diskTypeRef"
        v-model="diskTypeModel"
        :auto-focus="false"
        :list="diskTypeSelectList"
        :loading="optionsLoading"
        filterable
        clearable
        :rules="diskTypeRules"
      />
    </td>

    <!-- 现网该字段在 CBS 下叫「云磁盘容量/块」，表头无法按行切换，改由新增 CBS 行的 tooltip 说明 -->
    <td
      :class="cellClass('disk_per_size')"
      v-bk-tooltips="
        isCbsOnly && !isCbsAdjustRow ? { content: 'CBS 预测在此填写单块云盘容量' } : originTip('disk_per_size')
      "
    >
      <TextPlainColumn v-if="isCbsAdjustRow || hasNoDisk">-</TextPlainColumn>
      <InputColumn
        v-else
        ref="diskPerSizeRef"
        v-model="localData.disk_per_size"
        type="number"
        :min="0"
        :rules="diskPerSizeRules"
      />
    </td>

    <!-- 现网「编辑CBS时仅可调整云盘总量」：存量 CBS 行没有块数可依，改为直接填写 -->
    <td :class="cellClass('remained_disk_size')" v-bk-tooltips="originTip('remained_disk_size')">
      <TextPlainColumn v-if="hasNoDisk">-</TextPlainColumn>
      <InputColumn
        v-else-if="isCbsAdjustRow"
        ref="diskSizeRef"
        v-model="localData.remained_disk_size"
        type="number"
        :min="0"
        :rules="[{ validator: (val: number) => Number(val) > 0, message: '云盘总量应大于0' }]"
      />
      <TextPlainColumn v-else>
        <span :class="{ 'is-empty': !localData.remained_disk_size }">{{ localData.remained_disk_size || 0 }}</span>
      </TextPlainColumn>
    </td>

    <td :class="cellClass('disk_io')" v-bk-tooltips="originTip('disk_io')">
      <TextPlainColumn v-if="hasNoDisk">-</TextPlainColumn>
      <InputColumn
        v-else
        ref="diskIoRef"
        v-model="localData.disk_io"
        type="number"
        :min="0"
        :max="diskIoMax"
        :disabled="isReadonly('disk_io')"
        :rules="[{ validator: (val: number) => Number(val) > 0, message: '单实例磁盘IO应大于0' }]"
      />
    </td>

    <td :class="cellClass('return_plan_time')" v-bk-tooltips="returnTimeTip">
      <DateTimePickerColumn
        ref="returnTimeRef"
        v-model="localData.return_plan_time"
        type="date"
        format="yyyy-MM-dd"
        :clearable="false"
        :disabled="isReadonly('return_plan_time')"
        :disabled-date="isReturnDateDisabled"
        :rules="returnPlanTimeRules"
      >
        <template #footer>
          <div class="return-time-picker-shortcuts">
            <div v-for="month in RETURN_TIME_SHORTCUT_MONTHS" :key="month" @click="setReturnTimeByMonth(month)">
              {{ month }}个月
            </div>
          </div>
        </template>
      </DateTimePickerColumn>
    </td>

    <!-- 存量行的变更原因跟随原单据，只读展示；list 暂不返回该字段时为空，显示 "-" -->
    <td :class="cellClass('demand_source')">
      <TextPlainColumn v-if="!localData.is_new || !selectsReady">
        {{ localData.demand_source || '-' }}
      </TextPlainColumn>
      <SelectColumn
        v-else
        ref="demandSourceRef"
        v-model="demandSourceModel"
        :auto-focus="false"
        :list="demandSourceOptions"
        :clearable="false"
        :loading="optionsLoading"
      />
    </td>

    <AdjustOperationColumn
      :removeable="removeable"
      show-copy
      show-add
      show-remove
      @copy="handleCopy"
      @add="handleAdd"
      @remove="handleRemove"
    />
  </tr>
</template>

<style scoped lang="scss">
.adjust-cell {
  // 设计稿表格正文统一 #313238（含只读与禁用态），ediatable / bkui 默认是次级文字色 #63656e。
  // 由 td 定色再让各类控件继承，省得逐个组件覆盖；占位符走 ::placeholder，不受影响。
  color: #313238;

  :deep(.bk-ediatable-text-plain),
  :deep(.bk-ediatable-select),
  :deep(input) {
    color: inherit;
  }

  // 单元格底色只由 td 决定：只读灰底、可编辑白底（设计稿 #fafbfc / #ffffff）。
  // ediatable 的 td 与 SelectColumn 都不画背景（input 有 #fff、固定列有 #fff），
  // 不铺这层白底，下拉列就会透出页面底色。
  background-color: #fff;

  // 高亮写在后面，同优先级下压过只读，保证派生列联动变化时也能看到提示。
  &.is-readonly {
    background-color: #fafbfd;
  }

  &.is-changed {
    background-color: #fdf4e8;
  }

  &.is-new {
    background-color: #ebfaf0;
  }

  // TextPlainColumn 的灰底来自 .default-display（未传 data 的占位态），语义不是只读，
  // 交还给 td 统一控制，否则会盖住高亮。
  :deep(.bk-ediatable-text-plain.default-display) {
    background: transparent;
  }

  // 派生值尚未算出时按设计稿走占位色
  .is-empty {
    color: #c4c6cc;
  }

  // ediatable / bkui 的输入框自带不透明底色（hover 与 disabled 均为 #fafbfd），会盖住高亮。
  // 高亮态下一律放开，只把错误态交回组件自身；只改背景，hover 的边框反馈保留。
  &.is-changed,
  &.is-new {
    :deep(.bk-ediatable-select:not(.is-error)),
    :deep(.bk-ediatable-input:not(.is-error)),
    :deep(.bk-ediatable-input:not(.is-error) .input-box input),
    :deep(.bk-ediatable-time-picker:not(.is-error) .bk-date-picker-editor) {
      background-color: transparent;
    }
  }
}

// 日期面板 footer；面板 append-to-body，但插槽内容带的是本组件 scope id
.expect-time-picker-footer {
  // 单日期面板宽度由日历格子撑出，dropdown 取子元素 max-content。
  // width: 0 让提示不参与固有宽度计算，再用 min-width 回填，避免长文案把面板撑宽。
  box-sizing: border-box;
  width: 0;
  min-width: 100%;
  padding: 8px 12px;
  font-size: 12px;
  line-height: 20px;
  color: #63656e;

  .time-txt {
    margin: 0 2px;
    font-size: 11px;
    color: #ea3636;
  }

  .review-phase-tip {
    margin-top: 4px;
    line-height: 16px;
    color: #979ba5;
  }
}

// 与现网 .date-range-btns 一致
.return-time-picker-shortcuts {
  display: flex;
  align-items: center;
  justify-content: space-around;
  color: #5594fa;

  > div {
    padding: 10px 0;
    cursor: pointer;
  }
}

.is-new-row {
  td {
    background-color: #ebfaf0;
  }
}

.expect-time-cell {
  position: relative;

  &__tip {
    position: absolute;
    top: 50%;
    right: 12px;

    // bkui 图标的 svg 尺寸写死为 1em，只能用 font-size 锁成设计稿的 14×14
    font-size: 14px;
    color: #979ba5;
    cursor: pointer;
    transform: translateY(-50%);
  }

  // 给右侧提示图标让位，避免日期文本被压住
  &.has-tip :deep(.bk-date-picker-editor) {
    padding-right: 32px;
  }
}

// 下拉类单元格的右侧 ⓘ 补充说明（项目类型的特殊类型说明、机型规格的机型信息）
.select-tip-cell {
  position: relative;

  // 下拉箭头是 right: 4px 的 20px 方块，提示图标排在它左侧
  &__tip {
    position: absolute;
    top: 50%;
    right: 28px;

    // bkui 图标的 svg 尺寸写死为 1em，只能用 font-size 锁成 14×14
    font-size: 14px;
    color: #979ba5;
    cursor: pointer;
    transform: translateY(-50%);
  }

  // ediatable 的校验错误图标同样定位在右侧，出错时让位给它
  :deep(.bk-ediatable-select.is-error) + &__tip {
    display: none;
  }

  // 给下拉箭头 + 提示图标让位，避免选中项文本被压住
  &.has-tip :deep(.bk-select input) {
    padding-right: 46px;
  }
}
</style>

<!-- 日期面板 append-to-body，scoped 选择器无法命中，靠 ext-popover-cls 限定作用域 -->
<style lang="scss">
.expect-time-picker-dropdown {
  // datetime 仅用于开出确认栏，实际不选时间，隐藏面板自带的「时间」切换入口
  .bk-picker-confirm .bk-picker-confirm-time {
    display: none;
  }

  // footer 在 DOM 里排在 confirm 下方；把「13周后」抬到确定行居中（confirm 高 42px）
  .bk-date-picker-footer-wrapper {
    position: relative;
  }

  .expect-time-thirteen-weeks {
    position: absolute;
    top: -42px;
    left: 50%;
    z-index: 2;
    height: 42px;
    padding: 0;
    color: #5594fa;
    line-height: 42px;
    white-space: nowrap;
    cursor: pointer;
    transform: translateX(-50%);
  }
}
</style>
