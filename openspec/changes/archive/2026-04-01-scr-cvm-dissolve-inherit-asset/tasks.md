## 1. data-service 裁撤主机 CRUD 接口

- [x] 1.1 在 `pkg/api/data-service/dissolve/` 下定义 API 协议结构体：批量创建请求/响应、列表查询请求/响应、批量更新请求、批量删除请求，参考 `pkg/api/data-service/rolling-server/` 模式
- [x] 1.2 在 `cmd/data-service/service/dissolve/recycle-host/` 下新建 `init.go`，注册 4 个路由：`POST /dissolve/recycle_hosts/batch/create`、`PATCH /dissolve/recycle_hosts/batch`、`POST /dissolve/recycle_hosts/list`、`DELETE /dissolve/recycle_hosts/batch`
- [x] 1.3 实现 `create.go` — 批量创建，调用 `dao.RecycleHost().CreateWithTx`
- [x] 1.4 实现 `query.go` — 列表查询，调用 `dao.RecycleHost().List`
- [x] 1.5 实现 `update.go` — 批量更新，调用 `dao.RecycleHost().UpdateWithTx`
- [x] 1.6 实现 `delete.go` — 批量删除，调用 `dao.RecycleHost().DeleteWithTx`
- [x] 1.7 在 `cmd/data-service/service/service.go` 中注册 dissolve recycle-host 模块的 InitService

## 2. client 封装

- [x] 2.1 在 `pkg/client/data-service/tcloud-ziyan/` 新增 `dissolve.go`，定义 `DissolveClient` 结构体，封装 `BatchCreateRecycleHost`、`ListRecycleHost`、`BatchUpdateRecycleHost`、`BatchDeleteRecycleHost` 四个方法
- [x] 2.2 在 `pkg/client/data-service/tcloud-ziyan/client.go` 的 `Client` 结构体中新增 `Dissolve *DissolveClient` 字段，在 `NewClient` 中初始化

## 3. CRP 响应结构新增 generationType

- [x] 3.1 在 `pkg/thirdparty/cvmapi/cvmapi_response.go` 的 `QueryCvmInstanceTypeItem` 结构体中新增 `GenerationType string` 字段（json tag: `generationType`）

## 4. CheckInheritedHost 改造

- [x] 4.1 在 `cmd/woa-server/types/task/scheduler.go` 的 `CheckInheritedHostReq` 中新增 `RequireType enumor.RequireType` 字段（json: `require_type`）
- [x] 4.2 在 `cmd/woa-server/logics/task/scheduler/scheduler.go` 的 `CheckInheritedHost` 方法中，在调用 `checkInheritedHost` 之前，若 `param.RequireType == enumor.RequireTypeDissolve`，调用 data-service client 的 `ListRecycleHost` 校验固资号是否在裁撤表中
- [x] 4.3 改造 `checkInheritedHost` 方法，根据 `param.RequireType` 分派校验分支：`RequireTypeRollServer` 保持现有逻辑（CommonType 校验），`RequireTypeDissolve` 执行 GPU 机型族校验（DeviceGroup 不含 `constant.GpuInstanceClass`）
- [x] 4.4 在 `CheckInheritedHost` 中调用 `QueryCvmInstanceType` 获取被继承机型的 `generationType`，并在接口响应中返回
- [x] 4.5 确保 `CheckInheritedHost` 在 Handler 层（`cmd/woa-server/service/task/scheduler.go`）正常传递 `RequireType` 参数

## 5. 创建申领单据支持机房裁撤计费继承

- [x] 5.1 在创建申领单据的校验流程中（`cmd/woa-server/logics/task/scheduler/scheduler.go` 相关方法），增加 require_type=3 且 bk_asset_id 非空时的分支：调用 `CheckInheritedHost`(require_type=3) 获取计费信息
- [x] 5.2 校验通过后，用返回的 `InstanceChargeType` 覆盖 suborder.Spec.ChargeType，用返回的 `CloudInstID` 设置 suborder.Spec.InheritInstanceId，保留接口传入的 charge_months 不做覆盖

## 6. 修改单据重试支持固资号传递

- [x] 6.1 在 `cmd/woa-server/dal/task/table/modify_record.go` 的 `ModifyData` 结构体中新增 `BkAssetId string` 和 `InheritInstanceId string` 字段（json/bson tag 分别为 `bk_asset_id`、`inherit_instance_id`）
- [x] 6.2 修改构建 `ModifyRecord` 的逻辑（`createModifyRecord` 或相关函数），从请求参数中取 `bk_asset_id` 存入 `ModifyData.BkAssetId`，若 require_type 为裁撤(3)或滚服(6)且 bk_asset_id 非空，调用 check 接口获取 `CloudInstID` 存入 `ModifyData.InheritInstanceId`
- [x] 6.3 在 `auditApplyModifyCallback` 中构建 `ModifyApplyReq` 时，将 `modifyRecord.Details.CurData.InheritInstanceId` 赋值给 `maReq.Spec.InheritInstanceId`
- [x] 6.4 在 `modifyOrder` 方法的 `update` MapStr 中新增 `"spec.inherit_instance_id"` 和 `"spec.bk_asset_id"` 字段，确保写回 `cr_ApplyOrder`
