## Context

HCM 主机回收链路（`cmd/woa-server/logics/short-rental/device_type.go::ListDeviceTypeFamily`）已经在查本地表 `woa_device_type_physical_rel`，但该表目前**仅靠人工录入维护**：

```
当前态：
   CRP 新增机型  ────┐
                    │ （无自动同步）
                    ▼
              【运维手动 SQL 录入】 ──▶ woa_device_type_physical_rel
                                                      │
                                                      ▼
                                            短租回收业务读
```

CRP 已承诺 `queryCvmTypeList` 接口到位，本期补齐**写路径**（同步任务 + 手动触发），实现：

```
目标态：
   CRP queryCvmTypeList                              短租回收业务读
              │                                              ▲
              ▼                                              │
   定时同步任务 ──┐                                          │
                 ├──▶ 两段式 diff（仅新增/更新）──▶ woa_device_type_phys_rel
   手动触发 API ─┘
```

**关键现状盘点**：

- `pkg/thirdparty/cvmapi/cvmapi.go` 已存在完整的 `CVMClientInterface`，所有 JSON-RPC 调用走统一模式（`subPath="/yunti-demand/external"` + `WithParam(CvmApiKey, CvmApiKeyVal)` + `WithHeaders(kt.Header())`）
- `cmd/woa-server/task/` 下已有 2 个 cron task 实现（`DeviceCapacityTask`、`RollingMonthlyTerminateNoticeTask`），项目规范要求每个 task 实现 `Name() / Next() / Do() / GetURL()` 接口
- `cmd/woa-server/service/res-sync/` 已是"资源同步"类手动触发 API 的归属包，参考 `SyncCapacities` 行 `h.Add("SyncCapacities", POST, s.tasks[XXX].GetURL(), s.SyncCapacities)` 的注册模式
- `pkg/cron` 框架内置 master 选举（`sd.IsMaster()`）确保多节点不重复执行
- 本地表 `woa_device_type_physical_rel`（SQLVER=0055）已存在，唯一键 `idx_device_type`，data-service 已有完整 CRUD API

## Goals / Non-Goals

**Goals:**

- 替换人工录入流程，每日定时同步 CRP 映射数据到本地表（仅做新增 + 更新两段，不做删除）
- 提供运维手动触发能力（无前端，POST API + IAM 校验）
- 同步过程异常时**保留本地表上一次成功的快照**，业务读路径无感知
- 与项目现有 cron task 规范严格一致（task 自承载同步逻辑 + `GetURL()` 暴露手动 API 路径）

**Non-Goals:**

- 不改造业务读路径（已是查本地表）
- 不引入新的 logics 抽象层（同步逻辑直接写在 task struct 的 method 里，符合项目规范）
- 不支持 dry-run / 部分同步 / 多部门动态 deptName 等高级模式（本期 YAGNI）
- 不消费 CRP 的扩展字段（cpu/ram/gpu/technicalClass 等）写入本地表；扩展字段在 `CvmTypeItem` 结构体中已对接，便于后续按需消费
- 不自动删除 CRP 已下线的机型映射；如需清理由运维通过 SQL 人工处理
- 不引入运行元数据表（sync_run / sync_history 等）
- 不引入独立告警 API 调用，复用 `logs.Errorf` 通过日志平台关键字告警

## Decisions

### D1: 同步策略——两段式 diff（仅新增/更新，不删除）

**决策**：拉 CRP 快照 → List 本地表全量 → 内存计算 `toCreate` / `toUpdate` → 顺序执行 `BatchCreate` → `BatchUpdate`。**本期不删除**任何记录。

**理由**：

| 方案 | 优势 | 劣势 |
|------|------|------|
| A. **两段式 diff（仅 toCreate + toUpdate）** ✅ | 操作明确、便于审计、对 DB 写入量小（只写差异）、本地表无需结构改动；对回收链路最友好——一度支持生产、当前不再支持生产的机型仍可在本地表查到映射 | 2 次 RPC 往返；本地表会保留 CRP 已下线机型（需运维定期人工清理） |
| B. 三段式 diff（含 toDelete） | 与 CRP 状态完全对齐，无残留 | 风险：某机型先在 CRP 中存在 → 用户用此机型买了生产虚拟机 → CRP 中下线该机型 → 同步任务把映射删掉 → 用户退机时回收链路找不到机型族归属 |
| C. Truncate + 全量 Insert | 实现简单 | 同步窗口内存在"空表瞬间"，业务读会查不到映射 |
| D. INSERT...ON DUPLICATE KEY UPDATE + 批次标记 | 单 SQL 操作 | 需要给本地表加 `synced_at` / `batch_id` 字段，破坏现状 |

**为什么放弃删除**：业务方在 brainstorm 阶段明确反馈："某机型一度支持生产→用户已用此机型→机型在 CRP 下线"是真实存在的场景，此时本地保留旧映射比"与 CRP 严格对齐"更重要。`applyDelete` 函数在实现中暴露但**未在 `Do` 主流程中调用**，预留给后续运维侧批量清理工具使用。

预计 CRP 返回 < 500 条，两次 RPC 总耗时远 ≪ 5 分钟性能目标。

### D2: 异常时数据策略——保留上次成功快照

**决策**：

| 异常类型 | 处理 |
|----------|------|
| HTTP 失败 / 超时 | `logs.Errorf` + 立即 `return err`，**不动表** |
| JSON-RPC 错误码非 0 | `logs.Errorf` + 立即 `return err`，**不动表** |
| 返回 0 条 | `logs.Errorf` + 立即 `return nil`，**不动表**（避免空数据进入 diff） |
| 单条记录字段为空 | `logs.Warnf` + 跳过该记录，继续处理其他 |
| BatchCreate/Update 任一步失败 | `logs.Errorf` + 立即 `return err`（已写部分保留，**不回滚**）|

**理由**：业务可用性 > 数据一致性。即便同步出问题，业务读到的也是上一次成功的快照（最坏情况是机型映射略旧），不影响主机回收流程的可用性。

**Trade-off**：BatchCreate 后 BatchUpdate 失败，本地表会处于"半新半旧"状态，但仍优于"空表"。下一次同步会自然收敛。

### D3: 包结构——task 自承载同步逻辑

**决策**：同步主逻辑**直接写在 `cmd/woa-server/task/sync_device_type_physical_rel.go` 的 task struct method 里**，不抽独立的 `logics/` 层。

**理由**：

- 严格遵循项目现有规范。`DeviceCapacityTask` 和 `RollingMonthlyTerminateNoticeTask` 都是同样写法
- 手动 API 通过 `s.tasks[CronTaskSyncDeviceTypePhysicalRel].Do(cts.Kit)` 直接复用，无需额外抽象层
- 减少文件数，降低维护成本

**Alternatives 考虑**：

- 抽 `logics/res-sync/sync_device_type_physical_rel.go` → 与现有 `device_capacity_task.go` 风格不一致，且 `res-sync` 包当前是另一套 `wait.JitterUntil` 调度模式，混入会引起包内调度模式不统一

### D4: 调度框架——pkg/cron + Master 检查

**决策**：使用 `pkg/cron` 框架（与 `DeviceCapacityTask` 一致），而非 `cmd/woa-server/logics/res-sync/` 包内的 `wait.JitterUntil` 模式。

**理由**：

- 同步任务会批量写 DB，必须有 master 检查防止多节点重复执行；`pkg/cron` 内置该机制
- 同类型任务（"定时同步"）的 `DeviceCapacityTask` 已采用该模式
- 统一注册到 `service/service.go::initCronTask`，运维查"所有定时任务"时一目了然

### D5: 手动 API 路径——通过 `GetURL()` 与 cron 任务绑定

**决策**：参考 `RollingMonthlyTerminateNoticeTask.GetURL()` 和 `DeviceCapacityTask.GetURL()`，task 实现 `GetURL() string` 方法返回固定子路径 `/device_type_physical_rels/sync`，`service/res-sync/service.go` 通过 `s.tasks[XXX].GetURL()` 注册路由。

```go
// 注册示意（service/res-sync/service.go::initService）
h.Add("SyncDeviceTypePhysicalRel", http.MethodPost,
    s.tasks[enumor.CronTaskSyncDeviceTypePhysicalRel].GetURL(),
    s.SyncDeviceTypePhysicalRel)
```

**理由**：保持 cron 任务名和手动 API 路径的强绑定，避免两边定义漂移。该模式已是项目规范。

### D6: CRP 鉴权与请求参数——复用现有 cvmapi 模式 + 不传部门参数

**决策**：新增的 `QueryCvmTypeList(kt, params)` 方法严格复用现有 cvmapi method 模式：

```go
func (c *cvmApi) QueryCvmTypeList(kt *kit.Kit,
    params *QueryCvmTypeListParams) (*QueryCvmTypeListResp, error) {
    req := &QueryCvmTypeListReq{
        ReqMeta: ReqMeta{Id: CvmId, JsonRpc: CvmJsonRpc, Method: QueryCvmTypeListMethod},
        Params:  params,
    }
    // ... subPath="/yunti-demand/external" + WithHeaders(kt.Header())
}
```

同步任务侧**调用时传入空 `params`（即不传 `deptName`）**，由 CRP 返回该集群下的全量机型映射。`QueryCvmTypeListParams.DeptName` 字段在结构体中保留，便于将来其他场景按部门过滤复用。

**理由**：业务方反馈"`IEG技术运营部` 名下的机型族可能不完整"，传部门参数会缩小可见范围。回收链路只关心机型 → 物理机机型族映射本身、不区分部门，因此优先选择"尽可能查到更多机型"。

定时任务用 `core.NewBackendKit()` 创建的 kit 调用（与 `ressync.SyncLeftIP` 一致），CRP 侧的服务级身份认证由网络层处理，**不需要在代码里管理 Cookie**。

### D7: IAM 权限——复用 GlobalConfig

**决策**：手动触发 API 走 `meta.GlobalConfig` 资源的 `meta.Create` action 权限校验（与现有 `CreateDeviceTypePhysicalRel` 一致），运维角色已有该权限。

**理由**：避免新增 IAM 资源类型，降低部署侧改动。

### D8: 配置项命名

**决策**：配置项放在 `cc.WoaServer().ResourceSync.SyncDeviceTypePhysicalRel.Interval`（单位：分钟），默认 1440（即每日 1 次）。同步：

- `cmd/woa-server/etc/woa_server.yaml`
- `docs/support-file/helm/...`

## Risks / Trade-offs

| 风险 | 缓解 |
|------|------|
| 同步窗口内业务读到半新半旧状态 | 两段式 diff 仅写差异，5 分钟内完成；业务读容错（map 没 key 走兜底） |
| BatchCreate 后 BatchUpdate 失败，表处于中间态 | 业务可用性优先，下次同步会自然收敛；保留上次部分数据 > 空表 |
| 同步任务统一不传 `deptName`，无法按部门过滤 | 当前回收链路不区分部门；`QueryCvmTypeListParams.DeptName` 字段保留，后续多部门场景可在调用方按需传入 |
| CRP 接口扩展字段未来变化 | 同步任务仅写入 `cvmInstanceModel` + `deviceFamily` 两个字段；`CvmTypeItem` 虽然映射了全部字段，但其余字段在本期不参与持久化，未来变化影响面小 |
| 运维通过 SQL 直接改本地表会被下次同步更新覆盖 | 业务规则 R-001 已规约；可在表上加 `source` 字段留痕（本期不做） |
| 多节点重复执行导致数据脏写 | `pkg/cron` 框架内置 `sd.IsMaster()` 检查 |
| CRP 真的下线所有机型（返回 0 条） | 显式保护：告警 + 不写表；本期不做 toDelete，所以即便误判也不会清空映射 |
| CRP 下线的机型映射会在本地长期残留 | 本期接受残留：业务可用性 > 数据一致性；如需清理，由运维评估后通过 SQL（或预留的 `applyDelete` 后续封装的清理工具）人工处理 |

## Migration Plan

1. **配置下发**：先在测试环境的 `woa-server.yaml` 加 `resourceSync.syncDeviceTypePhysicalRel.interval`，开启同步任务
2. **首次同步对账**：手动触发一次同步，对比同步前后的 `woa_device_type_physical_rel` 数据，确认 diff 合理（人工录入数据 vs CRP 返回数据）
3. **观察 1-2 个周期**：验证定时任务正常触发、master 检查生效、异常告警通道可达
4. **生产环境灰度**：在生产环境分批开启（先 master 节点观察、再全量）

**回滚策略**：

- **代码回滚**：移除 cron task 注册即可（`service.go::initCronTask` 删除对应行），手动 API 自动失效
- **数据回滚**：本地表无 schema 变更，回滚后业务继续读旧数据；如需回到"人工录入"模式，需运维通过 SQL 重新录入历史数据

## Open Questions

- **首次同步前是否需要对账工具**：若人工录入数据与 CRP 返回数据存在大量冲突，建议提供一个 dry-run 模式（仅打日志不写表）便于运维评估冲击。本期暂不实现，留作下一期增量；如确实需要，可后续在 task 上加 `DoWithDryRun(kt)` 方法（参照 `RollingMonthlyTerminateNoticeTask.DoWithMonth`）
- **告警阈值**：连续 N 次同步失败是否升级告警？目前依赖日志平台的关键字告警机制，不在代码内做阈值控制
- **CRP 下线机型映射的清理时机**：实现里 `applyDelete` 已就绪但未在 `Do` 主流程中调用。后续若需要清理"CRP 已下线 + HCM 业务也确认不再使用"的机型，可作为一次性运维脚本或独立 manual API 暴露，本期不做
