## Why

调账明细的资源类别当前只有 `cpu` 与 `gpu` 两个值，同一个 `gpu` 值下混杂了三种性质完全不同的支出——具体 GPU 卡的算力成本、大模型 API 调用费、以及识别不出卡型的 GPU 支出。口径合并造成两个后果：调账金额无法按卡型或模型厂商归集，核算侧看不出 GPU 成本的实际构成；同步到 OBS 时 API 调用费被计入 GPU 分类，与 OBS 账单侧已经拆分出的 API 分类（AWS 6799 / GCP 6800）口径不一致。

现在做的原因是 OBS 账单侧的卡型与 API 厂商识别（`obs-bills-add-gpu-card-api-brand`）已经落地，账单侧已按卡型与厂商归集，调账侧不跟进就会形成长期的口径分裂。

## What Changes

- `BillAdjustmentResClass` 枚举由 2 值扩展为 4 值：`cpu`、`gpu_card`、`gpu_api`、`gpu_other`。**BREAKING**：原 `gpu` 值下线，创建与更新接口传 `gpu` 将被拒绝，须与前端需求 1069995598136722866 同期上线。
- `bill_adjustment_item` 表新增一列 `res_sub_class`（资源子类），单列承载卡型或模型厂商，语义由同一条记录的 `res_class` 决定。
- 创建与更新接口接收并校验 `res_sub_class`：`gpu_card` / `gpu_api` 下必填且取值须在该云厂商对应的清单内；`cpu` / `gpu_other` 下必须为空。类别从 GPU 类改为非 GPU 类时要求显式置空，服务端不自动清空。
- 列表查询接口返回 `res_sub_class`，导出 Excel 在「资源类别」列后新增「资源子类」列。
- 新增两个枚举查询接口，按云厂商返回卡型清单与模型厂商清单，供前端下拉使用。取值逻辑与创建/更新的校验共用同一份实现。
- 调账同步 OBS 时改用 `enumor.GetOBSResClassIDByType`，使 `gpu_api` 落到 OBS 的 API 资源分类；同时把单列 `res_sub_class` 按类别拆分写入 OBS 的 `GpuCardCategory` 与 `APIBrandName` 两列。
- 存量 `res_class='gpu'` 的记录一次性刷为 `gpu_card`，`res_sub_class` 留空。

## Capabilities

### New Capabilities

- `bill-adjustment-res-sub-class`: 资源类别四值语义、资源子类单列的数据模型、创建与更新的必填与取值域校验、列表与导出的展示，以及存量数据刷新。
- `bill-adjustment-sub-class-enum-api`: 按云厂商返回卡型与模型厂商枚举的两个查询接口，以及「厂商 → 清单」这份被下拉与校验共用的取值逻辑。
- `bill-adjustment-obs-sync-res-class`: 调账同步 OBS 时的资源分类 ID 映射（四类 × 三厂商）与资源子类到 `GpuCardCategory` / `APIBrandName` 两列的分发。

### Modified Capabilities

无。`openspec/specs/` 下当前没有调账相关的已归档能力。需要注意的是未归档变更 `bill-adjustment-add-res-class` 中存在 `bill-adjustment-res-class` 的 delta spec，本变更的 `bill-adjustment-res-sub-class` 在枚举取值上取代它——该变更归档时需以本变更的枚举定义为准，不可回退为 `cpu` / `gpu` 两值。

## Impact

**受影响的层级（跨三层 + 基础设施）**：

- **Service Layer / account-server**：调账明细的创建、更新、列表、导出接口；新增两个枚举查询接口的路由、鉴权与 service 逻辑。
- **Service Layer / task-server**：`cmd/task-server/logics/action/obs/sync/sync_adjustment.go` 中 AWS / GCP / 华为三处 OBS 记录构造。
- **Resource Layer / data-service**：调账明细的 CRUD 协议与 DAO，随新列同步扩展。
- **Infrastructure / MySQL**：`bill_adjustment_item` 表 DDL 变更（可空列，向后兼容）；存量数据刷新脚本。
- **跨层公共包**：`pkg/criteria/enumor/bill.go` 的枚举定义与卡型 / 厂商清单；`pkg/dal/table/bill/`、`pkg/api/core/bill/` 的表结构与核心模型。

**外部系统**：OBS 核算系统（DB 直写）。`ResClassId`、`GpuCardCategory`、`APIBrandName` 三列的写入口径变化，需与 OBS 账单上报侧保持一致。本变更不改 OBS 表结构。

**数据来源依赖**：卡型清单依赖 `global_config` 中的 `aws_gpu_instance_types` 与 `gcp_gpu_instance_prefixes` 两项运营配置。配置缺失时按空映射降级，不阻断接口。

**已知限制**：华为在代码中不存在卡型来源、也无 OBS API 资源分类，因此华为的两个清单均为空，实际只能选 `cpu` 与 `gpu_other`。这是「枚举按厂商区分」的必然结果，已确认符合预期。
