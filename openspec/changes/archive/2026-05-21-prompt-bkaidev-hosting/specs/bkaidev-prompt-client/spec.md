## ADDED Requirements

### Requirement: BKAIDev Client 新增 Prompt 接口定义与实现
BKAIDev Client 接口 SHALL 新增 `ListPrompts` 和 `RetrievePrompt` 两个方法，分别对应 `list_app_v1_prompts` 和 `retrieve_app_v1_prompts`。
在 `pkg/thirdparty/api-gateway/bkaidev/` 目录下新增 `prompt.go` 文件，包含请求/响应类型及实现。

Client 接口新增签名：
```go
ListPrompts(kt *kit.Kit, req *ListPromptsReq) ([]PromptListItem, error)
RetrievePrompt(kt *kit.Kit, req *RetrievePromptReq) (*PromptDetail, error)
```

`ListPromptsReq` 应映射 `list_app_v1_prompts` 的查询参数（page、page_size、prompt_id、space_id 等），`RetrievePromptReq` 包含路径参数 `PromptID int` 和可选查询参数 `SpaceID string`。

`PromptDetail` 包含 `PromptID`、`PromptName`、`PromptCode`、`Content`、`UpdatedAt` 等核心字段。

分页响应 `ListPromptsResp` 包含 `Page`、`NumPages`、`Count`、`Results []PromptListItem`。

#### Scenario: 按 prompt_id 获取 prompt 详情成功
- **WHEN** 调用 `RetrievePrompt` 且 BKAIDev 服务返回 `result=true`
- **THEN** 返回对应 prompt 的 content 等详情，error 为 nil

#### Scenario: RetrievePrompt 请求参数校验失败
- **WHEN** 调用 `RetrievePrompt` 时 `req.PromptID <= 0`
- **THEN** 返回参数校验错误，不发起 HTTP 请求

#### Scenario: BKAIDev 返回业务错误
- **WHEN** BKAIDev 返回 `result=false`
- **THEN** 返回包含 code/message/request_id/trace_id 的 error，格式与 skill client 保持一致

### Requirement: 复用现有 BKAIDev HTTP 基础设施
Prompt client SHALL 复用 `skillClient` 结构体（同一个 `client` 字段和 `authHeader` 方法），通过在同一个 `skillClient` 上新增方法实现，不引入新的 HTTP 客户端结构体。

#### Scenario: Prompt 接口使用与 skill 相同的鉴权头
- **WHEN** 调用任意 prompt 接口
- **THEN** HTTP 请求携带与 skill 接口完全相同格式的 `X-Bkapi-Authorization` 头
