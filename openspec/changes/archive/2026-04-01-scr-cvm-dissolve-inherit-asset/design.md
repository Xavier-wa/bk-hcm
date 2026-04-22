## Context

当前"主机申领"功能中，仅"滚服项目"(require_type=6)支持继承固资号。继承的核心链路是：

1. **CheckInheritedHost**：前端调用该接口，传入固资号，后端校验主机合法性并返回计费信息（机型、计费模式、时长等）
2. **CreateApplyTicket**：创建申领单据时，传入 `inherit_instance_id`（云实例 ID）和 `bk_asset_id`（固资号），后端进行校验后入单
3. **ModifyApplyTicket → auditApplyModifyCallback**：修改单据重试时，先把修改内容存入 MongoDB 的 `ModifyRecord`，审批通过后回调 `auditApplyModifyCallback` 将数据写回 `cr_ApplyOrder`

现需为"机房裁撤"(require_type=3)复用该链路，但校验规则不同：
- 固资号来源不同（来自 `recycle_host_info` 表而非滚服项目）
- 机型限制不同（滚服要求通用机型 `CommonType`，机房裁撤要求非 GPU 机型族）
- 校验接口响应中补充返回机型代次（`generation_type`）

**约束**：
- DAO 层（`pkg/dal/dao/dissolve/host/host.go`）已实现 CRUD，但未暴露 HTTP 接口
- `QueryCvmInstanceTypeItem` 需要新增 `generationType` 字段以接收 CRP 返回值
- `ModifyData`（MongoDB）目前不含 `bk_asset_id` 和 `inherit_instance_id`

## Goals / Non-Goals

**Goals:**
- 为 `recycle_host_info` 表新增 data-service HTTP CRUD 接口及 client 封装
- 改造 `CheckInheritedHost` 支持 require_type=3 的差异化校验
- 在 CRP 响应结构中增加 `generationType` 字段，并在校验接口响应中返回
- 创建单据时支持 require_type=3 的固资号校验和计费模式继承
- 修改单据重试流程支持 `bk_asset_id` 和 `inherit_instance_id` 的完整传递

**Non-Goals:**
- 前端页面及代码改造（固资号选择 UI、推荐列表等均不在本次范围）
- 固资号推荐逻辑（按标准型/高IO型/大数据型/计算型推荐）
- 运营管理员提供跨业务固资号的场景
- CRP 接口本身的改造（仅适配其返回结构）

## Decisions

### D1: data-service dissolve CRUD 接口设计

**方案**：参考 `cmd/data-service/service/rolling-server/rolling-returned/` 的模式，在 `cmd/data-service/service/dissolve/` 下新增 `recycle-host/` 子目录，包含 `init.go`（路由注册）、`create.go`、`update.go`、`query.go`、`delete.go`。

路由路径规划：
- `POST /dissolve/recycle_hosts/batch/create` — 批量创建
- `PATCH /dissolve/recycle_hosts/batch` — 批量更新
- `POST /dissolve/recycle_hosts/list` — 列表查询
- `DELETE /dissolve/recycle_hosts/batch` — 批量删除

API 协议结构在 `pkg/api/data-service/dissolve/` 下定义。

**理由**：完全复用现有 data-service CRUD 模式，降低认知负担，保持代码结构一致性。

### D2: client 封装位置

**方案**：在 `pkg/client/data-service/tcloud-ziyan/` 新增 `dissolve.go`，定义 `DissolveClient` 并注册到 `Client` 结构体。

**替代方案**：在 `pkg/client/data-service/` 下新建独立目录。
**理由**：`recycle_host_info` 是自研云(Ziyan)特有表，数据属于自研云体系，放在 `tcloud-ziyan` 目录更合适。

### D3: CheckInheritedHost 改造策略 — 通过 require_type 分派

**方案**：
- `CheckInheritedHostReq` 新增 `RequireType int` 字段（json: `require_type`）
- 在 `checkInheritedHost` 中根据 `RequireType` 分派不同校验分支：
  - `RequireTypeRollServer(6)`：保持现有逻辑不变（通用机型校验 `CommonType`）
  - `RequireTypeDissolve(3)`：新增校验逻辑
- 使用更中性的 `CheckInheritedHost` 命名，通过 require_type 区分不同场景

**机房裁撤校验链路**：
1. 调用 data-service List 接口，校验固资号是否在 `recycle_host_info` 表中
2. 调用 `getInheritedHostFromCC` 获取主机信息（复用现有逻辑）
3. 调用 `ListCvmInstanceInfoByDeviceTypes` 获取机型信息，校验 `DeviceGroup` 不含 `constant.GpuInstanceClass`("GPU")
4. 调用 `QueryCvmInstanceType` 获取机型详情，并将 `generationType` 返回给调用方

### D4: QueryCvmInstanceTypeItem 新增 generationType

**方案**：在 `pkg/thirdparty/cvmapi/cvmapi_response.go` 的 `QueryCvmInstanceTypeItem` 结构体中新增：
```go
GenerationType string `json:"generationType"`
```

**注意**：按当前 CRP 实际返回，`generationType` 使用 `string` 接收，不引入 `GenerationTypeName` 字段，也不在校验阶段做代次一致性比较。

### D5: 创建单据时的计费模式继承

**方案**：在现有 `validateSubOrder`（或对应的申领校验函数）中增加分支：
- 当 `require_type == RequireTypeDissolve(3)` 且 `bk_asset_id` 非空时：
  1. 调用 `CheckInheritedHost`（内部已含裁撤校验）获取返回的计费信息
  2. 用返回的 `InstanceChargeType` 覆盖接口传入的 `charge_type`（计费模式以 BKCC 为准）
  3. 保留接口传入的 `charge_months` 不做覆盖（计费时长由用户决定）
  4. 用返回的 `CloudInstID` 设置 `inherit_instance_id`

**理由**：复用 check 接口的校验结果，避免重复调用 BKCC 和 CRP 接口。

### D6: ModifyData 新增字段 & auditApplyModifyCallback 传递

**方案**：
1. `cmd/woa-server/dal/task/table/modify_record.go` 的 `ModifyData` 新增：
   ```go
   BkAssetId         string `json:"bk_asset_id" bson:"bk_asset_id"`
   InheritInstanceId string `json:"inherit_instance_id" bson:"inherit_instance_id"`
   ```
2. 修改单据接口 `UpdateApplyTicket` / `UpdateBizApplyTicket` 在构建 `ModifyRecord` 时，从请求参数中取 `bk_asset_id` 存入 `ModifyData`，同时通过 check 接口获取 `inherit_instance_id` 一并存入
3. `auditApplyModifyCallback` 中构建 `ModifyApplyReq` 时：
   - `maReq.Spec.InheritInstanceId = modifyRecord.Details.CurData.InheritInstanceId`
4. `modifyOrder` 中需要新增 `"spec.inherit_instance_id"` 和 `"spec.bk_asset_id"` 的更新字段，确保写回 `cr_ApplyOrder`

**替代方案**：在回调时重新调用 check 接口获取 `inherit_instance_id`。
**理由**：重复调用会增加外部依赖调用次数，且审批可能跨时较长（期间外部接口状态可能变化），使用存储值更稳定。

## Risks / Trade-offs

- **[CRP generationType 可用性]** → 当 CRP 未返回该字段时，接口返回空字符串，不影响主流程。后续若要增加代次相关业务规则，可基于返回值扩展。
- **[MongoDB 结构变更]** → `ModifyData` 新增字段为可选字段（零值不影响），旧数据兼容无风险。
- **[data-service 新接口]** → 新增的 CRUD 接口仅供 woa-server 内部调用，无需外部鉴权。但需确保路由注册到 data-service 的 WebService 中。
- **[校验逻辑耦合]** → `CheckInheritedHost` 承载滚服和裁撤两种逻辑，通过 require_type 分派。若后续还有更多场景，需考虑拆分为独立方法。当前两种场景差异明确，合并可接受。
