import { MessageRole, MessageStatus, type Message } from '@blueking/chat-x';

// 前端内部使用的扩展消息类型（交叉类型，因为 Message 是联合类型无法直接扩展）
export type HitlInterruptMessage = Message & {
  role: MessageRole.Assistant; // 实际以 assistant 角色展示
  content: HitlInterruptValue; // 结构化内容
  status: MessageStatus.Complete;
  // 自定义标记，用于 slot 中识别
  __type: 'hitl.interrupt';
};

export interface ChatSession {
  sessionCode: string;
  sessionName: string;
  sessionContentCount: number;
  sessionTag?: string;
  createdAt: string;
  updatedAt: string;
  messages: Message[];
}

export interface HitlInterruptValue {
  checkpoint_id: string;
  lineage_id: string;
  value: {
    question: string;
    options: string[];
  };
}

export enum EventType {
  RunStarted = 'RUN_STARTED',
  RunFinished = 'RUN_FINISHED',
  RunError = 'RUN_ERROR',
  TextMessageStart = 'TEXT_MESSAGE_START',
  TextMessageContent = 'TEXT_MESSAGE_CONTENT',
  TextMessageEnd = 'TEXT_MESSAGE_END',
  TextMessageChunk = 'TEXT_MESSAGE_CHUNK',
  ThinkingStart = 'THINKING_START',
  ThinkingTextMessageStart = 'THINKING_TEXT_MESSAGE_START',
  ThinkingTextMessageContent = 'THINKING_TEXT_MESSAGE_CONTENT',
  ThinkingTextMessageEnd = 'THINKING_TEXT_MESSAGE_END',
  ThinkingEnd = 'THINKING_END',
  ToolCallStart = 'TOOL_CALL_START',
  ToolCallArgs = 'TOOL_CALL_ARGS',
  ToolCallEnd = 'TOOL_CALL_END',
  ToolCallResult = 'TOOL_CALL_RESULT',
  ToolCallChunk = 'TOOL_CALL_CHUNK',
  StepStarted = 'STEP_STARTED',
  StepFinished = 'STEP_FINISHED',
  MessagesSnapshot = 'MESSAGES_SNAPSHOT',
  StateDelta = 'STATE_DELTA',
  StateSnapshot = 'STATE_SNAPSHOT',
  ActivityDelta = 'ACTIVITY_DELTA',
  ActivitySnapshot = 'ACTIVITY_SNAPSHOT',
  Custom = 'CUSTOM',
  Raw = 'RAW',
}
