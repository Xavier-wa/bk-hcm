import http from '@/http';
import type { IQueryResData } from '@/typings';

const AGENT_API_PREFIX = '/api/v1/agent';
export const AGENT_FEEDBACK_TAG_CONFIG_TYPE = 'agent_feedback_tag';

export type AgentFeedbackReaction = 'like' | 'dislike';

export interface CommitFeedbackReq {
  session_id: string;
  run_id: string;
  reaction: AgentFeedbackReaction;
  tags?: string[];
  comment?: string;
}

export interface CommitFeedbackResp {
  id: string;
}

export interface AgentFeedbackItem {
  run_id: string;
  reaction: AgentFeedbackReaction;
  tags: string[];
  comment: string;
}

export interface AgentConfigItem {
  config_key: string;
  config_value: Record<string, string>;
}

const bizPrefix = (bkBizId: number) => `${AGENT_API_PREFIX}/bizs/${bkBizId}`;

export const commitFeedback = async (bkBizId: number, payload: CommitFeedbackReq) => {
  const res: IQueryResData<CommitFeedbackResp> = await http.post(`${bizPrefix(bkBizId)}/feedback/commit`, payload);
  return res.data;
};

export const deleteFeedback = async (bkBizId: number, runId: string, sessionId: string) => {
  await http.delete(
    `${bizPrefix(bkBizId)}/feedback/${encodeURIComponent(runId)}?session_id=${encodeURIComponent(sessionId)}`,
    {},
  );
};

export const listFeedback = async (bkBizId: number, sessionId: string) => {
  const res: IQueryResData<{ details: AgentFeedbackItem[] }> = await http.get(`${bizPrefix(bkBizId)}/feedback/list`, {
    params: { session_id: sessionId },
  });
  return res.data?.details ?? [];
};

export const listAgentFeedbackTags = async (bkBizId: number) => {
  const res: IQueryResData<{ details: AgentConfigItem[] }> = await http.get(`${bizPrefix(bkBizId)}/config/list`, {
    params: { config_type: AGENT_FEEDBACK_TAG_CONFIG_TYPE },
  });
  return res.data?.details ?? [];
};
