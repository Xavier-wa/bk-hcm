# HCM MCP 安装与排障参考

仅在安装、连通性检查或 MCP 调用失败时读取本文档。日常资源查询和 CVM 申领由 OpenClaw MCP Runtime 调用 `send_message`，无需手工构造 JSON-RPC 请求。

## 连接信息

蓝鲸网关对外地址：

```text
https://{bk_apigw_domain}/{stage}/api/v2/mcp-servers/{mcp_server_name}/mcp/
```

devhk 环境地址（按实际网关域名、stage 与 mcp_server_name 替换占位符）：

```text
https://{bk_apigw_domain}/api/bk-hcm/{stage}/api/v2/mcp-servers/{mcp_server_name}/mcp/
```

HCM api-server 内网 ingress：

```text
/api/v1/mcp/servers/{mcp_server_name}/mcp/
```

二者不能混用。OpenClaw 应配置网关对外地址，transport 使用 `streamable-http`。

协议请求要求：

```http
Content-Type: application/json
Accept: application/json, text/event-stream
X-Bkapi-Request-Id: <rid>
```

devhk 环境使用 `X-Bkapi-Authorization` 传递 access token，推荐通过 `${BK_HCM_MCP_ACCESS_TOKEN}` 环境变量注入。不要由 OpenClaw 构造 `X-Bkapi-JWT`，该 header 由网关向 HCM api-server 注入。

## 工具契约

`tools/list` 应只返回一个外部工具：

```json
{
  "name": "send_message",
  "inputSchema": {
    "type": "object",
    "properties": {
      "text": { "type": "string" },
      "contextId": { "type": "string" },
      "bk_biz_id": { "type": "integer" },
      "model_name": { "type": "string" }
    },
    "required": ["text"]
  }
}
```

实际 schema 以当前环境的 `tools/list` 为准。默认 `text` 最大长度为 8000 字符。

首轮调用省略 `contextId`，HCM Agent 会生成并在最终结果的 `_meta.contextId` 返回。后续连续对话原样复用该值。

## 手工协议检查

只在 OpenClaw MCP Runtime 无法完成探测时使用以下流程：

1. `initialize`
2. `notifications/initialized`
3. `tools/list`
4. `tools/call`

最小 `tools/call` 示例：

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "send_message",
    "arguments": {
      "text": "查询业务 639 下运行中的 CVM",
      "bk_biz_id": 639
    },
    "_meta": {
      "progressToken": "openclaw-req-001"
    }
  }
}
```

最终结果示例：

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "查询结果..."
      }
    ],
    "isError": false,
    "_meta": {
      "contextId": "ctx-xxx",
      "taskId": "task-xxx"
    }
  }
}
```

## 流式响应、超时与取消

- 单次请求超时建议设置为 300 秒。
- 网关资源必须开启 streaming。
- SSE 心跳建议为 30 秒；不要把短暂无业务消息误判为失败。
- `notifications/progress` 是中间进度，最终业务结果以对应 JSON-RPC `id` 的 `CallToolResult` 为准。
- 不要把 progress 中的思考过程作为最终业务结论。

取消任务时，`requestId` 必须等于原 `tools/call` 的 `_meta.progressToken`：

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/cancelled",
  "params": {
    "requestId": "openclaw-req-001",
    "reason": "user cancelled"
  }
}
```

取消是 best-effort 行为。取消后不要立即重复提交同一操作，以免产生重复任务。

## 错误处理

| 错误 | 含义 | 处理 |
|---|---|---|
| HTTP 401/403 | 网关认证失败、JWT 未注入或用户无权限 | 停止重试，重新授权并检查网关配置、用户及业务权限。 |
| HTTP 429 | 网关或服务限流 | 优先按 `Retry-After` 退避，否则指数退避。 |
| HTTP 5xx | 网关、api-server、agent-server 或下游异常 | 最多重试 1～2 次，仍失败则提供 request ID 排查。 |
| JSON-RPC `-32601` | method 或工具不存在 | 检查 URL、MCP Server 配置及工具名 `send_message`。 |
| JSON-RPC `-32602` | 参数非法 | 检查 `text` 必填、长度和其他参数类型，不要添加额外字段。 |
| JSON-RPC `-32603` | 服务内部错误 | 有限重试后记录 request ID、MCP Server 名称、contextId 和 taskId。 |
| `result.isError=true` | HCM Agent 正常返回业务失败 | 展示 `content` 中的原因，不按协议异常自动重试。 |

## 排查清单

1. 执行 `openclaw mcp doctor hcm-mcp --probe`。
2. 执行 `openclaw mcp tools hcm-mcp`，确认只有 `send_message`。
3. 检查 URL 是否包含正确的网关域名、stage、MCP Server 名称及末尾 `/mcp/`。
4. 检查 transport 是否为 `streamable-http`，请求超时是否为 300 秒。
5. 检查蓝鲸网关是否开启 streaming 并完成用户鉴权和 JWT 注入。
6. 收到 404 时，请管理员确认 api-server 的北向 MCP ingress 和 agent-server A2A 服务已在目标环境显式启用。
7. CVM 申领失败但普通查询正常时，检查 `bk_biz_id` 是否已从北向 MCP metadata 正确注入 Agent 申领会话。
8. 记录 request ID；若已有响应，同时记录 `contextId`、`taskId`，但不要记录凭据。
