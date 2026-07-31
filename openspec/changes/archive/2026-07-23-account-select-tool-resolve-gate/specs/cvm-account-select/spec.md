## MODIFIED Requirements

### Requirement: 单账号自动选择
账号选择是否触发用户交互 SHALL 以**账号总数**为判定基准：账号总数恰为 1 时，系统 SHALL 自动将该账号 ID 写入 graph State 的 `account_id` 字段并持久化，无需用户干预、无需模型参与，直接路由至 `llm` 节点；账号总数 ≥ 2 时一律弹卡片请用户选择（可用性判定只影响卡片内选项是否禁用，不改变是否弹卡片）；账号总数为 0 时路由 `fallback`。

#### Scenario: 账号总数为 1 时自动写入
- **WHEN** 账号列表包含且仅包含 1 个账号（count == 1）
- **THEN** `state["account_id"]` 被设置为该账号 ID 并落库，路由至 `llm` 节点

#### Scenario: 多账号时一律弹卡片（含仅 1 个可用 / 全部不可用）
- **GIVEN** 账号总数 ≥ 2（无论其中可用账号数为 0、1 还是多个）
- **WHEN** 进入 `account_select` 且未命中已选账号复用
- **THEN** 系统 SHALL 触发 `account_select.interrupt` 弹卡片，卡片展示全部账号，非 `tcloud-ziyan` 账号标记为禁用并附原因

#### Scenario: 账号总数为 0 时路由 fallback
- **WHEN** 账号列表为空（count == 0）
- **THEN** 系统 SHALL 路由至 `fallback`，由主图兜底回复

## ADDED Requirements

### Requirement: 自由文本未匹配时不下传空账号
在多账号 HITL 场景下，当用户以自由文本 resume（如"选择自研云账号"）且未能解析出 `account_id` 时，系统 MUST NOT 将空 `account_id` 写入 graph State/`delta`（避免污染后续 `tryReuseAccountID` 复用判定）；账号的最终解析与持久化 SHALL 由 `cvm-account-tool-resolution` 能力（`select_account` 工具 + 申领工具门禁）保证。

#### Scenario: 自由文本未命中时不污染状态
- **GIVEN** 多账号 HITL，用户自由文本无法被 `matchAccountIDFromUserInput` 匹配为 `account_id`
- **WHEN** `resolveSelectedAccountID` 返回空
- **THEN** 系统不写入 `delta[account_id]=""`，且账号由 `select_account` 工具与门禁最终结构化接住并落库，使同会话下一轮不再重复弹账号选择
