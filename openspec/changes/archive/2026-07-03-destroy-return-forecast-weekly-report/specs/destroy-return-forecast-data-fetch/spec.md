## ADDED Requirements

### Requirement: 销毁返还预测候选集筛选
系统 SHALL 从 `cr_ReturnTask` 按 `status=SUCCESS`、`update_at` 落在统计周期内筛选候选集；不使用 `return_forecast` 作为筛选条件，也不再按 `GetBizOrgRel` 做部门过滤（海垒侧 `cr_ReturnTask` 已是本部门业务数据）。

#### Scenario: 命中候选集的记录
- **GIVEN** `cr_ReturnTask` 中存在 `status=SUCCESS` 的记录
- **AND** 该记录 `update_at` 落在统计周期内
- **WHEN** 系统执行候选集筛选
- **THEN** 该记录进入候选集

#### Scenario: 非 SUCCESS 状态被排除
- **GIVEN** `cr_ReturnTask` 中某记录 `status` 非 SUCCESS
- **WHEN** 系统执行候选集筛选
- **THEN** 该记录不进入候选集

#### Scenario: 跨周时间边界
- **GIVEN** 某记录创建于上周、本周才置为 SUCCESS
- **AND** 其 `update_at`（SUCCESS 写入时间）不在统计周期内
- **WHEN** 系统执行候选集筛选
- **THEN** 该记录不进入候选集

### Requirement: 调用 CRP 查询预测返还单
系统 SHALL 对每条候选记录取 `cr_ReturnTask.task_id` 作为销毁单号，调用 CRP `queryOrderList`（`ResPlanFetcher.GetOrderList`），将接口返回的全部 `QueryOrderInfo` 计入预测返还单（不做审批状态过滤，因为周报为事后一周查询、单据通常已审批结束）。

#### Scenario: CRP 返回预测返还单
- **GIVEN** 候选记录的 `task_id` 调 CRP `queryOrderList` 返回 `QueryOrderInfo`
- **WHEN** 系统处理命中结果
- **THEN** 该 `QueryOrderInfo` 被计入明细行

#### Scenario: CRP 无返回或不成功
- **GIVEN** 候选记录的 `task_id` 调 CRP 返回空或 `Status≠0`
- **WHEN** 系统处理命中结果
- **THEN** 该销毁单不贡献报表行

#### Scenario: 单条 CRP 查询失败
- **GIVEN** 某候选记录调 CRP `queryOrderList` 失败（如超时）
- **WHEN** 系统处理候选集
- **THEN** 系统记录错误日志并整体失败
- **AND** 不发送不完整报表，待手动重试
