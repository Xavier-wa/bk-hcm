import http from '@/http';
import { resolveBizApiPath } from '@/utils/search';
import type { IListResData, IQueryResData } from '@/typings';

export interface SessionApiItem {
  id: string;
  session_code: string;
  session_name: string;
  thread_id: string;
  is_temporary: boolean;
  session_content_count: number;
  session_tag?: string;
  bk_biz_id?: number;
  created_at: string;
  updated_at: string;
}

interface CreateSessionApiResponse {
  id: string;
  session_code: string;
  thread_id: string;
  session_name: string;
  session_tag?: string;
  bk_biz_id?: number;
}

const AGENT_API_PREFIX = '/api/v1/agent';

const sessionsPrefix = (bkBizId: number) => `${AGENT_API_PREFIX}/${resolveBizApiPath(bkBizId)}sessions`;

export const listSessions = async (bkBizId: number, sessionTag = '', start = 0, limit = 100) => {
  // sessionTag 为场景标识（如 host_apply）：按场景加载会话列表；为空则不过滤，加载全部
  const rules = sessionTag ? [{ field: 'session_tag', op: 'eq', value: sessionTag }] : [];
  const res: IListResData<SessionApiItem[]> = await http.post(`${sessionsPrefix(bkBizId)}/list`, {
    filter: { op: 'and', rules },
    page: { count: false, start, limit, sort: 'updated_at', order: 'DESC' },
  });
  return res.data;
};

export const createSession = async (bkBizId: number, sessionName = '', sessionTag = '') => {
  const res: IQueryResData<CreateSessionApiResponse> = await http.post(`${sessionsPrefix(bkBizId)}/create`, {
    session_name: sessionName,
    // session_tag 为场景标识，按场景创建会话时携带；普通新对话传空
    ...(sessionTag ? { session_tag: sessionTag } : {}),
  });
  return res.data;
};

export const deleteSession = async (bkBizId: number, sessionCode: string) => {
  await http.delete(`${sessionsPrefix(bkBizId)}/${sessionCode}`);
};

export const updateSession = async (bkBizId: number, sessionCode: string, sessionName: string) => {
  await http.patch(`${sessionsPrefix(bkBizId)}/${sessionCode}`, { session_name: sessionName });
};
