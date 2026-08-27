import type { IBusinessItem } from '@/store/business-global';

/**
 * 机房裁撤模块的"未知"业务约定：
 * 后端接口中 bk_biz_id === 0 统一标识无法匹配业务名称的数据行，
 * 其他未匹配的 bk_biz_id 不属于"未知"，展示保持 '--'
 */
export const UNKNOWN_BIZ_ID = 0;
export const UNKNOWN_BIZ_NAME = '未知';
export const UNKNOWN_BIZ_OPTION: IBusinessItem = { id: UNKNOWN_BIZ_ID, name: UNKNOWN_BIZ_NAME };
