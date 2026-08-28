## Context

资源预测通配的唯一入口是 `woa-server` 的 `Controller.IsDeviceMatched`（`cmd/woa-server/logics/plan/device_types.go`）。现口径：

1. 机型名相同 → 可通配
2. 双方都能在 `deviceTypesMap` 中取到规格，且 `TechnicalClass` 与 `CoreType` 都相同 → 可通配

该函数已被申领扣减 / 消耗池合并（`demand.go`）、预测列表展开（`demand_aggregate.go`）、升降配/可用机型校验（`verify.go`）复用。回收与申领走同一套资源池通配。需求附图中的 CRP「小模型分卡管理」只作业务背景，HCM 不改 CRP。

依赖单 `device-type-gpu-type-sync` 已在 `DistinctDeviceType` 增加 `GpuType`。`deviceTypesMap.GetDeviceTypes` 读到的规格已含 `GpuType`、`TechnicalClass`、`DeviceFamily`、`CoreType`。

需求文档称现网口径为「机型族 + 核心类型」，现网实现实际是 **技术分类 + 核心类型**。本期不改非 GPU 回退口径，只在文档与设计中对齐这一事实。

约束：TAPD `1069995598136604107`；不新增对外接口；不做卡类型别名；等 `gpu_type` 同步上线后再上，不做过渡兼容。

## Goals / Non-Goals

**Goals:**

- 在 `IsDeviceMatched` 内实现 GPU 类识别、真实卡比较；GPU 类禁止回退技术分类 + 核心类型
- 四处业务入口继续只调用该函数，保持同一口径
- GPU 类且卡类型不是真实卡（空或「无」）：可阻断入口返回错误；不可阻断入口告警且该机型不通配
- 用表驱动单测覆盖正反例

**Non-Goals:**

- 不改机型同步 / 主动登记（依赖单职责）
- 不改 CRP、不新增 API / 权限 / 前端筛选项
- 不做 `A100` 与 `NVIDIA A100` 等别名映射
- 不把现网非 GPU 口径从 TechnicalClass 改成 DeviceFamily
- 不覆盖其他云厂商

## Decisions

### 1. 只改 IsDeviceMatched，不拆四套规则

**决策**：把新规则全部放进 `IsDeviceMatched`（可抽私有纯函数便于单测），调用方不复制分支。

**理由**：四处入口已经共用该函数；拆到各入口会破坏「同一口径」。

**备选**：按申领/列表分别实现——否决，违反 F-006。

### 2. GPU 类只认技术分类

**决策**：`enumor.CvmTechnicalClass.IsGPUClass()` 为真当且仅当技术分类为 `推理GPU` / `训练GPU` / `GPU-其他`。不通过机型族判断。

**理由**：任务 1.1 只列技术分类；机型族「GPU型」会把未填技术分类的边缘机型误推进卡类型分支。

**备选**：技术分类 ∪ 机型族——否决，与任务口径不一致。

### 3. GPU 类必须看真实卡，禁止回退

**决策**：机型一旦判定为 GPU 类，必须读取并校验卡类型。卡类型不是真实卡（`TrimSpace` 后为空，或为「无」）时，该机型不可通配，且不得回退 `TechnicalClass` + `CoreType`。一侧 GPU、一侧非 GPU 同样不可通配（同名短路径除外）。仅双方都非 GPU 类时，才按 `TechnicalClass` + `CoreType` 判定。

| 归一后取值 | 语义 | GPU 类处理 |
|-----------|------|-----------|
| 空字符串 | 缺失，不是真实卡 | 不可通配；可阻断报错，不可阻断 Warn |
| `无` | 展示无卡，不是真实卡 | 同上，不得因「无」=「无」互配 |
| 其他 | 真实卡型号 | 双方都是真实卡时原文全等比较 |

**理由**：澄清后口径收紧——GPU 通配只认真实卡；回退会让「无」在并查集里把 A100/V100 重新焊在一起。Q-003 原选 B 已废止。

**备选**：一侧有卡一侧「无」回退现网口径——否决，GPU 机型不能回退。

### 4. 非真实卡：成对不通配；只拦操作对象

**决策**：GPU 类且卡类型不是真实卡时，该机型不与任何其他机型通配（同名短路径除外），**不得**因此让整次 `IsDeviceMatched` 失败。成对结果为 `false`，打 `Warnf`（含机型标识和 rid）。

仅当**操作对象**（用户正在申领 / 回收 / 升降配校验提交的那台机型）是 GPU 非真实卡时，才 `return`「GPU 类机型缺少真实卡类型」。池里的脏 GPU、列表里的脏 GPU 只退出通配，不拖垮其余机型。

| 入口 | 处理 |
|------|------|
| 预测列表展开 | Warn，该机型不展开，其余记录照出 |
| `GetPlanTypeAvlDeviceTypesV2`（可用机型 / 计费类型 / 推荐） | 同列表：Warn，该机型不展开，接口仍返回 |
| 池构建 / 消耗池并查 / compact（`GetProdResPlanPool*` 等） | 脏 GPU 不参与 Union，自己仍可按同名进桶；整池继续 |
| 申领 / 回收 / 升降配校验的**操作对象** | 该机型是脏 GPU → 本次操作失败 |

实现上：成对比较函数对非真实卡返回「不通配」而不是 error；可阻断入口在匹配前对操作对象做一次真实卡校验。

**理由**：脏 GPU 是数据问题，不应让同产品下的正常机型申领失败；查询列表必须出页。

**备选**：`IsDeviceMatched` 看到脏 GPU 就整次 `return err`——否决，池构建会误伤。给函数加 `strict bool`——否决，调用方容易传错。

### 5. 非 GPU 口径保持 TechnicalClass + CoreType

**决策**：仅非 GPU 主路径比较 `TechnicalClass` 与 `CoreType`，不改成 `DeviceFamily + CoreType`，也不把 GPU 机型引进这条路径。

**理由**：假设 5 与 F-005：本期不改非 GPU 口径。需求口语「机型族」对应现网实现的技术分类。

**备选**：改成 DeviceFamily——否决，会改变全部非 GPU 通配结果，超出范围。

### 6. 同名机型短路径保留

**决策**：`ele == deviceType` 仍直接可通配，不读卡类型。

**理由**：自身与自身通配是现网契约；避免缺字段时把自己判死。

### 7. 机型主数据缺失

**决策**：任一侧不在 `deviceTypeMap` 中：保持现网 `continue`（该对不通配）。若可中断入口后续取规格失败，沿用现有 `device type not found` 错误，不在 `IsDeviceMatched` 新增第二种缺卡错误。

**理由**：与现网一致，避免把「机型不存在」和「GPU 缺卡类型」混成一个错误。

## Risks / Trade-offs

- [列表展开把非真实卡从 Error 降为 Warn] → 可能掩盖数据质量问题。缓解：日志必须带机型名和 rid；可阻断入口仍报错。
- [只认技术分类可能漏判机型族为 GPU 但技术分类未填的机型] → 按非 GPU 走技术分类 + 核心类型。接受，与任务 1.1 一致。
- [卡类型原文全等导致 `A100` 与 `NVIDIA A100` 不通配] → 接受，别名不在本期。
- [依赖 `gpu_type` 未同步完就上线] → 大量 GPU 申领会报错。缓解：本需求等依赖单落地后再上，不做过渡回退。
- [池构建若仍整次 `return err`] → 会违背「脏 GPU 只退出通配」。缓解：池构建 / 可用机型列表对 subject 非真实卡改 Warn，不往上抛。

## Migration Plan

1. 确认 `device-type-gpu-type-sync` 已上线，且 GPU 类机型 `gpu_type` 已由同步或主动登记补齐
2. 发布 `woa-server`（仅判定逻辑）
3. 回滚：回退该服务版本即可恢复「技术分类 + 核心类型」通配，无数据迁移

## Open Questions

无。池内脏 GPU 只退出通配（不拖垮其他申领）；`GetPlanTypeAvlDeviceTypesV2` 按不可阻断列表处理。
