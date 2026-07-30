## 技能文档

### 基本信息
- 技能名: `openclaw-mcp-access`
- 创建人: @ironguo (ironguo@tencent.com)
- 版本: v1.0.0
- 更新时间: 2026-07-20

### 适用场景

把 HCM MCP 安装到 OpenClaw，通过自然语言使用 HCM Agent 当前及后续扩展的业务能力，包括：

- 按业务 ID 查询 CVM、VPC、子网、云硬盘、CLB 等云资源；
- 发起 CVM 主机申领，并在连续对话中补充配置、确认方案和提单；
- 查询主机库存、CVM 预测单据及预测余量；
- 查询申领单、审批状态和交付进度；
- 使用 HCM Agent 后续通过 `send_message` 扩展的其他能力。

### 前置条件

- 已安装支持远程 Streamable HTTP MCP Server 的 OpenClaw；
- HCM 管理员已提供蓝鲸网关地址、stage、MCP Server 名称及鉴权方式；
- 当前用户已获得对应蓝鲸业务和 HCM 资源权限。

### 安装 Skill

把整个 `openclaw-mcp-access` 目录复制到 OpenClaw 工作区：

```bash
mkdir -p ~/.openclaw/workspace/skills
cp -R openclaw-mcp-access ~/.openclaw/workspace/skills/
```

确认 Skill 已加载：

```bash
openclaw skills list
```

若当前会话没有感知到新 Skill，执行 `/new` 开启新会话，或重启 Gateway：

```bash
openclaw gateway restart
```

### 配置 HCM MCP Server

devhk 环境地址格式如下（按实际网关域名、stage 与 mcp_server_name 替换占位符）：

```text
https://{bk_apigw_domain}/api/bk-hcm/{stage}/api/v2/mcp-servers/{mcp_server_name}/mcp/
```

先把蓝鲸网关 access token 配置为 OpenClaw Gateway 进程可读取的环境变量 `BK_HCM_MCP_ACCESS_TOKEN`，不要把真实 token 写入 Skill 或提交到代码仓库。

推荐使用 OpenClaw CLI 注册，MCP 别名统一使用 `hcm-mcp`，避免与 HCM Agent 本身混淆：

```bash
openclaw mcp add hcm-mcp \
  --url "https://{bk_apigw_domain}/api/bk-hcm/{stage}/api/v2/mcp-servers/{mcp_server_name}/mcp/" \
  --transport streamable-http \
  --header 'X-Bkapi-Authorization={ "access_token": "${BK_HCM_MCP_ACCESS_TOKEN}" }' \
  --timeout 300 \
  --connect-timeout 10 \
  --include send_message
```

也可以把 `references/openclaw-config.example.json5` 中的 `mcp.servers.hcm-mcp` 合并到 `~/.openclaw/openclaw.json`。

OpenClaw 只携带蓝鲸网关要求的 `X-Bkapi-Authorization`。不要自行构造或持久化 `X-Bkapi-JWT`，该 header 应由蓝鲸网关在转发到 HCM 时注入。其他环境的 URL 和鉴权方式由 HCM 管理员提供。

配置后执行连通性检查：

```bash
openclaw mcp doctor hcm-mcp --probe
openclaw mcp tools hcm-mcp
```

工具列表应只包含 `send_message`。若不存在该工具，先检查 URL、stage、MCP Server 名称和网关鉴权。

### 使用示例

可直接在 OpenClaw 中输入：

```text
查询业务 639 下运行中的 CVM，列出实例 ID、名称、地域、内网 IP 和状态。
```

```text
查询业务 639 在南京地域的标准型 S5 库存和预测余量。
```

```text
查询业务 639 最近一个月的 CVM 预测单据，列出状态、机型和预测余量。
```

```text
帮业务 639 在南京申领 5 台标准型 S5，8 核 16G，用于开发测试。
```

```text
查看我刚才那批主机的申领和审批进度。
```

连续追问或补充申领参数时保持在同一个 OpenClaw 会话中，Skill 会复用 HCM 返回的 `contextId`。

### 注意事项

⚠️ CVM 申领会创建真实业务单据，必须完成 HCM Agent 返回的确认流程。

⚠️ 不要把 JWT、app secret、access token、cookie 写入 Skill 文件、聊天内容或普通日志。

⚠️ 不同用户、租户和无关业务会话之间不得复用 `contextId`。

### 已知问题

- [ ] 生产环境网关域名、stage 和 MCP Server 名称需由部署管理员提供。
- [ ] 蓝鲸网关鉴权配置因环境而异，当前包不内置凭据或自动化开通脚本。
- [ ] CVM 申领前需在目标环境完成 `bk_biz_id` 从北向 MCP metadata 传递到 Agent 申领流程的端到端验证。
- [x] v1.0.0 已将主文档从底层 JSON-RPC 接入说明调整为业务查询与 CVM 申领操作指南。

### 相关文档

- `references/openclaw-config.example.json5`: OpenClaw MCP 配置模板
- `references/integration-troubleshooting.md`: MCP 协议、错误码、超时与排障
