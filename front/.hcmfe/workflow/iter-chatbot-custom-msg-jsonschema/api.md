# API：aiagent - chatbot 自定义消息展示 - jsonschema 模式（云账号选择）

> 本迭代为**前端展示 + 复用现有 agent SSE 能力**。下行协议（§2）、上行提交（§5）、历史快照（§7）均已用**真实数据核对**，QA1~QA7 全部闭环。
> 机制对齐现有 HITL（`hitl.interrupt`）：CUSTOM 事件 → 带 `__type` 的消息 → 卡片渲染 → 选择回传 `/agui` 继续。

## 1. 结论摘要

| 能力 | 约定 |
|------|------|
| **下行识别** | SSE `CUSTOM` 事件，按 **`name === 'account_select.interrupt'`** 识别（与 `hitl.interrupt` 并列） |
| **下行结构** | `CUSTOM.value` = `{ checkpoint_id, lineage_id, value: { options[], type } }`（**无 message**）；提示语为独立 `TEXT_MESSAGE` 气泡（§2） |
| **选项字段** | 每个云账号：`account_id` / `account_name` / `vendor` / `enabled` / `reason`（§3） |
| **不可选** | `enabled: false` → 该选项**置灰不可选**，展示 `reason`（即使有权限，本期也不可选） |
| **vendor 映射** | `vendor` → 显示名（`VENDORS`/`VendorMap`）+ 图标（`@/assets/image/vendor-*.svg`）（§4） |
| **上行提交** | 复用 `/agui`，`forwardedProps.resumeValue = account_id`；手动输入时可不传，后端用 `content` 兜底（§5） |
| **历史回放** | `MESSAGES_SNAPSHOT` 中按 `activity/CUSTOM` + `name` 重建（对齐 HITL，§7） |
| **本迭代落地** | 仅 `account_select.interrupt`（云账号选择）；框架可扩展其它 name |
| **配额字段** | 真实 payload **无 quota 字段** → 本迭代**不展示配额**（与设计稿②差异，§6 QA4） |

## 2. 下行协议（agent → 前端，真实 eventstream）

一次中断的完整事件序列（按真实 SSE response 核对）：

```
RUN_STARTED
TEXT_MESSAGE_START / TEXT_MESSAGE_CONTENT / TEXT_MESSAGE_END   ← 提示语（普通 assistant 文本气泡）
ACTIVITY_DELTA (activityType=graph.node.interrupt)            ← 底层 interrupt 机制，前端当前 no-op
CUSTOM (name=account_select.interrupt)                        ← 前端实际消费，渲染账号选择卡
RUN_FINISHED
```

> **提示语是普通 `TEXT_MESSAGE`**（如「已进入主机申领模式。请先选择需要操作的云账号。」），由现有文本气泡逻辑**自然正常渲染**，本迭代无需处理。
> **本迭代只需实现 `CUSTOM`（厂商卡片）的渲染**：CUSTOM 的 `value.value` **只含 `options` + `type`，没有 `message`**（与 HITL 不同——HITL 把 `question` 内嵌在 value 里）。

### 2.1 CUSTOM 事件（前端消费）

原始事件（`value` 是 stringified JSON，真实一行）：

```json
{"type":"CUSTOM","timestamp":1781146379518,"name":"account_select.interrupt","value":"{\"checkpoint_id\":\"59f35799-206c-4d3b-ae36-f09e2adf1b30\",\"lineage_id\":\"000000b6\",\"value\":{\"options\":[{\"account_id\":\"0000002b\",\"account_name\":\"ziyan-hcm-test\",\"enabled\":true,\"reason\":\"\",\"vendor\":\"tcloud-ziyan\"},{\"account_id\":\"0000002a\",\"account_name\":\"tcloud-hcm\",\"enabled\":false,\"reason\":\"当前仅支持自研云（tcloud-ziyan）账号\",\"vendor\":\"tcloud\"}],\"type\":\"account_select.interrupt\"}}"}
```

`value`（字符串，`JSON.parse` 后）—— **`reason`、`enabled` 等均内联在每个 option 同一结构里**：

```json
{
  "checkpoint_id": "59f35799-206c-4d3b-ae36-f09e2adf1b30",
  "lineage_id": "000000b6",
  "value": {
    "options": [
      {
        "account_id": "0000002b",
        "account_name": "ziyan-hcm-test",
        "enabled": true,
        "reason": "",
        "vendor": "tcloud-ziyan"
      },
      {
        "account_id": "0000002a",
        "account_name": "tcloud-hcm",
        "enabled": false,
        "reason": "当前仅支持自研云（tcloud-ziyan）账号",
        "vendor": "tcloud"
      }
    ],
    "type": "account_select.interrupt"
  }
}
```

> 识别：`event.type === 'CUSTOM' && event.name === 'account_select.interrupt'`（对齐 `use-event.ts` 现有 `hitl.interrupt` 分支）。
> 前端可抽象**通用 interrupt 处理**（解析 `value` 字符串 → `{ checkpoint_id, lineage_id, value }`），再按 `name` 分流渲染。

### 2.2 ACTIVITY_DELTA 事件（前端 no-op，仅参考）

```json
{
  "type": "ACTIVITY_DELTA",
  "activityType": "graph.node.interrupt",
  "patch": [{
    "op": "add",
    "path": "/interrupt",
    "value": {
      "nodeId": "account_select",
      "key": "account_select.interrupt:1",
      "prompt": { "options": [ /* 同 CUSTOM */ ], "type": "account_select.interrupt" },
      "checkpointId": "59f35799-...",
      "lineageId": "000000b6"
    }
  }]
}
```

> 这是底层 ag-ui interrupt 机制，`use-event.ts` 中 `ACTIVITY_DELTA` 当前是 no-op。本迭代**不消费**该事件，仅依赖 CUSTOM。注意其字段为 camelCase（`checkpointId`/`lineageId`/`prompt`），与 CUSTOM 的 snake_case（`checkpoint_id`/`lineage_id`/`value`）不同。

## 3. 字段说明

### 3.1 外层（interrupt 信封）

| 字段 | 类型 | 说明 |
|------|------|------|
| `checkpoint_id` | string | 检查点 id，透传（恢复用，前端不展示） |
| `lineage_id` | string | 血缘 id，透传 |
| `value.options` | array | 云账号选项列表（§3.2） |
| `value.type` | string | 类型标识，值同 `name`（`account_select.interrupt`） |

> ⚠️ **没有 `value.message`**。提示语由**独立的 `TEXT_MESSAGE` 气泡**承载（§2），账号选择卡**只渲染 options**，不含头部提示文案。

### 3.2 选项 option（云账号）

| 字段 | 类型 | 说明 | 前端用途 |
|------|------|------|----------|
| `account_id` | string | 云账号唯一标识 | 选中提交时的标识（§5）；只读回显匹配 |
| `account_name` | string | 云账号名称 | 卡片副标题（如 `ziyan-hcm-test`） |
| `vendor` | string | 云厂商 key（如 `tcloud-ziyan`/`tcloud`/`aws`） | 映射厂商显示名 + 图标（§4） |
| `enabled` | boolean | 是否可选 | `false` → **置灰、不可选、不可 hover**，展示 `reason` |
| `reason` | string | 不可选原因 | `enabled=false` 时展示（如「当前仅支持自研云（tcloud-ziyan）账号」） |

> **`enabled` 重点**：标记"虽然有该账号权限，但本期不可选"。`enabled=false` 的选项仍展示在列表中（带原因），但不可点选、不可作为提交值。

## 4. vendor → 显示名 + 图标映射（前端）

数据源：`@/common/constant.ts`（`VendorEnum` / `VENDORS` / `VendorMap`）+ `@/assets/image/vendor-*.svg`。

| 后端 `vendor` | VendorEnum | 显示名（厂商名） | 图标 svg 文件 |
|---------------|-----------|------------------|---------------|
| `tcloud-ziyan` | `ZIYAN` | 自研云 | `vendor-tcloud.svg`（自研云复用腾讯云图标） |
| `tcloud` | `TCLOUD` | 腾讯云 | `vendor-tcloud.svg` |
| `aws` | `AWS` | 亚马逊云 | `vendor-aws.svg` |
| `azure` | `AZURE` | 微软云 | `vendor-azure.svg` |
| `gcp` | `GCP` | 谷歌云 | `vendor-gcp.svg` |
| `huawei` | `HUAWEI` | 华为云 | `vendor-huawei.svg` |

- 卡片：**左图标**（vendor svg）+ **厂商显示名**（加粗）+ **账号名 `account_name`**（弱色）。
- 未知 vendor 兜底：显示 vendor 原值 + 默认图标（不报错）。
- **显示名统一用 `VENDORS`/`VendorMap` 中文名**（自研云/腾讯云/亚马逊云…），不按稿面英文简称（QA3 确认）。

## 5. 上行协议（提交回写）—— 已确认

- **载体**：复用现有 `/agui`，通过 `forwardedProps.resumeValue` 回传所选 `account_id`。
- **请求体**：

```json
{
  "sessionCode": "",
  "messages": [
    { "role": "user", "content": "我选这个账号" }
  ],
  "forwardedProps": {
    "resumeValue": "0000002b"
  }
}
```

- **`forwardedProps.resumeValue`** = 选中选项的 **`account_id`**（如 `0000002b`）。
- **兜底规则**：`resumeValue` **可不传**——若用户不点选而是**手动输入**，则仅带 `messages[].content`，后端会把整个 `user_input`（即 `content`）当作 accountID 往下走。
  - 选了选项 → 带 `resumeValue`（account_id）；
  - 没选、手动输入 → 不带 `resumeValue`，只有 `messages[].content`。
- **前端实现要点**：现有 `sendMessage` 需支持透传 `forwardedProps.resumeValue`；点选账号卡时 `content` 取交互文案（如账号展示名），`resumeValue` 取 `account_id`。

## 6. 与现有 HITL 的复用 / 区分

| 维度 | HITL `hitl.interrupt` | 本迭代 `account_select.interrupt` |
|------|----------------------|-----------------------------------|
| 事件 | `CUSTOM` + name | `CUSTOM` + name（同机制） |
| 外层结构 | `{ checkpoint_id, lineage_id, value }` | **同构** |
| 内层 value | `{ question, options: string[] }` | `{ options: AccountOption[], type }`（**无 message**） |
| 提示语 | **内嵌** `value.question` | **独立 `TEXT_MESSAGE` 气泡**（卡片不含提示语） |
| 选项 | 字符串数组 | 对象数组（account_id/name/vendor/enabled/reason） |
| 提交回写 | `sendMessage(选项文本)` | `/agui` + `forwardedProps.resumeValue = account_id`（§5） |
| 渲染 | 单选 radio 列表 + 自定义输入 | 厂商卡（图标+名+账号）单选，响应式（全页卡片/浮窗整行），收起+展开只读+吸顶回显 |
| `__type` | `'hitl.interrupt'` | `'account_select.interrupt'` |

> 前端可把 interrupt 信封解析与历史重建做成**通用**（按 name 分流），渲染层分别用 HITL 卡片 / 账号选择卡片。

## 7. 历史回放（✅ 已用真实 `/history` 样例核对）

- account_select 中断持久化为 `{ role:'activity', activityType:'CUSTOM', content:{ name:'account_select.interrupt', value:'<stringified>' } }`——与实时 CUSTOM 一致，`use-stream.ts` `toHistoryMessage` 直接解析（已实现）。
- **所选账号不以独立字段落在 user 消息上**，而是分两处体现：
  - ① 紧随的 user 消息 `content`（如「我选择云账号：ziyan-hcm-test」，由前端 `handleAccountConfirm` 生成）；
  - ② 再后面的 `ACTIVITY_DELTA` patch `/resume` 的 `value.resume`（= account_id `0000002b`，权威值）。
- 只读态还原（已实现）：取 account_select 后的下一条 user 消息，按 `content` 命中 `options[].account_id` 或包含 `account_name`/`account_id` → 还原已选账号。因前端生成的 `content` 含 `account_name`，**round-trip 还原正确（已实测）**。
  - 备选：如需更稳，可改读 `ACTIVITY_DELTA` `/resume` 权威值；本期非必需。
  - 无后续 user 消息（未提交即中断）→ 保持可交互（对齐 HITL）。

## 8. 前端数据模型（提议）

```typescript
// 账号选择 interrupt 消息（对齐 HitlInterruptMessage 模式）
interface AccountSelectInterruptValue {
  checkpoint_id: string;
  lineage_id: string;
  value: {
    // 注意：无 message 字段，提示语由独立 TEXT_MESSAGE 承载
    type: 'account_select.interrupt';
    options: AccountSelectOption[];
  };
}

interface AccountSelectOption {
  account_id: string;
  account_name: string;
  vendor: string;       // VendorEnum 值，如 'tcloud-ziyan'
  enabled: boolean;     // false=置灰不可选
  reason: string;       // enabled=false 时的原因
}

// 消息标记（对齐 __type: 'hitl.interrupt'）
type AccountSelectMessage = Message & {
  __type: 'account_select.interrupt';
  content: AccountSelectInterruptValue;
};
```

## 9. 待确认问题（API 阻塞项）

| 编号 | 问题 | 结论 |
|------|------|------|
| ✅ **QA1** | **提交回写格式** | `/agui` 带 `forwardedProps.resumeValue = account_id`；不点选/手动输入时可不传，后端用 `messages[].content` 兜底（§5） |
| ✅ **QA2** | **历史快照结构** | 已用真实 `/history` 核对：CUSTOM 同实时结构；所选账号在 user 消息 `content` + `ACTIVITY_DELTA` `/resume`；前端按 `content` 还原已实测正确（§7） |
| ✅ **QA3** | **厂商显示名** | 统一用 `VENDORS`/`VendorMap` 中文名（自研云/腾讯云/亚马逊云…） |
| ✅ **QA4** | **配额** | **本迭代不展示配额**（真实 payload 无 quota；如需展示需后端加字段） |
| ✅ **QA5** | `enabled=false` 的 `reason` 展示方式 | **hover tooltip** 展示 reason |
| ✅ **QA6** | 本迭代支持的消息类型 | **仅 `account_select.interrupt`**（云账号选择）；框架按 name 可扩展，其它类型后续迭代 |
| ✅ **QA7** | **提示语渲染**：提示语是普通 `TEXT_MESSAGE`，由现有文本气泡逻辑**自然正常渲染**，无需特殊处理 | 本迭代**只实现 `CUSTOM`（厂商卡片）渲染**；设计稿中的"头部提示语"即这条普通消息，不属于卡片 |
