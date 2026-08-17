## ADDED Requirements

### Requirement: 离线聚合前过滤不支持的项目类型

系统 SHALL 在自研云（vendor: tcloud-ziyan）离线推荐统计将已交付设备记录按 `(require_type, region, device_type, image_id)` 聚合并写入推荐表之前，判定每条记录的项目类型是否属于当前受支持集合。判定口径 MUST 与创建推荐数据时的 `RequireType.Validate()` 同源，受支持集合为后端代码枚举：常规项目(1) / 春节保障(2) / 机房裁撤(3) / 滚服项目(6) / 小额绿通(7) / 春保资源池(8) / 短租项目(9)。不属于该集合的取值（含 0、4、5 及任何历史下线值）MUST 视为不支持。

判定为不支持的记录 MUST 直接跳过：不参与用户维度与业务维度计数，不写入 `ziyan_cvm_apply_user_recommend` / `ziyan_cvm_apply_biz_recommend`。滚服项目(6) 属于受支持集合，MUST NOT 被本过滤剔除。判定 MUST 在进程内完成，SHALL NOT 新增远程查询，SHALL NOT 修改 `ziyan_cvm_device_info` 源表，SHALL NOT 将不支持类型替换为其他类型。

被跳过的记录 MUST 逐条输出 Warn 级别日志，内容 MUST 包含该记录 ID、其 `require_type` 取值，以及 rid。系统 SHALL NOT 为此过滤额外输出汇总统计。

某个 (业务, 用户) 或 (业务) 分组在过滤后无任何候选时，本轮 MUST NOT 为该分组写入任何行（含 `require_type=0` 或其他占位行）；该分组的历史推荐行 MUST 按既有过期清理规则（`updated_at < 本轮任务开始时间`）删除。

本过滤 MUST NOT 改变在线读取接口 `ApplyRecommendTop` / `ApplyRecommendByStatic` / `ApplyRecommendByPlan` 的请求与响应结构；MUST NOT 改动读取侧既有过滤逻辑（含 `ApplyRecommendByStatic` 不返回滚服项目）。`ApplyRecommendByPlan` 候选不来自推荐表，不受本过滤影响。

#### Scenario: 不支持的项目类型不参与聚合且任务成功

- **GIVEN** 历史设备记录中存在一条 `require_type` 不在受支持集合内（如已下线类型或 0）的已交付记录
- **WHEN** 离线推荐任务执行
- **THEN** 该记录不参与聚合，两张推荐表中不出现该项目类型的行，任务整体执行成功，且日志中无 `unsupported require type` 创建失败

#### Scenario: 全部为受支持类型时聚合结果不变

- **GIVEN** 历史设备记录的 `require_type` 全部为受支持集合内的取值
- **WHEN** 离线推荐任务执行
- **THEN** 聚合与写入结果与增加本过滤前完全一致（同一份数据源下，两张推荐表的行数与内容逐行相同）

#### Scenario: 滚服项目不被本次过滤剔除

- **GIVEN** 历史设备记录中存在 `require_type=6`（滚服项目）的已交付记录
- **WHEN** 离线推荐任务执行
- **THEN** 该记录照常参与聚合并写入推荐表，不被本次过滤剔除

#### Scenario: 越界取值跳过并输出 Warn

- **GIVEN** 历史设备记录的 `require_type` 为 0 或 4、5 等不在受支持集合内的取值
- **WHEN** 离线推荐任务执行
- **THEN** 该记录被跳过，且日志中可查到对应 Warn 记录（含记录 ID 与该项目类型取值）

#### Scenario: 整组被过滤不写入占位行

- **GIVEN** 某 (业务, 用户) 分组下全部历史记录的项目类型都不受支持
- **WHEN** 离线推荐任务执行
- **THEN** 该分组本轮不写入任何行，且不产生 `require_type=0` 或其他占位行；其历史推荐行在过期清理后被删除

#### Scenario: 源表记录不被修改

- **GIVEN** 任务执行完成
- **WHEN** 查询 `ziyan_cvm_device_info`
- **THEN** 源表记录数与 `require_type` 取值均未被修改

#### Scenario: 读取推荐表链路结果正确且读侧滚服规则不变

- **GIVEN** 推荐表已按新规则重建
- **WHEN** 依次调用 `ApplyRecommendTop`、`ApplyRecommendByStatic`
- **THEN** 返回项的项目类型均属于受支持集合，接口响应结构与修复前一致，且 `ApplyRecommendByStatic` 仍按既有读侧规则不返回滚服项目
