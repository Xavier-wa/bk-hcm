## Why

需求「证书管理-支持证书分配给业务-后端」（TAPD `1069995598137438279`）经评审后结论为：

- **F-001 云打标已取消**：证书分配到业务仅更新 DB 的 `bk_biz_id`，不修改云上资源标签（对齐全平台其他资源「分配只改 DB」的现状）。
- **FR2 同步自动分配保留**：作为本后端需求的唯一净新增量 —— hc-service 同步自研云（ziyan）证书时，依据云上 `二级业务(Bs2)` 标签反查 `bk_biz_id`，自动完成业务归属。

当前 hc-service 同步自研云证书时（`cmd/hc-service/logics/res-sync/ziyan/cert.go` 的 `Cert`），`opt.BkBizID` 始终为 `constant.UnassignedBiz`，即同步后所有证书 `bk_biz_id` 均为未分配（-1），云上已存在的业务标签完全不被消费，导致证书无法自动归入业务、需人工逐一分配。

同类资源（LB、安全组）已在同步/打标流程中复用 `ziyan.GetBkBizIdByBs2` 实现「Bs2 标签 → bk_biz_id」的归属。证书需沿用同一能力，但遵循本需求决议 **OQ4=仅补充未分配**（DB 已分配则保留、不覆盖）。

## What Changes

- 复用 `ziyan.GetBkBizIdByBs2` + `ziyan.ParseResourceMetaIgnoreErr`，在自研云证书同步中新增「按云上二级业务(Bs2)标签自动归属业务」逻辑（仅在 DB 未分配时写入，已分配则保留）。
- 扩展 `pkg/adaptor/types/cert.TCloudCert`：新增 `BkBizID` 字段与 `GetTagMap()` 方法（与 `typeslb.TCloudClb` 对齐），承载同步解析出的业务归属。
- 修改 `ziyan.cert.go`：
  - `Cert()` 在拉取云/DB 证书后、执行 diff 前调用新增的 `fillCertBkBizId`，为云上证书填充 `BkBizID`。
  - `createCert`：新建证书时 `BkBizID` 改用云标签解析结果（替代固定 `opt.BkBizID`）。
  - `convCloudToDBUpdate`：更新请求补充 `BkBizID` 字段，使未分配证书被同步回填。
- 不引入任何云上打标调用（FR1 已取消），不改动「分配接口」与前端，不触碰非自研云同步路径。

## Capabilities

### New Capabilities

- `cert-sync-auto-biz-assign`：自研云（ziyan）证书同步时，依据云上 `二级业务(Bs2)` 标签自动反查并写入 `bk_biz_id`；DB 已分配（含手动分配）时保留、不覆盖（OQ4=仅补充未分配）。仅作用于 ziyan 证书同步。

### Modified Capabilities

（无现有 spec 级行为变更）

## Impact

- **adaptor 层**：`pkg/adaptor/types/cert/tcloud.go`（`TCloudCert` 新增 `BkBizID`、`GetTagMap()`）
- **HC Service 资源同步层**：`cmd/hc-service/logics/res-sync/ziyan/cert.go`（`Cert`/`createCert`/`convCloudToDBUpdate`/`fillCertBkBizId`）
- **依赖（BlueKing 生态）**：CMDB（`cmdb.Client.SearchBusiness`，经 `ziyan.GetBkBizIdByBs2` 调用）、`pkg/ziyan/resource_tag.go`（`GetBkBizIdByBs2`、`ParseResourceMetaIgnoreErr`、`TagKeyBs2`、`NotFoundID`）
- **不涉及**：Access 层、API 层、非 ziyan 云（`tcloud/aws/azure` 等）证书同步、证书「分配接口」本身
