## Why

主机申领流程中压测子网类型的可选 VPC/子网列表当前只能硬编码，运营人员无法动态调整。为支持压测场景的弹性配置，需要提供一套可管理的 API，允许运营人员维护各地域下的压测专用 VPC 和子网映射。

## What Changes

- **新增**：woa-server 新增 Upsert 接口，允许运营管理员全量替换压测子网配置（region → vpc_id → [subnet_id] 三层映射）
- **新增**：woa-server 新增 List 接口，返回完整的压测子网映射供前端按地域过滤展示
- **新增**：`pkg/criteria/enumor/global_config.go` 中新增 `GlobalConfigTypeCvmApply`（`cvm_apply`）枚举值及对应 key 类型
- **存储**：复用现有 `global_config` 表，以 `config_type=cvm_apply`、`config_key=load_test_subnet` 存储配置，不新建表

## Capabilities

### New Capabilities

- `load-test-subnet-config`：压测子网配置管理能力，包含 Upsert（全量替换）和 List（查询完整映射）两个接口，通过 woa-server 对外暴露，内部调用 data-service 的 global_config CRUD 接口访问 DB

### Modified Capabilities

（无现有能力的需求变更）

## Impact

- **新增接口**：
  - `POST /api/v1/woa/config/load_test_subnets/upsert`（需 GlobalConfig.Create 权限）
  - `GET /api/v1/woa/config/load_test_subnets`（无需权限）
- **修改文件**：`pkg/criteria/enumor/global_config.go`、`cmd/woa-server/logics/config/logics.go`、`cmd/woa-server/service/config/service.go`
- **新增文件**：woa-server types/logics/service 各一个文件，接口文档两份
- **无 breaking change**：全新接口，不影响现有功能
