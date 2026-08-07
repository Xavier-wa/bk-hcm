## ADDED Requirements

### Requirement: 业务维度镜像列表查询接口

系统 SHALL 在 cloud-server 提供业务维度镜像列表查询接口（`/api/v1/cloud` 前缀，入参含 `bk_biz_id` 与 path `vendor`），返回全部公共镜像、共享镜像加上当前业务的私有镜像。接口 MUST 为只读，不得产生任何镜像的创建、删除或变更。

#### Scenario: 查询返回公共镜像、共享镜像与本业务私有镜像

- **WHEN** 调用方传入合法 `bk_biz_id` 与 `vendor` 且不带其他过滤条件
- **THEN** 系统 SHALL 返回全部公共镜像与共享镜像，以及 `bk_biz_id` 等于该业务的私有镜像
- **AND** 系统 MUST NOT 返回其他业务的私有镜像

#### Scenario: 空数据返回空列表

- **WHEN** 目标过滤条件下无入库数据
- **THEN** 系统 SHALL 返回空列表且不报错

#### Scenario: 多厂商已入库数据可查询

- **WHEN** 调用方指定非 tcloud/自研云 vendor，且对应厂商存在入库镜像
- **THEN** 系统 SHALL 返回这些厂商的可见镜像（不得因「仅支持 tcloud/自研云」类假设而过滤）

### Requirement: 类型化入参与多维度过滤

接口 SHALL 使用类型化请求字段（`platform`、`name`、`type`、`region`、`page`）接收过滤条件，`vendor` MUST 来自 path 参数；MUST NOT 接受调用方透传的原始 filter 表达式。其中 `vendor`、`platform`、`region` MUST 使用精确匹配，`name` MUST 使用模糊匹配，`type` MUST 按统一语义匹配。

#### Scenario: 按 vendor 与 region 精确过滤

- **WHEN** 调用方指定 path `vendor` 与 body `region`
- **THEN** 系统 SHALL 仅返回同时满足该 vendor 与 region 的镜像

#### Scenario: 按 name 模糊过滤

- **WHEN** 调用方传入 `name` 关键字
- **THEN** 系统 SHALL 返回名称包含该关键字的镜像

#### Scenario: 不传过滤条件

- **WHEN** 调用方未指定 platform/name/type/region
- **THEN** 系统 SHALL 按业务可见性规则返回全部镜像（公共/共享 + 本业务私有）

### Requirement: 镜像类型统一为 public/private/shared

系统 SHALL 将本地镜像 `type` 统一为 `public` / `private` / `shared`。共享镜像 MUST 为独立 `shared` 语义，MUST NOT 并入 `public`。同步入库时 MUST 将云上枚举归一化后写入；查询侧 MUST 仅匹配统一值，MUST NOT 兼容云上大写枚举。入参 `type` MUST 仅接受统一语义；出参 MUST 为库内统一值。

#### Scenario: 按统一语义过滤私有镜像

- **WHEN** 调用方指定 `type` 为 `private`
- **THEN** 系统 SHALL 仅匹配 `private`

#### Scenario: 按统一语义过滤共享镜像

- **WHEN** 调用方指定 `type` 为 `shared`
- **THEN** 系统 SHALL 仅匹配 `shared`
- **AND** 系统 MUST NOT 将共享镜像计入 `type=public` 的过滤结果

#### Scenario: 出参为统一语义

- **WHEN** 返回镜像列表
- **THEN** 系统 SHALL 返回库内统一 type（`public` / `private` / `shared`）

### Requirement: enable_cvm 仅对自研云生效

系统 SHALL 仅对自研云（TCloudZiyan）附加 `extension.enable_cvm = true` 过滤条件；对其他 vendor MUST NOT 附加该条件。

#### Scenario: 自研云镜像受 enable_cvm 约束

- **WHEN** path `vendor` 为自研云（TCloudZiyan）
- **THEN** 系统 SHALL 仅返回 `extension.enable_cvm = true` 的自研云镜像

#### Scenario: 其他厂商不受 enable_cvm 影响

- **WHEN** path `vendor` 为非自研云厂商
- **THEN** 系统 MUST NOT 因 `enable_cvm` 条件过滤掉这些镜像

### Requirement: 业务访问鉴权

系统 SHALL 校验调用方对 `bk_biz_id` 的业务访问权限，防止越权查看其他业务的私有镜像。

#### Scenario: 无业务权限被拒绝

- **WHEN** 调用方无目标 `bk_biz_id` 的访问权限
- **THEN** 系统 SHALL 拒绝请求并返回鉴权错误
