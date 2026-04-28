## Why

OBS 上报系统需要在账单数据中携带城市 ID（CityId）和资源类型分类 ID（ResClassId），用于 OBS 平台按城市维度统计账单以及区分 GPU/CPU 资源消耗，目前三张 OBS 账单表（obs_aws_bills、obs_huawei_bills、obs_gcp_bills）均缺少这两个字段。

## What Changes

- 在 `obs_aws_bills`、`obs_huawei_bills`、`obs_gcp_bills` 三张表中新增 `CityId`（int11）和 `ResClassId`（int11）字段
- 新增 `account_bill_region_city_rel`（账单地域城市关系）表，存储各云厂商地域到城市 ID 的映射关系，提供完整 CRUD 接口
- OBS 上报时，根据账单地域在 `account_bill_region_city_rel` 中查找对应城市 ID，找不到时按账号站点兜底（国内站 300001，国际站 300002）
- OBS 上报时，根据各厂商规则判断账单项是 GPU 还是 CPU，映射为对应的 `ResClassId` 枚举值（AWS: CPU=451/GPU=6311，GCP: CPU=601/GPU=6312，HuaWei: CPU=1244/GPU=6315）
- GPU 判断配置（AWS GPU 机型列表、华为 GPU 实例前缀列表）存入 `global_config` 表，可运营管理
- 扩充 `IsAIBillItem` 关键词，新增：kimi、jina、Veo、Imagen、Lyria（对所有厂商生效）
- 华为 OBS 上报时新增读取原始账单的 `product_spec_desc` 字段，用于提取规格前缀判断 GPU

## Capabilities

### New Capabilities

- `obs-bill-city-id`: OBS 账单城市 ID 字段——新增地域城市关系表、CRUD 接口及 OBS 上报时的城市 ID 查找与兜底逻辑
- `obs-bill-res-class-id`: OBS 账单资源类型分类 ID 字段——GPU/CPU 判断逻辑（含 global_config 配置）及上报时的 res_class_id 填充

### Modified Capabilities

（无需求层级变更）

## Impact

- **数据库（OBS DB）**：三张 obs 账单表新增 2 个字段；新增 `account_bill_region_city_rel` 表
- **数据库（HCM DB）**：`global_config` 表新增两条 GPU 配置记录
- **pkg/criteria/enumor**：新增 OBSResClassID 枚举、GlobalConfigTypeAccountBill 相关枚举（config_type="account_bill"）；扩充 `IsAIBillItem` 关键词
- **pkg/criteria/constant**：新增 OBS 兜底城市 ID 常量
- **pkg/dal/table/obs**：三张 OBS 账单 table 结构体新增字段
- **pkg/dal/table/bill**：新增 `account_bill_region_city_rel` 表定义
- **pkg/dal/dao/bill**：新增 `account_bill_region_city_rel` DAO
- **cmd/data-service**：新增 `account_bill_region_city_rel` CRUD 接口
- **pkg/client/data-service**：新增对应 client 方法
- **cmd/task-server/logics/action/obs/sync**：三个 sync 文件的 convert 函数改造，新增 city_lookup.go 和 gpu_lookup.go 辅助文件
