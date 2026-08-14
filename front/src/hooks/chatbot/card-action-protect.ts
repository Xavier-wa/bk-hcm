/** 覆盖「点完立刻切屏再回来」的常见间隔；可按手测微调 */
export const CARD_ACTION_PROTECT_MS = 8000;

let protectUntil = 0;

/** 卡片本地运行态写入后打点：被动 history 刷新在窗口内可跳过 */
export const touchCardActionProtect = (): void => {
  protectUntil = Date.now() + CARD_ACTION_PROTECT_MS;
};

export const isCardActionProtected = (): boolean => Date.now() < protectUntil;
