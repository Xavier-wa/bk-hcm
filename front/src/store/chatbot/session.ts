import http from '@/http';
import type { IListResData, IQueryResData } from '@/typings';

export interface SessionApiItem {
  id: string;
  session_code: string;
  session_name: string;
  thread_id: string;
  is_temporary: boolean;
  session_content_count: number;
  created_at: string;
  updated_at: string;
}

interface CreateSessionApiResponse {
  id: string;
  session_code: string;
  thread_id: string;
  session_name: string;
}

const API_PREFIX = '/api/v1/agent/sessions';

export const listSessions = async (start = 0, limit = 100) => {
  const res: IListResData<SessionApiItem[]> = await http.post(`${API_PREFIX}/list`, {
    filter: { op: 'and', rules: [] },
    page: { count: false, start, limit, sort: 'updated_at', order: 'DESC' },
  });
  return res.data;
};

export const createSession = async (sessionName = '') => {
  const res: IQueryResData<CreateSessionApiResponse> = await http.post(`${API_PREFIX}/create`, {
    session_name: sessionName,
  });
  return res.data;
};

export const deleteSession = async (sessionCode: string) => {
  await http.delete(`${API_PREFIX}/${sessionCode}`);
};

export const updateSession = async (sessionCode: string, sessionName: string) => {
  await http.patch(`${API_PREFIX}/${sessionCode}`, { session_name: sessionName });
};
