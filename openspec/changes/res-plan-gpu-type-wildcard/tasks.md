## 1. GPU 类识别与卡类型归一

- [x] 1.1 在 `cmd/woa-server/logics/plan/device_types.go` 增加 `isGpuClass`：技术分类为 `推理GPU`/`训练GPU`/`GPU-其他`
- [x] 1.2 增加 `gpuTypeKind` 归一：`TrimSpace` 后空字符串=缺失，「无」=无卡，其余=真实卡型号
- [x] 1.3 将通配比较抽成可单测的私有函数（输入两侧规格，输出 match / 缺卡类型错误），`IsDeviceMatched` 只负责取缓存并逐对调用

## 2. 通配规则落地

- [x] 2.1 同名机型保持直接可通配
- [x] 2.2 双方均为 GPU 类且都是真实卡类型：卡类型原文全等则可通配（可跨机型族），不等则不可通配
- [x] 2.3 GPU 类机型禁止回退 `TechnicalClass` + `CoreType`：一侧 GPU、一侧非 GPU 时不可通配（同名短路径除外）
- [x] 2.4 仅双方都非 GPU 类时，才按 `TechnicalClass` + `CoreType` 判定
- [x] 2.5 GPU 类且卡类型不是真实卡（空或「无」）：成对比较标为不通配并 `Warnf`，**不得**让 `IsDeviceMatched` 整次失败；仅申领/回收/升降配的**操作对象**在匹配前校验失败并返回「GPU 类机型缺少真实卡类型」
- [x] 2.6 任一侧不在机型缓存中：保持现网不通配（`continue`），不与「非真实卡」错误混用

## 3. 列表展开降级

- [x] 3.1 预测列表展开（`expandDemandToDeviceTypes`）：对 GPU 非真实卡 `Warnf` + `continue`，该机型不展开，其余记录照出
- [x] 3.2 用户正在申领/回收/升降配校验的**目标机型**为 GPU 非真实卡：保持 `return err`（可阻断）
- [x] 3.3 池构建与合并（`GetProdResPlanPool` / `GetProdResPlanPoolMatch` / `remapConsumePoolDeviceTypes` / `compactResPlanPool*`）：脏 GPU 只退出通配（Warn，不 Union），不得 `return err` 拖垮其他机型申领
- [x] 3.4 `GetPlanTypeAvlDeviceTypesV2`（可用机型/计费类型列表、推荐）：按不可阻断处理，`Warnf` + 跳过该机型，整接口仍返回

## 4. 单测

- [x] 4.1 新增 `device_types_test.go` 表驱动覆盖：同卡跨族可通配、不同卡同族不可通配、`A100`≠`NVIDIA A100`、GPU「无」/空不可通配、GPU vs 非 GPU 不可通配、双方「无」不得因「无」=「无」互配、非 GPU 保持原口径、同名短路径、GPU 非真实卡报错
- [x] 4.2 覆盖两个空卡或两个「无」的 GPU 机型不得互配（报错或列表侧不通配）

## 5. 验证

- [x] 5.1 对照 AC-001 / AC-002 / AC-003 / AC-004 / AC-005 / AC-006 / AC-010 跑通单测
- [x] 5.2 对照 AC-007 / AC-008：仅操作对象非真实卡时报错；列表 / 可用机型 / 池中脏 GPU 告警且其余继续
- [x] 5.3 对照 AC-009：同一对同卡 GPU 在申领与列表均为可通配
- [x] 5.4 确认无新接口、无权限点变更；依赖 `device-type-gpu-type-sync` 已具备 `DistinctDeviceType.GpuType`
