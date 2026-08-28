## Context

自研云 CVM 机型主数据走两条入口：定时/手动从 CRP 同步，以及运营在「CVM 机型配置」页主动登记。同步链路已写入 `gpu_amount` 与 `tech_class_res_amt`，CRP `QueryCvmInstanceTypeItem` 已有 `GPUType`（json `gpuType`），但 HCM 未落库、未比较、未返回。

当前关键断点：

```
CRP QueryCvmInstanceType.gpuType  ──已有──▶  hc-service 构建 DeviceType  ──未赋值 GpuType
                                                          │
                                                          ▼
                                               isDeviceTypeChanged 未比较 gpu_type
                                                          │
                                                          ▼
device_type 表 / DeviceTypeCreate / Core DeviceType  ──无 gpu_type 字段
                                                          │
                          ┌─────────────────────────────────┴──────────────────────────┐
                          ▼                                                            ▼
              查询 ConvTableToDeviceType 自然透传（加字段即可）         创建页/文档未暴露 gpu_type
                                                                       tech_class_res_amt 后端已有、前端未录
```

约束：仅自研云；权限点不变；卡类型以 CRP/录入原文为准，HCM 不做翻译。TAPD `36810399` 已确认创建页为可搜索下拉+手输，配置列表不加列。

## Goals / Non-Goals

**Goals:**

- `device_type` 新增 `gpu_type`，全链路读写与查询返回
- 自研云同步消费 `gpuType`；空值不跳过；仅该字段变化也更新
- 主动登记创建接口与创建页支持 `gpu_type`、`tech_class_res_amt`
- 配置页按 `gpu_type` 筛选；已有规格详情可展示卡类型
- 补齐创建/查询接口文档

**Non-Goals:**

- 不改 CVM 申领筛选/推荐算法或筛选项
- 不覆盖其他云厂商机型
- 不提供一次性历史回填脚本
- 后端不维护 GPU 卡类型强制枚举
- 配置列表不加 `gpu_type` 列，不新增独立详情页
- 批量编辑仍只改 `disable`，不增加卡类型编辑
- 不改 CRP 客户端契约（只消费已有字段）

## Decisions

### 1. 字段类型与默认值用空字符串而非 NULL

**决策**：`gpu_type VARCHAR(64) NOT NULL DEFAULT ''`，Go 侧使用 `string`，校验 `lte=64`。

**理由**：与表内其他字符串规格字段一致；空串语义明确（无卡类型/未同步到）；避免指针与 NULL 分支。位置放在 `gpu_amount` 之后，便于规格字段相邻。

**备选**：NULL 默认值——否决，增加查询与同步比较复杂度。

### 2. 后端不按枚举拦截卡类型

**决策**：`gpu_type` 只做长度校验，不做白名单。前端预置选项仅辅助录入。

**理由**：Q-001 明确选项外手输必须落库；CRP 卡型号字符串可能演进，HCM 维护枚举会漏拦或误拦。

**备选**：后端枚举——否决，与 AC-007 冲突。

### 3. 同步映射与差异检测复用 gpu_amount 模式

**决策**：

1. `listDeviceTypeFromCloud` 构建时 `GpuType: item.GPUType`
2. `isDeviceTypeChanged` 增加字符串相等比较
3. `createDeviceType` / `updateDeviceType` 带上 `GpuType`
4. 空 `gpuType` **不** `continue`（与 `CalcTechClassResAmt` 失败跳过区分）

**理由**：最小侵入，与已落地的卡数/技术分类资源量同步同构；空卡类型是合法规格，不是计算失败。

**备选**：gpuType 为空则跳过——否决，违反 AC-002 / R-004。

### 4. tech_class_res_amt 只补入口，不改计算逻辑

**决策**：同步侧计算与落库保持现状。本期只在主动登记创建页增加录入，并在创建接口文档中声明可选字段。后端 `DeviceTypeCreate.TechClassResAmt` 已存在，未传即为 0。

**理由**：F-003 是入口对齐，不是重做资源量计算。

### 5. 创建页交互对齐实例族 allowCreate

**决策**：GPU 卡类型使用 `hcm-form-enum`（或等价可搜索选择器）`allowCreate=true`，预置常见卡型常量（如 `NVIDIA A100`、`NVIDIA V100`、`NVIDIA T4`、`NVIDIA H20`、`NVIDIA L20`、`NVIDIA L40` 等，实现时以现网常见原文为准，允许微调列表）。技术分类资源量使用非负数输入，精度与 `DECIMAL(10,2)` 对齐。

**理由**：Q-001 要求与实例族同类交互；预置降低录入成本，手输保证覆盖 CRP 原文。

### 6. 配置页筛选走现有 filter expression，列表不加列

**决策**：`gpu_type` 加入 `DeviceTypeColumnDescriptor`（String），配置页筛选区增加「GPU 卡类型」，有值时向 `/config/findmany/config/cvm/device` 追加 `{ field: 'gpu_type', op: 'eq', value }`。`cvmModelColumns` **不**新增列。

**理由**：Q-002；列描述符接入后 data-service 过滤无需新接口。

**备选**：新增 distinct 卡类型接口供下拉——本期不做，筛选项可用预置+手输，降低范围。

### 7. 规格详情在现有 extra-text 中附带卡类型

**决策**：在申领规格详情/已选机型文案中，当 `gpu_type` 非空时追加展示（例如卡数旁附带卡类型原文）。不新增页面、不改推荐。

**理由**：满足“已有规格详情可展示”，改动面最小。

### 8. 查询透传依赖 Core 转换，不改 woa 组装逻辑

**决策**：`DeviceType` / `DistinctDeviceType` 及 `ConvTableTo*` 增加 `GpuType`。`query.go` 已走转换函数；woa `GetCvmDeviceDetail` / `GetDeviceType` / `CreateManyDevice` 透传请求体，无需新 handler 逻辑。

### 9. SQL / 接口文档使用占位版本

**决策**：开发期不预占真实 SQL 序号与 HCM 版本。新增 `scripts/sql/9999_<date>_device_type_gpu_type.sql`，`SQLVER=9999`、`HCMVER=v9.9.9.9`，`hcm_version` 视图同步写占位。已有接口文档不改原「该接口提供版本」，新增字段描述标注 `v9.9.9.9+`。真实编号在出包前由发布流程替换。

**理由**：其他需求可能先出包，按当前最大号 +1 会打乱顺序。

## Risks / Trade-offs

| 风险 | 影响 | 缓解 |
|------|------|------|
| 历史机型 `gpu_type` 为空直到下次同步 | 上线后短期内筛选/详情看不到卡类型 | 需求已接受；依赖常规同步，不写回填脚本 |
| CRP `gpuType` 原文与运营手输不一致（空格/大小写） | 筛选 eq 可能漏匹配 | 不做归一，与 R-001/R-002 一致；文档说明按原文匹配 |
| 前端预置选项过时 | 运营需手输 | 允许手输；后端不拦截 |
| 同步耗时微增 | 多一个字符串赋值与比较 | 相对基线可忽略；AC-P01 用任务日志核对 |
| 列表不加列导致卡类型不直观 | 运营需靠筛选或详情 | Q-002 已确认 |

## Migration Plan

1. 执行 SQL：`device_type` 增加 `gpu_type`
2. 先发 `data-service`（读写新字段），再发 `hc-service`（同步写入），再发 `woa-server` / 前端（创建页、筛选、详情）
3. 触发或等待自研云机型同步，补齐历史 `gpu_type`
4. 合并接口文档

**回滚**：服务可回退到旧版本（忽略新列）。新增列带默认空串，旧代码不读不写可继续运行。不回滚 DROP COLUMN，除非运维明确要求。

## Open Questions

无。Q-001 / Q-002 已确认；历史回填、空值策略、非枚举校验均为需求假设。
