// 非本年预测起始日 (YYYY-MM-DD); 期望到货日期 >= 此日期即视为「非本年预测」
// 空字符串 '' 视为不做区间限制 (语义与 report_deadline 接口空值一致)
// 后续年度切换或评审完成后需要解除限制时, 直接修改本常量
export const NON_CURRENT_YEAR_START_DATE = '2027-01-01';
