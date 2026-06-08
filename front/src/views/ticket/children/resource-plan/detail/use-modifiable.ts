import { computed, type ComputedRef, type Ref } from 'vue';
import type { TicketByIdResult, TicketStatus } from '@/typings/resourcePlan';
import type { SubTicketItem } from '@/store/ticket/res-sub-ticket';

// 后端可覆盖修改的主单状态白名单, 与 MR 3093 对齐
const OVERWRITABLE_MAIN_STATUS: ReadonlySet<TicketStatus> = new Set<TicketStatus>([
  'rejected',
  'partial_rejected',
  'failed',
  'partial_failed',
  'revoked',
]);

// 判断资源预测单据 (CVM / CA) 是否可修改: 业务视角 + 主单状态白名单 + 无 done 子单
export default function useTicketModifiable(
  ticketDetail: Ref<TicketByIdResult | undefined>,
  subTickets: Ref<SubTicketItem[] | undefined>,
  isBusinessPage: boolean,
): ComputedRef<boolean> {
  return computed(() => {
    if (!isBusinessPage) return false;

    const detail = ticketDetail.value;
    if (!detail) return false;

    if (!OVERWRITABLE_MAIN_STATUS.has(detail.status_info?.status)) return false;

    // 子单未加载 (undefined) 与空数组 [] 均视为无 done 子单, 允许修改
    if (subTickets.value?.some((s) => s.status === 'done')) return false;

    return true;
  });
}
