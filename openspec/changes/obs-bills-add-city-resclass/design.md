## Context

OBS 消费 HCM 上报的账单数据，用于按城市维度成本分析和 GPU/CPU 资源分类统计。目前三张 OBS 账单表（obs_aws_bills、obs_huawei_bills、obs_gcp_bills）缺少城市 ID 和资源类型分类 ID，OBS 无法直接完成这两个维度的分析。

当前 OBS 上报流程：`account_bill_item（分账后）→ obs/sync/sync_{vendor}.go → obs_{vendor}_bills`。convert 函数在此过程中完成字段映射，是填充新字段的最合适位置。

## Goals / Non-Goals

**Goals:**
- 三张 OBS 账单表新增 `CityId` 和 `ResClassId` 字段
- 每次 OBS 上报时自动填充这两个字段
- 支持通过 `account_bill_region_city_rel` 表维护地域→城市映射
- 支持通过 `global_config` 管理 GPU 机型配置（AWS 机型列表、HuaWei 前缀列表）

**Non-Goals:**
- 不修改历史已上报数据（存量数据两个字段默认为 0）
- 不对 OBS 平台侧做任何修改
- 不修改 account_bill_item 分账表结构

## Decisions

### 决策 1：GPU 配置存 `global_config` 而不是专用配置表

**选择**：复用现有 `global_config` 表，新增 `config_type = "account_bill"`，存两条记录（AWS 机型列表、HW 前缀列表）。

**备选**：新建 `account_bill_gpu_device_type` 表。

**理由**：配置条数少（各 1 条 JSON 数组），不需要行级 CRUD，`global_config` 的 JSON value 完全能表达，且运营人员可通过现有 global_config 管理接口维护，避免引入新表。

---

### 决策 2：`account_bill_region_city_rel` 建为独立表而不是 global_config

**选择**：单独建表，提供 CRUD 接口，以 `(vendor, region)` 为唯一键。

**备选**：存入 global_config，每个厂商一条 JSON map。

**理由**：地域-城市映射预期条目较多（每个厂商 N 个地域），需要支持逐条增删改查，关系型表结构更合适。同时建立 unique key `(vendor, region)` 保证映射唯一。

---

### 决策 3：OBS sync 批量初始化映射，不在逐条账单时查 DB

**选择**：每个 batch（doSyncXxxBillItem）开始前，一次性加载以下数据到内存：
- `account_bill_region_city_rel` 全量 map（当前 vendor）→ `map[region]int32`
- GPU 配置（global_config）→ `map[string]struct{}` 或 `[]string`

**备选**：逐条账单查询 DB。

**理由**：单次 batch 最大 500 条（DefaultMaxPageLimit），若逐条查 DB 会产生 500+ 次查询。一次性加载映射表数据量小（预期 < 1000 条），完全可以在内存中。

---

### 决策 4：IsAIBillItem 不按厂商拆分

**选择**：kimi、jina、Veo、Imagen、Lyria 统一加入现有 `getAIBillItemAIFlag()` 列表，对所有厂商生效。

**理由**：这些关键词本质是 AI 产品标识，厂商边界在上层（dailysplit 阶段）已由各自 splitter 控制输入字段，加入共享列表不会引起误判。

---

### 决策 5：华为 GPU 判断通过 `product_spec_desc` 字段

**选择**：从 `item.Extension.ResFeeRecordV2.ProductSpecDesc` 提取，按 "|" 取第一段，再提取字母前缀匹配 GPU 前缀列表。

**理由**：华为 SDK `model.ResFeeRecordV2` 已有 `ProductSpecDesc` 字段，且已被整体序列化到 Extension JSON，无需额外透传；字母前缀匹配是华为官方 GPU 机型的命名规律（p2v.2xlarge → 前缀 p2v）。

---

### 决策 6：CityId 兜底值

- 国内站（`MainAccountChinaSite`）：300001
- 国际站：300002
- 查不到映射时按账号站点选兜底值

## Risks / Trade-offs

- **[Risk] 映射表数据未维护导致兜底** → 上报时记录 Warnf 日志（地域 + vendor + rid），便于运营排查补录映射关系
- **[Risk] global_config 中 GPU 配置不存在** → 视为空列表，所有条目判定为 CPU，记录 Warnf 日志；不阻断上报流程
- **[Risk] HuaWei ProductSpecDesc 为 nil** → 直接判定为 CPU，不报错
- **[Risk] 存量数据两字段为 0** → OBS 侧需知悉，CityId=0 和 ResClassId=0 表示历史无数据，不代表实际值

## Migration Plan

1. 执行 SQL 迁移：三张 OBS 账单表 ALTER ADD COLUMN（DEFAULT 0，不影响存量数据读写）
2. 执行 SQL 迁移：创建 `account_bill_region_city_rel` 表
3. 部署 data-service（包含新表 DAO 和接口）
4. 录入 global_config 中的 GPU 配置数据（aws_gpu_instance_types、huawei_gpu_instance_prefixes）
5. 录入 account_bill_region_city_rel 初始映射数据
6. 部署 task-server（包含改造后的 OBS sync 逻辑）

**回滚**：task-server 回滚不影响已写入数据；SQL 字段 ALTER DROP 可回滚（需评估 OBS 侧是否已依赖新字段）

## Open Questions

（无）
