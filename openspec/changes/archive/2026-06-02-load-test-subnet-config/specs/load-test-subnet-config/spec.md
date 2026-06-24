## ADDED Requirements

### Requirement: Upsert 压测子网配置
woa-server SHALL 提供 POST `/api/v1/woa/config/load_test_subnets/upsert` 接口，允许具有 GlobalConfig.Create 权限的运营管理员提交完整的 region→vpc_id→[subnet_id] 三层映射，以全量替换方式写入 global_config 表（config_type=cvm_apply，config_key=load_test_subnet）。

接口语义：
- 若记录不存在：调用 data-service BatchCreate 写入新记录
- 若记录存在：调用 data-service BatchUpdate 全量覆盖 config_value
- 传入空 map `{}` 为合法请求，效果为清空配置
- 请求体 JSON 格式错误时返回 `InvalidParameter`（HTTP 400）
- data-service 调用失败时返回 `Aborted`（HTTP 500）并记录 Error 日志

#### Scenario: 配置不存在时首次写入
- **GIVEN** global_config 表中 config_type=cvm_apply、config_key=load_test_subnet 的记录不存在
- **WHEN** 管理员 POST upsert 接口传入有效的 region→vpc→subnet 映射
- **THEN** DB 中新增一条记录，config_value 为传入的 JSON，HTTP 返回 200

#### Scenario: 配置已存在时全量替换
- **GIVEN** global_config 表中已存在 config_type=cvm_apply、config_key=load_test_subnet 的记录
- **WHEN** 管理员 POST upsert 接口传入新的完整映射
- **THEN** 该记录的 config_value 被全量替换为新值，HTTP 返回 200

#### Scenario: 传入空 map 清空配置
- **GIVEN** global_config 表中已存在压测子网配置记录
- **WHEN** 管理员 POST upsert 接口传入 `{}`
- **THEN** 该记录的 config_value 被更新为 `{}`，HTTP 返回 200

#### Scenario: 请求体格式错误时返回 400
- **GIVEN** 任意配置状态
- **WHEN** 管理员 POST upsert 接口传入非法 JSON 格式的请求体
- **THEN** HTTP 返回 400，错误码 InvalidParameter

#### Scenario: 无 GlobalConfig.Create 权限时返回 403
- **GIVEN** 调用方不具备 GlobalConfig.Create 权限
- **WHEN** 调用 POST upsert 接口
- **THEN** HTTP 返回 403，权限不足错误

### Requirement: List 压测子网配置
woa-server SHALL 提供 GET `/api/v1/woa/config/load_test_subnets` 接口，返回完整的 region→vpc_id→[subnet_id] 压测子网映射。该接口无需额外权限（woa-server 登录态即可访问）。

接口语义：
- 若 global_config 表中记录不存在：返回空 map `{}`，HTTP 200
- 若记录存在：将 config_value 反序列化后作为 data 字段返回
- data-service 查询失败时返回 `Aborted`（HTTP 500）并记录 Error 日志
- config_value 反序列化失败时返回 `Aborted`（HTTP 500）并记录 Error 日志

#### Scenario: 配置存在时返回完整映射
- **GIVEN** global_config 表中存在 config_type=cvm_apply、config_key=load_test_subnet 的有效配置记录
- **WHEN** 用户 GET list 接口
- **THEN** HTTP 返回 200，data 为完整的 region→vpc→subnet JSON 映射

#### Scenario: 配置不存在时返回空对象
- **GIVEN** global_config 表中不存在 config_type=cvm_apply、config_key=load_test_subnet 的记录
- **WHEN** 用户 GET list 接口
- **THEN** HTTP 返回 200，data 为 `{}`

### Requirement: 压测子网数据模型
压测子网配置 SHALL 使用 Go 类型 `LoadTestSubnetConfig = map[string]map[string][]string`，其中：
- 外层 key 为 region（如 `ap-nanjing`）
- 中层 key 为 vpc_id（如 `vpc-xxxxxxxx`）
- 内层 value 为 subnet_id 列表（如 `["subnet-xxx", "subnet-yyy"]`）

配置存储于 global_config 表中，config_type=`cvm_apply`，config_key=`load_test_subnet`。

#### Scenario: 三层嵌套结构序列化
- **WHEN** Upsert 接口接收包含多个 region、多个 vpc、多个 subnet 的 JSON 请求体
- **THEN** 系统 SHALL 将整个映射作为单条 JSON 存入 global_config.config_value

#### Scenario: config_type 枚举值注册
- **WHEN** 系统启动或访问 global_config 相关配置
- **THEN** `GlobalConfigTypeCvmApply`（值为 `"cvm_apply"`）SHALL 在 `pkg/criteria/enumor/global_config.go` 中注册为合法枚举值
