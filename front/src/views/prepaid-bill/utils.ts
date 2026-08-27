import dayjs from 'dayjs';

export const formatOrderMonth = (year?: number, month?: number) => {
  if (!year || !month) return '--';
  return `${year}-${String(month).padStart(2, '0')}`;
};

export const formatOrderMonthFromAt = (orderAt?: string) => {
  if (!orderAt) return '--';
  const parsed = dayjs(orderAt);
  if (!parsed.isValid()) return '--';
  return formatOrderMonth(parsed.year(), parsed.month() + 1);
};

export const formatBillPeriod = (year?: number, month?: number) => formatOrderMonth(year, month);

export const formatAmount = (value: string | number | undefined | null) => {
  if (value === null || value === undefined || value === '') return '--';
  const num = Number(value);
  if (Number.isNaN(num)) return String(value);
  return num.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
};

export const isInvalidDateRange = (value: unknown) => {
  if (!Array.isArray(value) || !value[0] || !value[1]) return false;
  return dayjs(value[0]).isAfter(dayjs(value[1]));
};
