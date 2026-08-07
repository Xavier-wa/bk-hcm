## ADDED Requirements

### Requirement: AI 扣减条目 OBS 归类与原始账单同口径

系统在将 AWS/GCP 的 AI 扣减账单项（`HcProductCode` 或 `HcProductName` 为 `AIDeduct`）同步写入 OBS 时，SHALL 使用与原始账单 OBS 同步相同的资源分类优先级（API > GPU > CPU）计算 `ResClassId`，并按可识别结果填充 `APIBrandName` 与 `GpuCardCategory`。系统 MUST NOT 仅因产品标识为 `AIDeduct` 而将条目一律归为 API-OFS。

#### Scenario: AWS API 类 AI 扣减归入 API-OFS 并填充品牌

- **WHEN** AWS 分账条目产品标识为 `AIDeduct`，且 extension 中产品名称可识别出 API 厂商品牌（如 claude）
- **THEN** OBS AWS 账单该条目的 `ResClassId` 为 AWS API 分类，`APIBrandName` 为识别到的品牌，`cost` 仍为冲销金额

#### Scenario: AWS GPU 卡类型 AI 扣减填充卡型且不强制 API

- **WHEN** AWS 分账条目产品标识为 `AIDeduct`，无法识别 API 品牌，但按与原始账单一致的 GPU 判定为 GPU，且可解析出 GPU 卡型
- **THEN** OBS AWS 账单该条目的 `ResClassId` 为 AWS GPU 分类，`GpuCardCategory` 为解析出的卡型，且 MUST NOT 被写成 API-OFS

#### Scenario: GCP AI 扣减与原始同口径

- **WHEN** GCP 分账条目产品标识为 `AIDeduct`，完成该账期 OBS 同步
- **THEN** 该条目的 `ResClassId`、`APIBrandName`、`GpuCardCategory` 按与 GCP 原始账单相同的判定规则填充（API > GPU > CPU）

#### Scenario: 识别失败时降级且不阻断同步

- **WHEN** AI 扣减条目无法从明细识别品牌与卡型
- **THEN** 系统按与原始账单相同的降级路径设置 `ResClassId`（不强制 API），记录告警日志，且 MUST NOT 因该条目标识失败而中断整批 OBS 同步

#### Scenario: 成本冲销口径不变

- **WHEN** AI 扣减条目完成 OBS 同步
- **THEN** 写入 OBS 的成本金额仍为分账中的冲销金额（通常为负），不因归类修正而改变金额符号或绝对值
