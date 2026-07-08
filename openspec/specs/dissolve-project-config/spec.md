# dissolve-project-config

## Purpose

裁撤项目动态配置能力：以 `global_config` 存储裁撤周期与项目，扩展裁撤配置接口、透传裁撤系统项目列表，并以 `ignore_biz` + `is_ignore` 取代原 `blacklist` 配置。

## Requirements

### Requirement: 裁撤项目动态配置

系统 SHALL 在 `global_config`（`config_type=res_dissolve`，`config_key=dissolve_project`）存储「裁撤周期」数组，每个周期含 `start`/`end`（`yyyy-MM-dd`）、`default`（bool，可多个或零个，不要求唯一）、`projects[]{id,memo}`。写入前 SHALL 校验日期格式合法且 `start <= end`、`projects` 非空且 `id > 0`。

#### Scenario: 写入合法裁撤周期配置

- **WHEN** 更新 `dissolve_project` 配置，传入日期合法且 `start <= end`、`projects` 非空且每项 `id > 0`
- **THEN** 系统 SHALL 全量覆盖写入 `global_config`

#### Scenario: 拒绝非法配置

- **WHEN** 更新配置传入 `start > end` 或 `projects` 为空或存在 `id <= 0`
- **THEN** 系统 SHALL 返回参数校验错误，不写入

### Requirement: 裁撤配置接口扩展 dissolve_projects

裁撤配置查询接口 `GET /api/v1/woa/dissolve/config` 的响应 SHALL 在原有字段基础上增加 `dissolve_projects`；更新接口 `PUT /api/v1/woa/dissolve/config/upsert` SHALL 支持全量覆盖 `dissolve_projects`，按上述规则校验。

#### Scenario: 查询返回裁撤项目配置

- **WHEN** 调用 `GET /dissolve/config`
- **THEN** 响应 SHALL 包含 `dissolve_projects` 字段及原有裁撤申领统计起始时间、审批阈值、配额系数、业务偏移字段

#### Scenario: 更新覆盖裁撤项目配置

- **WHEN** 调用 `PUT /dissolve/config/upsert` 传入 `dissolve_projects`
- **THEN** 系统 SHALL 全量覆盖 `dissolve_project` 配置

### Requirement: 裁撤系统项目列表透传

系统 SHALL 在裁撤系统客户端新增 `ListProjects` 方法（对接 `/openapi_gateway/abolish-backend/device/listProjects`，复用现有 token 鉴权，无请求参数），并通过 woa-server `GET /api/v1/woa/dissolve/projects` 透传裁撤系统项目列表，供前端配置裁撤项目时选择。响应每项含 `id`、`projectName`、`projectType`（透传裁撤系统原始驼峰字段，未做下划线转换）。

#### Scenario: 透传项目列表

- **WHEN** 前端调用 `GET /dissolve/projects`
- **THEN** woa-server SHALL 调用裁撤系统 `ListProjects` 并返回 `[{id, projectName, projectType}]`

### Requirement: ignore_biz 配置取代 blacklist

系统 SHALL 以服务配置 `ignore_biz`（`[]int64`）+ 主机 `is_ignore` 字段取代原顶层 `blacklist`（逗号分隔业务名）配置；裁撤查询的黑名单过滤 SHALL 改为查询条件 `is_ignore=false`，并移除 ES 客户端的 `blacklist` 入参。

#### Scenario: 查询用 ignore 过滤

- **WHEN** 执行任意裁撤查询
- **THEN** 系统 SHALL 用 `is_ignore=false` 过滤忽略主机，而非运行时按业务名黑名单转换排除
