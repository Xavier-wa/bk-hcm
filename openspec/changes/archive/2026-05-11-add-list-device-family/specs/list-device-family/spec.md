## ADDED Requirements

### Requirement: 获取实例族列表
系统 SHALL 提供接口，返回 `woa_device_type` 表中 `device_family` 字段所有非空值的去重列表。

#### Scenario: 正常返回实例族列表
- **WHEN** 客户端请求 `GET /api/v1/woa/meta/device_family/list`
- **THEN** 系统分页遍历 `woa_device_type` 表，提取所有非空 `device_family` 值并去重，以字符串数组形式返回

#### Scenario: 表中无数据时返回空列表
- **WHEN** `woa_device_type` 表中无任何记录，客户端请求该接口
- **THEN** 系统返回空的 `details` 数组，HTTP 状态码为 200

#### Scenario: device_family 字段为空时过滤
- **WHEN** 表中存在 `device_family` 为空字符串的记录
- **THEN** 系统不将空字符串包含在返回结果中
