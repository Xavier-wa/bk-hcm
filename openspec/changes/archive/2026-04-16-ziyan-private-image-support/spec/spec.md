# ziyan-private-image-sync

## ADDED Requirements

### Requirement: 自研云同步支持私有镜像

系统 SHALL 在自研云镜像同步时，同时同步公共镜像（`PUBLIC_IMAGE`）和私有镜像（`PRIVATE_IMAGE`），并正确记录镜像类型。

#### Scenario: 同步私有镜像入库

- **WHEN** 触发自研云镜像同步
- **THEN** 系统 SHALL 调用腾讯云 DescribeImages 接口，过滤条件包含 `PUBLIC_IMAGE` 和 `PRIVATE_IMAGE`，将返回的所有镜像入库，`type` 字段记录云端实际类型

#### Scenario: 镜像类型正确记录

- **WHEN** 同步到一个类型为 `PRIVATE_IMAGE` 的镜像
- **THEN** 系统 SHALL 将该镜像的 `type` 字段设置为 `PRIVATE_IMAGE`，而非硬编码的 `public`

# image-biz-tag-mgmt

## ADDED Requirements

### Requirement: 镜像业务标签管理接口

系统 SHALL 提供给镜像配置业务标签的接口 `PUT /api/v1/cloud/images/{id}/biz_tag`，用于本地配置私有镜像的业务归属。

#### Scenario: 成功给私有镜像打业务标签

- **WHEN** 调用 `PUT /api/v1/cloud/images/{id}/biz_tag`，传入 `bk_biz_id`，且目标镜像为私有镜像
- **THEN** 系统 SHALL 更新镜像表的 `bk_biz_id` 字段，返回成功

#### Scenario: 给公共镜像打标签失败

- **WHEN** 调用 `PUT /api/v1/cloud/images/{id}/biz_tag`，目标镜像为公共镜像
- **THEN** 系统 SHALL 返回错误，提示公共镜像不支持打业务标签

#### Scenario: 镜像不存在

- **WHEN** 调用 `PUT /api/v1/cloud/images/{id}/biz_tag`，镜像 ID 不存在
- **THEN** 系统 SHALL 返回错误，提示 "image id (xxx) not found"

#### Scenario: bk_biz_id 参数校验

- **WHEN** 调用 `PUT /api/v1/cloud/images/{id}/biz_tag`，`bk_biz_id` 小于 1
- **THEN** 系统 SHALL 返回参数校验错误

# biz-cvm-image-query

## ADDED Requirements

### Requirement: 业务维度镜像查询接口

系统 SHALL 提供业务维度的镜像查询接口 `POST /api/v1/woa/bizs/{bk_biz_id}/config/cvm/image`，返回公共镜像和该业务可用的私有镜像。

#### Scenario: 查询业务可用镜像列表

- **WHEN** 调用 `POST /api/v1/woa/bizs/{bk_biz_id}/config/cvm/image`，传入 `vendor`、`region`
- **THEN** 系统 SHALL 返回该地域下的公共镜像（`type = 'PUBLIC_IMAGE'`）和该业务的私有镜像（`type = 'PRIVATE_IMAGE' AND bk_biz_id = bk_biz_id`）

#### Scenario: 未配置业务标签的私有镜像不返回

- **WHEN** 某私有镜像的 `bk_biz_id` 为 NULL 或与请求的 `bk_biz_id` 不匹配
- **THEN** 系统 SHALL 不在响应中返回该私有镜像

#### Scenario: 参数校验 - vendor 必填

- **WHEN** 请求缺少 `vendor` 参数
- **THEN** 系统 SHALL 返回参数校验错误

#### Scenario: 参数校验 - region 必填

- **WHEN** 请求缺少 `region` 参数
- **THEN** 系统 SHALL 返回参数校验错误

# tcloud-image-adaptor

## MODIFIED Requirements

### Requirement: 镜像类型读取云端实际值

系统 SHALL 在 Adaptor 层将镜像类型从云端实际值（`*pImage.ImageType`）读取，而非硬编码。

#### Scenario: 公共镜像类型正确

- **WHEN** 同步到 `ImageType` 为 `PUBLIC_IMAGE` 的镜像
- **THEN** 系统 SHALL 将镜像的 `Type` 字段设置为 `PUBLIC_IMAGE`

#### Scenario: 私有镜像类型正确

- **WHEN** 同步到 `ImageType` 为 `PRIVATE_IMAGE` 的镜像
- **THEN** 系统 SHALL 将镜像的 `Type` 字段设置为 `PRIVATE_IMAGE`

# image-table-biz-id

## MODIFIED Requirements

### Requirement: 镜像表新增 bk_biz_id 字段

系统 SHALL 在镜像表（`image`）中新增 `bk_biz_id` 字段，用于记录私有镜像的业务归属。

#### Scenario: 字段定义

- **WHEN** 镜像表结构定义
- **THEN** `bk_biz_id` 字段 SHALL 为 `BIGINT` 类型，默认值为 `NULL`，允许为空

#### Scenario: 列描述定义

- **WHEN** 查询 `ImageColumnDescriptor`
- **THEN** SHALL 包含 `{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric}`

#### Scenario: 同步时不覆盖已配置的 bk_biz_id

- **WHEN** 同步镜像时，目标镜像已存在且 `bk_biz_id` 已配置
- **THEN** 系统 SHALL 保留原有 `bk_biz_id` 值，不覆盖为 NULL

#### Scenario: 新同步的私有镜像默认无业务标签

- **WHEN** 同步到新的私有镜像
- **THEN** 系统 SHALL 将 `bk_biz_id` 设置为 NULL，表示未配置业务归属，不展示给任何业务

# image-api-structure

## MODIFIED Requirements

### Requirement: BaseImage 结构体新增字段

系统 SHALL 在 `BaseImage` 结构体中新增 `BkBizID` 和 `Region` 字段。

#### Scenario: BkBizID 字段定义

- **WHEN** 查询镜像详情
- **THEN** 响应 SHALL 包含 `bk_biz_id` 字段（int64 类型）

#### Scenario: Region 字段定义

- **WHEN** 查询镜像详情
- **THEN** 响应 SHALL 包含 `region` 字段（string 类型）

### Requirement: 转换函数正确映射新字段

系统 SHALL 在 `conv.go` 的转换函数中正确映射 `BkBizID` 和 `Region` 字段。

#### Scenario: toProtoImageExtResult 映射

- **WHEN** 调用 `toProtoImageExtResult` 函数
- **THEN** 返回结果 SHALL 包含正确的 `BkBizID` 和 `Region` 值

#### Scenario: toProtoImageResult 映射

- **WHEN** 调用 `toProtoImageResult` 函数
- **THEN** 返回结果 SHALL 包含正确的 `BkBizID` 和 `Region` 值
