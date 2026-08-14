# API — 指令模板模式 · 主机申领（已对接后端真实协议）

> ✅ **后端已提供真实 SSE 协议**。本文档已按真实 eventstream 校准下行结构（§3/§4）。
> - 信封 / 事件机制沿用 jsonschema 迭代已验证的 `CUSTOM` 协议。
> - 业务字段直接采用后端 `suborder` 结构（字段值为**编码/枚举码**，非展示串）。
> - ⚠️ **上行 resume 承载形式（§6）后端尚未明确**，前端先按合理假设实现并隔离，待后端确认后改一处即可。

## 1. 结论摘要

| 能力 | 约定 |
|---|---|
| **下行识别** | SSE `CUSTOM` 事件，按 **`name`** 区分模板（与 `account_select.interrupt`/`hitl.interrupt` 并列） |
| **模板 name** | A=`after_tool_hitl.recommend_select.interrupt`、B=`after_tool_hitl.recommend_suborder_confirm.interrupt`、D=`tool_confirm.interrupt.create_cvm_apply` |
| **下行信封** | 同 jsonschema：`CUSTOM.value`(stringified) = `{ checkpoint_id, lineage_id, value: {...payload} }` |
| **A 数据** | `value.value` = `{ recommendations: { source, suborder }[] }`（多条推荐方案，可翻页）（§3） |
| **B 数据** | `value.value` = `{ suborders: Suborder[] }`（预提单配置，可逐条编辑）（§4） |
| **C（调整配置）** | 无独立下行：前端弹窗，编辑 B 的某条 `suborder`，保存写回该行（§5） |
| **D 数据** | `value.value` = `{ data:{ body_param:{ require_type, suborders:{ replicas, spec }[] }, path_param }, tool }`（确认提交申请单，**只读不可改、无操作列**）（§4.5） |
| **约定外事件** | 其余 `name`（含其他 `tool_confirm.interrupt.*`）不识别，保持原生文本气泡输出 |
| **字段值** | 均为后端编码（region/zone/device_type/image_id/charge_type/require_type/res_assign/disk_type），**前端本期原样展示**（§9） |
| **上行提交** | 复用 `/agui` `forwardedProps`（同 jsonschema `resumeValue` 通道）；承载形式见 §6【待后端确认】 |
| **只读/历史** | 同 jsonschema：`CUSTOM` 持久化 + 后续 user/`resume` 还原（§7） |
| **跨页跳转** | 「添加到配置清单」本迭代仅跳转主机申请页 + 预留入口（§8） |

## 2. 下行信封（沿用 jsonschema，已验证）

```json
{"type":"CUSTOM","timestamp":0,"name":"<模板name>","value":"<stringified JSON>"}
```

`value`（`JSON.parse` 后）：

```json
{ "checkpoint_id": "...", "lineage_id": "...", "value": { "...": "..." } }
```

> 识别：`event.type === 'CUSTOM' && event.name === '<模板name>'`，复用 `use-event.ts` 现有按 name 分流。
> 与 jsonschema 不同：本协议 `value.value` **不含 `type` 字段**，模板由外层 `name` 区分。

## 3. 模板 A：申领方案推荐 `after_tool_hitl.recommend_select.interrupt`

`value.value`（真实样例）：

```json
{
  "recommendations": [
    {
      "source": "user",
      "suborder": {
        "charge_type": "PREPAID",
        "data_disk": [{ "disk_num": 1, "disk_size": 500, "disk_type": "CLOUD_PREMIUM" }],
        "device_type": "S3.MEDIUM4",
        "image_id": "img-fjxtfi0n",
        "region": "ap-nanjing",
        "replicas": 10,
        "require_type": 7,
        "res_assign": 1,
        "system_disk": { "disk_num": 1, "disk_size": 100, "disk_type": "CLOUD_PREMIUM" },
        "zone": "all"
      }
    },
    { "source": "biz", "suborder": { "...": "..." } }
  ]
}
```

字段说明：

| 字段 | 类型 | 说明 |
|---|---|---|
| `recommendations` | array | 推荐方案列表，多条时卡片翻页（1/N）；**无 id，前端按下标定位** |
| `source` | string | 来源：`user`=用户历史配置 / `biz`=业务推荐；前端展示来源标签 |
| `suborder` | object | 方案规格（§9） |

## 4. 模板 B：预提单 / 确认配置 `after_tool_hitl.recommend_suborder_confirm.interrupt`

> **数据来源**：用户在 A 选择方案后，agent 返回的待确认 `suborders`（可逐条编辑后确认）。

`value.value`（真实样例）：

```json
{
  "suborders": [
    {
      "charge_type": "PREPAID",
      "data_disk": [{ "disk_num": 1, "disk_size": 500, "disk_type": "CLOUD_PREMIUM" }],
      "device_type": "S3.LARGE8",
      "image_id": "img-gqmik24x",
      "region": "ap-nanjing",
      "replicas": 13,
      "require_type": 7,
      "res_assign": 1,
      "system_disk": { "disk_num": 1, "disk_size": 100, "disk_type": "CLOUD_PREMIUM" },
      "zone": "all"
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `suborders` | array | 预提单配置行；**无 id、无 banner，前端按下标定位** |
| `suborder` | object | 同 §9（表格列：机型/操作系统/申请数量/地域/可用区/计费模式/需求类型，行尾编辑） |

> 与早期建议契约相比：去掉了 `config_id` 与 `banner`（后端不下发），前端预提单卡不再渲染顶部横幅。

## 4.5 模板 D：确认提交申请单 `tool_confirm.interrupt.create_cvm_apply`

> **数据来源**：用户在 B 确认配置后，agent 进入 `tool_confirm` 中断，返回最终待提交的申请单（**全只读、不可编辑、无操作列**），用户在卡片上选择动作完成提交。

`value.value`（真实样例，注：最终协议**不含 `actions`**，按钮前端固定渲染）：

```json
{
  "data": {
    "body_param": {
      "bk_username": "yunyaoyang",
      "expect_time": "2026-06-18 00:00:00",
      "require_type": 1,
      "suborders": [
        {
          "replicas": 10,
          "resource_type": "QCLOUDCVM",
          "spec": {
            "charge_months": 1,
            "charge_type": "PREPAID",
            "data_disk": [{ "disk_num": 1, "disk_size": 500, "disk_type": "CLOUD_PREMIUM" }],
            "device_type": "SA2.MEDIUM4",
            "image_id": "img-fjxtfi0n",
            "network_type": "TENTHOUSAND",
            "region": "ap-nanjing",
            "res_assign": 1,
            "resource_mode": 0,
            "system_disk": { "disk_num": 1, "disk_size": 100, "disk_type": "CLOUD_PREMIUM" },
            "zone": "all"
          }
        }
      ]
    },
    "path_param": { "bk_biz_id": "213" }
  },
  "tool": "create_biz_apply"
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.body_param.require_type` | string\|number | 需求类型编码，**全部 suborder 共用**（拍平到每行展示） |
| `data.body_param.suborders[].replicas` | number | 申请数量（在 suborder 外层，**非 spec 内**） |
| `data.body_param.suborders[].spec` | object | 规格，字段同 §9 并多出 `charge_months` / `network_type` / `resource_mode`；可用区为 `zones: string[]`（如 `["all"]`，前端兼容旧 `zone`） |
| `tool` / `path_param` | - | 后端工具名 / 业务 id，前端透传不展示 |

> **结构差异（与 A/B 不同）**：此处 `replicas`/`resource_type` 在 suborder 外层、规格在 `spec` 内、`require_type` 在 `body_param` 顶层。前端 `getSubmitRows` 把它们拍平为 `HostApplySuborder`，复用预提单表格（`readonly` 隐藏操作列、`show-disk` 增列系统盘/数据盘）。
> **按钮**：协议无 `actions`，前端固定渲染两个——「确认提交」（resume 回传 suborders）/「添加到配置清单」（跳转主机申请页，同 A/B）。
> **卡片文案**：交互态横幅「已确认 N 条申领配置方案，可点击按钮提交申请单」；只读回放摘要「请确认全部方案信息，并提交申请单」。

## 5. 模板 C：调整配置弹窗（前端，无独立下行）

- 由 B 行内编辑触发，初始值 = 该行 `suborder`，编辑后 `保存修改` 写回该行 `suborder`。
- **仅左侧表单**字段：`replicas`(必填) / `region`(必填) / `device_type`(必填) / `image_id` / `res_assign` / `require_type` / `charge_type`(只读，「基于预测状态自动推导」) / `system_disk`(必填) / `data_disk`。
- **选择类字段候选**：本迭代仍用前端静态枚举（磁盘类型已对齐后端编码 `CLOUD_PREMIUM` 等）；地域/机型/操作系统等动态候选待后端补（QA5）。

## 6. 上行协议（提交回写，沿用 jsonschema `/agui`）【⚠️ 承载形式待后端确认】

载体：复用 `/agui`，`forwardedProps` 透传（同 jsonschema `resumeValue` 机制）。两次提交：

**① A「选择方案」**（前端当前实现）：

```json
{ "messages": [{ "role": "user", "content": "我选择该申领方案" }],
  "forwardedProps": { "resumeValue": "帮我基于此方案进行拆单<选中 suborder 的 JSON 字符串>" } }
```

**② B「确认方案」**（前端当前实现，含 C 调整）：

```json
{ "messages": [{ "role": "user", "content": "确认申领配置" }],
  "forwardedProps": { "resumeValue": "<完整 suborders 数组的 JSON 字符串>" } }
```

**③ D「确认提交申请单」**（前端当前实现，仅「确认提交」按钮回写；「添加到配置清单」走跳转不回写）：

```json
{ "messages": [{ "role": "user", "content": "确认提交" }],
  "forwardedProps": { "resumeValue": "<D 的整个 data（含 body_param/path_param 两个 key）的 JSON 字符串>" } }
```

> ⚠️ **假设说明**：A、B、D 统一走 `forwardedProps.resumeValue` 通道——A 回传选中 `suborder` 的 JSON 串、B 回传 `suborders` 数组的 JSON 串、D 回传整个 `data` 对象（含 `body_param` 与 `path_param` 两个 key）的 JSON 串。
> 实际 resume 期望值（索引 / 对象 / 字符串）**待后端确认**；前端把回传逻辑集中在 `chat-message-list.vue` 的 `handleSelectPlan` / `handleConfirmPreorder` / `handleSubmitConfirm`，确认后改这几处即可。

## 7. 只读 / 历史回放（沿用 jsonschema）

- 模板 A/B 的 `CUSTOM` 事件按 jsonschema 同样方式持久化与重建（`use-stream.ts` `toHistoryMessage` 按 name 分流）。
- 提交后转只读展开态（默认收起、可展开、展开不可交互）。
- 只读还原：A 用 `__selectedIndex`（实时流写入）；历史回放无该字段时退化为首条。B 用 `__confirmedSuborders`；历史无该字段时退化为原始 `suborders`。
- ⚠️ 历史 `resume` 权威值字段【待后端确认】，跨页/切会话的「已选方案」精确还原暂为 best-effort。

## 8. 跨页跳转（「添加到配置清单」，本迭代仅跳转）

- 仅跳转到主机申请页面并**预留携带数据入口**（路由走 `routerAction`），目标路由 `MENU_SERVICE_HOST_APPLICATION`。
- 回填配置清单的数据契约与「联动打开 AI 助手」**本迭代不做**，留后续「回填」需求。

## 9. 前端数据模型（已对齐后端 suborder）

```typescript
export const HOST_APPLY_RECOMMEND_EVENT = 'after_tool_hitl.recommend_select.interrupt';
export const HOST_APPLY_CONFIRM_EVENT = 'after_tool_hitl.recommend_suborder_confirm.interrupt';
export const HOST_APPLY_SUBMIT_EVENT = 'tool_confirm.interrupt.create_cvm_apply';

interface HostApplyDisk {
  disk_type: string; // 磁盘类型编码，如 CLOUD_PREMIUM
  disk_size: number; // GB
  disk_num: number;  // 数量
}

interface HostApplySuborder {
  require_type?: string | number; // 需求类型编码（如 7）
  region?: string;                // 地域编码（如 ap-nanjing）
  zone?: string;                  // 可用区编码（如 all）
  device_type?: string;           // 机型编码（如 S3.MEDIUM4）
  image_id?: string;              // 镜像 id（如 img-fjxtfi0n）
  res_assign?: string | number;   // 资源分配方式编码（如 1）
  replicas?: number;              // 申请数量
  charge_type?: string;           // 计费模式编码（如 PREPAID）
  system_disk?: HostApplyDisk;
  data_disk?: HostApplyDisk[];
}

interface HostApplyRecommendValue {
  checkpoint_id: string;
  lineage_id: string;
  value: { recommendations: Array<{ source: string; suborder: HostApplySuborder }> };
}

interface HostApplyPreorderValue {
  checkpoint_id: string;
  lineage_id: string;
  value: { suborders: HostApplySuborder[] };
}

// 模板 D：嵌套结构（replicas/resource_type 在外层，规格在 spec）
interface HostApplySubmitSuborder {
  replicas?: number;
  resource_type?: string;
  spec: HostApplySuborder & { charge_months?: number; network_type?: string; resource_mode?: number };
}

interface HostApplySubmitValue {
  checkpoint_id: string;
  lineage_id: string;
  value: {
    data: {
      body_param: { bk_username?: string; expect_time?: string; require_type?: string | number; suborders: HostApplySubmitSuborder[] };
      path_param?: { bk_biz_id?: string };
    };
    tool: string;
  };
}
```

## 10. 待确认问题（仍依赖后端）

| 编号 | 问题 | 状态 |
|---|---|---|
| ✅ QA1 | 模板 `name` 实际命名 | 已确认：`recommend_select.interrupt` / `recommend_suborder_confirm.interrupt` |
| ✅ QA2 | `suborder` 字段名与值形态 | 已确认：字段如上，值为编码/枚举码 |
| ⚠️ QA3 | 上行 resume 承载形式（索引 / suborder / 完整 suborders） | 【待后端】前端暂按 §6 假设，逻辑已隔离 |
| ⚠️ QA4 | 历史 `resume` 权威值字段 | 【待后端】暂沿用 jsonschema 还原策略（best-effort） |
| ⚠️ QA5 | 编码→中文映射 & C 表单动态选项来源 | 【待后端/后续】本期原样展示编码、静态枚举 |
| QA6 | 「添加到配置清单」回填数据契约 | 本迭代不做，留后续需求 |
