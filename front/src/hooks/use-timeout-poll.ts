import { getCurrentScope, onScopeDispose, ref } from 'vue';
import type { Awaitable } from '@/typings';

export type TimeoutPollAction = ReturnType<typeof useTimeoutPoll>;

export default function useTimeoutPoll(
  fn: () => Awaitable<void>,
  interval: number,
  options?: { immediate?: boolean; max?: number },
) {
  const { immediate = false, max = 100 } = options || {};

  const isActive = ref(false);

  let timer: ReturnType<typeof setTimeout> | null = null;

  let times = 0;

  // 组件作用域销毁后置为 true，阻止在途请求 resolve 后再次唤起轮询
  let isDisposed = false;

  function clear() {
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
  }

  function start() {
    clear();
    timer = setTimeout(() => {
      timer = null;

      loop();
    }, interval ?? 5000);
  }

  async function loop() {
    if (!isActive.value) {
      return;
    }

    if (max !== -1 && times >= max) {
      return;
    }
    times += 1;

    await fn();
    start();
  }

  function resume() {
    // 作用域已销毁（组件已卸载）则不再启动轮询
    if (isDisposed) {
      return;
    }
    if (!isActive.value) {
      isActive.value = true;
      immediate ? loop() : start();
    }
  }

  function pause() {
    isActive.value = false;
  }

  function reset() {
    clear();
    isActive.value = false;
    times = 0;
  }

  if (immediate) {
    resume();
  }

  if (getCurrentScope()) {
    // 组件卸载时彻底停止轮询：清定时器 + 停用，并标记已销毁，
    // 防止卸载瞬间在途的请求 resolve 后再调用 resume 复活轮询
    onScopeDispose(() => {
      isDisposed = true;
      reset();
    });
  }

  return {
    isActive,
    pause,
    resume,
    reset,
  };
}
