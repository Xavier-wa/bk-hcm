import { onBeforeUnmount, onMounted } from 'vue';
import { useContainerScrollConsumer } from '@blueking/chat-x';
import { throttle } from 'lodash';

// 与 chat-x Markdown 的跟随节流一致
const FOLLOW_THROTTLE_MS = 100;

/**
 * 让手写的消息内容也能跟着流式输出贴底。
 *
 * chat-x 的自动滚动上下文由 message-container 用 useContainerScrollProvider 提供，但「持续跟随」不是容器
 * 主动做的：容器只在挂载时 jumpToBottom 两次，之后靠 Markdown 渲染器每挂载一个 token 就回调一次
 * toScrollBottom。过程区（思考正文 / 工具行）是手写结构、不走 Markdown 渲染，因此没有任何一处触发跟随，
 * 流式输出会长到可视区外。这里把同一个 toScrollBottom 补上，节流与触发时机对齐 Markdown。
 *
 * 用户上滚翻历史时 chat-x 会把 autoScrollEnabled 置 false，此时不抢滚动。
 */
export const useFollowScroll = () => {
  const containerScroll = useContainerScrollConsumer();

  const follow = throttle(
    () => {
      const scroll = containerScroll?.value;
      if (!scroll || scroll.autoScrollEnabled === false) return;
      // 不传 behavior：由 chat-x 按距底距离决定瞬时贴底还是平滑跟随
      scroll.toScrollBottom();
    },
    FOLLOW_THROTTLE_MS,
    { leading: true, trailing: true },
  );

  onBeforeUnmount(() => follow.cancel());

  return follow;
};

/**
 * 场景卡片挂载时贴一次底。
 *
 * 与过程区同一类问题：卡片是手写结构、不走 Markdown 渲染，chat-x 不会为它触发跟随。而这些卡（申领方案 /
 * 预提单 / 确认提交 / HITL / 账号选择）多在一轮末尾才由 CUSTOM 事件下发，挂载后整块内容长出可视区却没有
 * 任何一处贴底，就停在半路——表现为本轮结束后没滚到底、卡片被截断、「返回底部」按钮亮着。
 *
 * 只挂 onMounted：卡片高度在挂载时已定（数据来自 props，不异步取），无需再跟内容变化。
 */
export const useFollowScrollOnMount = () => {
  const follow = useFollowScroll();
  onMounted(() => follow());
};
