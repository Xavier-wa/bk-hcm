## Context

当前自研云(Ziyan)镜像同步仅同步公共镜像(`PUBLIC_IMAGE`)，私有镜像未入库。业务方申领自研云主机时只能选择公共镜像，无法使用自己构建的私有镜像。

**核心问题**：
1. 同步逻辑硬编码过滤条件 `image-type=PUBLIC_IMAGE`，私有镜像不会同步入库
2. `pkg/adaptor/tcloud/image.go` 中镜像类型 Type 字段硬编码为 `"public"`，无法区分公共/私有镜像
3. 镜像表无业务归属字段(`biz_id`)，无法标记私有镜像所属业务
4. 无业务维度镜像查询接口，业务方无法获取可用的私有镜像列表

**业务场景**：业务方需要使用自己构建的私有镜像，以确保 OS 版本统一——无论什么时间段申领的机器，OS 版本都是一致的。云上公共镜像的同一个镜像 ID，不同时间段对应的内核版本可能不同，无法满足业务版本锁定需求。

## Goals / Non-Goals

**Goals:**
- 修复 Adaptor 层镜像类型硬编码问题，读取云端实际类型
- 扩展同步逻辑，支持同步 `PRIVATE_IMAGE` 类型的镜像
- 镜像表增加 `biz_id` 字段，用于标记私有镜像的业务归属
- 新增镜像业务标签管理接口，支持给私有镜像配置业务归属
- 新增业务维度镜像查询接口，返回公共镜像 + 该业务的私有镜像

**Non-Goals:**
- 共享镜像支持（仅支持公共镜像和私有镜像）
- 私有镜像自动发现业务归属（需要平台管理员手动配置）

## Decisions

### D1: Adaptor 层镜像类型修复

**方案**：`pkg/adaptor/tcloud/image.go` 的 `ListImage` 函数中，将 Type 字段从硬编码改为读取云端实际返回的 `ImageType`。

**修改内容**：

```go
// 修改前
Type: "public",

// 修改后
Type: *pImage.ImageType,
```

**理由**：`ImageType` 字段由云端返回，可能为 `PUBLIC_IMAGE`、`PRIVATE_IMAGE` 或 `SHARED_IMAGE`，应该保留原始值以便后续业务逻辑判断。

### D2: 同步条件扩展

**方案**：`cmd/hc-service/service/sync/tcloud-ziyan/image.go` 的 `Next` 方法中，扩展 `image-type` 过滤条件，同时包含 `PUBLIC_IMAGE` 和 `PRIVATE_IMAGE`。

**修改内容**：

```go
// 修改前
Filters: []image.TCloudImageFilter{
    {
        Name:   "image-type",
        Values: common.StringPtrs([]string{string(enumor.TCloudPublicImage)}),
    },
},

// 修改后
Filters: []image.TCloudImageFilter{
    {
        Name: "image-type",
        Values: common.StringPtrs([]string{
            string(enumor.TCloudPublicImage),
            string(enumor.TCloudPrivateImage),
        }),
    },
},
```

**理由**：通过扩展过滤条件，一次性同步公共镜像和私有镜像，避免多次 API 调用。

### D3: 数据库变更 — 镜像表增加 biz_id 字段

**方案**：在 `pkg/dal/table/cloud/image/image.go` 的 `ImageModel` 中新增 `BizID` 字段，并提供对应的 SQL 变更脚本。

**表结构变更**：

```go
type ImageModel struct {
    // ... 原有字段 ...
    BizID int64 `db:"biz_id" json:"biz_id"`  // 新增
}
```

**SQL 脚本**（`scripts/sql/9999_20260416_1630_image_biz_id.sql`）：

```sql
ALTER TABLE `image` ADD COLUMN `biz_id` bigint(1) DEFAULT NULL 
    COMMENT '业务ID，仅私有镜像使用' AFTER `os_type`;
ALTER TABLE `image` ADD INDEX `idx_biz_id` (`biz_id`);
```

**理由**：
- 使用 `bigint` 类型与 BKCC 业务 ID 保持一致
- 默认值为 `NULL`，表示未配置业务归属
- 添加索引以支持按业务查询场景

### D4: API 结构定义 — BaseImage 新增 BizID 字段

**方案**：在 `pkg/api/core/cloud/image/image.go` 的 `BaseImage` 结构体中新增 `BizID` 字段。

```go
type BaseImage struct {
    // ... 原有字段 ...
    BizID int64 `json:"biz_id"`   // 新增：业务ID
}
```

**转换函数更新**（`cmd/data-service/service/cloud/image/conv.go`）：

```go
func convTableToBaseBill(table *tableimage.ImageModel) *coreimage.BaseImage {
    return &coreimage.BaseImage{
        // ... 原有字段 ...
        BizID: table.BizID,   // 新增映射
    }
}
```

### D5: data-service 接口 — 镜像业务标签管理

**方案**：在 `cmd/data-service/service/cloud/image/update.go` 中新增 `UpdateImageBizTag` 接口。

**接口定义**：

- 路由：`PUT /api/v1/cloud/images/{id}/biz_tag`
- 请求结构（`pkg/api/data-service/cloud/image/request.go`）：

```go
type UpdateImageBizTagReq struct {
    BizID int64 `json:"biz_id" validate:"required"`
}
```

**校验规则**：

1. 镜像 ID 必须存在
2. 仅私有镜像（`type=PRIVATE_IMAGE`）可设置 `biz_id`
3. `biz_id` 必须大于 0

**理由**：通过独立接口管理业务标签，与批量更新接口解耦，便于权限控制和审计。

### D6: woa-server 接口 — 业务维度镜像查询

**方案**：在 `cmd/woa-server/service/config/` 下新增业务维度镜像查询接口。

**接口定义**：

- 路由：`POST /api/v1/woa/bizs/{bk_biz_id}/config/cvm/image`
- 请求参数（`cmd/woa-server/types/config/types.go`）：

```go
type GetBizCvmImageParam struct {
    Vendor string `json:"vendor" validate:"required"`
    Region string `json:"region" validate:"required"`
}
```

- 响应结构：

```go
type BizCvmImage struct {
    ID        string `json:"id"`
    CloudID   string `json:"cloud_id"`
    Name      string `json:"name"`
    Type      string `json:"type"`
    Platform  string `json:"platform"`
    OsType    string `json:"os_type"`
    BizID     int64  `json:"biz_id"`
    Region    string `json:"region"`
    EnableCvm bool   `json:"enable_cvm"`
}

type GetBizCvmImageResult struct {
    Count int64          `json:"count"`
    Info  []*BizCvmImage `json:"info"`
}
```

**查询逻辑**（`cmd/woa-server/logics/config/cvm_image.go`）：

```sql
SELECT * FROM image 
WHERE vendor = ? 
  AND region = ?
  AND enable_cvm = true
  AND (type = 'PUBLIC_IMAGE' OR (type = 'PRIVATE_IMAGE' AND biz_id = ?))
```

**理由**：
- 公共镜像对所有业务可见
- 私有镜像仅对配置了 `biz_id` 的业务可见
- 未配置 `biz_id` 的私有镜像不对外展示

## Risks / Trade-offs

- **[数据库变更兼容性]** → `biz_id` 字段默认为 `NULL`，旧数据无影响。同步入库的私有镜像默认 `biz_id=NULL`，不会对任何业务展示，需要管理员手动配置。

- **[私有镜像可见性]** → 当前方案依赖管理员手动配置业务归属。若后续需要自动识别，可基于云端镜像的项目归属进行扩展。

- **[镜像删除同步]** → 云端私有镜像删除后，本地记录会通过 `RemoveDeleteFromCloud` 逻辑清理，已配置的 `biz_id` 会随记录一同删除。

- **[多业务共享]** → 当前 `biz_id` 为单一值，不支持一个私有镜像对多个业务可见。若有此需求，需改为关联表设计。当前场景下单业务归属已满足需求。
