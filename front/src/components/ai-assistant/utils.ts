// 复刻自 ai-blueking utils：平台检测与「打开面板」快捷键（Cmd/Ctrl + I）

/**
 * 检测当前操作系统平台
 */
export function getPlatform(): 'mac' | 'windows' | 'linux' | 'unknown' {
  const userAgent = window.navigator.userAgent.toLowerCase();

  if (userAgent.includes('mac')) {
    return 'mac';
  }
  if (userAgent.includes('win')) {
    return 'windows';
  }
  if (userAgent.includes('linux')) {
    return 'linux';
  }
  return 'unknown';
}

/**
 * 判断是否为 Mac 系统
 */
export function isMac(): boolean {
  return getPlatform() === 'mac';
}

/**
 * 获取打开面板的快捷键文本
 */
export function getTogglePanelShortcut(): string {
  return isMac() ? 'Cmd + I' : 'Ctrl + I';
}

/**
 * 检查键盘事件是否触发了「显隐面板」的快捷键（Cmd/Ctrl + I）
 */
export function isTogglePanelShortcut(event: KeyboardEvent): boolean {
  const isModifierPressed = isMac() ? event.metaKey : event.ctrlKey;
  const isIKey = event.key.toLowerCase() === 'i';
  return isModifierPressed && isIKey && !event.shiftKey && !event.altKey;
}
