import http, { getCommonHeaders } from '@/http';

const getBaseUrl = () => window.PROJECT_CONFIG.BK_HCM_AJAX_URL_PREFIX || '';

const sseHeaders = (): Record<string, string> => getCommonHeaders();

export const streamChat = (sessionCode: string, messages: { role: string; content: string }[], signal?: AbortSignal) =>
  fetch(`${getBaseUrl()}/api/v1/agent/agui`, {
    method: 'POST',
    headers: sseHeaders(),
    credentials: 'include',
    signal,
    body: JSON.stringify({ sessionCode, messages }),
  });

export const fetchHistoryStream = (sessionCode: string, signal?: AbortSignal) =>
  fetch(`${getBaseUrl()}/api/v1/agent/history`, {
    method: 'POST',
    headers: sseHeaders(),
    credentials: 'include',
    signal,
    body: JSON.stringify({ sessionCode }),
  });

export const cancelRun = async (sessionCode: string) => {
  try {
    await http.post('/api/v1/agent/cancel', { sessionCode }, { globalError: false });
  } catch {
    /* no active run, safe to ignore */
  }
};
