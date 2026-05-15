import type { Message } from '@blueking/chat-x';

export interface ChatSession {
  sessionCode: string;
  sessionName: string;
  sessionContentCount: number;
  createdAt: string;
  updatedAt: string;
  messages: Message[];
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
