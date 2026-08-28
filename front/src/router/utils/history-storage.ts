/* eslint-disable @typescript-eslint/member-ordering */
import type { RouteLocationRaw, RouteRecordNameGeneric } from 'vue-router';

export class HistoryStorage {
  private static key = 'history';

  get history() {
    return HistoryStorage.get();
  }

  static get() {
    let historyList = [];
    try {
      historyList = JSON.parse(window.sessionStorage.getItem(this.key)) || [];
      if (!Array.isArray(historyList)) {
        historyList = [historyList];
      }
    } catch (e) {
      historyList = [];
    }
    return historyList;
  }

  /**
   * btoa 只接受 Latin1。列表页 query 常含中文筛选条件，直接 btoa(JSON) 会抛 InvalidCharacterError。
   * 先按 UTF-8 转成字节再 btoa；ASCII 旧数据同样可被 TextDecoder 还原。
   */
  private static serialize(data: RouteLocationRaw): string {
    const bytes = new TextEncoder().encode(JSON.stringify(data));
    let binary = '';
    bytes.forEach((byte) => {
      binary += String.fromCharCode(byte);
    });
    return btoa(binary);
  }

  private static deserialize(record: string): RouteLocationRaw {
    const binary = atob(record);
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    return JSON.parse(new TextDecoder().decode(bytes));
  }

  static append(data: RouteLocationRaw) {
    const historyList = this.get();
    historyList.push(this.serialize(data));
    window.sessionStorage.setItem(this.key, JSON.stringify(historyList));
  }

  static remove(name: RouteRecordNameGeneric) {
    const historyList = this.get();
    const index = historyList.findIndex((item) => {
      try {
        return this.deserialize(item).name === name;
      } catch {
        return false;
      }
    });
    if (index !== -1) {
      historyList.splice(index, 1);
      window.sessionStorage.setItem(this.key, JSON.stringify(historyList));
    }
  }

  static pop(): RouteLocationRaw {
    const historyList = this.get();
    const record = historyList.pop();
    if (record === undefined) {
      throw new Error('history stack is empty');
    }
    const route = this.deserialize(record);
    window.sessionStorage.setItem(this.key, JSON.stringify(historyList));
    return route;
  }

  /** 只读栈顶，供面包屑判断「能否返回」；真正离开时再 pop */
  static peek(): RouteLocationRaw | null {
    const historyList = this.get();
    const record = historyList[historyList.length - 1];
    if (!record) return null;
    try {
      return this.deserialize(record);
    } catch {
      return null;
    }
  }

  static clear() {
    window.sessionStorage.setItem(this.key, JSON.stringify([]));
  }
}
