import Cookies from 'js-cookie';

import http from '@/http';

const getBaseUrl = () => window.PROJECT_CONFIG.BK_HCM_AJAX_URL_PREFIX || '';

const getCsrfToken = () => Cookies.get(`${window.PROJECT_CONFIG.BKPAAS_APP_ID}_csrftoken`) || '';

const sseHeaders = (): Record<string, string> => ({
  'Content-Type': 'application/json',
  'X-CSRFToken': getCsrfToken(),
  'X-REQUESTED-WITH': 'XMLHttpRequest',
});

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
