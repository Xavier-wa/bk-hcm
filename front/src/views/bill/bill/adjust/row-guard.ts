import { AdjustmentListRow } from '@/typings/bill';

export const PREPAID_ADJUST_LOCK_TIP = 'OFS 预付费调账不可人工修改。';

// canMutateAdjustmentRow reports whether row edit/delete should be enabled.
export const canMutateAdjustmentRow = (row: AdjustmentListRow) => {
  if (row.source === 'prepaid') return false;
  if (row.source !== 'manual') return row.state === 'unconfirmed';
  if (row.push_status === 'pushing') return false;
  if (row.settle_state === 'settled') return false;
  return true;
};

// canSelectAdjustmentRow reports whether the row can join batch confirm/delete.
export const canSelectAdjustmentRow = (row: AdjustmentListRow) => {
  if (row.state !== 'unconfirmed') return false;
  if (row.source === 'prepaid') return false;
  if (row.push_status === 'pushing') return false;
  if (row.settle_state === 'settled') return false;
  return true;
};

// getAdjustmentMutateDisableTip returns tooltip text for a disabled edit/delete button.
export const getAdjustmentMutateDisableTip = (row: AdjustmentListRow, action: 'edit' | 'delete') => {
  if (row.source === 'prepaid') return PREPAID_ADJUST_LOCK_TIP;
  if (row.push_status === 'pushing') {
    return action === 'edit' ? '当前调账单推送中，无法编辑' : '当前调账单推送中，无法删除';
  }
  if (row.settle_state === 'settled') {
    return action === 'edit' ? '当前调账单已定账，无法编辑' : '当前调账单已定账，无法删除';
  }
  return action === 'edit' ? '当前调账单已确认，无法编辑' : '当前调账单已确认，无法删除';
};
