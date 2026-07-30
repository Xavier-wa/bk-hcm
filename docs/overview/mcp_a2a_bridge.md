# api-server MCP / A2A Bridge 部署发布指南

本文说明 `api-server-mcp-a2a-bridge` 变更上线时的部署步骤、网关配置、Kubernetes 隔离、灰度顺序和监控告警。该能力默认关闭；未显式开启时，api-server 仍只承担原有 `/api/v1/{cloud,woa,account}/...` 代理职责。

## 链路说明

### 北向 MCP ingress

OpenClaw 等 MCP 客户端经蓝鲸网关访问：

```text
OpenClaw
  -> 蓝鲸网关 /api/v2/mcp-servers/{mcp_server_name}/mcp/
  -> api-server /api/v1/mcp/servers/{mcp_server_name}/mcp/
  -> agent-server /api/v1/agent/a2a
```

该链路只向 MCP 客户端暴露一个聚合工具 `send_message`。`mcp_server_name` 可以是 `bk-hcm-devhk-tcloud-ziyan-cvm`、`bk-hcm-devhk-tcloud-cvm-operate`、`bk-hcm-finops` 等网关侧已声明的名称；api-server 对这些名称统一处理，并把名称写入日志和 metrics 标签。

### 北向 A2A passthrough

原生 A2A 客户端经蓝鲸网关访问：

```text
A2A client
  -> 蓝鲸网关 /api/v1/agent/a2a 或 /api/v1/agent/.well-known/*
  -> api-server
  -> agent-server 同名路径
```

api-server 只做 JWT 解析、内部 header 注入和反向代理，不修改 request / response body，并保持 SSE 实时刷写。

### 南向内部 HCM MCP

agent-server LLM 调用 HCM 内部工具：

```text
agent-server
  -> api-server /api/v1/mcp/internal/hcm/mcp/
  -> api-server 自身 proxy 链路
  -> cloud-server / woa-server / account-server
```

该入口只允许内网 agent-server 调用，不在公网蓝鲸网关发布。工具集从本地 OpenAPI 3.0 yaml 一次性加载，默认文件为 `bk_apigw_resources_bk-hcm_internal_mcp.yaml`。

## api-server 配置

第一阶段灰度只开启北向 MCP ingress 与 A2A passthrough，不启用南向内部 MCP：

```yaml
mcp:
  ingress:
    enable: true
  bridge:
    connectTimeout: 5s
    readTimeout: 300s
    maxIdleConnsPerHost: 100
  internal:
    enable: false

a2aPassthrough:
  enable: true
```

OpenClaw 联调通过后，再评估开启南向内部 MCP：

```yaml
mcp:
  internal:
    enable: true
    enforceCallerSource: true
    openapiSpecPath: bk_apigw_resources_bk-hcm_internal_mcp.yaml
    includeOperationIDs:
      - list_*
      - get_*
      - describe_*
```

南向开启前必须确认 OpenAPI yaml 已随 api-server 配置一起部署到 `api_server.yaml` 同目录，且 agent-server 的 internal MCP toolset 指向 `http://api-server:8080/api/v1/mcp/internal/hcm/mcp/`。

## 蓝鲸网关配置

北向 MCP server 资源：

- 在网关声明 `bk-hcm-devhk-tcloud-ziyan-cvm` 以及其它需要接入的 `mcp_server_name`。
- 前端访问路径保持网关 MCP Server 规范，例如 `/api/v2/mcp-servers/{name}/mcp/`。
- 后端指向 api-server：`/api/v1/mcp/servers/{name}/mcp/`。
- 打开 streamable HTTP / SSE 支持，确认 `enable_streaming: true` 或等价配置生效。
- 保留 JWT 鉴权，由 api-server 复用 `gwparser` 解析 `X-Bkapi-JWT`。

A2A 资源：

- `POST /api/v1/agent/a2a` 后端指向 api-server 同名路径。
- `GET /api/v1/agent/.well-known/agent-card.json` 后端指向 api-server 同名路径。
- `GET /api/v1/agent/.well-known/agent.json` 后端指向 api-server 同名路径。

南向内部路径必须显式隔离：

- 不在公网网关发布 `/api/v1/mcp/internal/`。
- 如网关有统一前缀转发规则，必须配置 Path Strip / deny rule，确保外部访问 `/api/v1/mcp/internal/hcm/mcp/` 返回 404 或同等拒绝结果。
- 发布前用公网网关域名手动验证该路径无法到达 api-server。

## Kubernetes 隔离

`NetworkPolicy` 只能限制 Pod / namespace / port，不能按 HTTP path 限制 `/api/v1/mcp/internal/`。因此发布时需要同时满足：

- 网关侧不发布 `/api/v1/mcp/internal/`。
- 集群侧添加 NetworkPolicy，仅允许 agent-server 所在 Pod 或命名空间访问 api-server 的服务端口。
- 如果启用该 NetworkPolicy，还必须把 ingress-controller、蓝鲸网关内网出口、监控采集器等现有合法来源加入允许列表，避免误切断存量 api-server 流量。

Helm chart 已提供默认关闭的 `apiserver.networkPolicy` 模板。启用前按实际集群 label 调整 `apiserver.networkPolicy.ingress`，不要直接使用示例 label 上线。

## 灰度顺序

1. hk 联调环境部署新版本，保持 `mcp.internal.enable: false`。
2. 开启 `mcp.ingress.enable: true` 与 `a2aPassthrough.enable: true`。
3. 用 `scripts/mcp_a2a_smoke.sh` 验证 `initialize`、`tools/list`、`tools/call(send_message)`。
4. 验证 `/api/v1/cloud/...` proxy 抽样请求与 AGUI 链路无回归。
5. 配置 OpenClaw 指向 `bk-hcm-devhk-tcloud-ziyan-cvm`，进行多轮 `contextId` 联调。
6. 北向稳定后，再单独启用南向内部 MCP，并只开放查询类 `includeOperationIDs`。
7. 若出现异常，关闭对应开关并重启 api-server；proxy 与 AGUI 链路不需要回滚。

## 监控告警

Grafana 看板至少包含：

- `hcm_api_server_mcp_tools_call_total{tool,mcp_server_name,status}`：按 `mcp_server_name`、`status` 统计调用量和失败量。
- `hcm_api_server_mcp_tools_call_duration_seconds_bucket`：查看 `tools/call(send_message)` P50 / P90 / P99。
- `hcm_api_server_mcp_bridge_active_tasks`：观察 MCP ↔ A2A bridge 进行中的任务数。
- api-server 原有 HTTP QPS、5xx、延迟指标，用于确认新增 mux 路径未影响 proxy。
- agent-server A2A / LLM / tool calling 指标，用于定位下游耗时。

建议告警：

- `tools/call` 5 分钟错误率超过 5%。
- `tools/call` P99 连续 5 分钟超过 3s（不含 LLM 推理的压测口径；线上需结合 agent-server 耗时拆分判断）。
- `bridge_active_tasks` 长时间高于灰度期基线，或持续不下降。
- `/api/v1/{cloud,woa,account}` proxy 5xx / 延迟相对发布前基线异常上升。

已下线的 `hcm_api_server_mcp_internal_schema_stale` / `bk_hcm_api_server_internal_mcp_schema_stale` 不应再配置看板或告警；南向工具 schema 改为本地 OpenAPI yaml 启动时一次性加载，不存在网关周期同步陈旧状态。

## 发布验收清单

- 北向 MCP ingress 的 `initialize`、`tools/list`、`tools/call(send_message)` 可用。
- 任意已注册 `mcp_server_name` 的 `tools/list` 都只返回 `send_message`。
- A2A AgentCard 透传返回的 URL 不暴露 agent-server 内网地址。
- 公网访问 `/api/v1/mcp/internal/hcm/mcp/` 不到达 api-server。
- 原 `/api/v1/{cloud,woa,account}` proxy 链路抽样通过。
- web-server -> agent-server AGUI 冒烟通过。
- Grafana 看板和关键告警已创建并完成一次数据校验。
