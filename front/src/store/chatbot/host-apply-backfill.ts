import { defineStore } from 'pinia';

import { type HostApplySuborder } from '@/hooks/chatbot/types';

// 浮窗（floating）chatbot 与同页 ApplicationForm 的「添加到配置清单」同页传参通道：
// 浮窗里点击「添加到配置清单」时 pushBackfill 写入，表单页 watch 到后消费并追加到配置清单。
// 仅用于浮窗同页场景；全页 chatbot 走新标签页 + URL query，不经此 store。
export const useHostApplyBackfillStore = defineStore('hostApplyBackfill', {
  state: () => ({
    pending: [] as HostApplySuborder[],
    // 聊天中前置选择的云账号 id，供申领页预选同一账号。consume 仅清 pending，不清此值（账号在页面侧消费）
    accountId: '',
  }),
  actions: {
    // 浮窗侧写入待追加的 suborder 列表（每次点击均为新数组引用，触发表单页 watch）；同时带上所选账号 id
    pushBackfill(suborders: HostApplySuborder[], accountId = '') {
      this.pending = [...suborders];
      this.accountId = accountId;
    },
    // 表单页取回并清空 suborder 列表
    consume(): HostApplySuborder[] {
      const data = this.pending;
      this.pending = [];
      return data;
    },
  },
});
