import { onMounted, ref, type Ref } from 'vue';
import type { IToolBtn, Message } from '@blueking/chat-x';

import * as feedbackApi from '@/store/chatbot/feedback';
import type { AgentFeedbackReaction } from '@/store/chatbot/feedback';
import { Message as bkMessage } from 'bkui-vue';

import {
  clipFeedbackComment,
  findActiveFeedbackButton,
  findGroupMessagesFromFeedbackButton,
  getGroupRunId,
  invertTagMap,
  labelsToTagKeys,
  toolIdToReaction,
  type FeedbackMessageGroup,
} from './agent-feedback';

interface UseAgentFeedbackOpts {
  getBkBizId: () => number;
  getSessionId: () => string;
  messageGroups: Ref<FeedbackMessageGroup[]>;
}

const emptyTagMaps = (): Record<AgentFeedbackReaction, Record<string, string>> => ({
  like: {},
  dislike: {},
});

export function useAgentFeedback(opts: UseAgentFeedbackOpts) {
  const tagMaps = ref(emptyTagMaps());
  const deletingRunIds = new Set<string>();
  let configLoaded = false;
  let configLoading: Promise<void> | null = null;

  const resolveBkBizId = () => {
    const bkBizId = opts.getBkBizId();
    return Number.isFinite(bkBizId) && bkBizId > 0 ? bkBizId : 0;
  };

  const loadTagConfig = async () => {
    if (configLoaded) return;
    if (configLoading) {
      await configLoading;
      return;
    }
    const bkBizId = resolveBkBizId();
    if (!bkBizId) return;

    configLoading = (async () => {
      try {
        const details = await feedbackApi.listAgentFeedbackTags(bkBizId);
        const next = emptyTagMaps();
        details.forEach((item) => {
          if (item.config_key === 'like' || item.config_key === 'dislike') {
            next[item.config_key] = item.config_value && typeof item.config_value === 'object' ? item.config_value : {};
          }
        });
        tagMaps.value = next;
        configLoaded = true;
      } catch {
        tagMaps.value = emptyTagMaps();
      } finally {
        configLoading = null;
      }
    })();
    await configLoading;
  };

  const chipLabels = (reaction: AgentFeedbackReaction): string[] => Object.values(tagMaps.value[reaction]);

  const resolveContext = (msgs: Message[]) => {
    const bkBizId = resolveBkBizId();
    const sessionId = opts.getSessionId();
    const runId = getGroupRunId(msgs);
    return { bkBizId, sessionId, runId };
  };

  const deleteCommittedFeedback = async (msgs: Message[]) => {
    const { bkBizId, sessionId, runId } = resolveContext(msgs);
    if (!bkBizId || !sessionId || !runId || deletingRunIds.has(runId)) return;
    deletingRunIds.add(runId);
    try {
      await feedbackApi.deleteFeedback(bkBizId, runId, sessionId);
    } catch {
      /* http 已全局提示 */
    } finally {
      deletingRunIds.delete(runId);
    }
  };

  const handleLikeUnlikeAction = async (tool: IToolBtn, _msgs: Message[]): Promise<string[]> => {
    const reaction = toolIdToReaction(tool.id);
    if (!reaction) return [];
    await loadTagConfig();
    return chipLabels(reaction);
  };

  const handleLikeUnlikeFeedback = (tool: IToolBtn, msgs: Message[], reasonList: string[], otherReason: string) => {
    const reaction = toolIdToReaction(tool.id);
    if (!reaction) return;

    const { bkBizId, sessionId, runId } = resolveContext(msgs);
    if (!bkBizId || !sessionId) {
      bkMessage({ theme: 'error', message: '无法提交反馈：缺少业务或会话信息' });
      return;
    }
    if (!runId) {
      bkMessage({ theme: 'error', message: '无法提交反馈：缺少本轮对话标识' });
      return;
    }

    const tags = labelsToTagKeys(invertTagMap(tagMaps.value[reaction]), reasonList);
    const comment = clipFeedbackComment(otherReason.trim());

    void (async () => {
      try {
        await feedbackApi.commitFeedback(bkBizId, {
          session_id: sessionId,
          run_id: runId,
          reaction,
          ...(tags.length ? { tags } : {}),
          ...(comment ? { comment } : {}),
        });
        bkMessage({ theme: 'success', message: '已提交反馈' });
      } catch {
        /* http 已全局提示 */
      }
    })();
  };

  // chat-x 再点已选中 👍/👎：Tippy onShow 取消激活并 return false，不发 feedback，也不保证走 onAction。
  // 在捕获阶段看 is-active（此时尚未被 onShow 清掉）发 DELETE。
  const handleFeedbackCancelClick = (ev: MouseEvent) => {
    const btn = findActiveFeedbackButton(ev.target);
    if (!btn) return;
    const msgs = findGroupMessagesFromFeedbackButton(btn, opts.messageGroups.value);
    if (!msgs.length) return;
    void deleteCommittedFeedback(msgs);
  };

  onMounted(() => {
    void loadTagConfig();
  });

  return {
    handleLikeUnlikeAction,
    handleLikeUnlikeFeedback,
    handleFeedbackCancelClick,
  };
}
