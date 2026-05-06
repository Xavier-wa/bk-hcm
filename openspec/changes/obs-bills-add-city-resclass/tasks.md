## 1. 数据库迁移

- [x] 1.1 新建 `scripts/obssql/9999_xxx_obs.sql`：为 obs_aws_bills、obs_huawei_bills、obs_gcp_bills 三张表执行 ALTER TABLE ADD COLUMN `CityId` int(11) NOT NULL DEFAULT 0 和 ADD COLUMN `ResClassId` int(11) NOT NULL DEFAULT 0
- [x] 1.2 同一 SQL 文件中 CREATE TABLE `account_bill_region_city_rel`（含 id、region、vendor、city_id、creator、reviser、created_at、updated_at，UNIQUE KEY `idx_vendor_region`(`vendor`,`region`)）

## 2. 枚举与常量

- [x] 2.1 在 `pkg/criteria/enumor/global_config.go` 新增 GlobalConfigTypeAccountBill（config_type="account_bill"）、GlobalConfigKeyAwsGpuInstanceTypes、GlobalConfigKeyHuaweiGpuInstancePrefixes 枚举值
- [x] 2.2 在 `pkg/criteria/enumor/bill.go` 新增 OBSResClassID 类型及枚举常量（AwsCPU=451、AwsGPU=6311、GcpCPU=601、GcpGPU=6312、HuaweiCPU=1244、HuaweiGPU=6315），新增 GetOBSResClassID(vendor, isGPU) 函数
- [x] 2.3 在 `pkg/criteria/enumor/bill.go` 的 `getAIBillItemAIFlag()` 中补充关键词：kimi、jina、veo、imagen、lyria
- [x] 2.4 在 `pkg/criteria/constant/bill.go` 新增 OBSDefaultCityIDChina=300001、OBSDefaultCityIDOverseas=300002 常量

## 3. account_bill_region_city_rel 表层（table / DAO）

- [x] 3.1 在 `pkg/dal/table/table.go` 新增 AccountBillRegionCityRelTable = "account_bill_region_city_rel"，并加入 ValidTable map
- [x] 3.2 新建 `pkg/dal/table/bill/account_bill_region_city_rel.go`：定义 AccountBillRegionCityRel 结构体、ColumnDescriptor、TableName、InsertValidate、UpdateValidate
- [x] 3.3 新建 `pkg/dal/dao/bill/account_bill_region_city_rel.go`：实现 AccountBillRegionCityRelDao（含 CreateWithTx、List、UpdateByIDWithTx、DeleteWithTx），定义 Interface 接口
- [x] 3.4 在 `pkg/dal/dao/dao.go` 的 DaoSet 中注册 AccountBillRegionCityRel() 方法

## 4. account_bill_region_city_rel API 层

- [x] 4.1 新建 `pkg/api/core/bill/account_bill_region_city_rel.go`：定义 AccountBillRegionCityRel 核心类型
- [x] 4.2 新建 `pkg/api/data-service/bill/account_bill_region_city_rel.go`：定义 CreateReq、ListResult、UpdateReq 等请求/响应结构体
- [x] 4.3 新建 `cmd/data-service/service/bill/account_bill_region_city_rel.go`：实现 CRUD Handler（BatchCreate、List、Update、BatchDelete），注册到 data-service 路由
- [x] 4.4 新建 `pkg/client/data-service/bill/account_bill_region_city_rel.go`：实现 client 方法，通过 `pkg/client/common/request.go` 封装调用

## 5. OBS 账单表 Go 层结构体更新

- [x] 5.1 在 `pkg/dal/table/obs/bill_aws.go` 的 OBSBillItemAwsColumnDescriptor 和 OBSBillItemAws 结构体中新增 CityId（db tag: "CityId"，int32）和 ResClassId（db tag: "ResClassId"，int32）字段
- [x] 5.2 在 `pkg/dal/table/obs/bill_huawei.go` 的对应位置新增 CityId 和 ResClassId 字段
- [x] 5.3 在 `pkg/dal/table/obs/bill_gcp.go` 的对应位置新增 CityId 和 ResClassId 字段

## 6. OBS sync 辅助逻辑

- [x] 6.1 新建 `cmd/task-server/logics/action/obs/sync/city_lookup.go`：实现 loadRegionCityMap（从 data-service 加载指定 vendor 的地域-城市映射到 map[string]int32）和 lookupCityID（map 查找 + 站点兜底 + Warn 日志）函数
- [x] 6.2 新建 `cmd/task-server/logics/action/obs/sync/gpu_lookup.go`：实现 loadAwsGpuInstanceTypes（从 global_config 加载 AWS GPU 机型 set）、loadHuaweiGpuPrefixes（加载 HW GPU 前缀 slice）、isAwsGPU、isGcpGPU、isHuaweiGPU 判断函数，以及 extractSpecPrefix 函数

## 7. AWS OBS sync 改造

- [x] 7.1 在 `cmd/task-server/logics/action/obs/sync/sync_aws.go` 的 doSyncAwsBillItem 中，批量初始化前调用 loadRegionCityMap 和 loadAwsGpuInstanceTypes，将结果传入 convertAwsBill
- [x] 7.2 更新 convertAwsBill 函数签名，接收 regionCityMap 和 awsGpuSet 参数，为每条记录填充 CityId（通过 product_region）和 ResClassId

## 8. GCP OBS sync 改造

- [x] 8.1 在 `cmd/task-server/logics/action/obs/sync/sync_gcp.go` 的 doSyncGcpBillItem 中，批量初始化前调用 loadRegionCityMap，将结果传入 convertGcpBill
- [x] 8.2 更新 convertGcpBill 函数签名，接收 regionCityMap 参数，为每条记录填充 CityId（通过 Region 字段）和 ResClassId（通过 isGcpGPU 判断 SkuDescription 和 hc_product_name）

## 9. HuaWei OBS sync 改造

- [x] 9.1 在 `cmd/task-server/logics/action/obs/sync/sync_huawei.go` 的 doSyncHuaweiBillItem 中，批量初始化前调用 loadRegionCityMap 和 loadHuaweiGpuPrefixes，将结果传入 convertHuaweiBill
- [x] 9.2 更新 convertHuaweiBill 函数签名，接收 regionCityMap 和 hwGpuPrefixes 参数，为每条记录从 `item.Extension.ResFeeRecordV2.ProductSpecDesc` 提取规格前缀判断 GPU，并填充 CityId（通过 Region 字段）和 ResClassId
