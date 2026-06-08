import { SESSION_TAG_NAME } from './constants';

// 将 session_tag 原始值映射为展示名，未命中回退原值
export const resolveSessionTagName = (tag: string): string => SESSION_TAG_NAME[tag] || tag;
