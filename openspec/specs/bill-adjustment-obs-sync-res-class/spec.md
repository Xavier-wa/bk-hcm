# bill-adjustment-obs-sync-res-class

## Purpose

调账明细同步到 OBS 账单明细时的资源分类映射能力：按资源类别四值与云厂商映射 OBS 资源分类 ID，区分 GPU 卡类与 API 类；并把单列 `res_sub_class` 按类别分发写入 OBS 已有的卡型列与 API 厂商列，避免调账数据同步过去后卡型信息丢失。

## Requirements

### Requirement: 调账同步 OBS 时按四类映射资源分类

调账明细同步到 OBS 账单明细表时，写入的资源分类 ID MUST 按资源类别与云厂商映射，且 MUST 区分 API 类与 GPU 卡类。支持的云厂商为 AWS、GCP、华为云。

映射关系：

| 资源类别 | AWS | GCP | 华为云 |
|---|---|---|---|
| `cpu` | 451 | 601 | 1244 |
| `gpu_card` | 6311 | 6312 | 6315 |
| `gpu_other` | 6311 | 6312 | 6315 |
| `gpu_api` | 6799 | 6800 | 6315 |

实现 MUST 复用既有的按类型取资源分类 ID 的能力，由资源类别推出「是否 GPU」与「是否 API」两个入参，MUST NOT 沿用仅按「是否 GPU」二分的旧调用方式。

华为云无 API 资源分类，`gpu_api` 在华为云下 MUST 回落到 GPU 分类 6315。该分支在本期实际不可达——华为云无法创建 `gpu_api` 记录——保留为防御性逻辑。

云厂商不在 AWS、GCP、华为云范围内时，资源分类 ID 返回 0，与既有行为一致，本变更 MUST NOT 改变该行为。

#### Scenario: AWS 的 GPU API 调账落到 API 分类

- **GIVEN** AWS 二级账号下存在一条 `res_class=gpu_api` 的调账明细
- **WHEN** 执行 OBS 同步
- **THEN** OBS 账单明细的资源分类 ID 为 6799

#### Scenario: GCP 的 GPU 其他调账落到 GPU 分类

- **GIVEN** GCP 二级账号下存在一条 `res_class=gpu_other` 的调账明细
- **WHEN** 执行 OBS 同步
- **THEN** OBS 账单明细的资源分类 ID 为 6312

#### Scenario: 华为云的 GPU API 调账回落到 GPU 分类

- **GIVEN** 华为云二级账号下存在一条 `res_class=gpu_api` 的调账明细
- **WHEN** 执行 OBS 同步
- **THEN** OBS 账单明细的资源分类 ID 回落为 6315

### Requirement: 资源子类分发到 OBS 的卡型与 API 厂商两列

OBS 账单明细表（AWS、GCP、华为云三张）已有卡型列与 API 厂商列。调账同步时 MUST 把单列 `res_sub_class` 按资源类别拆分写入这两列：

| 资源类别 | OBS 卡型列 | OBS API 厂商列 |
|---|---|---|
| `cpu` | 空 | 空 |
| `gpu_card` | `res_sub_class` | 空 |
| `gpu_api` | 空 | `res_sub_class` |
| `gpu_other` | 空 | 空 |

本要求 MUST NOT 改动 OBS 表结构，只补调账同步时的写入逻辑。此前调账同步未设置这两列，导致调账数据同步过去后卡型信息丢失。

#### Scenario: GPU 卡调账写入卡型列

- **GIVEN** 一条 `res_class=gpu_card`、`res_sub_class=H200` 的调账明细
- **WHEN** 执行 OBS 同步
- **THEN** OBS 记录的卡型列为 `H200`，API 厂商列为空

#### Scenario: GPU API 调账写入 API 厂商列

- **GIVEN** 一条 `res_class=gpu_api`、`res_sub_class=gemini` 的调账明细
- **WHEN** 执行 OBS 同步
- **THEN** OBS 记录的 API 厂商列为 `gemini`，卡型列为空

#### Scenario: 非 GPU 细分类别两列均为空

- **GIVEN** 一条 `res_class=gpu_other` 或 `res_class=cpu` 的调账明细
- **WHEN** 执行 OBS 同步
- **THEN** OBS 记录的卡型列与 API 厂商列均为空字符串
