import http, { getCommonHeaders } from '@/http';

const getBaseUrl = () => window.PROJECT_CONFIG.BK_HCM_AJAX_URL_PREFIX || '';

const sseHeaders = (): Record<string, string> => getCommonHeaders();

export const streamChat = (
  sessionCode: string,
  messages: { role: string; content: string }[],
  signal?: AbortSignal,
  resumeValue?: string,
  forwardedProps?: Record<string, unknown>,
) => {
  // forwardedProps 用于中断恢复回写：resumeValue（如云账号 account_id / 申领方案 suborder JSON），
  // 或结构化数据（如预提单确认时的完整 suborders）。两者可共存，无值时不带 forwardedProps。
  const merged: Record<string, unknown> = {
    ...(resumeValue ? { resumeValue } : {}),
    ...(forwardedProps ?? {}),
  };
  return fetch(`${getBaseUrl()}/api/v1/agent/agui`, {
    method: 'POST',
    headers: sseHeaders(),
    credentials: 'include',
    signal,
    body: JSON.stringify({
      sessionCode,
      messages,
      ...(Object.keys(merged).length ? { forwardedProps: merged } : {}),
    }),
  });
};

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
