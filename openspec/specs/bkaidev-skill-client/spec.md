## ADDED Requirements

### Requirement: BKAIDev Skill API Client 封装

系统 SHALL 在 `pkg/thirdparty/api-gateway/bkaidevapi` 提供 BKAIDev Skill 开放 API 的 HTTP Client，构建方式 MUST 与现有 api-gateway Client（如 `bkchatapi`）一致：使用 `rest.NewClient` + `ServerDiscovery` + `client.Capability`。

Client MUST 实现以下方法：

- `ListSkills(ctx, appID, tags)` → 调用 `list_app_v1_skills`
- `RetrieveSkill(ctx, appID, skillName)` → 调用 `retrieve_app_v1_skills`
- `GetSkillDownloadURL(ctx, appID, skillName, version)` → 调用 `retrieve_app_v1_skills_download`
- `ListSkillVersions(ctx, appID, skillName)` → 调用 `retrieve_app_v1_skills_versions`

#### Scenario: ListSkills 使用 tag 过滤 enabled 状态

- **WHEN** 调用 `ListSkills` 且传入 tag 包含 `status:enabled`（或配置等价 tag）
- **THEN** 请求 MUST 携带 tag 查询参数
- **THEN** 响应 MUST 仅包含 status 为 enabled 的 Skill 条目

#### Scenario: RetrieveSkill 返回详情

- **WHEN** 调用 `RetrieveSkill` 传入有效的 appID 与 skillName
- **THEN** 响应 MUST 包含 SKILL.md 内容、版本号、文件 md5 等字段

#### Scenario: GetSkillDownloadURL 返回 zip 下载地址

- **WHEN** 调用 `GetSkillDownloadURL` 传入 appID、skillName 与 version
- **THEN** 响应 MUST 返回可用于 HTTP GET 下载 skill.zip 的 URL

#### Scenario: API 鉴权

- **WHEN** Client 发起任意 BKAIDev Skill API 请求
- **THEN** 请求 MUST 携带 `X-Bkapi-Authorization` 头，使用配置的 `appCode` 与 `appSecret` 构建

#### Scenario: API 错误处理

- **WHEN** BKAIDev API 返回 `result=false` 或非零 `code`
- **THEN** Client MUST 返回包含 API message 的 error，日志 MUST 记录 request_id（如有）
