# rolling-server-ai-mcp-exposure Specification

## Purpose

把滚服"以旧换新"链路所需的接口暴露到 AI 可调用的两条 MCP 通道（APIGW MCP 资源文件与 api-server 内置 MCP 白名单），使 AI Agent 既能在用户主动指定固资号时校验固资、也能在用户未指定时主动列出继承固资候选并完成提单；同时约束 MCP 暴露面，避免 glob pattern 意外放开非预期接口。

## Requirements

### Requirement: APIGW MCP 通道暴露 check_biz_apply_order_host

系统 SHALL 在 MCP 资源文件 `docs/support-file/helm/files/bk_apigw_resources_bk-hcm_internal_mcp.yaml` 中新增 `check_biz_apply_order_host` 的 openapi 定义（照该文件中已存在的 `create_biz_apply` 与 `get_biz_apply_recommend_by_static` 的形态编写 path / operationId / 请求体 schema / `x-bk-apigateway-resource` 段），供 AI 场景 2（用户主动在对话中指定固资号）使用。同一份定义 SHALL 同步落到对外网关资源文件 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm.yaml`，使该接口在两条通道上口径一致。

该工具的入参 schema SHALL 含 `bk_asset_id`、`region`、`require_type`、`bk_biz_id`，其中 `region` SHALL 标记为 required。现网 `check/apply/order/host` 接口本身的入参、出参与校验逻辑 MUST NOT 被改动——本需求只做暴露；`region` 保持必填，后端 SHALL NOT 做地域反查兜底，用户只给固资号时由 AI skill 追问一次地域。

#### Scenario: APIGW 通道工具列表含该工具

- **GIVEN** APIGW MCP 资源文件已新增 `check_biz_apply_order_host` 的 openapi 定义并部署
- **WHEN** AI Agent 经 APIGW 通道列出可用工具
- **THEN** 工具列表中存在 `check_biz_apply_order_host`，其入参 schema 含 `bk_asset_id`、`region`、`require_type`、`bk_biz_id`，且 `region` 标记为 required

#### Scenario: region 缺失时参数校验失败

- **GIVEN** `check_biz_apply_order_host` 已暴露
- **WHEN** 只传 `bk_asset_id` 与 `bk_biz_id`、不传 `region` 调用
- **THEN** 返回参数校验失败（`region` 必填），后端不做地域反查兜底

### Requirement: create_biz_apply 补齐 bk_asset_id 字段声明

系统 SHALL 在 APIGW MCP 资源文件中为已存在的 `create_biz_apply` 补齐缺失的 `bk_asset_id` 字段声明。不补齐则 AI 拿到固资号也无法在提单请求里把它传出去，滚服提单链路走不通。

#### Scenario: 提单工具入参含固资号

- **GIVEN** APIGW MCP 资源文件已更新并部署
- **WHEN** 查看 `create_biz_apply` 的入参 schema
- **THEN** `bk_asset_id` 字段已声明

### Requirement: api-server 内置 MCP 白名单成对补 check 前缀 pattern

api-server 内置 MCP 的 `includeOperationIDs` 白名单当前只有 `get_biz_*`、`list_biz_*`、`get_apply_*` 这类形态，`check_` 前缀不命中任何一条。系统 SHALL 为该白名单补一条能匹配 `check_biz_apply_order_host` 的 glob pattern，取值 SHALL 为 `check_biz_apply_*` 而非宽泛的 `check_*`。

按项目"配置变更需成对提交"的约定，该 pattern MUST 同时出现在 `cmd/api-server/etc/api_server.yaml`（真实生效的白名单）与 `docs/support-file/helm/values.yaml` 的 `mcp.internal.servers[]` 配置示例中，两处 pattern 文本 MUST 一致。只改其中一处会导致容器化部署环境白名单不生效、通道半可用。

#### Scenario: 内置 MCP 通道工具列表含该工具

- **GIVEN** `cmd/api-server/etc/api_server.yaml` 与 `docs/support-file/helm/values.yaml` 的 `includeOperationIDs` 均已补上匹配 `check_` 前缀的 pattern 并重启 api-server
- **WHEN** 经内置 MCP 通道调 `tools/list`
- **THEN** 返回结果中存在 `check_biz_apply_order_host`

#### Scenario: 配置成对提交

- **GIVEN** 本次变更的提交
- **WHEN** 检查变更文件列表
- **THEN** `cmd/api-server/etc/api_server.yaml` 与 `docs/support-file/helm/values.yaml` 同时出现，两处 `includeOperationIDs` 的 pattern 文本均为 `check_biz_apply_*`

### Requirement: 业务视角继承固资推荐接口进 MCP 通道

系统 SHALL 在 MCP 资源文件中新增业务视角继承固资推荐接口的 openapi 定义（path `/api/v1/woa/bizs/{bk_biz_id}/rolling_servers/inherited_hosts/list`，operationId `list_biz_rolling_server_inherited_hosts`）。该 operationId 命中白名单中**既有**的 `list_biz_*` pattern，因此暴露它 SHALL NOT 需要新增任何 pattern。

暴露的动机是让 AI 在场景 1（用户不指定固资号）也能主动列出候选并向用户确认，而不是只依赖 `by_static` 在服务端内部闷头挑首条——固资是"以旧换新"里用户真正在意的那个选择，服务端替他选完不告诉他并不合适。`by_static` 的服务端内部补全 SHALL 保留，两者互为补充。

资源视角推荐接口（`POST /api/v1/woa/rolling_servers/inherited_hosts/list`）MUST NOT 进入 MCP 资源文件——AI 链路一律带业务上下文，资源视角只服务管理端页面。

#### Scenario: 业务视角推荐接口出现在 MCP 工具列表

- **GIVEN** MCP 资源文件已更新并部署、api-server 已重启
- **WHEN** 经内置 MCP 通道调 `tools/list`
- **THEN** 返回结果中存在 `list_biz_rolling_server_inherited_hosts`，其入参 schema 含 `bk_biz_id`、`region`、`device_families`

#### Scenario: 资源视角推荐接口不进 MCP

- **GIVEN** 两条 MCP 通道均已部署
- **WHEN** 查看工具列表与资源文件
- **THEN** 不存在资源视角路径 `/api/v1/woa/rolling_servers/inherited_hosts/list` 对应的工具或定义

### Requirement: MCP 暴露面约束

新增的 glob pattern MUST NOT 意外放开其他非预期的 `check_` 前缀接口：定 pattern 前 MUST 先列出 openapi spec 中全部 `check_` 前缀 operationId，确认匹配集合仅含预期暴露的接口。

复用既有 pattern 暴露新接口时 MUST 反向确认：新增 operationId 命中哪些既有 pattern、是否为有意暴露。命名让接口意外落进 `list_biz_*` 这类宽泛 pattern 属暴露面失控，MUST 在评审时显式确认而非默认接受。

#### Scenario: 暴露面核对

- **GIVEN** 新增的 `check_biz_apply_*` glob pattern
- **WHEN** 列出 openapi spec 中全部匹配该 pattern 的 operationId
- **THEN** 匹配结果仅包含预期暴露的接口，无意外放开的其他 `check_` 前缀接口

#### Scenario: 新增 operationId 的命中确认

- **GIVEN** 本次新增到 MCP 资源文件的全部 operationId
- **WHEN** 逐个比对白名单中的既有 pattern
- **THEN** 每个 operationId 命中的 pattern 均为有意暴露的结果，无因命名巧合而被放开的接口
