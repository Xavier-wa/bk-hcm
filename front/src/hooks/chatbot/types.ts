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

// 云账号选择选项（agent 下发，字段内联在同一结构）
export interface AccountSelectOption {
  account_id: string;
  account_name: string;
  vendor: string; // VendorEnum 值，如 'tcloud-ziyan' / 'tcloud'
  enabled: boolean; // false 表示有权限但本期不可选，置灰
  reason: string; // enabled=false 时的不可选原因（tooltip 展示）
}

// 云账号选择中断消息内容（CUSTOM name=account_select.interrupt）
// 注意：value 内无 message 字段，提示语由独立的 TEXT_MESSAGE 承载
export interface AccountSelectInterruptValue {
  checkpoint_id: string;
  lineage_id: string;
  value: {
    type: 'account_select.interrupt';
    options: AccountSelectOption[];
  };
}

// 前端内部扩展消息类型，用于 slot 中识别账号选择卡
export type AccountSelectInterruptMessage = Message & {
  role: MessageRole.Assistant;
  content: AccountSelectInterruptValue;
  status: MessageStatus.Complete;
  __type: 'account_select.interrupt';
  // 实时流中确认选择后写入，供只读态与吸顶回显复用（历史回放无此字段，走兜底匹配）
  __selectedAccountId?: string;
};

// ---- 指令模板模式 · 主机申领 ----
// 数据结构对齐后端真实 agent 协议（recommend_select / recommend_suborder_confirm 两次中断）。
// 详见 .hcmfe/workflow/iter-chatbot-custom-msg-command-template/api.md

// CUSTOM 事件 name：模板 A（申领方案推荐）与模板 B（预提单/确认配置）
export const HOST_APPLY_RECOMMEND_EVENT = 'after_tool_hitl.recommend_select.interrupt';
export const HOST_APPLY_CONFIRM_EVENT = 'after_tool_hitl.recommend_suborder_confirm.interrupt';

// 磁盘规格（系统盘 / 数据盘统一结构）
export interface HostApplyDisk {
  disk_type: string; // 磁盘类型编码，如 CLOUD_PREMIUM
  disk_size: number; // 容量（GB）
  disk_num: number; // 数量
}

// 主机申领子单规格，A 推荐方案 / B 预提单 / C 调整弹窗共用。
// 注意：字段值均为后端编码/枚举码（如 region=ap-nanjing、charge_type=PREPAID、require_type=7），前端本期原样展示。
export interface HostApplySuborder {
  require_type?: string | number; // 需求类型编码
  region?: string; // 地域编码，如 ap-nanjing
  zone?: string; // 可用区编码，如 all
  device_type?: string; // 机型编码，如 S3.MEDIUM4
  image_id?: string; // 镜像 id，如 img-fjxtfi0n
  res_assign?: string | number; // 资源分配方式编码
  replicas?: number; // 申请数量
  charge_type?: string; // 计费模式编码，如 PREPAID
  system_disk?: HostApplyDisk; // 系统盘
  data_disk?: HostApplyDisk[]; // 数据盘
}

// 单条推荐方案
export interface HostApplyRecommendation {
  source: string; // 来源：'user'=历史配置 / 'biz'=业务推荐
  suborder: HostApplySuborder; // 方案规格
}

// 模板 A：申领方案推荐（CUSTOM name=after_tool_hitl.recommend_select.interrupt）
export interface HostApplyRecommendValue {
  checkpoint_id: string;
  lineage_id: string;
  value: {
    recommendations: HostApplyRecommendation[];
  };
}

// 模板 B：预提单 / 确认配置（CUSTOM name=after_tool_hitl.recommend_suborder_confirm.interrupt）
export interface HostApplyPreorderValue {
  checkpoint_id: string;
  lineage_id: string;
  value: {
    suborders: HostApplySuborder[];
  };
}

// 前端内部扩展消息类型，用于 slot 中识别申领方案推荐卡
export type HostApplyRecommendMessage = Message & {
  role: MessageRole.Assistant;
  content: HostApplyRecommendValue;
  status: MessageStatus.Complete;
  __type: 'host_apply.recommend';
  // 实时流中选择方案后写入所选下标，供只读态复用（历史回放无此字段，退化为首条）
  __selectedIndex?: number;
};

// 前端内部扩展消息类型，用于 slot 中识别预提单卡
export type HostApplyPreorderMessage = Message & {
  role: MessageRole.Assistant;
  content: HostApplyPreorderValue;
  status: MessageStatus.Complete;
  __type: 'host_apply.preorder';
  // 实时流中确认方案后写入，标记已提交并保留确认时的 suborders（含 C 弹窗修改）
  __confirmedSuborders?: HostApplySuborder[];
};

// ---- 模板 D：确认提交申请单（agent tool_confirm 中断） ----

// CUSTOM 事件 name；其余 tool_confirm.interrupt.* 事件不识别，保持原生文本输出
export const HOST_APPLY_SUBMIT_EVENT = 'tool_confirm.interrupt.create_cvm_apply';

// 提交确认 suborder 的 spec（嵌套结构，字段为后端编码）
export interface HostApplySubmitSpec {
  charge_type?: string; // 计费模式编码，如 PREPAID
  charge_months?: number; // 包年包月时长（月）
  data_disk?: HostApplyDisk[]; // 数据盘
  device_type?: string; // 机型编码
  image_id?: string; // 镜像 id
  network_type?: string; // 网络类型编码
  region?: string; // 地域编码
  res_assign?: string | number; // 资源分配方式编码
  resource_mode?: number; // 资源模式
  system_disk?: HostApplyDisk; // 系统盘
  zone?: string; // 可用区编码（旧字段，兼容保留）
  zones?: string[]; // 可用区编码列表（如 ["all"]）
}

// 提交确认 suborder：replicas / resource_type 在外层，规格在 spec
export interface HostApplySubmitSuborder {
  replicas?: number; // 申请数量
  resource_type?: string; // 资源类型，如 QCLOUDCVM
  spec: HostApplySubmitSpec;
}

// 模板 D：确认提交申请单（CUSTOM name=tool_confirm.interrupt.create_cvm_apply）
export interface HostApplySubmitValue {
  checkpoint_id: string;
  lineage_id: string;
  value: {
    data: {
      body_param: {
        bk_username?: string;
        expect_time?: string; // 期望交付时间
        require_type?: string | number; // 需求类型编码（全部 suborder 共用）
        suborders: HostApplySubmitSuborder[];
      };
      path_param?: { bk_biz_id?: string };
    };
    tool: string; // 后端工具名，如 create_biz_apply
  };
}

// 前端内部扩展消息类型，用于 slot 中识别确认提交卡
export type HostApplySubmitMessage = Message & {
  role: MessageRole.Assistant;
  content: HostApplySubmitValue;
  status: MessageStatus.Complete;
  __type: 'host_apply.submit';
  // 实时流中点击「确认提交」后写入，标记已提交（历史回放无此字段，退化为后续 user 消息判断）
  __submitted?: boolean;
};

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
