## Why

HCM「资源查询助手」在能力介绍中声称支持「镜像 / 可用镜像列表」查询（来源于系统提示词 `resource_query_system_prompt.md` 的职责范围），但实际的 `hcm-resource-search` skill 并无镜像查询工具，用户真正查询时只能得到「不支持」的回复。这一「声称支持、实际查不了」的不一致会误导用户、损害助手能力清单的可信度。本变更通过**补齐镜像查询能力**消除该不一致。

## What Changes

- 在 **cloud-server 新增**业务维度镜像查询接口（`/api/v1/cloud` 前缀），底层复用 data-service `ListImage`：
  - 业务维度：path 含 `bk_biz_id`/`vendor`，返回全部公共镜像、共享镜像 + 当前业务的私有镜像。
  - 支持过滤维度：`platform`、`name`、`type`、`region`（**类型化入参** + `page`，不透传原始 filter）。
  - `enable_cvm = true` 过滤仅对自研云（TCloudZiyan）生效，其他 vendor 不附加。
  - 本地镜像 `type` 统一为 `public` / `private` / `shared`（同步入库归一化；`shared` 独立且全业务可见，不算 public）。
- 在 `hcm-resource-search` skill 的 references 中新增 `list_biz_image.md`，并在 SKILL.md「references 索引」的「计算」分类下登记。
- `hcm-res-manager` MCP 工具注册**不在本仓库本变更范围内**，由负责人另行处理；本变更仅提供 cloud-server 接口与 skill reference。
- 校验系统提示词「镜像」职责描述与实际能力一致（提示词已列「镜像」，无需改动，仅验证两个场景回复一致）。
- 存量 type 刷数另记 task；同步 filter 不改。

## Capabilities

### New Capabilities

- `biz-image-list-query`: cloud-server 业务维度镜像列表查询接口，支持全部云厂商、类型化多维度过滤、type 统一为 public/private/shared（shared 独立全可见）、私有镜像按业务过滤、enable_cvm 仅自研云生效。
- `resource-search-image-tool`: 在 hcm-resource-search skill 中登记业务维度镜像查询 reference（`list_biz_image.md` + 索引），对接 cloud-server 新接口；MCP 侧工具注册另轨交付。

### Modified Capabilities

<!-- 无：系统提示词已在职责范围中列出「镜像」，本变更不修改任何已有 spec 的需求，仅补齐实现使其名副其实。 -->

## Impact

- **新增接口**：cloud-server 新增 `/api/v1/cloud` 前缀的业务维度镜像查询接口 + 对应接口文档（遵循 api-principle）。
- **复用**：data-service `ListImage`（镜像表 schema 跨厂商统一，含 vendor/type/bk_biz_id/platform/region）。
- **AI 平台（本仓）**：`hcm-resource-search` skill references 与索引；MCP 工具另仓/另轨处理。
- **数据语义约束**：多厂商镜像均可查询；私有口径主要对 tcloud/自研云有效；本地 type 统一 `public`/`private`/`shared`；shared 全业务可见但独立于 public。
- **写路径影响**：tcloud adaptor 入库归一化；woa/data-service 私有判断兼容新 type。
- **不涉及前端**：本变更为后端接口 + skill reference，无任何前端页面/组件改动。
- **只读查询接口**：业务列表接口只读；同步入库仍按既有同步链路写 type。
