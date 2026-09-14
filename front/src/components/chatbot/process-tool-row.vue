<script setup lang="ts">
import { computed, onMounted, watch } from 'vue';
import { formatDuration, MessageStatus } from '@blueking/chat-x';

import { extractMessageText } from '@/hooks/chatbot/tool-intent';
import type { ProcessToolCall, ProcessToolMessage } from '@/hooks/chatbot/types';
import { useFollowScroll } from '@/hooks/chatbot/use-follow-scroll';

const props = defineProps<{
  message: ProcessToolMessage;
}>();

const emit = defineEmits<{
  toggleRow: [toolCallId: string];
}>();

const intent = computed(() => props.message.__intent.trim());

type ToolRow = {
  id: string;
  // 本次调用的说明文案，画在这一行上方；一句说明对一条工具行
  intent: string;
  title: string;
  // pending：本轮已结束（或历史回放）却没有结果消息 —— 被 HITL 中断挂起等确认，或中途被停掉
  state: 'running' | 'success' | 'error' | 'pending';
  duration: string;
  description: string;
  args: string;
  result: string;
};

// 详情区保留数据里的换行（值节点与 pre 都是 pre-wrap），所以值自带的空白会被原样画成空行。
// 单行摘要里的换行一律压成空格；多行内容最多留一个空行
const inline = (value: unknown): string =>
  String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim();
const squeezeBlankLines = (text: string): string => text.replace(/\n{3,}/g, '\n\n').trim();

const formatPretty = (raw: string): string => {
  const text = raw.trim();
  if (!text) return '';
  try {
    const parsed = JSON.parse(text) as unknown;
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const record = parsed as Record<string, unknown>;
      const keys = Object.keys(record);
      const simple = keys.every((key) => {
        const value = record[key];
        return value == null || ['string', 'number', 'boolean'].includes(typeof value);
      });
      if (simple && keys.length > 0 && keys.length <= 4) {
        return keys.map((key) => `${key}:${inline(record[key])}`).join(' ');
      }
      return JSON.stringify(parsed, null, 2);
    }
    return typeof parsed === 'string' ? squeezeBlankLines(parsed) : JSON.stringify(parsed, null, 2);
  } catch {
    return squeezeBlankLines(text);
  }
};

// 拿不到耗时（/history 恒缺）就不画耗时段，不显示 0ms；格式走 chat-x（650ms / 20s345ms），与思考条一致
const formatToolDuration = (ms: unknown): string => {
  if (typeof ms !== 'number' || !Number.isFinite(ms) || ms <= 0) return '';
  return formatDuration(Math.round(ms));
};

const toolTitle = (name: string, mcpName: string): string => {
  if (mcpName) return `MCP调用：${mcpName} / ${name}`;
  return name ? `调用工具 ${name}` : '调用工具';
};

// 对齐 chat-x ToolcallRender（进行中转圈 / 成功对勾 / 失败叉），只是按稿面不带「调用成功」文字。
// 不能用 message.status 判进行中：TOOL_CALL_END 只代表入参给完了，此时状态已是 Complete，工具还在跑；
// 唯一可靠的完成信号是配到了结果消息（role=tool）。
// 多出的 pending 是本产品特有的常态而非异常：需确认的工具（如 create_biz_apply）会在 TOOL_CALL_END 后被
// tool_confirm 中断挂起，本轮直接 RUN_FINISHED，结果要等用户确认后的下一轮才回来（且那时已是新的一轮，
// 配不回这条）。所以这里必须给个终态图标，不能留空把标题挤到最左
const resolveState = (message: ProcessToolMessage, toolCall: ProcessToolCall): ToolRow['state'] => {
  const result = toolCall.toolMessage;
  const failed = message.status === MessageStatus.Error || result?.status === MessageStatus.Error;
  if (failed || (result as { error?: unknown } | undefined)?.error) return 'error';
  if (result) return 'success';
  return message.__streaming ? 'running' : 'pending';
};

// 展开态由 useProcessZone 持有：要独立于过程区总开关长期保留，放组件内会随重挂载丢失
const isRowExpanded = (id: string) => !!props.message.__rowExpanded[id];

const rows = computed<ToolRow[]>(() =>
  (props.message.__toolCalls ?? []).map((toolCall) => {
    const fn = toolCall.function ?? { name: '', arguments: '' };
    const result = toolCall.toolMessage;
    return {
      id: toolCall.id,
      intent: (toolCall.intent ?? '').trim(),
      title: toolTitle(String(fn.name ?? ''), String((fn as { mcpName?: string }).mcpName ?? '')),
      state: resolveState(props.message, toolCall),
      duration: formatToolDuration(result?.duration),
      description: squeezeBlankLines(String((fn as { description?: string }).description ?? '')),
      args: formatPretty(String(fn.arguments ?? '')),
      result: formatPretty(extractMessageText(result?.content)),
    };
  }),
);

// 工具行是手写节点，流式增量不会触发 chat-x 的跟随滚动，按内容规模变化补一次贴底。
// 只看内容规模，不看展开态：用户手动展开是为了读详情，不该被拽到底部
const followScroll = useFollowScroll();
const contentSize = computed(() =>
  rows.value.reduce(
    (total, row) => total + row.intent.length + row.args.length + row.result.length,
    intent.value.length,
  ),
);
watch([contentSize, () => rows.value.length], () => followScroll(), { flush: 'post' });
onMounted(() => followScroll());
</script>

<template>
  <!-- 过程区收起时仅留一个空壳（外层 .ai-message-item 由 process-hidden 一起收掉），不从消息列表里摘除本条 -->
  <div v-if="!message.__visible" class="process-tool-row process-hidden" />
  <div v-else class="process-tool-row">
    <!-- 模型在调工具的同时说的正文，通常为空；各次调用的说明来自参数 tool_intent，在下面逐行画 -->
    <p v-if="intent" class="process-tool-intent">{{ intent }}</p>
    <div v-for="row in rows" :key="row.id" class="process-tool-block">
      <p v-if="row.intent" class="process-tool-intent">{{ row.intent }}</p>
      <div class="process-tool-header" @click="emit('toggleRow', row.id)">
        <div class="process-tool-title-wrap">
          <!-- 进行中：稿面没给独立字形，用项目 iconfont 的转圈环，尺寸比成功/失败小一档 -->
          <i v-if="row.state === 'running'" class="hcm-icon bkhcm-icon-loading-circle process-tool-status is-running" />
          <i
            v-else-if="row.state === 'success'"
            class="hcm-icon bkhcm-icon-circle-success process-tool-status is-success"
          />
          <i v-else-if="row.state === 'error'" class="hcm-icon bkhcm-icon-circle-error process-tool-status is-error" />
          <!-- 无结果（等确认 / 被停掉）：沿用项目里表示待处理的 waiting 字形 -->
          <i v-else class="hcm-icon bkhcm-icon-waiting process-tool-status is-pending" />
          <span class="process-tool-title">{{ row.title }}</span>
        </div>
        <div v-if="row.duration" class="process-tool-duration">
          <i class="hcm-icon bkhcm-icon-circle-time" />
          <span>{{ row.duration }}</span>
        </div>
      </div>
      <!-- 三个字段常显，缺值写 --：字段位置固定，展开后不会因为这次调用没描述/没返回而错位 -->
      <div v-if="isRowExpanded(row.id)" class="process-tool-detail">
        <p>
          <span class="process-tool-label">描述：</span>
          <span class="process-tool-value">{{ row.description || '--' }}</span>
        </p>
        <p>
          <span class="process-tool-label">参数：</span>
          <span class="process-tool-value">{{ row.args || '--' }}</span>
        </p>
        <p>
          <span class="process-tool-label">返回内容：</span>
          <!-- 有返回内容时另起一行走 pre（结果多是格式化 JSON，得独占整行宽度）；无值就跟在标签后 -->
          <span v-if="!row.result" class="process-tool-value">--</span>
        </p>
        <pre v-if="row.result" class="process-tool-result">{{ row.result }}</pre>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.process-tool-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.process-tool-intent {
  margin: 0;
  font-size: 12px;
  line-height: 20px;
  color: #4d4f56;
}

.process-tool-block {
  width: 100%;

  // 行内的说明文案自带下间距：外层 flex 的 gap 只管块与块之间，管不到块内
  > .process-tool-intent {
    margin-bottom: 8px;
  }
}

.process-tool-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 24px;
  padding: 0 12px;
  cursor: pointer;
  user-select: none;
  background: #f5f7fa;
  border-bottom: 1px solid #eaebf0;
}

.process-tool-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.process-tool-status {
  flex-shrink: 0;
  font-size: 16px;
  line-height: 16px;

  // 转圈环是细描边，压到 14px 才和 16px 的 circle-* 视觉等重；line-height 保持 16px 以对齐同列其它状态
  &.is-running {
    font-size: 14px;
    color: #3a84ff;
    animation: process-tool-spin 1s linear infinite;
  }

  &.is-success {
    color: #2dcb56;
  }

  &.is-error {
    color: #ea3636;
  }

  &.is-pending {
    color: #ff9c01;
  }
}

@keyframes process-tool-spin {
  0% {
    transform: rotate(0deg);
  }

  100% {
    transform: rotate(360deg);
  }
}

.process-tool-title {
  overflow: hidden;
  font-size: 12px;
  line-height: 20px;
  color: #4d4f56;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.process-tool-duration {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 4px;
  font-size: 12px;
  line-height: 20px;
  color: #c4c6cc;

  .hcm-icon {
    font-size: 14px;
  }
}

// 不在容器上开 pre-wrap：那样连标记里的缩进换行都会被画成真空行，字段之间凭空多出一行（实测行距 68px，
// 按样式只该有 28px）。要保留换行的只有取值本身，放到 .process-tool-value / .process-tool-result 上
.process-tool-detail {
  padding: 8px 12px;
  font-size: 12px;
  line-height: 20px;
  color: #979ba5;
  word-break: break-word;
  background: #fafbfd;
  border-radius: 4px;

  p {
    margin: 0 0 4px;

    &:last-child {
      margin-bottom: 0;
    }
  }
}

.process-tool-label {
  color: #4d4f56;
}

.process-tool-value {
  white-space: pre-wrap;
}

.process-tool-result {
  margin: 0;
  font-family: inherit;
  font-size: inherit;
  line-height: inherit;
  color: inherit;
  word-break: break-word;
  white-space: pre-wrap;
}
</style>
