# Capability: resource-query-res-plan-prompt

## Purpose

轻量更新资源查询系统提示词，将资源预测与 GPU 预测纳入可查询职责范围，并保持写操作边界不变。

## Requirements

### Requirement: 提示词轻量纳入资源预测与 GPU 预测

系统 SHALL 更新 `cmd/agent-server/etc/prompts/resource_query_system_prompt.md` 的职责范围，明确**资源预测**与 **GPU 预测**属于可查询范围。更新 MUST 保持轻量：MUST NOT 在提示词中枚举单据/子单/过滤字段/接口路径等细节（细节以 `hcm-resource-search` skill reference 为准）。

#### Scenario: 职责范围包含预测与 GPU

- **WHEN** 阅读更新后的资源查询系统提示词职责范围
- **THEN** 文本 SHALL 体现资源预测与 GPU 预测可查询

#### Scenario: 提示词不过度细化

- **WHEN** 审查提示词变更内容
- **THEN** 变更 MUST NOT 引入完整接口参数表或与 skill reference 重复的字段级说明

### Requirement: 查询场景与写操作边界不变

系统提示词更新后 MUST 继续遵守既有行为约束：资源查询场景仅处理查询；MUST NOT 引导助手执行报预测、创建/调整预测、GPU 提报写操作。写操作意图仍不属于本场景交付范围。

#### Scenario: 仍拒绝写操作类请求

- **WHEN** 用户在资源查询场景请求「帮我提交资源预测」或「新增 GPU 需求」
- **THEN** 助手 SHALL 按既有约束说明当前场景仅处理查询（或不支持该写操作），MUST NOT 通过本 skill 发起写接口调用
