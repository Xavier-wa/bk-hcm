# API - 资源预测 2027 CVM 预算提报 · 修改入口 + 截止期限制

- **Requirement**: req-resource-plan-2027-cvm-budget
- **Iteration**: feat-resplan-cvm-reqmodify
- **后端 MR**: https://&lt;GIT_HOST&gt;/bcc/hcm/-/merge_requests/3093 (修改接口与新增接口结构一致)

> 本文档列出本迭代两个功能点 (修改入口 / 截止期限制) 涉及的所有前端接口调用与聚合判断逻辑。

## 1. 接口清单

### 1.1 覆盖修改主单 (新增接口) ★ 本迭代新增

来源: MR 3093 `overwrite_biz_resource_plan_ticket.md` (v9.9.9+)

- **URL**: `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/{ticket_id}/overwrite`
- **路径参数**:
  - `bk_biz_id` (int, 必选)
  - `ticket_id` (string, 必选, 原主单 ID)
- **请求体** (所有字段均**可选**, 仅更新传入字段, 未传保持原值):
  ```ts
  {
    demand_class?: string;       // 'CVM' | 'CA' (修改 demands 时建议传)
    demands?: IPlanTicketDemand[]; // 整体替换主单 demands; 结构同新增接口
    remark?: string;             // 长度 [20, 1024]
  }
  ```
  > **前端实际行为**: 修改页表单为整体回填 + 整体提交, 即使用户只改一个字段也提交全量 (`demand_class` + `demands` + `remark`), 简化前端逻辑。
- **响应**: `{ code: 0, message: 'success', data: null }` — **无 id 返回**, 前端用原 `ticket_id` 直接跳回原详情
- **后端业务约束**:
  - 修改成功后, 主单状态自动重置为「审批中」, 原子单被解绑作废 (子单 ticket_id 变为 `drop_{主单ID}`, 状态为 failed)
  - **若存在任意已完成 (done) 子单则不允许覆盖** — 后端硬约束, 前端聚合规则需与此对齐
- **后端可覆盖主单状态白名单**:
  | 状态值 | 说明 |
  |---|---|
  | `rejected` | 审批驳回 |
  | `partial_rejected` | 部分审批驳回 |
  | `failed` | 失败 |
  | `partial_failed` | 部分失败 |
  | `revoked` | 已撤销 |
- **前端调用位置**: 修改页 Button (复用新增页 Button 结构, 内部分支调不同 action)
- **跳转**: 提交成功后 `routerAction.redirect` 跳回原单据详情页, 详情页自动 reload 看到「审批中」状态
- **错误处理**: 后端返回非 0 `code` 时, 前端 toast `message` (例: 已存在 done 子单尝试覆盖会被后端拦截)

### 1.2 单据详情 (已有, 复用)

- **业务视角**: `GET /api/v1/woa/bizs/{bizId}/plans/resources/tickets/{id}` → `TicketByIdResult`
- **响应关键字段** (修改功能依赖):
  - `id`
  - `base_info` (含 `type` / `bk_biz_id` / `bk_biz_name` / `op_product_id` / `plan_product_id` / `virtual_dept_id` / `remark` / `submitted_at`)
  - `status_info.status` — 主单状态 (枚举见 §2.1)
  - `demands[].original_info` / `demands[].updated_info` / `demands[].demand_class`
- **store 方法**: `useResourcePlanStore().getBizResourcesTicketsById(bizId, id)`
- **用途**:
  - 修改页**初次加载**: 拉取后回填 Basic / List / Memo
  - 详情页**入口可见性判断**: 提供主单 `status`

### 1.3 子单列表 (已有, 复用)

- **业务视角**: `POST /api/v1/woa/bizs/{bizId}/plans/resources/sub_tickets/list` → `SubTicketsResult`
- **响应关键字段**: `details[].status` — 子单状态 (枚举见 §2.2)
- **store 方法**: `useResSubTicketStore().getList({ ticket_id }, bizId)`
- **用途**: 详情页**入口可见性判断**所需的子单状态聚合

### 1.4 非本年预测提报截止日期 (新接口) ★ 本迭代新增

来源: 后端 v9.9.9 新增配置接口, 全局配置, 无业务隔离, 无鉴权。

- **URL**: `GET /api/v1/woa/plans/resources/tickets/report_deadline`
- **请求参数**: 无
- **响应**:
  ```ts
  {
    code: 0,
    message: 'ok',
    data: {
      deadline: string;  // 'YYYY-MM-DD'; 空字符串 = 未配置 = 不做截止限制
    }
  }
  ```
- **store 方法** (本迭代新增): `useResourcePlanStore().getReportDeadline()` → `Promise<{ data: { deadline: string } }>`
- **调用时机**:
  - 进入「资源预测添加」页面时拉取 (用于禁选未来日期)
  - 进入「资源预测列表」 (业务侧 + 服务侧共用组件) 时拉取 (用于行禁用)
  - **不做全局缓存 / 不在 App 启动时拉取**
- **失败兜底**: 接口失败 / 超时 → `deadline = ''` (按申报期处理, 不阻断用户)

## 2. 字段语义与枚举

### 2.1 主单状态 `TicketByIdResult.status_info.status`

| 枚举值 | 含义 | 业务语义 |
|---|----|----|
| `init` | 初始化 | 单据刚提交, ITSM 流转中 |
| `auditing` | 审批中 | 在某审批节点 (itsm / crp / admin) 流转 |
| `rejected` | 全部驳回 | 全部子单审批被拒 |
| `partial_rejected` | 部分驳回 | 部分子单审批被拒, 其他可能通过 / 失败 |
| `done` | 已完成 | 全部子单审批通过且执行完成 |
| `failed` | 全部失败 | 全部子单执行失败 |
| `partial_failed` | 部分失败 | 部分子单执行失败, 其他可能通过 / 驳回 |
| `revoked` | 已撤销 | 提单人主动撤回 |
| `terminated` | 已终止 | 管理员 / 系统终止 (例如点「终止」按钮) |

### 2.2 子单状态 `SubTicketItem.status`

来源: `store/ticket/res-sub-ticket.ts` 中的 `STATUS_ENUM`

| 枚举值 | 含义 |
|---|---|
| `init` | 待审批 |
| `auditing` | 审批中 |
| `rejected` | 审批拒绝 |
| `failed` | 失败 (执行失败) |
| `done` | 成功 (审批通过且执行完成) |
| `invalid` | 已失效 |

## 3. 「可修改」聚合判断逻辑 ★ 前端核心逻辑

### 3.1 输入

- 主单详情: `ticketDetail` (`TicketByIdResult`)
- 子单列表: `subTickets` (`SubTicketItem[]`)
- 上下文: `isBusinessPage` (是否业务视角)
- 单据类型: 不在前端按 `demand_class` 过滤; 资源预测仅 CVM / CA, overwrite 接口本身按白名单拦截

### 3.2 推导规则

与后端 MR 3093 「可覆盖主单状态白名单 + 不允许存在 done 子单」对齐。

```ts
// 伪代码
const OVERWRITABLE_MAIN_STATUS = new Set([
  'rejected',
  'partial_rejected',
  'failed',
  'partial_failed',
  'revoked',
]);

function isTicketModifiable(
  ticketDetail: TicketByIdResult | undefined,
  subTickets: SubTicketItem[] | undefined, // 可能为空数组 / undefined
  isBusinessPage: boolean,
): boolean {
  // 1. 仅业务视角
  if (!isBusinessPage) return false;

  // 2. 主单详情未就绪 → 不展示 (没有 detail 无法判断 status)
  if (!ticketDetail) return false;

  // 3. 主单状态必须在后端可覆盖白名单中
  if (!OVERWRITABLE_MAIN_STATUS.has(ticketDetail.status_info?.status)) {
    return false;
  }

  // 4. 子单中不能有 done; subTickets 为 undefined / [] 均视为无 done, 允许修改
  //    (不再按 demand_class 过滤: 资源预测仅 CVM / CA 两类, 后端 overwrite 接口本身按白名单拦截)
  if (subTickets?.some(s => s.status === 'done')) return false;

  return true;
}
```

### 3.3 判断表 (穷举主单状态 × 子单情况)

| 主单 status | 子单情况 | 可修改? | 备注 |
|---|----|---|----|
| `init` | 任意 | 否 | 审批中, 不在白名单 |
| `auditing` | 任意 | 否 | 审批中, 不在白名单 |
| `done` | 全部 done | 否 | 已完成, 不在白名单 |
| `terminated` | 任意 | 否 | 已终止, 不在白名单 |
| `rejected` | 任意 (一般全部 rejected) | **是** | 全部驳回 |
| `failed` | 任意 (一般全部 failed) | **是** | 全部失败 |
| `partial_rejected` | 含 done | 否 | 有通过子单, 后端硬拦截 |
| `partial_rejected` | 无 done | **是** | 部分驳回无通过 |
| `partial_failed` | 含 done | 否 | 有通过子单, 后端硬拦截 |
| `partial_failed` | 无 done | **是** | 部分失败无通过 |
| `revoked` | 含 done | 否 | 有通过子单, 后端硬拦截 |
| `revoked` | 无 done | **是** | 已撤销, 允许修改重提 (与后端对齐) |

### 3.4 数据时效性

- 详情页轮询周期 30s, 主单与子单数据同步刷新
- 「修改需求」按钮的可见性需要**响应式依赖**主单 status + 子单列表; 状态翻转时按钮自动显隐
- 修改页提交前不做客户端预校验状态翻转 (与 design.md §2.6 一致), 由后端拦截

### 3.5 数据未就绪兜底

- **主单详情**请求失败 / loading: `isTicketModifiable = false` (无 status 无法判断)
- **子单列表 undefined / 空数组 `[]`**: 均视为无 done 子单, `isTicketModifiable = true` (按主单状态判断)
- **子单列表有数据但无 done**: `isTicketModifiable = true`

## 4. 「截止期限制」聚合判断逻辑 ★ 前端核心逻辑

### 4.1 输入

- 后端截止日: `deadline` (来自 §1.4 接口, `YYYY-MM-DD` 或 `''`)
- 前端非本年起始日: `NON_CURRENT_YEAR_START_DATE` (前端常量, 默认 `'2027-01-01'`, 可为 `''`)
- 当前日期: `today` (`YYYY-MM-DD`, 按客户端本地时区)

### 4.2 推导规则

```ts
// 伪代码 - 是否处于评审期 (=即是否启用截止期限制)
function isInReviewPhase(deadline: string, today: string): boolean {
  if (!deadline) return false; // 接口返回空 → 申报期
  // 严格大于: 过了 deadline 那一刻进入评审期 (精确到秒)
  // 后端约定 deadline 格式 'YYYY-MM-DD HH:MM:SS'; 定长字符串字典序 === 时序, 无需 dayjs 转换
  // 期望到货日期 / 起始日的字段是 YYYY-MM-DD, 比较时各自纯日期粒度, 不混用
  return now > deadline;
}

// 伪代码 - 期望到货日期是否属于「非本年」
function isNonCurrentYearDate(expectTime: string, startDate: string): boolean {
  if (!startDate) return false; // 起始日常量为空 → 不视为非本年
  if (!expectTime) return false;
  return expectTime >= startDate;
}

// 伪代码 - 添加表单中, 某个候选日期是否应禁选
function shouldDisableDateInAddForm(
  candidateDate: string,
  deadline: string,
  startDate: string,
  today: string,
): boolean {
  if (!isInReviewPhase(deadline, today)) return false; // 申报期 → 不禁选
  return isNonCurrentYearDate(candidateDate, startDate);
}

// 伪代码 - 列表中, 某行操作是否应禁用
function shouldDisableRowOps(
  rowExpectTime: string,
  deadline: string,
  startDate: string,
  today: string,
): boolean {
  if (!isInReviewPhase(deadline, today)) return false; // 申报期 → 不禁用
  return isNonCurrentYearDate(rowExpectTime, startDate);
}
```

### 4.3 判断表 (穷举两侧空值组合)

| `deadline` | 起始日常量 | 行为 |
|---|---|---|
| `''` | 任意 | 申报期, 无任何限制 |
| 非空 | `''` | 视为不限制 (前端关闭限制) |
| 非空 且 `now ≤ deadline` | 非空 | 申报期, 无任何限制 |
| 非空 且 `now > deadline` | 非空 | 评审期, 启用限制 (过了 deadline 那一刻起, 精确到秒) |

### 4.4 数据时效性

- `report_deadline` 接口在添加页 / 列表页**各自挂载时拉取一次**, 不缓存
- 同一页面停留期间不重新拉取 (跨日不刷新), 由项目其他场景的页面 reload 自然带入
- 「非本年起始日」常量改动需重新发版, 不依赖配置中心

### 4.5 与修改入口的关系

- 修改入口 (§3) 与修改页**不应用** §4 任何限制
- 详情页 / 修改页**不拉取** `report_deadline`
- 即评审期内: 业务侧详情页仍可见「修改需求」入口, 修改页期望到货日期 picker 无禁选, 可正常提交 2027 单据

## 5. 修改页数据流

```
进入修改页 (带 ticket_id, bk_biz_id query)
  │
  ▼
useResourcePlanStore().getBizResourcesTicketsById(bizId, id)
  │
  ├─ success
  │   └─ 把 TicketByIdResult.base_info / demands / remark
  │       映射成 IPlanTicket 结构, 回填表单
  │       (注意 demands 类型映射: TicketByIdResult.demands[].updated_info
  │        → IPlanTicketDemand; 取 updated_info 还是 original_info 待确认)
  │
  └─ fail
      └─ 整页 error 兜底 (toast + 返回详情页 / 或停留在错误态由用户重试)

提交
  │
  ▼
POST /api/v1/woa/bizs/{bizId}/plans/resources/tickets/{id}/modify
  │
  ├─ success → toast 成功 → routerAction.redirect 回详情页 → 详情页重新拉取
  └─ fail → toast 错误, 表单保留
```

### 5.1 字段映射: `TicketByIdResult.demands[]` → `IPlanTicketDemand[]`

`TicketByIdResult.demands` 是 `{ original_info, updated_info, demand_class }[]` 结构, 表单使用的是 `IPlanTicketDemand`。后端接口对 demands 是**整体替换**, 无需带 `demand_id`。

| 目标字段 (提交) | 来源 (回填) | 备注 |
|---|----|----|
| `obs_project` | `updated_info.obs_project` | 取 updated_info (用户最近一次的预测意图) |
| `expect_time` | `updated_info.expect_time` | `YYYY-MM-DD` |
| `return_plan_time` | `updated_info.return_plan_time` | OBS 项目为「短租项目」时必填 |
| `region_id` / `zone_id` | `updated_info.region_id` / `updated_info.zone_id` | |
| `region_name` / `zone_name` | 通过 meta 接口 (`getRegions` / `getZones`) 反查拼接 | 详情接口未返回 name, 前端展示需要 |
| `demand_source` | `updated_info.demand_source` (假设详情会返回) | 若详情接口未返回, 修改场景下沿用空值; 字段为可选 |
| `demand_res_types` | `updated_info.demand_res_types` | `['CVM', 'CBS']` |
| `cvm.res_mode` / `device_type` / `cpu_core` / `memory` / `os` | `updated_info.cvm.*` | `os` 字段后端约定单位「台」 |
| `cbs.disk_type` / `disk_io` / `disk_size` | `updated_info.cbs.*` | |
| 顶层 `demand_class` | `demands[0].demand_class` 或 base_info | 固定 `'CVM'` (本迭代仅 CVM) |
| 顶层 `remark` | `base_info.remark` | 长度 [20, 1024] |
| `bk_biz_id` | route `bk_biz_id` query / `base_info.bk_biz_id` | 路径参数, 不放请求体 |

**前端处理要点**:
- demands 整体替换, **不需要 demand_id**
- 回填取 `updated_info` 而非 `original_info` (原始值是历史快照)
- region/zone 的中文名前端自行通过 meta 接口拼接, 与新增页处理方式一致
- `demand_source` 若详情接口未返回则展示为空, 用户可自行选择 (该字段为可选)

## 6. 错误处理 (前端关心的部分)

- **网络错误**: 走全局 http 拦截器, toast 提示
- **403 权限**: 业务视角下用户不应到这一步, 若到达, 走全局权限申请弹窗 (`window.hcmPermissionDialog`)
- **业务错误 (例如「单据已不可修改」)**: 后端按现有 `{ code, message }` 结构返回, 前端 toast `message`
- **校验错误 (字段)**: 沿用新增页校验逻辑, 不依赖后端
- **`report_deadline` 失败**: 按申报期兜底, 控制台 warn, **不弹 toast** (避免打扰用户)

## 7. 已确认决策

| # | 决策 | 来源 |
|---|----|---|
| 1 | 修改接口: `POST .../{ticket_id}/overwrite`, 字段可选, response 无 id | MR 3093 |
| 2 | demands 整体替换, 无需 `demand_id` | MR 3093 |
| 3 | 主单 + 子单聚合判断「可修改」, 与后端可覆盖白名单 + done 子单硬规则对齐 | MR 3093 + 用户 |
| 4 | 详情用 `getBizResourcesTicketsById` 复用 | 用户 |
| 5 | `terminated` 状态归入**不可修改** | 用户 + MR 3093 (不在白名单) |
| 6 | `revoked` 状态归入**可修改** (与后端对齐) | 用户 (方案 B) + MR 3093 |
| 7 | **主单详情**未就绪 → 按钮不展示; **子单 undefined / `[]`** → 均视为无 done, 按主单状态判断 | 用户 2026-05-20 |
| 8 | 提交成功后用原 `ticket_id` 跳回详情页 (响应无 id) | MR 3093 |
| 9 | `report_deadline` 接口**只在添加 / 列表页按需拉取**, 不全局缓存 | 用户 2026-05-20 |
| 10 | 「非本年起始日」做成前端常量 (`'2027-01-01'` 默认), 空值=不限制 | 用户 2026-05-20 |
| 11 | `deadline` / 起始日**任一为空** → 申报期 (无限制) | 用户 2026-05-20 |
| 12 | 评审期内: 修改入口 / 修改页**不应用**截止期限制 | 用户 2026-05-20 |
| 13 | 列表禁用范围: 业务侧 + 服务侧 (共用组件); 资源侧无此列表 | 用户 2026-05-20 |
| 14 | 列表禁用粒度: 行内 (编辑/撤销/删除) + 批量 (含非本年行的批量操作) | 用户 2026-05-20 |
| 15 | Phase 3 解除: 后端 `deadline` 回空即解除, 前端无额外逻辑 | 用户 2026-05-20 |
| 16 | 接口失败兜底: 按申报期处理, 控制台 warn, 不弹 toast | 用户 2026-05-20 |

> 所有阻塞已闭环, 可进入 coding。`demand_source` / region_name / zone_name 等表单字段, 联调时若详情接口未返回则前端按 §5.1 假设方案处理 (取空 / meta 反查), 无需后端再改。
