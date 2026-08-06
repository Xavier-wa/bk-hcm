## Context

调账明细的 `res_class` 目前是 `cpu` / `gpu` 二值，定义在 `pkg/criteria/enumor/bill.go`。OBS 账单上报侧已经通过 `obs-bills-add-gpu-card-api-brand` 落地了卡型（`GpuCardCategory`）与 API 厂商（`APIBrandName`）两列的识别，调账侧不跟进就形成口径分裂。

本变更跨三层：account-server（写入校验、读出展示、两个新接口）、task-server（OBS 同步）、data-service（调账明细 CRUD 协议与 DAO），加上 MySQL 的 DDL 与存量数据。

三个现状约束直接影响设计：

1. **卡型与厂商清单的现有实现分散且多为私有**。`gcpGpuCardL1Keywords`（一级卡型关键字表）与 `getAIBillItemAIFlag()`（API 厂商七值）都是包私有的，对外只暴露了匹配函数 `MatchGcpGpuCardByKeyword` 与 `MatchAPIBrandName`——能判断「某个字符串命中哪个卡型」，但拿不到「清单本身」。
2. **`global_config` 的读取逻辑绑定在 task-server**。`loadAwsGpuInstanceTypes` 与 `loadGcpGpuInstancePrefixes` 位于 `cmd/task-server/logics/action/obs/sync/gpu_lookup.go`，通过 task-server 专有的 `actcli.GetDataService()` 取客户端，account-server 无法直接调用。
3. **创建与更新请求的字段风格不一致**。创建请求的 `ResClass` 是值类型加 `validate:"required"`；更新请求的 `ResClass` 也是值类型，用 `if r.ResClass != ""` 判断是否要校验。这个风格无法表达「显式置空」。

## Goals / Non-Goals

**Goals:**

- 资源类别四值化，资源子类以单列承载卡型或模型厂商
- 写入侧的必填、互斥、取值域校验按类别与云厂商双重区分
- 两个枚举查询接口，与写入校验共用同一份取值逻辑
- 调账同步 OBS 时资源分类 ID 与卡型 / API 厂商两列的口径与账单上报侧一致
- 存量 `gpu` 数据一次性刷新，且刷新可重复执行

**Non-Goals:**

- 不改 OBS 表结构，只补调账同步的写入逻辑
- 不修正已同步到 OBS 的历史账单数据
- 不重构 task-server 侧既有的卡型识别与 `global_config` 读取实现
- 不做卡型 / 厂商清单的运营可配置化改造（沿用现有来源）
- 不支持按资源类别或资源子类筛选调账明细
- 不含前端改动（归需求 1069995598136722866）

## Decisions

### 决策一：资源子类用单列而非双列

`bill_adjustment_item` 新增一列 `res_sub_class varchar(64) NULL`，语义由同一行的 `res_class` 解释。

**备选方案**：按原需求「需要两个不同字段」的字面意思建两列（卡型列、厂商列）。

**选择单列的理由**：两者在业务上永不共存，双列会让「CPU 类别却带着卡型」这类脏数据在数据结构层面成为可能。单列从结构上排除了它。

**代价**：取值域校验必须结合 `res_class` 判定，不能只校验「落在两个清单的并集内」——否则 `gpu_card` + `gemini` 这种错配会通过。此外单列要在同步 OBS 时拆分写入两列（决策六）。

原需求说的「两个字段」实际描述的是前端两个选择框：选项来源不同、切换时互斥渲染，但最终提交到同一个后端字段。

### 决策二：更新请求的资源子类用指针类型

更新请求中 `ResSubClass` 定义为 `*string`，创建请求中定义为值类型 `string`。

**理由**：规格要求「类别从 GPU 细分改为非细分时必须显式置空，服务端不自动清空」。值类型无法区分「请求没带这个字段」和「请求把它传成了空串」——两者都是 `""`。用 `*string` 后，`nil` 表示未传、指向空串表示显式置空。

**备选方案**：服务端在类别改为 `cpu` / `gpu_other` 时自动清空子类。**否决理由**：运营误改类别时会静默丢掉卡型信息且没有任何提示，而调账数据是核算口径的输入，静默丢数据的代价高于多报一次错。

`ResClass` 在两个请求中都保持现有的值类型写法不动——它的「未传」语义现有代码已用 `if r.ResClass != ""` 表达，且本变更不需要把类别置空。

### 决策三：取值逻辑落在 account-server，不跨服务复用 task-server 的实现

在 account-server 侧新增一份「云厂商 → 卡型清单 / 厂商清单」的取值实现，供两个枚举接口与写入校验共同调用。task-server 侧 `gpu_lookup.go` 的既有实现保持不动。

**理由**：`gpu_lookup.go` 的 loader 依赖 task-server 专有的 `actcli.GetDataService()`，抽公共函数就得改动其调用方式，属于本变更范围外的重构。两侧共用的是**配置键常量与配置值的 JSON 结构约定**（`enumor.GlobalConfigKeyAwsGpuInstanceTypes`、`GlobalConfigKeyGcpGpuInstancePrefixes`），而不是函数实现。

**「同源」的范围界定**：规格要求的「下拉与校验同源」指 account-server 内部这两个消费方共用一份实现，不要求跨服务与账单上报侧共用。两侧消费的是同一份配置数据，口径一致由配置键保证。

**风险**：两处解析逻辑并存，配置结构变化时需同步改两处。已在风险一节记录。

### 决策四：在 enumor 补两个导出函数暴露清单

现有实现只暴露匹配函数，拿不到清单本身，因此需要补：

- 一个返回一级卡型清单的函数（`gcpGpuCardL1Keywords` 中各项的 `card` 字段去重）
- 一个返回调账可选 API 厂商清单的函数

**API 厂商清单必须是新函数而不是复用 `getAIBillItemAIFlag()`**：后者返回七值，含 `veo`、`imagen`、`lyria`。这三者在上报侧已被 `MatchAPIBrandName` 归并为 `gemini`，若出现在调账下拉里会产生「选了 veo，但核算口径落在 gemini」的分裂。调账侧只提供归并后的四值：`gemini`、`claude`、`kimi`、`jina`。

清单定义留在 `enumor` 而不是搬到 account-server，是为了与匹配函数共处一地——将来往 `gcpGpuCardL1Keywords` 加卡型时，清单函数自动跟着变，不会漏改。

### 决策五：华为云的两个清单为空，源于其配置形态不含卡型映射

华为云在 `global_config` 中确实有一项 GPU 配置（`GlobalConfigKeyHuaweiGpuInstancePrefixes`），但它是 `[]string` 的**规格前缀清单**，只用于 `isHuaweiGPU` 判定「这条账单是不是 GPU」，不含「前缀 → 卡型」的映射。AWS 与 GCP 的配置是 `map[string]string`，value 才是卡型。

所以华为云不是「没配置」，而是**配置形态上产不出卡型**。模型厂商侧同理：华为云无 OBS API 资源分类，上报侧的 `APIBrandName` 本期统一留空。

**推论**：华为云实际只能选 `cpu` 与 `gpu_other`。这是「枚举按厂商区分 + 华为两个清单都为空」的必然结果，不是额外加的限制，已确认符合预期。若将来要支持，需先补一份华为的「前缀 → 卡型」映射配置，属另一变更。

### 决策六：资源子类到 OBS 两列的分发在同步侧完成

`sync_adjustment.go` 构造 OBS 记录时，按 `res_class` 把单列拆开：`gpu_card` 写卡型列、`gpu_api` 写 API 厂商列、其余两列都写空串。同时把三处 `enumor.GetOBSResClassID(vendor, isGPU)` 换成 `enumor.GetOBSResClassIDByType(vendor, isGPU, isAPI)`，`isAPI` 由 `res_class == gpu_api` 推出。

`GetOBSResClassIDByType` 是既有函数，本变更只改调用方，不改它的内部逻辑——包括华为云 `gpu_api` 回落到 6315 这个分支。该分支在本期不可达（华为云创建不出 `gpu_api` 记录），保留为防御性逻辑。

### 决策七：取值域严格比对，包括大小写；不做规范化

`res_sub_class` 的取值必须与清单中的写法完全一致，包括大小写。代码侧**不做**大小写归一，也不在落库前把取值改写成清单中的规范写法——落库值即请求值。

**备选方案**：忽略大小写比对，命中后以清单中的原始写法落库。**否决理由**：规范化是一层隐式改写，运营提交 `h200` 却在库里看到 `H200`，会让「我填的和存的不一样」变成一个需要额外解释的行为；而且一旦引入归一，后续每加一处取值来源都要考虑归一规则，复杂度只增不减。宁可在运营录错时直接报错。

由于下拉与校验同源（决策三），运营从下拉里选出来的值必然与清单完全一致，正常路径上不会因大小写被拒。严格比对真正拦下的是绕过下拉直接调接口、且大小写与清单不符的请求——这类请求本就该报错。

**代价转移到配置治理**：`gpu_card` 约定大写，但 AWS 与 GCP 的卡型来自运营在 `global_config` 录入的 value，代码不控制其大小写。若运营在 AWS 配置里录成 `h200`、而 GCP 一级清单是 `H200`，库里就会出现两种写法，后续按卡型归集会算成两类。这个约束只能靠运营侧录入规范保证，已在风险一节记录。

`gpu_api` 的四值是代码内硬编码的小写常量，不受此影响。

### 决策八：存量刷新随 SQL 变更脚本执行，不另写 Go 工具

新增列的 DDL 与存量刷新的 `UPDATE` 放在同一个 SQL 变更文件（`SQLVER=9999`、`HCMVER=v9.9.9`，文件名以 `9999` 开头）。

**理由**：`UPDATE ... WHERE res_class = 'gpu'` 天然幂等，二次执行匹配 0 行，不需要额外的幂等控制。调账明细的数据量级下单条 UPDATE 足够，不需要 Go 侧的分批调度。

**备选方案**：独立的 Go 迁移工具。**否决理由**：为一条幂等 UPDATE 引入工具、配置与执行流程，收益不足。

## Risks / Trade-offs

**[破坏性变更导致前后端版本错配]** → `res_class` 移除 `gpu` 值后，旧前端提交 `gpu` 会被拒。必须与前端需求 1069995598136722866 同期上线，且存量刷新须在后端新版本上线前执行完成。

**[回滚不完全干净]** → 代码回滚后，库里已存在的 `gpu_card` / `gpu_api` / `gpu_other` 记录对旧版本是非法值。由于校验只在写入路径生效，这些记录的查询、导出、OBS 同步都不受影响，但**编辑会失败**。新增列本身是可空列，旧版本忽略它，DDL 无需回滚。回滚前需评估这批记录是否有编辑需求。

**[两处 global_config 解析逻辑并存]** → account-server 与 task-server 各有一份解析。配置值结构变化时需同步改两处。缓解：两处共用 `enumor` 中的配置键常量，并在 account-server 侧的解析函数注释中指向 `gpu_lookup.go` 的对应实现。

**[运营录入的卡型大小写不规范会导致口径分裂]** → 按决策七代码不做大小写归一，而 AWS 的 `aws_gpu_instance_types` 与 GCP 的 `gcp_gpu_instance_prefixes` 的 value 大小写由运营录入决定。若同一个卡型在两处录成不同大小写（如 `h200` 与 `H200`），库里会出现两种存储值，按卡型归集时算成两类。缓解：卡型枚举查询接口的返回值即库中将出现的写法，运营可通过该接口自查；上线前核对两项配置中已有 value 的大小写是否统一为大写。**该风险不由代码兜底，属配置治理范畴。**

**[Q-001：业务举例的卡型不可选]** → 原需求举例的 `B300`、`H300` 不在任何现有清单内，本期运营选不到。这是唯一会实质影响交付价值的遗留项。缓解：`gcp_gpu_instance_prefixes` 由运营维护、无需发版即可新增，若这两个卡型能通过实例族前缀映射出来则可即时补上；否则需补 `gcpGpuCardL1Keywords` 并同步改上报侧识别逻辑，属另一变更。**建议在开发前与产品确认。**

**[存量记录不满足新校验]** → 刷新产出的 `gpu_card` + 空子类记录一旦被编辑就必须补卡型；华为云的此类记录无卡型可补，须先把类别改为 `gpu_other`。已确认接受该现状，校验不为此放宽——放宽会给「漏填卡型」留一个长期后门。

**[OBS 侧写入口径变化未经对方确认]** → 本变更改变了 `ResClassId` 的取值分布（新增 6799 / 6800）并开始写入此前为空的两列。表结构不变，但核算侧的统计口径会变。建议上线前与 OBS 侧对齐。

**[华为云 gpu_api 分支不可达但保留]** → `GetOBSResClassIDByType` 中华为云 `gpu_api` 回落 6315 的分支在本期无法触发。保留是因为该回落是函数既有语义，改它会波及账单上报侧的调用方。作为防御性逻辑接受。

## Migration Plan

1. **执行 SQL 变更**：新增 `res_sub_class` 可空列，并把存量 `res_class='gpu'` 刷为 `gpu_card`。可空列对旧版本服务透明，此步可先行。
2. **发布后端**：account-server、data-service、task-server 同批发布。
3. **发布前端**：与需求 1069995598136722866 同期，此前旧前端仍可正常工作（`gpu` 值在步骤 1 后已无存量，但旧前端提交 `gpu` 会在步骤 2 后开始被拒——因此步骤 2 与 3 之间的窗口应尽可能短）。
4. **验证**：抽查四类调账各一条的创建、列表、导出，以及一轮 OBS 同步后 `ResClassId` 与两列的落值。

**回滚**：代码按服务回滚，DDL 不回滚（可空列，旧版本忽略）。回滚后已存在的四值记录无法编辑，见风险一节。

## Open Questions

- **Q-001**：`B300`、`H300` 是否必须在本期可选？若是，需确认它们能否通过 `gcp_gpu_instance_prefixes` 的实例族前缀映射产出，否则本变更范围需扩大到上报侧识别逻辑。
- **Q-002**：`gcpGpuCardL1Keywords` 中 `tpu7x` 的卡型短名，代码里是 `TPU`，而 `obs-bills-add-gpu-card-api-brand/proposal.md` 写的是 `TPU7x`。本变更按代码取 `TPU`。需确认哪一侧为准——若以文档为准，需同时改上报侧，会影响已上报数据的一致性。
- **Q-003**：新列名 `res_sub_class` 与两个枚举接口的路由命名待确认。接口路由建议沿用创建接口的 `/api/v1/account/vendors/{vendor}/bills/adjustment_items/` 前缀。
- 未归档变更 `bill-adjustment-add-res-class` 中的 `bill-adjustment-res-class` delta spec 仍是二值枚举。该变更归档时须以本变更的四值定义为准，不可回退。
