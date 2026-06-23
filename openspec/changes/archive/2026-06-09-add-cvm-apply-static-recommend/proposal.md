## Why

离线推荐仅产出 `(require_type, region, device_type, image_id, count)` 历史三元组，不含库存校验，也不补全可用区/计费模式/磁盘等下单必需字段，无法直接转化为申领单据。本变更在离线历史偏好之上叠加实时库存（静态表 `device_capacity`）校验与默认值补全，把历史偏好补全为「可直接下单的单子单完整方案」，供主机申领 AI Agent 消费。

## What Changes

- 新增业务视角接口 `POST /bizs/{bk_biz_id}/task/apply/recommend/by_static_recommend`：基于静态推荐表 + 库存校验 + 默认值补全，返回若干推荐方案，每方案含 1 个子单。
- 复用并扩展现有 `listUserRecommend` / `listBizRecommend`：增加可选 A 类过滤（require_type/region/device_type/image_id），保持 `GetBizApplyRecommendTop` 行为不变。
- 库存校验走静态表 `device_capacity`（`DeviceCapacity.List`），并按 `require_type.NotNeedVerifyCapacity()` 跳过免校验需求类型（小额绿通）。
- 新增配置项 `cc.ApplyRecommend` 默认申请数量（默认 10），同步 `etc/woa_server.yaml` 与 helm。
- 不含：image_id 字段扩展与回填（已就绪）、离线统计改造、MCP 工具化、修改既有 `GetApplyRecommendTop`。

## Capabilities

### New Capabilities
<!-- 无新增独立 capability，复用现有推荐域 -->

### Modified Capabilities
- `apply-recommend`: 新增「离线偏好叠加库存的在线单子单推荐查询接口」相关需求（在现有离线推荐 + Top-N 查询之上叠加在线库存校验与方案组装）。

## Impact

- 受影响 specs：`apply-recommend`
- 受影响代码：
  - `pkg/api/woa-server/cvm_apply_recommend.go`（新增 Req/Resp/方案 Elem/子单 Elem）
  - `cmd/woa-server/service/task/recommend.go`（新增 Handler + Logics，扩展推荐查询 helper 的可选过滤）
  - `cmd/woa-server/service/task/service.go`（`bizService` 注册路由）
  - `pkg/cc/`（新增 `ApplyRecommend` 默认申请数量配置）
  - `pkg/criteria/constant/ziyan.go`（新增磁盘默认值常量 `RecommendSystemDiskSize/RecommendDataDiskSize/RecommendDiskNum`）
  - `etc/woa_server.yaml` + `docs/support-file/helm`（配置同步）
  - `docs/api-docs/api-server/docs/zh/get_biz_apply_recommend_by_static.md`（接口文档）
- 复用：`s.configLogics` 不涉及；库存走 `s.client.DataService().Global.DeviceCapacity.List`；推荐表走 `DataService().TCloudZiyan.ZiyanCvmApplyUserRecommend/BizRecommend.List`，无需新建 data-service 接口。
