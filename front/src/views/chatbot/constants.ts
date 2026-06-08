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

// session_tag key → 文件夹/场景展示名映射（与侧栏标签文件夹共用）。目前仅支持主机申领。
export const SESSION_TAG_NAME: Record<string, string> = {
  host_apply: '主机申领',
};

// 主内容区底部联系人：name 为企业微信账号（点击拉起会话），alias 为展示文案。
// TODO: 待产品/稿面确认「小助手」对应的真实企微账号后替换 name。
export const ASSISTANT_CONTACT = {
  name: '小助手',
  alias: '@小助手',
};

export const BIG_CARDS: BigCard[] = [
  {
    icon: 'bkhcm-icon-host-application',
    title: '申领 10 台主机',
    desc: '可通过智能推荐快捷申领主机',
    prompt: '我要申领 10 台主机',
    sessionTag: 'host_apply',
  },
  {
    icon: 'bkhcm-icon-host-inventory',
    title: 'GPU 库存情况',
    desc: '可查看 GPU 库存情况及使用率',
    prompt: '我想查看 GPU 库存情况',
  },
  {
    icon: 'bkhcm-icon-host-recycle',
    title: '如何提交回收申请',
    desc: '可通过智能引导快速回收资源',
    prompt: '如何提交回收申请',
  },
  {
    icon: 'bkhcm-icon-host-multi',
    title: '申领 50 台服务器',
    desc: '可通过智能推荐快捷申领服务器',
    prompt: '我要申领 50 台服务器',
    sessionTag: 'host_apply',
  },
];

export const PROMPT_CHIPS: PromptChip[] = [
  { tag: '主机申领', icon: 'bkhcm-icon-host-application', prompt: '我要申请主机', sessionTag: 'host_apply' },
  { tag: '主机回收', icon: 'bkhcm-icon-host-recycle', prompt: '我要回收主机' },
  { tag: '预测提单', icon: 'bkhcm-icon-resource-plan', prompt: '我要提交预测单' },
  { tag: '预测调整', icon: 'bkhcm-icon-resource-plan', prompt: '我要调整预测' },
  { tag: 'CLB申领', icon: 'bkhcm-icon-loadbalancer', prompt: '我要申领 CLB' },
  { tag: 'CLB删除', icon: 'bkhcm-icon-loadbalancer', prompt: '我要删除 CLB' },
  { tag: 'CLB批量导入', icon: 'bkhcm-icon-loadbalancer', prompt: '我要批量导入 CLB' },
  { tag: '安全组创建', icon: 'bkhcm-icon-security-group', prompt: '我要创建安全组' },
  { tag: '安全组规则管理', icon: 'bkhcm-icon-security-group', prompt: '我要管理安全组规则' },
  { tag: '安全组绑定/解绑', icon: 'bkhcm-icon-security-group', prompt: '我要绑定或解绑安全组' },
];
