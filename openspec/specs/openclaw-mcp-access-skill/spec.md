# openclaw-mcp-access-skill Specification

## Purpose
TBD - created by archiving change api-server-mcp-a2a-bridge. Update Purpose after archive.
## Requirements
### Requirement: 新建独立 SKILL 文件

api-server 本变更 SHALL 在仓库 `.cursor/skills/openclaw-mcp-access/SKILL.md` 新建独立 skill 文件（**不复用 `.cursor/skills/code-review-vibe/SKILL.md`**），用于约束 OpenClaw 等外部 MCP 客户端调用蓝鲸网关 HCM MCP 服务的行为。

SKILL.md 文件首部 SHALL 包含 YAML frontmatter，至少含字段：`name: openclaw-mcp-access`、`description`（≤200 字 ASCII，含触发关键词如 "openclaw mcp"、"hcm mcp gateway"）、`license: MIT`、`triggers`（关键词数组）。

SKILL.md SHALL 与已有 skill（如 `.cursor/skills/openspec-new-change/SKILL.md`）保持一致的章节结构与 markdown 风格。

#### Scenario: SKILL 文件存在且 frontmatter 合法

- **WHEN** 检查 `.cursor/skills/openclaw-mcp-access/SKILL.md` 文件
- **THEN** 文件 SHALL 存在
- **AND** frontmatter `name` 字段 SHALL 等于 `openclaw-mcp-access`
- **AND** `description` 字段长度 SHALL ≤ 200 字
- **AND** `triggers` 数组 SHALL 至少包含 `openclaw mcp`、`hcm mcp gateway`、`call hcm agent` 三项之一

#### Scenario: 不污染 code-review-vibe

- **WHEN** 本变更 PR 提交时
- **THEN** `.cursor/skills/code-review-vibe/SKILL.md` 文件 SHALL 不被本变更修改

### Requirement: SKILL 必须覆盖蓝鲸网关访问入口与凭证

SKILL.md SHALL 包含"访问入口"章节，明确说明：

- 蓝鲸网关 base URL 模板：`https://<bk-apigw-host>/api/v1/mcp/servers/{mcp_server_name}/mcp/`（含 `{mcp_server_name}` 占位符示例如 `bk-hcm-devhk-tcloud-ziyan-cvm`）；
- 鉴权：必传 HTTP header `X-Bkapi-JWT` 由蓝鲸网关签发，OpenClaw 通过蓝鲸 APIGW 鉴权流程申请；
- 内容协商：必传 `Content-Type: application/json`、`Accept: application/json, text/event-stream`；
- 协议版本：MCP `protocolVersion`，对应 `trpc-mcp-go` 当前支持版本。

SKILL.md SHALL 显式提示：**外部客户端只能调用 `send_message` 一个工具**（不存在 `list_cvms` 等 HCM 业务工具直接对外暴露）。

#### Scenario: 访问入口章节完整

- **WHEN** 阅读 SKILL.md
- **THEN** SHALL 找到 base URL 模板、JWT 鉴权说明、Content-Type / Accept 要求三项必备说明
- **AND** SHALL 明确提示"OpenClaw 不要尝试直接调用 HCM 业务工具"

### Requirement: SKILL 必须提供完整 MCP 流程调用示例

SKILL.md SHALL 包含"调用流程示例"章节，按时序提供以下 4 步示例（JSON-RPC 2.0 报文）：

1. **`initialize`** 请求 + 响应；
2. **`notifications/initialized`** 通知（仅请求）；
3. **`tools/list`** 请求 + 响应（响应中明确仅含 `send_message`）；
4. **`tools/call(send_message)`** 请求 + SSE 增量响应（含 `notifications/progress` 示例与最终 `CallToolResult`）。

每个示例 SHALL 提供完整 HTTP 请求示例（含 method、URL、关键 header、body）与响应示例（含状态码、body）。

#### Scenario: 4 步流程齐全

- **WHEN** 阅读 SKILL.md "调用流程示例" 章节
- **THEN** SHALL 找到 `initialize` / `notifications/initialized` / `tools/list` / `tools/call(send_message)` 4 个完整示例
- **AND** 每个示例 SHALL 同时包含 request 与 response（notification 仅 request）

#### Scenario: tools/list 示例只暴露 send_message

- **WHEN** 阅读 `tools/list` 响应示例
- **THEN** 响应 `result.tools` 数组 SHALL 长度为 1
- **AND** 唯一工具 `name` SHALL 等于 `send_message`

#### Scenario: tools/call 示例含 SSE 流

- **WHEN** 阅读 `tools/call(send_message)` 示例
- **THEN** 响应示例 SHALL 包含至少一条 `notifications/progress` 事件
- **AND** SHALL 包含最终 `CallToolResult` 响应

### Requirement: SKILL 必须约束参数语义与必填项

SKILL.md SHALL 明确 `send_message` 工具的 `arguments` 入参约束：

| 参数 | 类型 | 必填 | 含义 |
|---|---|---|---|
| `text` | string | 是 | 用户自然语言输入，长度上限 8000 字符 |
| `contextId` | string | 否 | 会话连续性 ID，首轮不传，后续轮次回填上一轮返回的 `contextId` |
| `bk_biz_id` | integer | 否 | 蓝鲸业务 ID，未传时由 HCM 默认业务上下文兜底 |
| `model_name` | string | 否 | 模型名称（仅在 OpenClaw 需要 override 默认模型时使用） |

SKILL.md SHALL 显式禁止 OpenClaw 在 `arguments` 中传入未在表格中列出的字段（防止参数污染）。

SKILL.md SHALL 显式说明：`contextId` 由 HCM agent-server 生成与维护，OpenClaw 不要自行编造。

#### Scenario: 参数列表完整

- **WHEN** 阅读 SKILL.md
- **THEN** SHALL 找到 `text` / `contextId` / `bk_biz_id` / `model_name` 4 个字段的完整说明
- **AND** SHALL 明确 `text` 为必填且有长度限制

#### Scenario: 禁止扩展字段提示

- **WHEN** 阅读 SKILL.md `arguments` 章节
- **THEN** SHALL 包含"OpenClaw 不要在 arguments 中传入额外字段"的明确警示

### Requirement: SKILL 必须给出错误码与重试策略

SKILL.md SHALL 包含"错误处理"章节，至少列出以下错误场景与 OpenClaw 应有的客户端响应：

| HTTP / JSON-RPC code | 含义 | OpenClaw 应有动作 |
|---|---|---|
| HTTP 403 / 401 | JWT 失效 / 缺失 | 重新申请 JWT 后重试；MUST NOT 立即重试 |
| HTTP 429 | 网关限流 | 按 `Retry-After` 头退避；如未提供按指数退避 |
| HTTP 5xx | 网关 / api-server 异常 | 按指数退避重试，最多 3 次 |
| JSON-RPC `-32601` | Method not found | 工具名错误，OpenClaw 应检查配置 |
| JSON-RPC `-32602` | Invalid params | 参数校验失败，OpenClaw 应修正 `arguments` 后重试 |
| JSON-RPC `-32603` | Internal error | 后端异常，按指数退避重试 |

SHALL 显式说明：OpenClaw MUST NOT 在 `notifications/cancelled` 后立即重新发起 `tools/call`（避免任务取消风暴）。

#### Scenario: 错误表格齐全

- **WHEN** 阅读 SKILL.md 错误处理章节
- **THEN** SHALL 找到 HTTP 403/429/5xx 与 JSON-RPC -32601/-32602/-32603 至少 6 个场景的说明

### Requirement: SKILL 必须明确取消与超时语义

SKILL.md SHALL 包含"取消与超时"章节，明确：

- **客户端取消**：OpenClaw 通过发送 `notifications/cancelled with params.requestId=<progressToken>` 取消正在进行的 `tools/call`；
- **服务端 push**：服务端通过 `notifications/progress` 增量推送，`progress` 字段为 0~1 浮点；客户端 SHALL 解析 `message` 字段累积文本作为最终展示；
- **超时建议**：客户端单次 `tools/call` 总超时 SHOULD 设置 ≥ 300s（HCM 复杂查询可能需要长时间）；
- **空闲断连**：SSE 长连接默认服务端心跳 30s，客户端 30s 内未收到任何事件可主动断连并重连。

#### Scenario: 取消与超时章节完整

- **WHEN** 阅读 SKILL.md
- **THEN** SHALL 找到客户端取消方式、progress 解析、推荐超时三个明确说明

### Requirement: SKILL 必须包含安全与审计提示

SKILL.md SHALL 包含"安全与审计"章节，明确：

- OpenClaw 的所有调用都会被蓝鲸网关与 HCM agent-server 完整审计（含 `bk_username`、`mcp_server_name`、`tool_name`、`request_id`）；
- OpenClaw MUST NOT 通过 `text` 字段尝试 prompt injection 攻击；
- OpenClaw MUST NOT 在多用户场景下复用同一 JWT（每次调用 SHALL 携带本次用户的 JWT）。

#### Scenario: 安全章节齐全

- **WHEN** 阅读 SKILL.md
- **THEN** SHALL 找到审计字段说明、prompt injection 警示、JWT 多租户警示三项内容

