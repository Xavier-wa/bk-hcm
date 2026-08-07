## Context

HCM「资源查询助手」在系统提示词 `resource_query_system_prompt.md` 的职责范围中声称支持「镜像 / 可用镜像列表」查询，但 `hcm-resource-search` skill 与 `hcm-res-manager` MCP 均无对应工具，用户实际查询只能得到「不支持」的回复。本变更补齐该能力，消除「声称支持、实际查不了」的不一致。

现网相关实现现状：

- **cloud-server 已有独立 image 服务**（`cmd/cloud-server/service/image/`，包 `image`，含 `imageSvc`），路由前缀 `/api/v1/cloud`，已注册：
  - `POST /images/list`（`ListImage`）：跨厂商泛化查询，底层 `data-service Global.ListImage`，不含 `bk_biz_id`、私有镜像业务过滤、`enable_cvm`、type 归一化。
  - `POST /vendors/{vendor}/images/list`（`ListImageExt`）：单厂商查询。
- **woa-server 已有业务维度实现** `GetBizCvmImage`（`cmd/woa-server/logics/config/cvm_image.go`），但**硬编码 `vendor = TCloudZiyan` 且强制 `extension.enable_cvm = true`**，仅服务自研云 CVM 重装场景，无法直接复用为「全厂商业务维度镜像查询」。
- **镜像 `type` 现状不统一**：`enumor.TCloudImageType` 为大写枚举（`PUBLIC_IMAGE`/`PRIVATE_IMAGE`/`SHARED_IMAGE`），tcloud 与自研云入库为大写；aws/azure/gcp/huawei 入库为小写 `public` 且同步侧主要为公共镜像。
- **数据现状**：各云厂商镜像均已入库；私有镜像业务归属语义主要存在于 tcloud/自研云。

约束：非破坏性（不改动现有 woa `GetBizCvmImage` 与 cloud-server 既有两个镜像接口）；只读；本期不新增/改造各厂商镜像同步链路；本仓库不包含 MCP 工具注册；不涉及任何前端改动。

## Goals / Non-Goals

**Goals:**

- 在 cloud-server 新增**业务维度镜像列表查询接口**（`/api/v1/cloud` 前缀），入参含 `bk_biz_id`，返回全部公共镜像（含共享镜像）+ 当前业务的私有镜像。
- 支持过滤维度：`vendor`、`platform`、`name`、`type`、`region`（类型化入参）。
- 对 `type` 统一为 `public` / `private` / `shared`；`shared` 独立且全业务可见，不算 public。
- `enable_cvm = true` 过滤**仅对自研云（TCloudZiyan）生效**，其他 vendor 不附加该条件（其他厂商有数据，绝不能被 enable_cvm 误伤）。
- 在 `hcm-resource-search` skill 中新增 reference 与索引登记，使助手侧具备对接文档。

**Non-Goals:**

- 不改动 woa `GetBizCvmImage` 及 cloud-server 既有 `ListImage` / `ListImageExt`。
- 不改造/新增各厂商镜像数据同步链路。
- 不做镜像的创建 / 删除 / 变更（写操作）。
- 不做任何前端页面 / 组件改动。
- **不在本仓库实现 `hcm-res-manager` MCP 工具注册**（另轨交付）。

## Decisions

### D1. 在 cloud-server 现有 image 包新增独立 handler，而非改造 woa

**选择**：在 `cmd/cloud-server/service/image/` 新增业务维度 handler（如 `biz.go`），复用现有 `imageSvc` 结构体（已持有 `client` 与 `authorizer`），在 `init.go` 注册路由 `POST /bizs/{bk_biz_id}/images/list`（完整路径 `/api/v1/cloud/bizs/{bk_biz_id}/images/list`）。

**理由**：

- woa `GetBizCvmImage` 与自研云 CVM 重装场景强耦合（硬编码 ziyan + 强制 enable_cvm），改造它会破坏既有语义，风险高。
- cloud-server image 服务已是镜像查询的自然归属地，且已有跨厂商 `Global.ListImage` 依赖，复用成本最低。

**备选**：① 直接改造 woa `GetBizCvmImage` — 破坏既有语义，被否；② 复用 cloud-server 泛化 `ListImage` — 缺少业务私有过滤 / type 归一化 / enable_cvm 条件，语义不达标，被否。

### D2. 底层复用 data-service `Global.ListImage` 单次跨厂商查询

**选择**：底层调用 `client.DataService().Global.ListImage`，一次查询覆盖全部厂商，而非按 vendor 循环调用各厂商 `ListImage` 再合并。

**理由**：全厂商维度天然契合 Global 跨厂商查询，避免多次 RPC 与结果合并、分页拼接的复杂度。

### D3. 采用「类型化请求」而非透传原始 filter 表达式

**选择**：定义显式请求结构体（字段 `Vendor` / `Platform` / `Name` / `Type` / `Region` / `Page`），由服务端组装最终 filter，**不**接受调用方透传 `core.ListReq.Filter`。

**理由**：业务私有镜像注入、type 归一化、enable_cvm 条件化都需要服务端掌控 filter 组装；透传原始 filter 无法保证这些不变量，且易被绕过（如查到其他业务私有镜像）。类型化入参更利于接口文档与后续 MCP 工具 schema 描述。

**过滤规则组装约定**：`vendor`/`platform`/`region` 用精确匹配（`RuleEqual`/`RuleIn`），`name` 用模糊匹配（`RuleLike`/`Contains`）以贴合助手自然语言查询，`type` 经归一化后转换为各厂商底层枚举再匹配。

### D4. `enable_cvm` 条件化：OR 复合表达式

**选择**：由于单次 Global 查询跨厂商，无法用单一全局 `extension.enable_cvm=true` 规则（会误伤无该扩展字段、但已有入库数据的其他厂商）。采用 OR 复合：

```
(vendor == tcloud_ziyan AND extension.enable_cvm == true) OR (vendor != tcloud_ziyan)
```

即：自研云镜像必须 `enable_cvm=true`，其他厂商不附加此条件。

**备选**：按 vendor 拆多次查询分别加条件再合并 — 复杂度高，被否。

### D5. 业务可见性：public + shared 全见，private 按业务

**选择**：`public` 与 `shared` 对所有业务可见；`private` 仅当 `bk_biz_id` 匹配当前业务时返回。`shared` **独立类型，不再并入 public**。以 filter 表达式表达：

```
type == public OR type == shared OR (type == private AND bk_biz_id == {bk_biz_id})
```

私有镜像归属语义实际主要对 tcloud/自研云生效。

### D6. type 统一落库与归一化

**选择**：本地存储与查询均只使用 `public` / `private` / `shared`，不做大写枚举兼容，减少冗余逻辑。同步入库时在 adaptor 将云上枚举归一化后写入。

- `PUBLIC_IMAGE` → `public`
- `PRIVATE_IMAGE` → `private`
- `SHARED_IMAGE` → `shared`（独立，不算 public）
- 同步 filter **不改**（仍不主动同步 `SHARED_IMAGE`）
- 存量刷数另记 task，**上线前需完成**

### D7. 权限：业务访问鉴权

**选择**：业务维度接口需校验调用方对 `bk_biz_id` 的访问权限（复用 `imageSvc.authorizer` 与既有业务鉴权模式），保证不越权查看他业务私有镜像。

### D8. 本仓库交付 skill reference；MCP 另轨

**选择**：本仓库本变更仅在 `hcm-resource-search` skill 的 references 新增 `list_biz_image.md`，并在 `SKILL.md` 索引「计算」分类下登记。`hcm-res-manager` MCP 工具注册由负责人在仓库外另轨完成，不纳入本 change tasks。系统提示词已列「镜像」，仅需在 MCP 就绪后验证两个场景回复一致。

## Risks / Trade-offs

- [enable_cvm 条件写错导致其他厂商结果被滤空] → 单测覆盖「非自研云不受 enable_cvm 影响」。
- [type 归一化遗漏] → 写路径统一走 `enumor.NormalizeImageType`；查询只认统一值。
- [写路径改 private 后旧逻辑仍比 PRIVATE_IMAGE] → woa/data-service 改为 `IsPrivateImageType`。
- [存量未刷数导致查空] → 上线前完成刷数 task 7.1。
- [越权访问他业务私有镜像] → D7 业务鉴权 + D5 filter 双重保障。
- [MCP 另轨未就绪时助手仍查不了] → skill reference 可先落地；端到端依赖 MCP 另轨联调。

## Migration Plan

- 新增接口 + 同步写路径归一化；查询只认统一 type。
- 存量刷数（`PUBLIC_IMAGE`→`public` 等）另排期但须在上线前完成。
- 回滚：摘除新增路由；写路径归一化回滚需评估已写入的新 type。

## Open Questions

- （已关闭）对外 type 命名：`public` / `private` / `shared`。
