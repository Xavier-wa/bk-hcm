# API：推荐卡片展示机型族、CPU、内存

> 本迭代**不修改** agent SSE / CUSTOM 消息协议；`suborder.device_type` 仍只下发机型编码。  
> 机型族 / CPU / 内存由前端调用**已有** WOA 机型目录接口 enrichment。  
> 上一迭代协议见：`.hcmfe/workflow/iter-chatbot-custom-msg-command-template/api.md`  
> 上一迭代展示映射见：`.hcmfe/workflow/feat-chatbot-hostapply/api.md`（其中 `device_type` 曾约定「原样展示」——本迭代在此基础上叠加规格富文本）。

## 0. 结论摘要

| 项 | 结论 |
|----|------|
| Agent / SSE | **无变更** |
| 新增后端接口 | **无** |
| 数据源 | 复用 `POST /api/v1/woa/config/findmany/config/cvm/devicetype` |
| 前端封装 | `useCvmDeviceStore().getOneDevicetype` / `getDeviceTypeFullList` |
| vendor | 本期固定 `VendorEnum.ZIYAN`（与 host apply chatbot 一致） |
| 查不到 / 失败 | 展示回退为原始 `device_type` 编码（PRD AC-002 / AC-006） |

## 1. Agent 协议（不变）

Template A/B/C/D 的 `suborder` 仍仅含：

```ts
device_type?: string; // 例：S5.LARGE8
```

**不要求** agent 下发 `device_family` / `cpu_core` / `memory`。

## 2. 机型目录接口（复用）

### 2.1 接口

| 项 | 值 |
|----|-----|
| Method | `POST` |
| Path | `/api/v1/woa/config/findmany/config/cvm/devicetype` |
| Store | `src/store/cvm/device.ts` → `getOneDevicetype` / `getDeviceTypeFullList` |

### 2.2 按机型编码单条查询（推荐卡片 enrichment）

与现网 backfill（`use-chatbot-backfill.ts` → `fetchCpuCoreMap`）同模式：

**Request（示意）**

```json
{
  "filter": {
    "op": "and",
    "rules": [
      { "field": "vendor", "op": "eq", "value": "ziyan" },
      { "field": "device_type", "op": "eq", "value": "S5.LARGE8" }
    ]
  },
  "page": { "start": 0, "limit": 1, "count": false }
}
```

> 实际 `page` / `count` 由 `enableCount` + `onePageParams()` 组装，coding 阶段跟 store 现有写法即可。

**Response 关注字段**（`ICvmDevicetypeItem`）

| 字段 | 类型 | 用途 |
|------|------|------|
| `device_type` | string | 机型编码（与 suborder 对齐） |
| `device_family` | string | 机型族（如「标准型」） |
| `cpu_core` | number | CPU 核数 |
| `memory` | number | 内存（GB 数值，展示时拼「G」） |

**空结果**：`details` 为空或首条缺失 → 前端视为查不到，回退原始编码。

### 2.3 列表查询（调整配置下拉）

`getDeviceTypeOptions` 已走 `getDeviceTypeFullList`（可按 `vendor` + `region` 过滤）。  
本迭代将选项 `name` 从纯 `device_type` 改为富文本 label，**仍用同一列表接口**，消费同一套字段：

```ts
name: `${device_type} (${device_family}, ${cpu_core}核${memory}G)`
// 缺字段时省略对应片段；完全无规格则 name = device_type
id: device_type  // 选中值不变
```

## 3. 展示契约（前端约定，非后端新协议）

统一格式化函数（建议落在 `host-apply-display.ts` 或共享 helper）：

```
formatDeviceTypeDisplay(device_type, meta?) → string
```

| 输入 | 输出 |
|------|------|
| 有完整 meta | `S5.LARGE8 (标准型, 4核8G)` |
| 仅有部分 meta | 有则拼，缺则省略片段 |
| 无 meta / 查询失败 | `S5.LARGE8` |
| `device_type` 空 | `''`（由调用方过滤或显示 `--`） |

消费点：

| 场景 | 落点 | 用法 |
|------|------|------|
| Template A 键值表 | `toSpecDisplayItems` / `getSpecFieldText('device_type')` | 异步 meta 注入后刷新「机型」行 |
| Template B/D 表格 | `host-apply-preorder-table.vue` 机型列 | 同上 |
| Template C 下拉 | `getDeviceTypeOptions` | 选项 `name` 富文本，`id` 仍为编码 |

## 4. 缓存与批量

| 规则 | 说明 |
|------|------|
| 去重 | 同一会话内按 `device_type` 去重请求 |
| 共享 | A 卡片多方案、B/D 多行共用同一缓存 Map |
| 首屏 | 可先渲染原始编码，meta 返回后原地更新（不阻塞按钮） |
| 失败 | 单条失败不影响其它机型；该条回退编码 |

可选实现：扩展 backfill 的 `fetchCpuCoreMap` 为 `fetchDeviceMetaMap`，返回 `device_type → { device_family, cpu_core, memory }`；或列表接口一次拉齐后建索引。coding 阶段择一，优先复用现有 store 调用。

## 5. 错误与边界

| 场景 | 行为 |
|------|------|
| HTTP 错误 / 超时 | 回退原始 `device_type`，不弹全局错误打断对话 |
| 某字段为 `null`/`undefined` | 拼接时跳过该片段 |
| 非 ZIYAN（预留） | 本期不 enrichment，展示原始编码 |

## 6. 与上一迭代 api.md 的差分

| 字段 | feat-chatbot-hostapply | 本迭代 |
|------|------------------------|--------|
| `device_type` 展示 | 原样编码 | 编码 + 机型族/CPU/内存（配置 enrichment） |
| Agent 字段 | 不变 | 不变 |
| 新接口 | 无 | 无 |

## 7. 待 coding 核对

| 编号 | 问题 | 默认假设 |
|------|------|----------|
| Q1 | 单条 `getOneDevicetype` vs 全量 list 建索引，哪个更省请求 | 卡片少量机型用单条/批量去重；C 下拉继续 list |
| Q2 | `memory` 展示是否需要小数（如 0.5） | 按接口数值直接拼 `G`，不做额外单位换算 |
| Q3 | region 过滤是否影响 enrichment 命中 | 展示 enrichment 按 `vendor + device_type` 查；与 backfill 一致，不强制 region |
