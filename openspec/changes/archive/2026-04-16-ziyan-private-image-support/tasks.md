## 1. Adaptor 层镜像类型修复

- [x] 1.1 修改 `pkg/adaptor/tcloud/image.go` 第 79 行，将硬编码的 `"public"` 改为 `*pImage.ImageType`，读取云端实际镜像类型

## 2. 同步条件扩展

- [x] 2.1 修改 `cmd/hc-service/service/sync/tcloud-ziyan/image.go` 的 `Next()` 方法，在 `ImageType` 数组中增加 `PRIVATE_IMAGE` 类型
- [x] 2.2 在 `pkg/criteria/enumor/image.go` 中确认 `TCloudPrivateImage` 枚举值存在（已存在）

## 3. 镜像表 biz_id 字段

- [x] 3.1 修改 `pkg/dal/table/cloud/image/image.go` 的 `ImageModel` 结构体，新增 `BizID int64` 字段（db tag: `biz_id`, json tag: `biz_id`）
- [x] 3.2 修改 `pkg/dal/table/cloud/image/image.go` 的 `ImageColumnDescriptor`，增加 `biz_id` 列描述
- [x] 3.3 编写数据库变更 SQL：`scripts/sql/0076_20260416_1630_image_biz_id.sql`
- [x] 3.4 修改 `pkg/api/core/cloud/image/image.go` 的 `BaseImage` 结构体，新增 `BizID`字段
- [x] 3.5 修改 `cmd/data-service/service/cloud/image/conv.go` 的 `toProtoImageExtResult()` 和 `toProtoImageResult()` 函数，添加 `BizID` 字段映射

## 4. 镜像业务标签接口（data-service）

- [x] 4.1 在 `pkg/api/data-service/cloud/image/request.go` 下定义打标签接口请求结构 `UpdateImageBizTagReq`，包含 `biz_id` 字段
- [x] 4.2 在 `cmd/data-service/service/cloud/image/update.go` 中实现 `UpdateImageBizTag` 方法
- [x] 4.3 接口实现：校验镜像 ID 存在、校验镜像类型为 `PRIVATE_IMAGE`、使用事务更新 `biz_id` 字段
- [x] 4.4 在 `cmd/data-service/service/cloud/image/image.go` 中注册路由

## 5. 业务维度镜像查询接口（woa-server）

- [x] 5.1 在 `cmd/woa-server/types/config/types.go` 下定义业务维度镜像查询请求/响应结构
- [x] 5.2 在 `cmd/woa-server/service/config/cvm_image.go` 下新增业务维度镜像查询 Handler `GetBizCvmImage`
- [x] 5.3 在 `cmd/woa-server/logics/config/cvm_image.go` 下实现查询逻辑
- [x] 5.4 在 `cmd/woa-server/service/config/service.go` 注册路由

## 6. 单元测试

- [x] 6.1 为 `pkg/adaptor/tcloud/image.go` 的 `changeArchitecture` 函数编写单元测试（`pkg/adaptor/tcloud/image_test.go`）
- [x] 6.2 为业务维度镜像查询参数 `GetBizCvmImageParam.Validate()` 和 `BatchOpImageToApplyCVMReq.Validate()` 编写单元测试（`cmd/woa-server/types/config/image_test.go`）
- [x] 6.3 为镜像业务标签请求 `UpdateImageBizTagReq.Validate()` 编写单元测试（`pkg/api/data-service/cloud/image/request_test.go`）
- [x] 6.4 所有单元测试通过

## 7. 集成验证

- [x] 7.1 编译验证：`go build` 通过
- [x] 7.2 代码格式化：`gofmt -w` 完成
- [ ] 7.3 部署到测试环境验证同步逻辑（待部署）
- [ ] 7.4 端到端接口验证（待部署）
