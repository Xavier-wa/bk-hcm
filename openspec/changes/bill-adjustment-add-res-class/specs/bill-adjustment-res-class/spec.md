## ADDED Requirements

### Requirement: 账单调整明细支持资源类型字段
系统 SHALL 在账单调整明细（`account_bill_adjustment_item`）中记录资源类型（`res_class`）字段，枚举值为 `cpu` 或 `gpu`，用于区分调账费用所属的资源类型。

#### Scenario: 创建调账时必须提供 res_class
- **WHEN** 调用创建账单调整接口，未提供 `res_class` 字段
- **THEN** 系统返回参数校验失败（InvalidParameter），拒绝创建

#### Scenario: 创建调账时提供合法 res_class
- **WHEN** 调用创建账单调整接口，提供 `res_class = "cpu"` 或 `res_class = "gpu"`
- **THEN** 系统成功创建调账明细，`res_class` 字段正确存储

#### Scenario: 创建调账时提供非法 res_class
- **WHEN** 调用创建账单调整接口，提供 `res_class = "other"`（非枚举值）
- **THEN** 系统返回参数校验失败，拒绝创建

#### Scenario: 更新调账明细时可选填 res_class
- **WHEN** 调用更新账单调整接口，可携带或不携带 `res_class` 字段
- **THEN** 携带时更新该字段，不携带时保持原值不变

### Requirement: 存量数据 res_class 为空时默认视为 CPU
系统 SHALL 在 OBS 同步处理时，对 `res_class` 为空的历史数据，默认将其资源类型视为 CPU，保持向后兼容。

#### Scenario: 存量空值数据同步 OBS
- **WHEN** OBS 同步任务处理 `res_class` 为空字符串的调账明细（AWS / 华为 / GCP）
- **THEN** 将 `ResClassId` 设置为对应厂商的 CPU 资源分类 ID（使用 `GetOBSResClassID(vendor, false)`）

### Requirement: OBS 同步时根据 res_class 正确设置 ResClassId
系统 SHALL 在将调账明细同步至 OBS 时，根据 `res_class` 字段的值，为 AWS、华为云、GCP 三个厂商正确设置 `ResClassId`。

#### Scenario: GPU 调账同步 AWS OBS
- **WHEN** OBS 同步任务处理 `res_class = "gpu"` 的 AWS 调账明细
- **THEN** OBS 条目的 `ResClassId` 被设置为 `OBSResClassIDAwsGPU`（6311）

#### Scenario: CPU 调账同步 AWS OBS
- **WHEN** OBS 同步任务处理 `res_class = "cpu"` 的 AWS 调账明细
- **THEN** OBS 条目的 `ResClassId` 被设置为 `OBSResClassIDAwsCPU`（451）

#### Scenario: GPU 调账同步华为 OBS
- **WHEN** OBS 同步任务处理 `res_class = "gpu"` 的华为云调账明细
- **THEN** OBS 条目的 `ResClassId` 被设置为 `OBSResClassIDHuaweiGPU`（6315）

#### Scenario: GPU 调账同步 GCP OBS
- **WHEN** OBS 同步任务处理 `res_class = "gpu"` 的 GCP 调账明细
- **THEN** OBS 条目的 `ResClassId` 被设置为 `OBSResClassIDGcpGPU`（6312）

#### Scenario: Zenlayer 调账不设置 ResClassId
- **WHEN** OBS 同步任务处理任意 `res_class` 值的 Zenlayer 调账明细
- **THEN** Zenlayer OBS 条目不设置 `ResClassId`（表结构不含该字段，行为不变）

### Requirement: OBS 同步时根据账号站点正确设置 CityId
系统 SHALL 在将调账明细同步至 OBS 时，根据一级账号的 `site` 字段，为 AWS、华为云、GCP 三个厂商正确设置 `CityId`。

#### Scenario: 国内站账号同步 OBS 设置 CityId
- **WHEN** OBS 同步任务处理 `site = china`（国内站）账号的调账明细
- **THEN** OBS 条目的 `CityId` 被设置为 `OBSDefaultCityIDChina`

#### Scenario: 国际站账号同步 OBS 设置 CityId
- **WHEN** OBS 同步任务处理国际站账号的调账明细
- **THEN** OBS 条目的 `CityId` 被设置为 `OBSDefaultCityIDOverseas`

### Requirement: Excel 导出展示资源类型列
系统 SHALL 在账单调整明细的 Excel 导出文件中，包含"资源类型"列，展示 `res_class` 的中文名称（"CPU" 或 "GPU"）。

#### Scenario: 导出含 res_class 的调账明细
- **WHEN** 用户导出账单调整明细 Excel
- **THEN** 导出文件包含"资源类型"列，值为"CPU"或"GPU"

#### Scenario: 导出 res_class 为空的历史数据
- **WHEN** 用户导出包含存量空值调账的 Excel
- **THEN** 对应行"资源类型"列显示空字符串（不报错）
