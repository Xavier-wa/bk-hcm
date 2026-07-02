## 1. 数据库迁移

- [x] 1.1 新建 `scripts/obssql/0005_add_gpu_card_api_brand_fields.sql`：参照 `0004_add_city_res_class_fields.sql` 格式，为 `obs_aws_bills`、`obs_huawei_bills`、`obs_gcp_bills` 三张表执行 `ALTER TABLE ADD COLUMN GpuCardCategory varchar(64) NOT NULL DEFAULT ''` 和 `ADD COLUMN APIBrandName varchar(64) NOT NULL DEFAULT ''`，并更新 `hcm_version` 视图 sql_ver 为 0005
- [x] 1.2 整理 `global_config` 中 `aws_gpu_instance_types` 的新格式数据（实例类型→卡型 JSON 对象，如 `{"g5.12xlarge":"A10G","g6.xlarge":"L4"}`），键集合 MUST 与改造前数组元素一致，作为上线运营刷新依据（不在代码仓库执行，记录到迁移说明）
- [x] 1.3 整理 `global_config` 新增 `config_key=gcp_gpu_instance_prefixes` 的数据（实例族前缀→短卡型名 JSON 对象，如 `{"A3Ultra":"H200","A3 Ultra":"H200","A3":"H100","A2":"A100","G2":"L4","G4":"RTX6000PRO"}`），作为上线运营录入依据，记录到 `migration-config.md`

## 2. 枚举与品牌匹配函数

- [x] 2.1 在 `pkg/criteria/enumor/bill.go` 新增 `MatchAPIBrandName(str string) string` 函数：**必须先 `strings.ToLower(str)` 再匹配**（`aiBillItemRegexp` 的词边界只含 `[a-z]`、无大写，不先转小写则 `VEO`/`Gemini` 等大写文本无法命中，与 `IsAIBillItem` 的处理一致）；复用 `aiBillItemRegexp`（忽略大小写、词边界），用 `FindStringSubmatch` 取第 2 个捕获组得到命中关键词；将 `veo`/`imagen`/`lyria` 归并为 `gemini`，其余取原值，未命中返回空字符串
- [x] 2.2 在 `pkg/criteria/enumor/global_config.go` 更新 `GlobalConfigKeyAwsGpuInstanceTypes` 的注释，说明 `config_value` 由 JSON 数组改为「实例类型→卡型」JSON 对象
- [x] 2.3 在 `pkg/criteria/enumor/global_config.go` 新增 `GlobalConfigKeyGcpGpuInstancePrefixes = "gcp_gpu_instance_prefixes"`（`GlobalConfigKeyAccountBill` 类型），注释说明 `config_value` 为「实例族前缀→短卡型名」JSON 对象
- [x] 2.4 在 `pkg/criteria/enumor/bill.go` 新增 GCP L1 显式卡型关键词识别函数（如 `MatchGcpGpuCardByKeyword(str) string`）：忽略大小写 + 词边界，有序匹配 `H200`/`H100`/`A100`(含 `Tesla A100`)/`L4`/`RTX (Pro )?6000`(→`RTX6000PRO`)/`TPU7x`/`V100`/`P100`/`P4`/`K80`，命中返回短卡型名，未命中返回空字符串（去掉 T4，不含 L3）

## 3. OBS 账单表 Go 层结构体更新

- [x] 3.1 在 `pkg/dal/table/obs/bill_aws.go` 的 `OBSBillItemAwsColumnDescriptor`（紧邻 `ResClassId` 后）新增 `GpuCardCategory`、`APIBrandName`（Type: enumor.String），并在 `OBSBillItemAws` 结构体新增对应字段（`db:"GpuCardCategory"`、`db:"APIBrandName"`，string）
- [x] 3.2 在 `pkg/dal/table/obs/bill_huawei.go` 的对应位置新增 `GpuCardCategory`、`APIBrandName` 字段（descriptor + struct）
- [x] 3.3 在 `pkg/dal/table/obs/bill_gcp.go` 的对应位置新增 `GpuCardCategory`、`APIBrandName` 字段（descriptor + struct）

## 4. gpu_lookup 配置改造（数组→映射）

- [x] 4.1 在 `cmd/task-server/logics/action/obs/sync/gpu_lookup.go` 将 `loadAwsGpuInstanceTypes` 返回类型由 `map[string]struct{}` 改为 `map[string]string`（实例类型→卡型），`config_value` 按 JSON 对象解析；解析失败记录 Warnf 并返回错误；配置缺失返回空 map（不阻断）
- [x] 4.2 调整 `isAwsGPU` 的 `awsGpuSet` 参数类型为 `map[string]string`，GPU 判定改为 `if _, ok := awsGpuMap[productInstanceType]; ok`（语义不变，保证 `ResClassId` 兼容）
- [x] 4.3 在 `gpu_lookup.go` 新增取卡型逻辑（直接读 map value 即可，未命中返回空字符串；卡型 MUST 仅为卡型字符串，不含卡数）
- [x] 4.4 在 `gpu_lookup.go` 新增 `loadGcpGpuInstancePrefixes(kt)`：从 `global_config` 加载 `gcp_gpu_instance_prefixes`，返回 `map[string]string`（前缀→短卡型名）；缺失返回空 map（不阻断），解析失败记录 Warnf 并返回错误
- [x] 4.5 在 `gpu_lookup.go` 新增 `lookupGcpGpuCardCategory(skuDescription, gcpGpuPrefixes)`：先调 L1 关键词识别，未命中再走 L2 前缀（忽略大小写 + 词边界 + 最长前缀优先）；均未命中返回空字符串
- [x] 4.6 改造 `isGcpGPU`：删除 `calendar mode` 判定，改为「`lookupGcpGpuCardCategory` 非空 OR `HcProductName` 含 `_HCM_AI_` 前缀 OR `IsAIBillItem(skuDescription)` 兜底」；更新函数签名传入 `gcpGpuPrefixes`
- [x] 4.7 删除 `pkg/criteria/constant/bill.go` 中不再被引用的 `GcpCalendarMode` 常量

## 5. AWS OBS sync 填充两字段

- [x] 5.1 在 `cmd/task-server/logics/action/obs/sync/sync_aws.go` 的 `convertAwsBill` 中，更新 `awsGpuSet` 形参类型为 `map[string]string`，为每条记录填充 `GpuCardCategory`（查 `record.ProductInstanceType` 命中取卡型、未命中含 SageMaker/`_HCM_AI_` 前缀场景留空）与 `APIBrandName`（`MatchAPIBrandName(item.HcProductName)`）

## 6. GCP OBS sync 填充 APIBrandName 与 GpuCardCategory

- [x] 6.1 在 `cmd/task-server/logics/action/obs/sync/sync_gcp.go` 的 `convertGcpBill` 中，为每条记录填充 `APIBrandName`：优先 `MatchAPIBrandName(item.HcProductName)`，结果为空时兜底 `MatchAPIBrandName(skuDescription)`（与 `isGcpGPU` 双路径对称）
- [x] 6.2 在 `doSyncGcpBillItem` 批次开始时调用 `loadGcpGpuInstancePrefixes` 一次性加载前缀配置，透传至 `convertGcpBill`
- [x] 6.3 在 `convertGcpBill` 中将 `GpuCardCategory` 由留空改为 `lookupGcpGpuCardCategory(skuDescription, gcpGpuPrefixes)`，并将 `isGcpGPU` 调用更新为传入 `gcpGpuPrefixes`

## 7. 华为 OBS sync（两字段留空）

- [x] 7.1 确认 `cmd/task-server/logics/action/obs/sync/sync_huawei.go` 的 `convertHuaweiBill` 中 `GpuCardCategory`、`APIBrandName` 均保持空字符串（结构体默认值，无需赋值），如有需要补充注释说明本期留空原因

## 8. 单元测试

- [x] 8.1 为 `MatchAPIBrandName` 编写单测：单一品牌命中、大小写混合、多命中取首个、`veo`/`imagen`/`lyria` 归并 `gemini`、未命中返回空、子串不误判（如 `claudexx`）
- [x] 8.2 为 `loadAwsGpuInstanceTypes`/`isAwsGPU`/取卡型逻辑编写单测：对象格式解析、命中取卡型、未命中留空、配置缺失视为空 map、解析失败返回错误
- [x] 8.3 为三厂商 convert 函数补充/更新单测：AWS 卡型+品牌填充、GCP 品牌优先 HcProductName 兜底 SkuDescription、华为两字段留空
- [x] 8.4 为 GCP 卡型识别编写单测：L1 各关键词命中（含 `RTX 6000`/`RTX Pro 6000`→`RTX6000PRO`、`V100`/`P100`/`P4`/`K80`）、L1 词边界防误判（如 `A1000` 不命中 `A100`、随机串不命中 `L4`/`P4`）、L2 前缀命中（`G4`→`RTX6000PRO`）、A3/A3Ultra 最长前缀优先、T4 不识别、L2 配置缺失仅 L1 生效、均未命中留空
- [x] 8.5 为改造后的 `isGcpGPU` 编写单测：卡型命中为 true、AI 前缀为 true、AI 兜底为 true、仅含 `calendar mode` 为 false

## 9. 验证与上线协同

- [ ] 9.1 本地 `go build ./...` 与相关包 `go test` 通过
- [ ] 9.2 按迁移计划顺序验证：先执行 SQL 迁移 → 刷新 `aws_gpu_instance_types` 为对象格式 → 新增 `gcp_gpu_instance_prefixes` 配置 → 部署 task-server → 触发/等待 6 月账单同步，抽样校验两字段按规则填充（重点抽查 GCP 卡型）
