## Why

主机申领功能目前仅支持"滚服项目"(require_type=6)继承固资号，机房裁撤场景下用户无法继承被裁撤机器的套餐（计费模式、时长等），导致需要重新购买套餐，增加了成本。需要为"机房裁撤"(require_type=3)场景增加继承固资号能力，让用户可以享受原有机器的计费优惠。

## What Changes

- 新增裁撤主机表（recycle_host_info）的 data-service CRUD 接口及对应的 client 封装，供上层服务查询裁撤机器列表
- 改造现有的 `CheckInheritedHost` 接口，通过 `require_type` 参数区分"滚服项目"和"机房裁撤"两种校验逻辑
- 机房裁撤场景增加两项校验：固资号必须在裁撤表中、机型族不能是GPU型
- CRP 接口响应结构新增 `generationType` 字段，并在校验接口响应中返回该字段
- 创建申领单据时，require_type=3 且传入 bk_asset_id 时，计费模式以 BKCC 查到的为准，忽略接口传入的 charge_type
- 修改单据重试流程中，`ModifyData` 新增 `bk_asset_id` 和 `inherit_instance_id` 字段的存储，`auditApplyModifyCallback` 回调时将存储的 `inherit_instance_id` 赋值给 CRP 申领请求、`bk_asset_id` 更新到 cr_ApplyOrder 表

## Capabilities

### New Capabilities
- `dissolve-host-crud`: 裁撤主机表（recycle_host_info）的 data-service CRUD HTTP 接口及 pkg/client 封装

### Modified Capabilities
- `check-apply-order-host`: 校验继承主机接口增加 require_type=3（机房裁撤）的校验逻辑，包括裁撤表校验、GPU机型限制，并返回机型代次信息
- `create-ticket-apply`: 创建申领单据接口支持 require_type=3 时的固资号校验和计费模式继承
- `modify-ticket-apply`: 修改单据重试接口支持 bk_asset_id 和 inherit_instance_id 的存储与回调传递

## Impact

- **data-service 层**：新增 `cmd/data-service/service/dissolve/` 目录注册 CRUD 路由
- **client 层**：`pkg/client/data-service/tcloud-ziyan/` 新增 dissolve 相关 client 文件
- **woa-server 业务逻辑**：`cmd/woa-server/logics/task/scheduler/scheduler.go` 改造 `CheckInheritedHost` 和 `checkInheritedHost`，新增机房裁撤校验分支
- **woa-server 类型定义**：`cmd/woa-server/types/task/scheduler.go` 的 CheckInheritedHostReq 新增 require_type 字段
- **CRP 响应结构**：`pkg/thirdparty/cvmapi/cvmapi_response.go` 的 QueryCvmInstanceTypeItem 新增 generationType 字段
- **MongoDB 存储结构**：`cmd/woa-server/dal/task/table/modify_record.go` 的 ModifyData 新增 bk_asset_id 和 inherit_instance_id
- **API 接口文档**：check_apply_order_host.md、scr_create_ticket_apply.md、create_ticket_apply.md、update_ticket_apply.md（已由需求方更新）
