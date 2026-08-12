# 预算同步预测-新增“预算申报”类型，管理员免审

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1069995598136417767 |
| 需求名称 | 预算同步预测-新增“预算申报”类型，管理员免审 |
| 优先级 | High |
| 处理人 | pandafyang |
| 父需求 | 1069995598135604383（预算同步预测-从finops预算生成预测和退回计划） |
| 创建时间 | 2026-07-23 10:27:45 |
| 原始需求文档 | docs/reqs/预算申报类型免审.md |
| 需求类型 | 开发需求（分类 ID：1069995598002822402；分类原始值：迭代任务） |
| 预估工时 | 14.5h |
| 工时估算 | 跨模块逻辑基线；PERT（O=8h / M=14h / P=22h / E=14.5h） |
| 价值规模（RICE） | 88 |

## 需求背景

### 业务背景

父需求已上线资源预测 `overwrite_append` 覆盖追加能力，供 finops 将预算同步为资源预测。当前主单类型仍按明细推导为「新增 / 调整 / 取消」，在审批详情中展示为普通预测类型，无法区分「预算申报」来源；同时预算同步场景需要跳过 HCM 侧管理员审批，但不得跳过 CRP 侧管理员节点。

### 用户故事

作为 **finops 系统**，  
我想要 **在调用资源预测 overwrite_append 时显式指定主单类型为「预算申报」，并让该类主单下的子单跳过 HCM 管理员审批**，  
以便于 **预算同步单据在列表/详情中可识别，且审批链路符合预算申报免审规则**。

作为 **资源预测审批人/运营人员**，  
我想要 **在预测主单列表筛选与详情中看到「预算申报」类型**，  
以便于 **区分预算同步单据与人工提单单据**。

### 需求来源

- **需求渠道**：产品规划（父需求预算同步预测增量）
- **关联需求**：1069995598135604383
- **参考资料**：申请单详情截图（红框标注「需求类型」展示位），见 `docs/reqs/13641767/type-screenshot.png`

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | 资源预测主单新增类型「预算申报」（`budget_declare`） | P0 | 系统/运营 | 必须 |
| F-002 | 资源预测 overwrite_append 增加必填 `type`，允许传 `budget_declare`；不自动赋值 | P0 | finops | 必须 |
| F-003 | 主单为 `budget_declare` 时，子单类型仍按明细推导；覆盖/追加/删除行为由明细决定 | P0 | 系统 | 必须 |
| F-004 | 主单为 `budget_declare` 时，子单一律跳过 HCM 管理员审批；CRP 管理员节点不跳过 | P0 | 系统 | 必须 |
| F-005 | 主单列表筛选、meta ticket_types、详情「需求类型」露出「预算申报」 | P0 | 运营 | 必须 |
| F-006 | 前端人工新建预测单不可选择/提交 `budget_declare` | P0 | 业务用户 | 必须 |

### 详细功能描述

#### [F-001] 新增主单类型「预算申报」

- **输入**：系统枚举扩展
- **处理逻辑**：
  1. 资源预测主单类型新增 `budget_declare`，中文名「预算申报」
  2. 仅作用于资源预测（`res_plan_ticket`），不涉及退回计划
- **输出**：枚举可被接口、列表、详情识别
- **边界条件**：
  - 子单类型枚举不新增「预算申报」
- **异常处理**：非法 type 校验失败

#### [F-002] overwrite_append 必填 type

- **输入**：`POST .../plans/resources/tickets/overwrite_append` 请求体增加必填字段 `type`
- **处理逻辑**：
  1. `type` 必填，不做默认赋值/不按明细自动推导主单类型
  2. 允许传入 `budget_declare`（预算同步场景）
  3. 仅该接口允许传入 `budget_declare`；其他创建入口拒绝该类型
- **输出**：创建的主单 `type`/`type_name` 为请求指定值
- **边界条件**：
  - 未传 `type` → 参数非法
  - 人工前端提单路径不可选该类型
- **异常处理**：参数校验失败返回参数错误

#### [F-003] 主单类型与子单/行为解耦

- **输入**：主单 `type=budget_declare` + 覆盖/追加明细
- **处理逻辑**：
  1. 主单类型固定为请求中的 `budget_declare`
  2. 子单类型仍按明细内容推导为既有子类型（如 add/adjust/delete 等）
  3. 实际覆盖、追加、删除行为仍由明细内容决定（与现网 overwrite_append 行为一致）
- **输出**：主单展示「预算申报」；子单类型与行为符合明细
- **边界条件**：仅覆盖、仅追加、覆盖+追加 三种组合均适用
- **异常处理**：沿用现有明细校验与失败重试语义

#### [F-004] HCM 管理员免审

- **输入**：主单类型为 `budget_declare` 的子单进入审批流转
- **处理逻辑**：
  1. 子单 HCM 管理员审批状态置为 skip（一律跳过，含跨年明细）
  2. CRP 侧管理员审批节点保持现有逻辑，不因本类型跳过
- **输出**：子单不经 HCM 管理员人工审批即可进入后续阶段（在其他既有条件满足时）
- **边界条件**：不影响 ITSM 跳过开关（`skip_itsm`）既有语义
- **异常处理**：CRP 审批失败按现有子单失败/重试处理

#### [F-005] 列表与详情露出

- **输入**：用户在资源预测主单列表筛选或打开详情
- **处理逻辑**：
  1. meta `ticket_types` 包含「预算申报」
  2. 列表筛选可选「预算申报」
  3. 详情「需求类型」展示「预算申报」（对应截图红框位置）
- **输出**：类型可筛可选可展示
- **边界条件**：子单列表仍展示子单自身类型名，不展示「预算申报」为主类型替代
- **异常处理**：无

#### [F-006] 限制创建入口

- **输入**：前端人工新建/调整预测单
- **处理逻辑**：创建表单与相关 API 不允许选择或提交 `budget_declare`
- **输出**：仅 overwrite_append 可合法创建该类型主单
- **边界条件**：其他系统入口若复用创建逻辑，同样拒绝该类型
- **异常处理**：非法提交返回参数/业务错误

## 非功能需求

### 性能需求

- **响应时间**：overwrite_append 仍异步返回主单 ID，不因新增 type 字段引入额外同步阻塞；P95 保持既有目标（< 1s 返回主单号，若父需求已定义则沿用）
- **并发能力**：无新增要求
- **数据容量**：无新增要求

### 安全需求

- **权限控制**：沿用 `biz_resource_plan_operate`
- **数据保护**：无新增
- **合规要求**：无新增

### 可用性与稳定性

- **系统可用性**：无新增
- **容错能力**：子单失败可单独重试（沿用现网）

### 兼容性

- **接口兼容**：overwrite_append 新增必填 `type`，对已上线调用方为破坏性变更，需同步改调用传参
- **系统兼容**：退回计划 overwrite_append 本期不改

## 业务规则

### 业务逻辑规则

- **规则 R-001**：资源预测主单新类型枚举值为 `budget_declare`，展示名「预算申报」。
- **规则 R-002**：仅资源预测主单支持该类型；退回计划本期不改。
- **规则 R-003**：overwrite_append 的 `type` 必填，系统不得自动赋值，也不得再按明细自动推导主单类型；允许显式传 `budget_declare` / `add` / `adjust` / `delete`。
- **规则 R-004**：`budget_declare` 仅允许通过资源预测 overwrite_append 传入。
- **规则 R-005**：主单为 `budget_declare` 时，子单类型与覆盖/追加/删除行为仍由明细决定。
- **规则 R-006**：主单为 `budget_declare` 时，子单一律跳过 HCM 管理员审批（含跨年）；CRP 管理员节点不跳过。
- **规则 R-007**：主单列表筛选与详情需露出「预算申报」。

### 数据校验规则

- **必填字段**：overwrite_append 请求体 `type`
- **格式要求**：`type` 为合法资源预测主单类型枚举
- **取值范围**：`budget_declare`，以及显式传入的既有主单类型 `add` / `adjust` / `delete`；禁止省略 `type`，禁止按明细自动推导

### 权限规则

- 调用 overwrite_append 需具备业务资源预测操作权限（`biz_resource_plan_operate`）

## 外部依赖与集成

### 外部系统集成

| 系统名称 | 交互方式 | 接口说明 | 认证方式 | 文档链接 |
|---------|---------|---------|---------|---------|
| CRP（云梯） | 现有 | 子单进入 CRP 后管理员节点不跳过 | 现有 | — |
| ITSM | 现有 | `skip_itsm` 语义不变 | 现有 | — |
| finops | HTTP | 调用 overwrite_append 时必填 `type=budget_declare` | 现有 | — |

### 接口契约

资源预测覆盖追加接口（增量）：

- 路径：`POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/overwrite_append`
- 新增必填字段：`type`（string），取值：`budget_declare` | `add` | `adjust` | `delete`；禁止省略、禁止自动推导
- 预算同步场景传 `type=budget_declare`，主单落库 `type_name=预算申报`
- 显式传 `add` / `adjust` / `delete` 时，主单类型与展示名按对应既有类型落库
- 其余字段与父需求 overwrite_append 契约保持一致

### 数据模型

- **主单**：`res_plan_ticket.type` 扩展 `budget_declare`
- **子单**：`res_plan_sub_ticket.sub_type` 不新增预算申报；管理员审批状态可直接为 `skip`

## 验收标准

### 功能验收

- [ ] **AC-001**：Given 调用资源预测 overwrite_append 且 `type=budget_declare`，When 创建主单成功，Then 主单类型为「预算申报」，详情「需求类型」展示「预算申报」
- [ ] **AC-002**：Given 调用 overwrite_append 未传 `type`，When 请求，Then 返回参数错误，不创建主单
- [ ] **AC-003**：Given 主单 `type=budget_declare` 且明细含覆盖与追加，When 拆子单，Then 子单类型仍按明细为既有类型（非预算申报），覆盖/追加行为符合明细
- [ ] **AC-004**：Given 主单 `type=budget_declare`（含跨年明细），When 子单进入 HCM 管理员审批阶段，Then 管理员审批被跳过（status=skip）
- [ ] **AC-005**：Given 主单 `type=budget_declare` 的子单进入 CRP，When 到达 CRP 管理员节点，Then 不因本类型自动跳过该节点
- [ ] **AC-006**：Given 前端人工新建资源预测单，When 选择类型，Then 不出现「预算申报」，提交 `budget_declare` 被拒绝
- [ ] **AC-007**：Given 主单列表，When 按类型筛选「预算申报」，Then 仅返回该类型主单；meta ticket_types 含该选项
- [ ] **AC-008**：Given 退回计划 overwrite_append，When 调用，Then 行为与本期前一致（无预算申报类型改造）

### 性能验收

- [ ] **AC-P01**：overwrite_append 仍快速异步返回主单 ID，不因 type 校验引入同步长耗时

### 安全验收

- [ ] **AC-S01**：无 `biz_resource_plan_operate` 权限时拒绝调用

## 边界范围

### 本期包含

- 资源预测主单类型 `budget_declare` / 「预算申报」
- overwrite_append 必填 `type`，支持传 `budget_declare`（不自动赋值）
- 该类型主单下子单一律跳过 HCM 管理员审批；CRP 管理员不跳过
- 列表筛选与详情展示露出
- 限制仅 overwrite_append 可传该类型

### 本期不包含

- 退回计划类型与审批改造
- 前端人工提单选择「预算申报」
- 修改 CRP 管理员自动过单策略
- finops 侧预算换算逻辑

## 约束条件

- **技术限制**：须兼容父需求已上线的 overwrite_append 明细与拆单行为；新增必填 `type` 需同步调用方
- **时间限制**：配合预算同步预测使用
- **资源限制**：无

## 人力与工时

- 全量工作 1 位高级工程师完成工时预估：14.5h（PERT 三点：O=8h / M=14h / P=22h）
- 全量工作 1 位中级工程师完成工时预估：约 20h（按高级工程师 ×1.4）

### 工时预估说明

- 需求类型：开发需求（分类 ID：1069995598002822402；分类原始值：迭代任务）
- 估时依据：跨模块逻辑（后端枚举/接口/拆单免审 + 前端类型筛选展示 + 调用方契约）
- 调整因子：接口破坏性变更联调（+10%）、需求边界清晰（-5%）
- 最乐观：8h / 最可能：14h / 最悲观：22h
- PERT：(8 + 4×14 + 22) / 6 = 14.5h

### RICE 评分明细

| 参数 | 值 | 说明 |
|------|-----|------|
| Reach | 20 | 影响少部分用户（finops 调用方 + 资源预测运营/审批人） |
| Impact | 8 | High 优先级，预算同步可识别与免审链路 |
| Confidence | 100% | 澄清完成并已评审通过，边界清晰 |
| Effort | 1.8125 人天 | 14.5h / 8 |
| RICE | 88 | (20 × 8 × 1.0) / 1.8125 ≈ 88 |

### 拆分结论

- 用户故事数：2
- 综合复杂度：中（功能/联调中，风险低）
- 判定：**无需拆分**，对本需求直接评分

## 未解决问题

无。

| 问题 ID | 问题描述 | 负责人 | 截止日期 | 状态 |
|--------|---------|--------|---------|------|
| Q-001 | overwrite_append 的 `type` 除 `budget_declare` 外是否仍允许显式传 `add`/`adjust`/`delete` | pandafyang | — | 已确认：允许；禁止省略与自动推导 |

---

## 原需求描述

预测主单类型增加“预算申报”类型：

（截图：申请单详情「需求类型：新增」红框位置）

子单类型不变。

通过 overwrite_append 接口提交的单据，默认都为该类型

当主单为该类型时，实际行为根据单据明细内容决定（是追加、删除还是修改）

当主单为该类型时，子单跳过管理员审批阶段（CRP中的管理员阶段不需要跳过）

## 澄清记录

### 第 1 轮澄清

**提问时间**：2026-07-23 11:56

**Agent 提问**：
1. 英文枚举值命名？
2. 是否仅资源预测？
3. 主单/子单类型关系？
4. HCM 管理员免审是否含跨年？
5. 是否自动赋类型 / 前端是否可选？
6. 列表是否露出？
7. 退回计划是否改造？

**用户回复**：
1. 使用推荐 A：`budget_declare`
2. 仅资源预测（退回计划应无管理员审批节点）
3. 主单固定，子单按明细
4. 一律跳过（含跨年）
5. 仅 overwrite_append 允许传该类型（**不自动赋值**），`type` 参数必填（修正原文「默认都为该类型」）
6. 列表需露出
7. 退回计划可以不改

### 第 2 轮澄清

**提问时间**：2026-07-23 16:01

**Agent 提问**：
1. Q-001：`type` 除 `budget_declare` 外是否仍允许显式传 `add` / `adjust` / `delete`？

**用户回复**：
1. 是，仍允许显式传 `add` / `adjust` / `delete`；禁止省略、禁止自动推导

## 技术澄清

> 澄清日期：2026-07-23
> 需求复杂度：中等
> 澄清轮次：1（clarify+specify attempt=1）

### 技术审查结论

- **技术可行性**：✅ 可行
- **技术风险等级**：低
- **审查说明**：在现有资源预测 overwrite_append、拆单与审批链路之上扩展枚举与分支即可；主要风险为接口破坏性变更需 finops 同步联调。

### 技术方案概述

- **实现方式**：
  1. 后端：在 `enumor.RPTicketType` 新增 `budget_declare` 及中文名映射；`OverwriteAppendResPlanTicketReq` 增加必填 `type`；`OverwriteAppendResPlanTicket` 使用请求 `type` 落库主单，移除 `deriveOverwriteAppendTicketType` 自动推导；拆单路由 `budget_declare` → `SplitAdjustTicket`；`SubTicketSplitter` 对主单 `budget_declare` 子单 HCM 管理员一律 `skip`（含跨年）；其他创建入口校验拒绝 `budget_declare`；meta/列表支持该类型。
  2. 前端：**本期不做**（纯后端交付）。
  3. 文档：更新 overwrite_append API 文档，标注 `type` 必填及破坏性变更。
- **涉及模块**：
  - 后端：`pkg/criteria/enumor/woa_ziyan_resplan.go`、`cmd/woa-server/types/plan/ticket_overwrite_append.go`、`cmd/woa-server/logics/plan/overwrite_append.go`、`cmd/woa-server/logics/plan/splitter/sub_ticket.go`、dispatcher/sub_ticket.go、meta
  - 前端：无（本期）
  - 文档：`docs/api-docs/web-server/docs/biz/scr/resource-plan/overwrite_append_biz_resource_plan_ticket.md`
- **技术选型**：沿用现有 Go 枚举 + woa-server 逻辑层，无新依赖。

### 架构影响

- **新增组件**：无
- **变更组件**：资源预测主单类型枚举、overwrite_append 请求/逻辑、拆单路由、子单 HCM 管理员 skip 规则、meta/列表 ticket_types
- **数据模型变更**：`res_plan_ticket.type` 允许新枚举值 `budget_declare`；子单 `sub_type` 不新增预算申报
- **向后兼容性**：⚠️ 不兼容——overwrite_append 新增必填 `type`，已上线调用方须同步改参；finops 传 `type=budget_declare`

### 外部依赖

| 依赖项 | 类型 | 状态 | 接口文档 | 备注 |
|--------|------|------|---------|------|
| finops | HTTP | ✅ 已确认 | req.md 接口契约 | 须同步传 `type=budget_declare` |
| CRP（云梯） | 现有 RPC | ✅ 已确认 | — | 管理员节点不跳过 |
| ITSM | 现有 | ✅ 已确认 | — | `skip_itsm` 语义不变 |

### 安全与合规

- **权限控制**：沿用 `biz_resource_plan_operate`
- **审计要求**：无新增
- **加密要求**：无新增

### 技术风险

| 风险 ID | 风险描述 | 影响 | 概率 | 应对措施 |
|---------|---------|------|------|---------|
| TR-001 | finops 未同步传 `type` 导致创建失败 | 中 | 中 | 发布前联调；接口文档标注破坏性变更 |
| TR-002 | 前端 TypeScript 联合类型未扩展导致编译/展示异常 | 低 | 低 | 扩展 `IResourcePlanTicketItem.ticket_type` |

### 非功能性约束

| NFR 维度 | 状态 | 结论 |
|---------|------|------|
| 性能 | 已知 | overwrite_append P95 返回主单 ID < 1s，不因 type 校验增加同步阻塞 |
| 可用性 | 不适用 | 无 SLA 变更 |
| 安全性 | 已知 | 沿用既有权限 |
| 兼容性 | 已知 | overwrite_append 破坏性变更；退回计划不改 |
| 合规性 | 不适用 | 无新增 |

### 测试策略

- **单元测试**：`OverwriteAppendResPlanTicketReq.Validate` 的 `type` 必填与枚举校验；`budget_declare` 主单拆单子单 AdminAudit skip（含跨年）
- **集成测试**：overwrite_append 传/不传 `type`；主单列表筛选与详情展示；退回计划 overwrite_append 回归
- **端到端测试**：finops 预算同步场景（`type=budget_declare` + skip_itsm）全链路

### 补充的验收标准

- [ ] **AC-T01**：Given overwrite_append 显式传 `type=add` 且明细仅含 add，When 创建成功，Then 主单类型为「新增」而非自动推导异常
- [ ] **AC-T02**：Given 主单 `budget_declare` 且明细含非本年度期望时间，When 拆子单，Then HCM 管理员仍为 skip

### 待解决问题

无阻塞项；questions.md 中 10 条均为 `resolved_by_doc`。
