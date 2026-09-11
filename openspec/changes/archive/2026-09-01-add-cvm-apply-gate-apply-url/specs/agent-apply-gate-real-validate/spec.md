## ADDED Requirements

### Requirement: 门禁无权限拒绝时返回 apply_url

主机申领门禁（`createCvmApplyGate`，守护 `create_biz_apply`）在用户确认后、调用 woa 提单前预检**之前**，SHALL 对当前用户、当前业务主动判定是否具备主机申领权限。鉴权资源属性 MUST 与 woa `CheckBizApplyOrder` 一致：`meta.Biz` + `meta.Create` + 当前 `bk_biz_id`（集成点：`pkg/iam/auth.Authorizer.Authorize`）。

当且仅当主动鉴权判定无权限时，门禁 SHALL 拒绝放行（MUST NOT 进入正式提单），MUST NOT 调用 woa 预检，并将无权限标识与可打开的 `apply_url` 写入该 tool_call 的结果。

`apply_url` MUST 由同一 Authorizer 的 `GetPermissionToApply` + `GetApplyPermUrl` 生成（集成点：auth-server / ESB IAM `/iam/application/`），且 MUST 对应该用户、该业务、主机申领操作。MUST NOT 用平台「智能体助手」权限或其它业务资源顶替。`permission` 字段本期不强制返回。

生成成功时 tool 结果 MUST 为可解析结构，至少包含：无权限标识（`code` = 2030403）、面向用户的简短原因（含账号与业务）、非空 `apply_url`（http/https）。该路径 MUST NOT 把「联系管理员」当作唯一引导。

#### Scenario: 主动判定无权限且申请地址可生成

- **GIVEN** 业务视角用户已登录并在某业务的 Agent 会话中确认主机申领，门禁主动鉴权判定该账号没有该业务主机申领权限，且 IAM 申请地址可以生成
- **WHEN** 主机申领门禁完成该次主动鉴权
- **THEN** 门禁拒绝放行，不进入正式提单
- **AND** MUST NOT 调用 woa `CheckBizApplyOrder`
- **AND** tool 结果带无权限标识 `2030403`
- **AND** tool 结果带非空 `apply_url`，打开后的申请目标为该用户、该业务、主机申领，而不是智能体助手或其它业务
- **AND** 结果中的引导 MUST NOT 仅有「联系管理员」

#### Scenario: 消费方可用 apply_url 一步打开申请页

- **GIVEN** 门禁主动判定无权限且 `apply_url` 可用
- **WHEN** 消费方依据门禁返回打开该地址
- **THEN** 用户只需 1 步即可进入权限申请页，无需先联系管理员

### Requirement: 申请地址不可用时降级

当门禁**已经主动判定无权限**，但 `apply_url` 生成失败、为空或不是可打开的 http/https 地址时，系统 SHALL 仍返回可识别的无权限标识（`2030403`），并说明当前无法直接跳转申请、请联系该业务管理员开通主机申领权限后再重试。系统 MUST NOT 提供空的或伪造的 `apply_url`。该次对话 MUST NOT 因申请地址生成失败而变为系统错误（5xx / 「暂时无法提单」类系统异常文案）。降级文案 MUST NOT 替代申请地址可用时的主路径。本需求不适用于「尚未判定是否无权限、IAM Authorize 本身失败」——后者见主动鉴权系统失败。

#### Scenario: 申请地址生成失败走降级

- **GIVEN** 门禁主动鉴权已判定用户没有该业务主机申领权限，且 `apply_url` 生成失败或为空
- **WHEN** 门禁构造拒绝结果
- **THEN** 拒绝结果带无权限标识 `2030403`
- **AND** 结果中不含可跳转的 `apply_url`
- **AND** 说明无法直接跳转、需联系该业务管理员开通后再重试
- **AND** 门禁仍拒绝放行，该次对话不因此变为系统错误

#### Scenario: 已判定无权限后权限中心超时仍返回无权限

- **GIVEN** 门禁主动鉴权已判定无权限，且随后 `GetPermissionToApply` 或 `GetApplyPermUrl` 超时或失败
- **WHEN** 门禁构造拒绝结果
- **THEN** 仍返回无权限标识与降级说明
- **AND** MUST NOT 将此次拒绝升格为系统异常提示

### Requirement: 主动鉴权系统失败不得放行

当门禁主动鉴权调用 IAM/auth-server 本身报错或超时（尚不能判定用户是否无权限）时，系统 SHALL 拒绝放行，MUST NOT 进入正式提单，MUST NOT 将此次失败当作有权限而继续 woa 预检，MUST NOT 附带 `apply_url`（不得假装可以跳转申请）。authorizer 未注入时同样拒绝。

主动鉴权的判定主体 MUST 只取自请求上下文中的蓝鲸用户名（`X-Bkapi-User-Name`）。申领参数中的提单人（`bk_username`）由模型生成、且用户可在确认卡片上编辑，MUST NOT 被用作权限判定主体；后端操作用户等内部账号同样 MUST NOT 顶替真实用户。上下文缺少用户名时视为无法判定，SHALL 按本需求拒绝放行。woa 调用侧沿用现网身份传递方式，本需求 MUST NOT 收紧。

#### Scenario: IAM 超时或报错则拒绝且不调 woa

- **GIVEN** 用户已确认主机申领，且门禁 `Authorize` 因超时或 IAM 错误返回失败
- **WHEN** 门禁处理该次结果
- **THEN** 拒绝放行，不进入正式提单
- **AND** MUST NOT 调用 woa `CheckBizApplyOrder`
- **AND** 结果中不含 `apply_url`
- **AND** MUST NOT 标成无权限（2030403）引导

#### Scenario: 上下文缺用户名不得用提单参数顶替鉴权

- **GIVEN** 用户已确认主机申领，请求上下文中没有蓝鲸用户名，而申领参数的 `bk_username` 填的是某个有权限的账号
- **WHEN** 门禁处理该次确认
- **THEN** 拒绝放行，不进入正式提单
- **AND** MUST NOT 以申领参数中的提单人为主体发起鉴权
- **AND** MUST NOT 调用 woa `CheckBizApplyOrder`
- **AND** 结果中不含 `apply_url`，MUST NOT 标成无权限（2030403）引导

### Requirement: 非权限失败不附带 apply_url

门禁上的参数不合法、额度或库存不足、用户取消确认、缺少业务 ID、woa 非权限系统故障等非权限失败，SHALL 按现网对应失败类型返回原因，MUST NOT 附带 `apply_url`。错误文案中偶尔出现「权限」字样但主动鉴权并未判定无权限时，MUST NOT 标成无权限，MUST NOT 附带 `apply_url`。其它 Agent 工具失败、以及门禁放行之后的正式提单路径，本能力 MUST NOT 要求返回 `apply_url`。

#### Scenario: 额度或参数失败不含 apply_url

- **GIVEN** 门禁主动鉴权判定用户具备该业务主机申领权限
- **WHEN** 随后 woa 预检因额度不足或参数不合法拒绝（`Pass=false`）
- **THEN** 返回对应非权限失败原因
- **AND** 结果中不含 `apply_url`
- **AND** MUST NOT 标成无权限（2030403）

#### Scenario: 取消确认或文案含权限字样不得误判

- **GIVEN** 用户取消申领确认，或失败文案中仅偶然出现「权限」字样但门禁主动鉴权并未判定无权限
- **WHEN** 门禁处理该次结果
- **THEN** MUST NOT 标成无权限
- **AND** MUST NOT 附带 `apply_url`

#### Scenario: woa 非权限系统异常不含 apply_url

- **GIVEN** 门禁主动鉴权已通过，且 woa 预检返回非无权限的 error（下游故障等）
- **WHEN** 门禁拒绝放行
- **THEN** 向用户提示前置校验失败、暂时无法提单
- **AND** 结果中不含 `apply_url`

#### Scenario: 其它工具与正式提单不在范围

- **GIVEN** 其它 Agent 工具失败，或主机申领已过门禁进入正式提单
- **WHEN** 发生无权限或其它失败
- **THEN** 本能力不要求这些路径返回 `apply_url`

### Requirement: 有权限放行不得返回 apply_url

当门禁主动鉴权判定用户具备该业务主机申领权限，且随后 woa 预检其它校验通过时，系统 SHALL 按现网放行到后续提单步骤。通过条件 MUST NOT 因本需求放宽或收紧。woa 侧 `AuthorizeWithPerm` MUST 保持为纵深防御，本期 MUST NOT 删除或绕过。放行结果 MUST NOT 含无权限标识，MUST NOT 含 `apply_url`。无权限用户 MUST NOT 被标记为通过，也 MUST NOT 进入正式提单。`apply_url` 与若存在的权限信息 MUST 只描述当前用户、当前业务、当前主机申领操作，MUST NOT 泄露其他用户或其他业务的权限清单。

若主动鉴权已通过，但 woa 预检仍以无权限（错误码 2030403）拒绝，门禁 SHALL 按无权限引导处理（`apply_url` 或降级），MUST NOT 当作系统异常。该兜底 MUST NOT 替代主动鉴权主路径。

#### Scenario: 预检通过放行且无 apply_url

- **GIVEN** 门禁主动鉴权通过，且 woa 预检其它校验通过
- **WHEN** 门禁完成确认后预检
- **THEN** 不出现无权限标识，也不出现 `apply_url`
- **AND** 后续按现网放行到 tool 节点执行真实提单
- **AND** woa 预检仍被调用（纵深防御鉴权仍在 woa 侧执行）

#### Scenario: 无权限不得放行提单

- **GIVEN** 门禁主动鉴权判定用户没有该业务主机申领权限
- **WHEN** 门禁返回无权限结果
- **THEN** 申领不得被标记为通过
- **AND** 不得进入正式提单或创建申领单据

#### Scenario: apply_url 不泄露其它主体权限

- **GIVEN** 门禁主动鉴权判定用户没有业务 213 的主机申领权限
- **WHEN** 查看返回的 `apply_url`（及若存在的 `permission`）
- **THEN** 其中不含其他用户、以及其他业务的权限清单

#### Scenario: 主动通过后 woa 仍无权限则走引导兜底

- **GIVEN** 门禁主动鉴权判定有权限，但随后 woa 预检返回错误码 2030403
- **WHEN** 门禁处理该次 woa 结果
- **THEN** 拒绝放行
- **AND** 按无权限引导返回标识与 `apply_url` 或降级说明
- **AND** MUST NOT 使用「暂时无法提单」类系统异常文案作为唯一结果

## MODIFIED Requirements

### Requirement: 申领门禁执行真实前置校验

agent-server 申领门禁（`createCvmApplyGate`，守护工具 `create_biz_apply`）SHALL 在用户在确认卡片点击确认后、放行到 tool 节点执行真实提单之前，先主动判定主机申领权限，再在鉴权通过时调用 woa-server 的提单前只读校验接口执行真实校验，替换原占位实现（直接返回通过）。woa 校验 MUST 只读、无副作用。主动鉴权 MUST NOT 替代 woa 侧 `AuthorizeWithPerm`。

用户确认后的映射为：

- 主动鉴权失败（IAM 报错/超时）→ 拒绝放行，不调 woa，提示权限校验失败，MUST NOT 附带 `apply_url`。
- 主动鉴权判定无权限 → 拒绝放行，不调 woa 预检，走无权限引导（`apply_url` 或降级）。
- 主动鉴权通过后，woa `err != nil` 且为 2030403 → 拒绝放行，走无权限引导兜底，MUST NOT 当作系统异常。
- 主动鉴权通过后，woa `err != nil` 且非无权限 → 拒绝放行，向用户提示系统异常（可重试），MUST NOT 附带 `apply_url`。
- 主动鉴权通过后，woa `!ok`（业务不通过）→ 拒绝放行，向用户回显业务原因并建议调整规格或数量，MUST NOT 附带 `apply_url`。
- 主动鉴权通过且 woa `ok` → 放行到 tool 节点执行真实提单，MUST NOT 返回无权限标识或 `apply_url`。

#### Scenario: 校验通过放行提单

- **GIVEN** 用户在确认卡片点击确认，门禁主动鉴权通过，且 woa 校验接口返回 `pass=true`
- **WHEN** `createCvmApplyGate.OnResume` 完成确认后预检
- **THEN** 门禁路由到 tool 节点，执行真实 `create_biz_apply` 提单
- **AND** 放行结果不含无权限标识，也不含 `apply_url`

#### Scenario: 业务校验不通过回退

- **GIVEN** 门禁主动鉴权通过，且 woa 校验接口返回 `pass=false` 且携带原因
- **WHEN** `createCvmApplyGate.OnResume` 执行 `validateApply`
- **THEN** 门禁不提单，路由回 llm 节点，向用户回显"申领前置校验未通过：<原因>"并建议调整后重试
- **AND** 结果中不含 `apply_url`

#### Scenario: 系统异常不放行

- **GIVEN** 门禁主动鉴权通过，且 woa 校验接口返回 error（系统异常），且不是无权限
- **WHEN** `createCvmApplyGate.OnResume` 执行 `validateApply`
- **THEN** 门禁不提单，向用户提示前置校验失败、暂时无法提单
- **AND** 结果中不含 `apply_url`

#### Scenario: 主动判定无权限不走系统异常文案

- **GIVEN** 门禁主动鉴权判定当前用户对当前业务没有主机申领权限
- **WHEN** `createCvmApplyGate.OnResume` 处理确认
- **THEN** 门禁不提单，且不调用 woa 预检
- **AND** MUST NOT 使用「暂时无法提单」类系统异常文案作为唯一结果
- **AND** 按无权限引导需求返回标识与 `apply_url` 或降级说明
