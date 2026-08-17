## Context

自研云（vendor: tcloud-ziyan）主机申领推荐由 woa-server 离线任务 `apply_recommend_offline` 驱动。主流程在 `cmd/woa-server/logics/applyrecommend/logics.go` 的 `GenerateRecommend`：拉取已交付设备 → 补全地域 → 过滤无效镜像 → 聚合 → 写用户/业务推荐表 → 清理过期。

核实到的现状（相对派活线索核对后确认，不是照抄）：

- 聚合前过滤循环在 `collectCounts`（约 L125–137）：当前只跳过「地域为空 / 镜像为空 / 镜像失效」，**没有** `require_type` 校验。
- 创建推荐数据时，`ZiyanCvmApplyUserRecommendCreateReq` / `ZiyanCvmApplyBizRecommendCreateReq` 的 `Validate()` 会调用 `RequireType.Validate()`（`pkg/api/data-service/cvm-apply/ziyan_cvm_apply_recommend.go`）。不合法即整批拒绝，错误文案 `unsupported require type: %d`。
- `RequireType.Validate()` 以 `GetRequireTypeMembers()` 为受支持集合：1/2/3/6/7/8/9（`pkg/criteria/enumor/woa_ziyan.go`）。这与创建失败的校验口径同源。
- 写入是按分组先 `BatchDelete` 再 `BatchCreate`，任一失败即 `return`（`writeUserRecommends` / `writeBizRecommends`）。本期不改这段（Q3=A）。
- 读取侧 `buildStaticFilterRules` 已 `RuleNotEqual("require_type", RequireTypeRollServer)`（`recommend.go` 约 L283）。滚服(6) 在枚举内，写入侧仍会产出滚服行，读时被丢弃。该差异已知且本期不处理（Q2=A）。
- 同类先例：无效镜像过滤（`docs/reqs/申领推荐过滤无效镜像.md`）——同一循环、同一「跳过 + Warn、不占位、不改源表」模式。
- `applyrecommend` 包内尚无 `_test.go`。

约束：只改 `openspec/changes/` 之外的业务代码属于下一阶段；本设计只锁定落点与口径。判定必须与创建校验同源，避免再次漂移。不新增远程查询。

## Goals / Non-Goals

**Goals:**

- 在按 `(require_type, region, device_type, image_id)` 聚合计数之前，跳过 `RequireType.Validate()` 失败的历史设备记录（F-001）。
- 过滤后某 (业务, 用户) / (业务) 分组无候选时，本轮不写入、不产生占位行；旧行交给既有过期清理（F-002）。
- 每条被跳过的记录打 Warn，含记录 ID、`require_type` 取值与 rid（F-003）。
- 上线后下一轮定时任务（或可选手动 `POST /api/v1/woa/apply_recommend/sync`）覆盖推荐表；读取侧接口契约不变。

**Non-Goals:**

- 写入容错与写入保护（单分组失败跳过、先建后删）。
- 过滤滚服(6)；对齐读取侧「临时禁止滚服」。
- 清理 `ziyan_cvm_device_info` 源表。
- 改在线读取侧过滤逻辑。
- 把已下线类型替换/降级为常规项目。
- 以运营配置（前端下拉那套）作为判定口径。
- 改 Top-K / 去重键 / 用户业务补足策略。
- 改表结构或对外接口。
- 改 `ApplyRecommendByPlan`（候选不来自推荐表）。

## Decisions

### 决策 1：过滤加在 `collectCounts` 聚合前循环，与无效镜像过滤同位置

**选择**：在 `collectCounts` 里、地域/镜像检查之后、构造 `userCountKey` / `bizCountKey` 并 `++` 之前，增加 `require_type` 合法性判断；失败则 `continue`，不计数、不写入。

**理由**：

- 与无效镜像过滤同一阶段，脏数据不会进入聚合 map，也就不会进入 `BatchCreate`，从源头避开 `unsupported require type`。
- 空分组自然不会出现在 `AggregateUser` / `AggregateBiz` 的结果 map 里，`write*` 不会为它们 Delete+Create；旧行靠 `cleanupExpired`（`updated_at < startTime`）删除，满足 F-002，无需新写空行逻辑。
- 不改 SQL 拉取条件：源表无枚举约束，且逐条 Warn 需要记录 ID，内存过滤更直接。

**备选（否决）**：

- 写入前再滤：聚合结果已按脏类型成组，BatchCreate 仍可能整批失败；且先删后建的风险窗口仍在。
- 读侧兜底：Q 已锁定只做写入侧源头治理；读侧过滤不能阻止创建失败。
- SQL `require_type IN (1,2,3,6,7,8,9)`：与 `Validate()` 两处维护，易漂移；也无法逐条打 Warn。

### 决策 2：判定复用 `RequireType.Validate()`，不手写集合、不读运营配置

**选择**：对 `item.RequireType` 调用已有 `Validate()`。失败即视为不支持（含 0、4、5 及任何历史下线值）。滚服(6) 在 `GetRequireTypeMembers()` 内，`Validate()` 通过，**不会**被过滤。

**理由**：创建推荐数据走的就是同一方法；Q1=A 要求与报错口径完全一致。枚举日后增删时过滤自动跟上，避免再出现「写入侧集合」与「创建校验集合」不一致。

**备选（否决）**：

- 硬编码 `1/2/3/6/7/8/9`：与 `Validate()` 重复，后续改枚举必漏。
- 读运营「需求类型」配置：Q1 已否；且会新增远程查询，违反「新增查询 0 次」。

### 决策 3：日志对齐无效镜像过滤——逐条 Warn，不汇总

**选择**：`logs.Warnf("skip unsupported require type, id: %s, require_type: %d, rid: %s", item.ID, item.RequireType, kt.Rid)`（文案以实现时与现有 `skip invalid image` 风格对齐为准）。不打汇总计数。

**理由**：Q5 默认假设；运维可按关键字检索统计条数。`Validate()` 失败是预期脏数据，不是本轮任务失败，用 Warn 而非 Error，避免误报任务失败。

### 决策 4：不抽公共「设备跳过」框架；若 `collectCounts` 超 80 行再就地抽私有判断

**选择**：优先在现有循环里加 4～6 行 `Validate()` + Warn + `continue`。`collectCounts` 当前约 78 行，加上过滤后可能略超 80 行项目上限——若超限，只把「是否跳过 + 打日志」抽成同文件私有函数（例如 `skipDeviceForCount`），**不**借机重构拉取/补地域/查镜像。

**理由**：本期是最小过滤修复；镜像过滤已是循环内联模式。抽一个私有函数只为守行数上限，不引入新抽象层。

### 决策 5：单测补「过滤判定」小函数，端到端 AC 靠造脏数据 / 读侧回归

**选择**：若抽出私有/可测的 `require_type` 跳过判断，补表驱动单测覆盖 1/2/3/6/7/8/9 通过、0/4/5 与未知值失败。`collectCounts` 依赖 data-service 分页，包内无现成 mock，本期不强制为整条拉取链路补集成测试。AC-001～007 中读侧与脏数据场景在实现后的自测清单里覆盖。

**理由**：阶段 2 已标明「包内尚无单测、验证成本 3/5」；判定本身是纯函数，单测成本低且能锁住与 `Validate()` 同源。

## Risks / Trade-offs

- **[风险] 过滤后某用户/业务推荐变少或整组被过期清理删空** → 这是预期：那些行本就不能创建。AC-002 用「全是合法类型」对照修复前结果；AC-005 覆盖整组被滤。
- **[风险] 误伤滚服(6)** → `Validate()` 包含 6；AC-003 显式回归。读取侧仍过滤滚服，写入/读取口径差异保持原样。
- **[风险] 枚举与运营配置不一致，运营下拉已下线但代码仍支持（或相反）** → Q1=A 接受以代码为准；两套口径统一不在本期。
- **[风险] 未知异常仍会触发「先删后建 + 整轮中止」** → 已知，Q3=A 另立需求；本过滤只消除 `unsupported require type` 这一类触发源。
- **[风险] 存量推荐表仍可能含脏类型，直到下一轮任务成功覆盖** → Q4：等定时任务；可选手动 sync。设计不要求上线时强制 sync。
- **[权衡] 逐条 Warn 在脏数据很多时日志量上升** → 与无效镜像过滤一致；脏数据应是少数历史值。不在本期做采样或汇总。

## Migration Plan

1. 部署带过滤逻辑的 woa-server（无 DDL、无接口变更）。
2. 等待下一轮 `apply_recommend_offline`（默认间隔 720 分钟）成功跑完，推荐表按新规则重建。
3. 如需立即生效：调用既有 `POST /api/v1/woa/apply_recommend/sync`（仍受 master 节点约束）。
4. 回滚：回退该过滤提交并重新部署；下一轮任务恢复「不过滤 require_type」。回滚后若源表仍有脏类型，创建失败可能复现。无需反向 DDL。

## Open Questions

无。澄清 Q1～Q7 已锁定：Q1=A 代码枚举、Q2=A 不过滤滚服、Q3=A 只做过滤、Q4 等定时覆盖、Q5 逐条 Warn、Q6 按既有稳定性/性能指标验收、Q7 不治理源表。
