# Spec: obs-bill-city-id

## Purpose

OBS 账单城市 ID 字段：在三张 OBS 账单表（obs_aws_bills、obs_huawei_bills、obs_gcp_bills）中新增 `CityId` 字段，并在上报时通过账单地域到城市 ID 的映射表（account_bill_region_city_rel）自动填充，无映射时按账号站点类型兜底。

## ADDED Requirements

### Requirement: account_bill_region_city_rel 表 CRUD
系统 SHALL 提供 `account_bill_region_city_rel` 表的完整 CRUD 接口（通过 data-service），字段包含 id、region、vendor、city_id、creator、reviser、created_at、updated_at，以 `(vendor, region)` 为唯一键。接口支持批量创建、列表查询（带过滤条件）、单条更新、批量删除。

#### Scenario: 创建地域城市映射
- **GIVEN** 传入 vendor="aws"、region="us-east-1"、city_id=100001
- **WHEN** 调用创建接口
- **THEN** 数据库中插入一条记录，返回新生成的 id

#### Scenario: 重复创建同 vendor+region 映射失败
- **GIVEN** 数据库中已存在 vendor="aws"、region="us-east-1" 的映射
- **WHEN** 再次创建相同 vendor+region 的映射
- **THEN** 返回错误（RecordDuplicated），不插入新记录

#### Scenario: 按 vendor 列表查询
- **GIVEN** 数据库中有 aws 和 gcp 各若干条映射记录
- **WHEN** 以 vendor="aws" 过滤条件调用列表查询接口
- **THEN** 仅返回 vendor="aws" 的记录

#### Scenario: 更新 city_id
- **GIVEN** 存在 id="abc" 的映射记录
- **WHEN** 调用更新接口修改其 city_id
- **THEN** 数据库中该记录的 city_id 被更新，updated_at 刷新

#### Scenario: 删除映射记录
- **GIVEN** 存在若干映射记录
- **WHEN** 传入若干 id 调用批量删除接口
- **THEN** 这些记录从数据库中删除

---

### Requirement: OBS 账单 city_id 字段填充
在 OBS 账单上报时，系统 SHALL 为每条账单记录填充 `city_id`。填充逻辑：先根据账单所属云厂商（vendor）和地域（region）在 `account_bill_region_city_rel` 中查找对应 city_id；若找不到映射，则按主账号站点类型兜底——国内站（MainAccountChinaSite）填充 300001，国际站填充 300002。三个厂商的地域字段来源：HuaWei 使用 `region`，GCP 使用 `Region`，AWS 使用 `product_region`。

#### Scenario: 找到地域映射时填充匹配的 CityId
- **GIVEN** account_bill_region_city_rel 中存在 vendor="huawei"、region="cn-north-4"、city_id=100010 的记录
- **WHEN** 上报华为账单，且账单的 region="cn-north-4"
- **THEN** obs_huawei_bills 中该记录的 CityId=100010

#### Scenario: 未找到映射时国内站兜底
- **GIVEN** account_bill_region_city_rel 中不存在 vendor="aws"、region="ap-east-1" 的记录，且该主账号为国内站
- **WHEN** 上报 AWS 账单，且账单的 product_region="ap-east-1"
- **THEN** obs_aws_bills 中该记录的 CityId=300001，并记录 Warn 日志（含地域、rid）

#### Scenario: 未找到映射时国际站兜底
- **GIVEN** account_bill_region_city_rel 中不存在对应映射，且该主账号为国际站
- **WHEN** 上报账单
- **THEN** 该记录的 CityId=300002，并记录 Warn 日志（含地域、rid）

#### Scenario: 批量上报时一次性加载映射
- **GIVEN** 单次 batch 包含多条账单记录
- **WHEN** 执行 OBS 上报
- **THEN** 映射数据在 batch 开始前加载一次，整个 batch 内复用，不逐条查询数据库
