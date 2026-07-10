// 首页空态主内容区卡片配置：前端固定、全业务统一（见 .hcmfe/workflow/iter-biz-index-chatbot-default-main-content）。
// 文案为合理默认，可按产品/稿面微调。

// 大卡片：点击直接发送 prompt
export interface BigCard {
  // icon 为 iconfont 类名
  icon: string;
  title: string;
  desc: string;
  // prompt 为点击直接发送的预设消息
  prompt: string;
  // sessionTag 为场景标识，创建会话时通过 session_tag 入参传递；无则为普通会话
  sessionTag?: string;
}

// 小卡片（提示词 chip）：点击注入「场景 tag + 默认词」到输入框，待用户点发送
export interface PromptChip {
  // tag 为 chip 文案（场景名）
  tag: string;
  icon: string;
  // prompt 为注入输入框的默认提示词
  prompt: string;
  sessionTag?: string;
}

// 场景标识常量：主机申领。用于按场景过滤会话列表、创建会话时打 session_tag。
export const SESSION_TAG_HOST_APPLY = 'host_apply';

// 场景标识常量：资源查询。
export const SESSION_TAG_RESOURCE_QUERY = 'resource_query';

// session_tag key → 文件夹/场景展示名映射（与侧栏标签文件夹共用）。
export const SESSION_TAG_NAME: Record<string, string> = {
  [SESSION_TAG_HOST_APPLY]: '主机申领',
  [SESSION_TAG_RESOURCE_QUERY]: '资源查询',
};

// 主内容区底部联系人：name 为企业微信账号（点击拉起会话），alias 为展示文案。
export const ASSISTANT_CONTACT = {
  name: 'ICR',
  alias: '@小助手',
};

export const BIG_CARDS: BigCard[] = [
  {
    icon: 'bkhcm-icon-host-application',
    title: '自研云主机申请',
    desc: '可通过智能推荐快捷申领主机',
    prompt: '给我推荐申请主机的方案',
    sessionTag: SESSION_TAG_HOST_APPLY,
  },
  {
    icon: 'bkhcm-icon-search',
    title: '资源查询',
    desc: '可查询主机、负载均衡等资源信息',
    prompt: '列举出有哪些资源可以查询',
    sessionTag: SESSION_TAG_RESOURCE_QUERY,
  },
];

export const PROMPT_CHIPS: PromptChip[] = [
  {
    tag: '主机申领',
    icon: 'bkhcm-icon-host-application',
    prompt: '我要申请主机，地域：南京，数量：1 台',
    sessionTag: SESSION_TAG_HOST_APPLY,
  },
  {
    tag: '资源查询',
    icon: 'bkhcm-icon-search',
    prompt: '列举出有哪些资源可以查询',
    sessionTag: SESSION_TAG_RESOURCE_QUERY,
  },
];
