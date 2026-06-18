import { type Ref } from 'vue';

import { MessageRole, type Message } from '@blueking/chat-x';

import {
  type HostApplyPreorderMessage,
  type HostApplyPreorderValue,
  type HostApplyRecommendMessage,
  type HostApplyRecommendValue,
  type HostApplySuborder,
  type HostApplySubmitMessage,
  type HostApplySubmitValue,
} from './types';

export interface HostApplyReadonlyState {
  readonly: boolean;
}

// useHostApply 提供主机申领指令模板（推荐方案 / 预提单）消息的识别、内容提取与只读态推断，供全页与浮窗复用。
// 只读态推断对齐 useAccountSelect：存在显式提交标记或后续 user 消息（已提交）→ 只读；否则保持可交互。
export const useHostApply = (messages: Ref<Message[]>) => {
  const hasFollowingUserMessage = (message: Message): boolean => {
    const currentIndex = messages.value.findIndex((item) => item.id === message.id);
    if (currentIndex < 0) return false;
    const nextMessage = messages.value[currentIndex + 1];
    return !!nextMessage && nextMessage.role === MessageRole.User;
  };

  // ---- 模板 A：申领方案推荐 ----
  const isRecommendMessage = (message: Message): boolean =>
    (message as HostApplyRecommendMessage).__type === 'host_apply.recommend';

  const getRecommendContent = (message: Message): HostApplyRecommendValue | null => {
    if (!isRecommendMessage(message)) return null;
    return (message as HostApplyRecommendMessage).content as HostApplyRecommendValue;
  };

  const getRecommendReadonlyState = (message: Message): HostApplyReadonlyState => {
    if (!isRecommendMessage(message)) return { readonly: false };
    const explicit = (message as HostApplyRecommendMessage).__selectedIndex !== undefined;
    return { readonly: explicit || hasFollowingUserMessage(message) };
  };

  // 已选方案下标；未选（含历史回放无该字段）返回 -1，由卡片退化为首条
  const getSelectedIndex = (message: Message): number => (message as HostApplyRecommendMessage).__selectedIndex ?? -1;

  // ---- 模板 B：预提单数据 ----
  const isPreorderMessage = (message: Message): boolean =>
    (message as HostApplyPreorderMessage).__type === 'host_apply.preorder';

  const getPreorderContent = (message: Message): HostApplyPreorderValue | null => {
    if (!isPreorderMessage(message)) return null;
    return (message as HostApplyPreorderMessage).content as HostApplyPreorderValue;
  };

  const getPreorderReadonlyState = (message: Message): HostApplyReadonlyState => {
    if (!isPreorderMessage(message)) return { readonly: false };
    const explicit = !!(message as HostApplyPreorderMessage).__confirmedSuborders;
    return { readonly: explicit || hasFollowingUserMessage(message) };
  };

  // 只读态优先展示确认时的 suborders（含 C 弹窗修改）；历史回放无该字段时退化为原始 suborders
  const getPreorderReadonlySuborders = (message: Message): HostApplySuborder[] => {
    const confirmed = (message as HostApplyPreorderMessage).__confirmedSuborders;
    if (confirmed) return confirmed;
    return getPreorderContent(message)?.value.suborders ?? [];
  };

  // ---- 模板 D：确认提交申请单 ----
  const isSubmitMessage = (message: Message): boolean =>
    (message as HostApplySubmitMessage).__type === 'host_apply.submit';

  const getSubmitContent = (message: Message): HostApplySubmitValue | null => {
    if (!isSubmitMessage(message)) return null;
    return (message as HostApplySubmitMessage).content as HostApplySubmitValue;
  };

  const getSubmitReadonlyState = (message: Message): HostApplyReadonlyState => {
    if (!isSubmitMessage(message)) return { readonly: false };
    const explicit = !!(message as HostApplySubmitMessage).__submitted;
    return { readonly: explicit || hasFollowingUserMessage(message) };
  };

  // 把提交确认的嵌套 suborders 拍平为表格行：合并 body_param.require_type（全局共用）+ 外层 replicas + spec
  const getSubmitRows = (message: Message): HostApplySuborder[] => {
    const content = getSubmitContent(message);
    if (!content) return [];
    const { require_type, suborders } = content.value.data.body_param;
    return suborders.map((sub) => ({
      require_type,
      region: sub.spec.region,
      zone: sub.spec.zone ?? (Array.isArray(sub.spec.zones) ? sub.spec.zones.join('、') : undefined),
      device_type: sub.spec.device_type,
      image_id: sub.spec.image_id,
      res_assign: sub.spec.res_assign,
      replicas: sub.replicas,
      charge_type: sub.spec.charge_type,
      system_disk: sub.spec.system_disk,
      data_disk: sub.spec.data_disk,
    }));
  };

  return {
    isRecommendMessage,
    getRecommendContent,
    getRecommendReadonlyState,
    getSelectedIndex,
    isPreorderMessage,
    getPreorderContent,
    getPreorderReadonlyState,
    getPreorderReadonlySuborders,
    isSubmitMessage,
    getSubmitContent,
    getSubmitReadonlyState,
    getSubmitRows,
  };
};
