# Capability: resource-search-image-tool

## Purpose

在 `hcm-resource-search` skill 中登记业务维度镜像查询 reference，并约定 MCP 工具另轨交付与能力介绍一致性验收口径。

## Requirements

### Requirement: skill reference 与索引登记

系统 SHALL 在 `hcm-resource-search` skill 的 references 目录新增 `list_biz_image.md` 说明文档，并在 `SKILL.md` 的「references 索引」的「计算」分类下登记该 reference。文档 MUST 说明对接 cloud-server 业务维度镜像查询接口，以及 path 入参（`bk_biz_id`/`vendor`）与 body 类型化入参（`platform`/`name`/`type`/`region`/`page`；`type` 为 `public|private|shared`）。

#### Scenario: reference 文档可被检索

- **WHEN** 助手依据 SKILL.md 索引查找镜像查询能力
- **THEN** 助手 SHALL 能定位到 `list_biz_image.md` 并据此调用镜像查询工具

### Requirement: MCP 工具另轨交付

`hcm-res-manager` MCP 中业务视角只读镜像列表查询工具的注册与接入 MUST 对接 cloud-server 业务维度镜像查询接口，但其实现与注册**不在本仓库本变更的完成标准内**，由负责人另轨交付。工具入参 MUST 含 `bk_biz_id`，并支持 `vendor`、`platform`、`name`、`type`、`region` 过滤维度；MUST 为只读。

#### Scenario: MCP 就绪后助手可查询镜像

- **WHEN** MCP 工具已另轨注册且资源查询助手调用该工具并传入业务与过滤条件
- **THEN** 工具 SHALL 返回符合业务可见性规则（公共/共享 + 本业务私有）与过滤条件的镜像列表

#### Scenario: 只读约束

- **WHEN** 通过该工具发起任何请求
- **THEN** 工具 MUST NOT 执行镜像的创建、删除或变更操作

### Requirement: 能力介绍与实际能力一致

系统 SHALL 保证系统提示词中「镜像」职责描述与实际可用工具一致。系统提示词已列「镜像」，MUST NOT 需要改动；在 MCP 另轨就绪后，变更 MUST 验证「列举可查询资源」与「实际查询镜像」两个场景回复一致，不再出现「声称支持、实际查不了」的不一致。

#### Scenario: 列举能力与实际查询一致

- **WHEN** 用户先询问「可以查询哪些资源」，再请求查询「可用镜像列表」（且 MCP 工具已就绪）
- **THEN** 助手 SHALL 在两个场景中均体现镜像可查询，并能真实返回镜像结果
