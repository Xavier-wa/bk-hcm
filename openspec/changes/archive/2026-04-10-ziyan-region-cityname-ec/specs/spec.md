## ADDED Requirements

### Requirement: 通过 CRP 接口获取完整 city_name

地域同步流程 SHALL 通过调用 CRP `queryZoneCityList` 接口获取包含 EC 后缀的完整 city_name，替换从云上 DescribeRegions 接口 RegionName 字段提取的不完整 city_name。

#### Scenario: 新增地域时获取完整 city_name
- **WHEN** 系统执行自研云地域同步且发现新增地域
- **THEN** 系统 SHALL 调用 CRP `queryZoneCityList` 接口
- **AND** 使用接口返回的 cityName（含 EC 后缀）作为该地域的 city_name

#### Scenario: 更新地域时获取完整 city_name
- **WHEN** 系统执行自研云地域同步且发现地域信息变更
- **THEN** 系统 SHALL 调用 CRP `queryZoneCityList` 接口
- **AND** 使用接口返回的 cityName（含 EC 后缀）更新该地域的 city_name

#### Scenario: CRP 接口调用失败处理
- **WHEN** CRP `queryZoneCityList` 接口调用失败
- **THEN** 系统 SHALL 记录错误日志
- **AND** 回退到原有逻辑（从 RegionName 提取 city_name）
- **AND** 地域同步流程继续执行

### Requirement: CRP 接口配置管理

系统 SHALL 复用现有的 CVM API 客户端基础设施调用 CRP 接口，无需独立配置管理。

#### Scenario: 复用 CVM API 客户端
- **WHEN** 系统调用 CRP `queryZoneCityList` 接口
- **THEN** 系统 SHALL 使用现有的 `pkg/thirdparty/cvmapi` 客户端
- **AND** 复用 CVM API 的地址配置和连接管理

### Requirement: CRP 接口响应解析

系统 SHALL 正确解析 CRP `queryZoneCityList` 接口的响应，提取与当前 region 匹配的 city_name。

#### Scenario: 解析成功响应
- **WHEN** CRP 接口返回成功响应
- **THEN** 系统 SHALL 解析 result 数组
- **AND** 根据 region 字段匹配找到对应记录
- **AND** 提取该记录的 cityName 字段

#### Scenario: 未找到匹配的 region
- **WHEN** CRP 接口响应中不包含当前同步的 region
- **THEN** 系统 SHALL 记录警告日志
- **AND** 回退到原有逻辑（从 RegionName 提取 city_name）
