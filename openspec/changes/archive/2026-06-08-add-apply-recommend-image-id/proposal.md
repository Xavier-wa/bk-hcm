# Change: 申领推荐扩展 image_id 维度与历史数据回填

## Why
离线推荐链路当前只产出 `(require_type, region, device_type)` 三元组，缺少镜像信息。接口1（离线偏好叠加库存在线推荐）需要把历史偏好补全为「可直接下单的完整子单方案」，必须取到历史 `image_id`。因此需在设备信息表与两张推荐表扩展 `image_id`，并补齐离线聚合逻辑与历史存量数据。

## What Changes
- **BREAKING**（数据层）：`ziyan_cvm_device_info`、`ziyan_cvm_apply_user_recommend`、`ziyan_cvm_apply_biz_recommend` 三表新增 `image_id` 字段；两张推荐表唯一键由 `...|device_type` 调整为 `...|device_type|image_id`。
- 离线聚合最小推荐单元由三元组调整为四元组（含 `image_id`）：用户维度聚合 key 五元组→六元组，业务维度聚合 key 四元组→五元组。
- 设备生成/交付写入链路透传 `image_id`（取自 `order.Spec.ImageId`），保证新交付设备自带镜像。
- Top-N 推荐查询接口响应新增 `image_id`，内存去重键由三元组改为四元组。
## Impact
- Affected specs: `apply-recommend`
- Affected code:
  - SQL: `scripts/sql/9999_*_apply_recommend_image_id.sql`
  - Table: `pkg/dal/table/cvm-apply/ziyan_cvm_device_info.go`、`ziyan_cvm_apply_user_recommend.go`、`ziyan_cvm_apply_biz_recommend.go`
  - 协议/DS: `pkg/api/data-service/cvm-apply/*`、`cmd/data-service/service/cvm-apply/*`
  - 增量写入: `cmd/woa-server/types/task/scheduler.go`、`cmd/woa-server/logics/task/scheduler/generator/generator.go`、`cmd/woa-server/model/task/device_info.go`
  - 离线聚合: `cmd/woa-server/logics/applyrecommend/aggregator.go`、`logics.go`
  - 在线接口: `cmd/woa-server/service/task/recommend.go`、`pkg/api/woa-server/cvm_apply_recommend.go`、`docs/api-docs/.../get_apply_recommend_top.md`