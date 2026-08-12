# 规范：预算申报类型与管理员免审（Story 13641767）

> 需求 ID：1069995598136417767  
> 技术类型：backend（纯后端；本期不含前端变更）  
> 版本：1.1  
> 日期：2026-07-23

## 概述

为资源预测主单新增「预算申报」（`budget_declare`）类型，供 finops 通过 `overwrite_append` 显式创建预算同步单据；该类主单下子单跳过 HCM 管理员审批（含跨年），CRP 管理员节点不跳过；后端 meta/列表接口支持该类型识别与筛选。**本期不改前端代码。**

## 用户故事

### US-1：finops 预算同步（P0）

**作为** finops 系统，  
**我想要** 在调用资源预测 `overwrite_append` 时必填 `type=budget_declare` 并创建预算申报主单，  
**以便于** 预算同步单据在审批链路中跳过 HCM 管理员且可被运营识别。

**验收要点**：
- 请求必须携带 `type`，预算同步场景值为 `budget_declare`
- 主单落库类型为「预算申报」；子单类型与覆盖/追加行为仍由明细决定
- 子单 HCM 管理员审批 skip；CRP 管理员不 skip

### US-2：后端类型露出与创建限制（P0）

**作为** 调用方/运营系统，  
**我想要** 通过 meta/列表接口识别并筛选「预算申报」主单，且非 overwrite_append 入口无法创建该类型，  
**以便于** 预算同步单据可被后端检索，且不会被普通创建 API 误用。

**验收要点**：
- meta `ticket_types` 含「预算申报」
- 列表 `ticket_types` 筛选参数接受 `budget_declare`
- 详情接口返回 `type_name=预算申报`（前端展示本期不改）
- 非 overwrite_append 创建 API 拒绝 `budget_declare`

## 功能需求

### FR-001 主单类型枚举

系统 SHALL 在资源预测主单类型中新增 `budget_declare`，展示名「预算申报」。  
系统 SHALL NOT 在子单类型枚举中新增「预算申报」。

### FR-002 overwrite_append 必填 type（破坏性变更）

`POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/overwrite_append` 请求体 SHALL 包含必填字段 `type`（string）。

| 取值 | 场景 |
|------|------|
| `budget_declare` | finops 预算同步 |
| `add` | 显式指定主单为新增 |
| `adjust` | 显式指定主单为调整 |
| `delete` | 显式指定主单为取消 |

系统 SHALL NOT 对 `type` 默认赋值或按 cancel/add 明细自动推导主单类型。  
未传 `type` 或非法枚举值 SHALL 返回参数错误，不创建主单。

### FR-003 主单类型与拆单路径

当 `type=budget_declare` 时：
- 主单 `type`/`type_name` SHALL 为请求指定值（预算申报）
- 拆单 SHALL **统一**走 `SplitAdjustTicket` 路径（与主单 `adjust` 相同），SHALL NOT 再按 cancel/add 选择 SplitAdd/SplitDelete
- 覆盖、追加、删除的**数据处理** SHALL 仍由 cancel/add 明细经 adjust 拆单内部逻辑完成
- 子单 `sub_type` SHALL NOT 为 `budget_declare`；常规 cancel/add 组合经 adjust 路径合并后多为 `adjust`（transfer/delay 等特殊组仍按现网 adjust 拆单产出）

### FR-004 HCM 管理员免审

当主单 `type=budget_declare` 时，其下所有子单进入 HCM 管理员审批阶段时：
- `admin_audit_status` SHALL 为 `skip`
- 该规则 SHALL 适用于含跨年明细的子单（不受「非本年度不可 skip」限制）

CRP 侧管理员审批节点 SHALL 保持现有逻辑，不因主单为 `budget_declare` 自动跳过。

`skip_itsm` 开关语义 SHALL 不变。

### FR-005 后端类型露出与筛选

系统 SHALL 在后端露出「预算申报」：
- meta 接口 `ticket_types` 列表包含该类型
- 资源预测主单列表请求的 `ticket_types` 筛选接受 `budget_declare`
- 主单详情/列表响应中 `type`/`type_name` 正确返回

本期 SHALL NOT 修改 `front/` 下任何代码。前端展示/TS 类型适配若有需要，划归后续需求。

### FR-006 创建入口限制

除资源预测 `overwrite_append` 外，所有创建/调整资源预测主单 API SHALL 拒绝 `budget_declare`（后端校验）。

### FR-007 退回计划不变

退回计划 `overwrite_append` 及类型体系 SHALL 保持本期改造前行为（AC-008）。

## 验收场景（Given-When-Then）

### 主流程

**AC-001**  
- **Given** 调用资源预测 overwrite_append 且 `type=budget_declare`  
- **When** 创建主单成功  
- **Then** 主单类型为「预算申报」，详情「需求类型」展示「预算申报」

**AC-002**  
- **Given** 调用 overwrite_append 未传 `type`  
- **When** 提交请求  
- **Then** 返回参数错误，不创建主单

**AC-003**  
- **Given** 主单 `type=budget_declare` 且明细含覆盖与/或追加  
- **When** 拆子单  
- **Then** 走 `SplitAdjustTicket`；子单 `sub_type` 不为 `budget_declare`；覆盖/追加行为按明细处理（常规组多为 `adjust`）

**AC-004**  
- **Given** 主单 `type=budget_declare`（含跨年明细）  
- **When** 子单进入 HCM 管理员审批阶段  
- **Then** 管理员审批被跳过（`admin_audit_status=skip`）

**AC-005**  
- **Given** 主单 `type=budget_declare` 的子单进入 CRP  
- **When** 到达 CRP 管理员节点  
- **Then** 不因本类型自动跳过该节点

**AC-006**  
- **Given** 调用非 overwrite_append 的资源预测主单创建 API，且 `type=budget_declare`  
- **When** 提交  
- **Then** 返回参数/业务错误，不创建主单

**AC-007**  
- **Given** 主单列表存在 `budget_declare` 类型单据  
- **When** 列表请求 `ticket_types` 含 `budget_declare`，或查询 meta `ticket_types`  
- **Then** 列表仅返回该类型主单；meta 含「预算申报」选项

**AC-008**  
- **Given** 退回计划 overwrite_append  
- **When** 调用  
- **Then** 行为与本期前一致

### 补充技术验收

**AC-T01**  
- **Given** overwrite_append 显式传 `type=add` 且明细仅含 add  
- **When** 创建成功  
- **Then** 主单类型为「新增」

**AC-T02**  
- **Given** 主单 `budget_declare` 且明细含非本年度期望时间  
- **When** 拆子单  
- **Then** HCM 管理员仍为 skip

**AC-P01**  
- **Given** 正常 overwrite_append 请求（含必填 `type`）  
- **When** 调用接口  
- **Then** 仍异步快速返回主单 ID，P95 < 1s

**AC-S01**  
- **Given** 调用方无 `biz_resource_plan_operate` 权限  
- **When** 调用 overwrite_append  
- **Then** 拒绝调用

## 边界与范围

### 本期包含

- 资源预测主单 `budget_declare` / 「预算申报」
- overwrite_append 必填 `type`，支持 `budget_declare` | `add` | `adjust` | `delete`
- `budget_declare` 主单下子单 HCM 管理员一律 skip（含跨年）；CRP 管理员不 skip
- 列表筛选参数、meta、详情响应中的 type/type_name（后端）
- 限制仅 overwrite_append 可传 `budget_declare`

### 本期不包含

- 退回计划类型与审批改造
- 前端代码变更（TS 类型、列表列渲染、表单选项等）
- 修改 CRP 管理员自动过单策略
- finops 侧预算换算逻辑

### 边界场景

| 场景 | 预期行为 |
|------|---------|
| 仅覆盖不追加 + `type=budget_declare` | 主单为预算申报；拆单走 SplitAdjust；cancel 行为执行；常规子单多为 adjust |
| 仅追加 + `type=budget_declare` | 主单为预算申报；拆单走 SplitAdjust；追加行为执行；常规子单多为 adjust |
| 覆盖+追加 + `type=budget_declare` | 主单为预算申报；拆单走 SplitAdjust；覆盖+追加均执行 |
| 显式 `type=adjust` + 混合明细 | 主单为调整；行为与明细一致 |
| 非法 `type` 值 | 参数错误 |
| 非 overwrite_append 传 `budget_declare` | 参数/业务错误 |

## 接口契约（增量）

**路径**：`POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/overwrite_append`

**新增/变更**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | **是** | `budget_declare` \| `add` \| `adjust` \| `delete` |

**破坏性说明**：已上线调用方必须同步增加 `type` 参数；finops 预算同步须传 `budget_declare`；其他调用方可根据明细语义显式传 `add`/`adjust`/`delete` 以保持与旧自动推导一致的主单类型。

其余字段与父需求 overwrite_append 契约一致。

## 数据模型

| 实体 | 字段 | 变更 |
|------|------|------|
| `res_plan_ticket` | `type` | 允许 `budget_declare` |
| `res_plan_ticket` | `type_name` | 允许「预算申报」 |
| `res_plan_sub_ticket` | `sub_type` | 不新增 budget_declare |
| `res_plan_sub_ticket` | `admin_audit_status` | budget_declare 主单下可为 skip |

## 非功能需求（NFR）

| 维度 | 要求 |
|------|------|
| 性能 | overwrite_append 异步返回主单 ID；P95 < 1s；type 校验不引入额外同步阻塞 |
| 安全 | 沿用 `biz_resource_plan_operate` |
| 可用性 | 无新增 SLA |
| 兼容性 | overwrite_append 破坏性变更；须 finops 联调；退回计划不改 |
| 容错 | 子单失败可单独重试（沿用现网） |

## 假设

1. finops 可在同一发布窗口同步修改 overwrite_append 调用，传入 `type=budget_declare`。
2. 除 finops 外，现有 overwrite_append 调用方可根据明细语义显式传 `add`/`adjust`/`delete`，无需继续使用自动推导。
3. CRP 管理员审批节点逻辑无需本需求改动即可满足「不跳过」要求。
4. 人工创建 API 本身不暴露可选 `budget_declare`；F-006 由后端拒绝保障，本期不改前端。
5. `GetRPTicketTypeMembers()` 扩展后，meta `ticket_types` 与列表筛选参数自动支持新选项；前端 UI/TS 适配不在本期范围。

## 依赖

| 依赖 | 说明 |
|------|------|
| 父需求 1069995598135604383 | overwrite_append 基础能力已上线 |
| finops | 调用方须同步改参 |
| CRP / ITSM | 现有审批链路 |

## 成功标准

1. finops 可通过 `type=budget_declare` 创建可识别的预算申报主单，且子单不经 HCM 管理员人工审批（在其余条件满足时进入后续阶段）。
2. 后端 meta/列表/详情接口可正确暴露并筛选「预算申报」；前端展示适配如需，另开需求。
3. 未同步传 `type` 的旧调用在 overwrite_append 上得到明确参数错误（可观测、可推动联调）。
4. 退回计划行为无回归。

## 澄清引用

技术澄清结论见 `req.md`「技术澄清」章节；问题审计见 `questions.md`（Q1–Q10，均为 `resolved_by_doc`，无 open 阻塞项）。
