## Context

主机申领 AI Agent 需要「可直接下单的完整子单方案」，而现有离线推荐（`apply-recommend` 能力中的 `GetBizApplyRecommendTop`）只返回历史三元组 `(require_type, region, device_type, image_id, count)`，缺库存校验与下单必需字段。本变更新增独立的在线推荐接口，在离线偏好基础上叠加静态库存校验与默认值补全。

依赖前置（已就绪，不在本变更范围）：
- `ziyan_cvm_apply_user_recommend` / `ziyan_cvm_apply_biz_recommend` 已含 `image_id` 列（已确认代码与表结构均就绪）。
- 库存静态表 `device_capacity`（require_type/region/zone/device_type/capacity）已由定时任务维护。

技术方案出处：iWiki 4021537729「接口1: 离线数据 + 库存情况推荐」。

## Goals / Non-Goals

**Goals:**
- 新增业务视角接口 `POST /bizs/{bk_biz_id}/task/apply/recommend/by_static_recommend`，返回若干单子单推荐方案。
- 复用现有推荐查询逻辑（user 优先 biz 补足去重），最大化代码复用且不改动既有接口行为。
- 库存走静态表，按需求类型决定是否校验（复用 `RequireType.NotNeedVerifyCapacity()`）。
- 新增可配置的默认申请数量。

**Non-Goals:**
- image_id 字段扩展与历史回填（已就绪）。
- 离线统计任务改造、刷历史脚本。
- MCP 工具化、Graph 编排、Skill 沉淀（父需求其他子项）。
- 修改既有 `GetApplyRecommendTop` / `GetBizApplyRecommendTop` 行为。
- 实时库存查询（明确采用静态表方案）。

## Decisions

### 决策 1：落点 service/task，与现有推荐接口同包
新接口实现放在 `cmd/woa-server/service/task/recommend.go`，与 `GetBizApplyRecommendTop` 同包同文件族，`bizService` 注册路由。
- 备选：放 `service/cvm`（技术方案接口2/3 的落点）。因现有推荐接口已在 service/task，同包可直接复用私有 helper，故选 service/task。

### 决策 2：扩展现有 helper 支持可选 A 类过滤（向后兼容）
为 `listUserRecommend` / `listBizRecommend` 增加「可选 filter rules」入参：
- `GetBizApplyRecommendTop` 传空 → SQL 与行为完全不变。
- 新接口传 A 类规则（require_type/region/device_type/image_id 中入参非空者）→ 在 SQL 层收窄候选，保证 limit 作用于过滤后的集合。
- 备选：复制一份新 helper。为避免重复逻辑、保持去重一致，选择扩展。

### 决策 3：库存走静态表 + 按需求类型分支
- 数据源：`s.client.DataService().Global.DeviceCapacity.List`，参考 `cmd/woa-server/task/device_capacity.go:getRelInfoFromCapacity` 的分页 List 模式。
- 批量策略：候选最多 N 条，按所涉 `require_type` / `region` / `device_type` 用 IN 一次批量查出，内存按 `capacity ≥ applyCount` 过滤。
- 需求类型分支：候选先判 `require_type.NotNeedVerifyCapacity()`（当前仅小额绿通），为真则跳过库存校验直接保留。
- 备选：实时库存（`configLogics.Capacity().BatchGetCapacity`）。因延迟、外部依赖与回写副作用，且偏离技术方案，明确不采用。

### 决策 4：可用区与资源分配方式联动
- 未传 zone → 返回 `zone=全部(all)`；`res_assign` 优先取 B 类入参 `req.ResAssign`，未传时默认 `ResPriorityResAssign(1)`；库存仅做候选过滤（region 下任一 zone 满足阈值即保留），不回填具体 zone。
- 传了 zone → 返回该具体 zone + `res_assign` 不返回（nil，子单字段为 `*enumor.ResAssign`）；库存按该 zone 校验，不足剔除。

### 决策 5：默认值与配置
| 字段 | 默认值 | 出处 |
|---|---|---|
| image_id | 静态推荐表 image_id（无默认） | 推荐表 |
| 申请数量 | 入参优先，未传默认 10 | 新增 `cc.ApplyRecommend` 配置 |
| 计费模式 | `PREPAID` | cvmapi |
| 系统盘 | `CLOUD_PREMIUM` / 100G / 1 块 | enumor DiskSpec |
| 数据盘 | `CLOUD_PREMIUM` / 500G / 1 块 | enumor DiskSpec |
| res_assign | 未传 zone→1；传了 zone→不返回 | enumor |

### 决策 6：候选去重与排序
- 去重 key：`require_type|region|device_type|image_id`，user 优先 biz 补足。
- 排序：按推荐表 `count` 倒序；取前 N（入参返回方案数）。
- 申请数量语义：取入参 / 默认 10，**不使用 count 作为数量**（count 仅排序）。

## Risks / Trade-offs

- [静态库存有新鲜度滞后] → 与技术方案一致，接受不做新鲜度兜底；库存查不到即视为不足剔除。
- [扩展现有 helper 可能影响 GetBizApplyRecommendTop] → 新增入参可选、默认空，保证旧调用 SQL 不变；通过对照测试验证行为不变。
- [A 类过滤传了但查不到] → 按技术方案直接返回空（不强构、不降级）。
- [新增配置项遗漏 helm/yaml 同步] → tasks 中显式列出 `etc/woa_server.yaml` 与 helm 同步步骤。
