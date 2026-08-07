## ADDED Requirements

### Requirement: 按云厂商查询卡型枚举

系统 MUST 提供一个查询接口，按云厂商返回该厂商可选的卡型清单，供前端渲染下拉选项。云厂商 MUST 作为路径参数传入。支持的云厂商为 AWS、GCP、华为云。

各厂商的卡型来源 MUST 以现有代码实现为准，MUST NOT 另建一套清单：

- AWS：`global_config` 中 `aws_gpu_instance_types` 配置项的 value 集合
- GCP：`pkg/criteria/enumor` 中的一级卡型关键字表，并上 `global_config` 中 `gcp_gpu_instance_prefixes` 配置项的 value 集合
- 华为云：无来源，清单为空

返回结果 MUST 去重。返回顺序 MUST 稳定，以避免下拉选项在多次请求间跳动。

运营在 `global_config` 中新增卡型后 MUST 无需重启服务即可生效。

#### Scenario: 查询 GCP 卡型

- **GIVEN** `gcp_gpu_instance_prefixes` 已录入配置
- **WHEN** 以云厂商 GCP 调用卡型枚举查询接口
- **THEN** 返回一级卡型关键字表中的全部卡型并上该配置的全部 value，结果已去重

#### Scenario: 查询 AWS 卡型

- **GIVEN** `aws_gpu_instance_types` 已录入配置
- **WHEN** 以云厂商 AWS 调用卡型枚举查询接口
- **THEN** 返回该配置的全部 value，且不含仅 GCP 侧存在的卡型

#### Scenario: 查询华为云卡型

- **GIVEN** 任意配置状态
- **WHEN** 以云厂商华为云调用卡型枚举查询接口
- **THEN** 返回空列表且不报错

#### Scenario: 运营新增卡型即时生效

- **GIVEN** 运营在 `gcp_gpu_instance_prefixes` 中新增一个卡型 value
- **WHEN** 不重启服务，再次以云厂商 GCP 调用卡型枚举查询接口
- **THEN** 新卡型出现在返回结果中

#### Scenario: 传入不支持的云厂商

- **GIVEN** 路径参数中的云厂商不在支持范围内
- **WHEN** 调用卡型枚举查询接口
- **THEN** 返回参数非法错误

### Requirement: 卡型来源配置缺失时降级不阻断

`global_config` 中的卡型配置缺失时，卡型枚举查询接口 MUST 降级返回而非报错：AWS 返回空列表，GCP 只返回一级卡型关键字表部分。

配置值解析失败时 MUST 记录警告日志并忽略该来源，MUST NOT 阻断接口。

该行为与 OBS 账单上报侧「配置缺失视为空映射、不阻断」的既有处理保持一致。

#### Scenario: 两处 GPU 配置均缺失

- **GIVEN** `global_config` 中 `aws_gpu_instance_types` 与 `gcp_gpu_instance_prefixes` 均不存在
- **WHEN** 分别以云厂商 GCP 与 AWS 调用卡型枚举查询接口
- **THEN** GCP 返回一级卡型关键字表中的全部卡型，AWS 返回空列表，两者均不报错

#### Scenario: 配置值解析失败

- **GIVEN** `global_config` 中某项 GPU 配置的值无法解析
- **WHEN** 调用卡型枚举查询接口
- **THEN** 记录警告日志、忽略该来源并返回其余来源的卡型，接口不报错

### Requirement: 按云厂商查询模型厂商枚举

系统 MUST 提供一个查询接口，按云厂商返回该厂商可选的模型厂商清单，供前端渲染下拉选项。云厂商 MUST 作为路径参数传入。

各厂商的清单：AWS 与 GCP 返回 `gemini`、`claude`、`kimi`、`jina` 四值；华为云返回空列表，因为华为云无 OBS API 资源分类、账单上报侧的 API 厂商本期统一留空。

四值取自账单上报侧 API 厂商识别的归并后结果。`veo`、`imagen`、`lyria` 在上报侧已归并为 `gemini`，本接口 MUST NOT 返回这三者——若出现在下拉中会产生「选了 veo 但核算口径落在 gemini」的口径分裂。

#### Scenario: 查询 AWS 或 GCP 的模型厂商

- **GIVEN** 任意时刻
- **WHEN** 以云厂商 AWS 或 GCP 调用模型厂商枚举查询接口
- **THEN** 返回恰好 `claude`、`gemini`、`jina`、`kimi` 四值，不含 `veo`、`imagen`、`lyria`

#### Scenario: 查询华为云的模型厂商

- **GIVEN** 任意时刻
- **WHEN** 以云厂商华为云调用模型厂商枚举查询接口
- **THEN** 返回空列表

#### Scenario: 传入不支持的云厂商

- **GIVEN** 路径参数中的云厂商不在支持范围内
- **WHEN** 调用模型厂商枚举查询接口
- **THEN** 返回参数非法错误

### Requirement: 下拉取值与写入校验同源

两个枚举查询接口返回的清单，与创建、更新接口校验 `res_sub_class` 取值域所用的清单，MUST 出自同一份「云厂商 → 清单」的实现。MUST NOT 各自实现一遍。

该约束的目的是排除「下拉能选、提交被拒」这类不一致。

#### Scenario: 下拉可选值必然通过写入校验

- **GIVEN** 某云厂商卡型枚举查询接口的返回结果
- **WHEN** 取其中任一值作为 `res_sub_class`，配 `res_class=gpu_card` 与同一云厂商调用创建接口
- **THEN** 创建成功

#### Scenario: 模型厂商下拉可选值必然通过写入校验

- **GIVEN** 某云厂商模型厂商枚举查询接口的返回结果
- **WHEN** 取其中任一值作为 `res_sub_class`，配 `res_class=gpu_api` 与同一云厂商调用创建接口
- **THEN** 创建成功
