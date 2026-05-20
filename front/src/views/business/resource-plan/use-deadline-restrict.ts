import { ref, computed, onMounted, unref, type MaybeRefOrGetter, type Ref, type ComputedRef } from 'vue';
import { useResourcePlanStore } from '@/store';
import { NON_CURRENT_YEAR_START_DATE } from './constants';

// 取日期字符串 (YYYY-MM-DD), 客户端本地时区, 不引入 dayjs
const formatDate = (d: Date): string => {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
};

// 取当前时间字符串 (YYYY-MM-DD HH:MM:SS), 客户端本地时区, 用于与 deadline (含时分秒) 比较
const formatDateTime = (d: Date): string => {
  const datePart = formatDate(d);
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  return `${datePart} ${hh}:${mm}:${ss}`;
};

const now = (): string => formatDateTime(new Date());

interface UseDeadlineRestrictReturn {
  deadline: Ref<string>;
  isInReviewPhase: ComputedRef<boolean>;
  // DatePicker disabledDate 钩子; 评审期内 ≥ 起始日的候选日期禁选
  isDateDisabled: (date: Date) => boolean;
  // 列表行级判定: 给定 expect_time (YYYY-MM-DD) 是否应锁定操作
  shouldDisableRow: (expectTime: string) => boolean;
  // 起始日年份字符串, 用于 UI 文案动态拼接 (常量为空时返回 '')
  nonCurrentYearLabel: string;
}

// 截止期限制聚合 Hook
//   - 内部自动 onMounted 拉取 report_deadline (失败按申报期兜底, 不弹 toast)
//   - 空值兜底: deadline 或起始日常量任一为空 → 视为申报期, 不限制
//   - enabled 传入 false 时, 不拉取接口, 所有判定函数恒返回不限制 (用于「修改单据入口/修改页」场景)
//   - 添加页与列表均通过此 Hook 拿到统一的 isInReviewPhase / isDateDisabled / shouldDisableRow
export default function useDeadlineRestrict(enabled?: MaybeRefOrGetter<boolean>): UseDeadlineRestrictReturn {
  const store = useResourcePlanStore();
  const deadline = ref('');
  const startDate = NON_CURRENT_YEAR_START_DATE;

  const isEnabled = computed(() => {
    if (enabled === undefined) return true;
    return typeof enabled === 'function' ? enabled() : unref(enabled);
  });

  onMounted(async () => {
    if (!isEnabled.value) return;
    try {
      const { data } = await store.getReportDeadline();
      // 保留后端原值 (含时分秒, 如 '2026-05-22 18:00:00'), 后续与当前时间精确到秒比较
      deadline.value = data?.deadline || '';
    } catch (err) {
      // 失败兜底: 按申报期处理, 控制台 warn 仅用于排查, 不打扰用户
      console.warn('[resource-plan] getReportDeadline failed', err);
      deadline.value = '';
    }
  });

  const isInReviewPhase = computed(() => {
    if (!isEnabled.value) return false;
    if (!deadline.value) return false;
    if (!startDate) return false;
    // 严格大于: 过了 deadline 那一刻才进入评审期 (精确到秒)
    // 后端约定 deadline 返回格式 'YYYY-MM-DD HH:MM:SS', 定长字符串字典序 === 时序, 无需 dayjs 转换
    return now() > deadline.value;
  });

  const isDateDisabled = (date: Date): boolean => {
    if (!isInReviewPhase.value) return false;
    return formatDate(date) >= startDate;
  };

  const shouldDisableRow = (expectTime: string): boolean => {
    if (!isInReviewPhase.value) return false;
    if (!expectTime) return false;
    return expectTime >= startDate;
  };

  return {
    deadline,
    isInReviewPhase,
    isDateDisabled,
    shouldDisableRow,
    nonCurrentYearLabel: startDate.slice(0, 4),
  };
}
