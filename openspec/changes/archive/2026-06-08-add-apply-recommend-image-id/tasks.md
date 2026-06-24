## 1. 表结构与字段扩展
- [x] 1.1 新建 `scripts/sql/9999_*_apply_recommend_image_id.sql`：三表加 `image_id` 列；两张推荐表唯一键改为含 `image_id`（SQLVER=9999, HCMVER=v9.9.9）
- [x] 1.2 `pkg/dal/table/cvm-apply/ziyan_cvm_device_info.go`：ColumnDescriptor + struct 加 `ImageID`
- [x] 1.3 `ziyan_cvm_apply_user_recommend.go` / `ziyan_cvm_apply_biz_recommend.go`：同上
- [x] 1.4 `pkg/api/data-service/cvm-apply/*`：三表 Create/Update Req 加 `ImageID`
- [x] 1.5 `cmd/data-service/service/cvm-apply/*`：三表 BatchCreate/Update 映射 `ImageID`

## 2. 增量写入透传
- [x] 2.1 `cmd/woa-server/types/task/scheduler.go` `DeviceInfo` 加 `ImageID` 字段
- [x] 2.2 `cmd/woa-server/logics/task/scheduler/generator/generator.go` `buildSingleDeviceInfo`：`ImageID: order.Spec.ImageId`
- [x] 2.3 `cmd/woa-server/model/task/device_info.go` `CreateDeviceInfos` + `convertMySQLToDeviceInfo`：映射 `ImageID`

## 3. 离线聚合带 image_id
- [x] 3.1 `cmd/woa-server/logics/applyrecommend/aggregator.go`：`userCountKey` 加 image_id（六元组）、`bizCountKey` 加 image_id（五元组），写入 CreateReq
- [x] 3.2 `cmd/woa-server/logics/applyrecommend/logics.go` `collectCounts`：构造 key 时带 `item.ImageID`

## 4. 在线接口改造
- [x] 4.1 `pkg/api/woa-server/cvm_apply_recommend.go` `ApplyRecommendTopElem` 加 `ImageID`
- [x] 4.2 `cmd/woa-server/service/task/recommend.go`：去重键改四元组 `buildRecommendKey`；响应组装带 `image_id`
- [x] 4.3 接口文档 `docs/api-docs/.../get_apply_recommend_top.md` 补 `image_id`（提供版本 v9.9.9+）

## 5. 验证
- [x] 5.1 `openspec validate add-apply-recommend-image-id --strict` 通过 + 受影响包 `go build` 通过
- [ ] 5.2 部署自测（需 DB 环境）：迁移后三表有 image_id（AC-101）；cron 后推荐表四元组去重正确（AC-102）
