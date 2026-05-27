## Why

业务申领自研云主机时，只能选择公共镜像。然而业务方需要使用自己构建的私有镜像，以确保 OS 版本统一——无论什么时间段申领的机器，OS 版本都是一致的。云上公共镜像的同一个镜像 ID，不同时间段对应的内核版本可能不同，无法满足业务版本锁定需求。

**核心问题**：
- 同步仅过滤公共镜像（`image-type=PUBLIC_IMAGE`），私有镜像未入库
- `pkg/adaptor/tcloud/image.go` 中镜像类型硬编码为 `"public"`，无法区分公共/私有
- 无业务维度镜像查询接口
- 无私有镜像业务归属配置能力
- 腾讯云镜像不支持标签，需要本地补充 biz_id

## What Changes

### 方案：修改同步逻辑 + 镜像表增加业务标签 + 新增业务维度接口

1. **修复 Adaptor 层硬编码**：`pkg/adaptor/tcloud/image.go` 中 Type 字段改为读取云端实际类型 `*pImage.ImageType`
2. **修改同步过滤条件**：同步条件增加 `PRIVATE_IMAGE` 类型，支持同步私有镜像
3. **数据库增加字段**：镜像表增加 `biz_id` 字段，标记私有镜像的业务归属
4. **新增接口：给镜像打业务标签**：平台审核通过后，给镜像配置业务归属（本地补充 biz_id）
5. **新增业务维度镜像查询接口**：新增接口，支持按业务 ID 过滤私有镜像

### 规则与处理逻辑

- **私有镜像**：支持同步入库，按业务 ID 配置可见范围
- **公共镜像**：原有逻辑不变，所有业务均可正常使用
- **未配置业务标签的私有镜像**：不对外展示
- **已绑定业务标签的私有镜像**：仅对应业务可在申领时选择

## Capabilities

### New Capabilities

- `ziyan-private-image-sync`: 自研云同步时支持同步私有镜像的能力
- `image-biz-tag-mgmt`: 给镜像打业务标签的管理能力（本地补充 biz_id）
- `biz-cvm-image-query`: 业务维度的镜像查询接口，返回公共镜像 + 该业务的私有镜像

### Modified Capabilities

- `ziyan-image-sync`: 同步条件扩展，支持 `PRIVATE_IMAGE` 类型
- `tcloud-image-adaptor`: 镜像类型读取云端实际值，不再硬编码

## Impact

- **涉及服务层**：
  - hc-service（同步逻辑）
  - data-service（数据访问、打标签接口）
  - woa-server（业务维度镜像查询接口）

- **代码影响**：
  - `pkg/adaptor/tcloud/image.go` - 修复 Type 硬编码，`"public"` → `*pImage.ImageType`
  - `cmd/hc-service/logics/res-sync/ziyan/image.go` - 同步条件增加 `PRIVATE_IMAGE`
  - `pkg/dal/table/cloud/image/image.go` - 镜像表增加 `biz_id` 字段
  - `pkg/api/data-service/cloud/image/` - 打标签接口请求/响应结构
  - `cmd/data-service/service/cloud/image/` - 新增镜像业务标签管理接口
  - `cmd/woa-server/service/` - 新增业务维度镜像查询接口

- **数据库影响**：
  ```sql
  ALTER TABLE image ADD COLUMN biz_id BIGINT DEFAULT NULL COMMENT '所属业务ID，仅私有镜像使用';
  CREATE INDEX idx_image_biz_id ON image(biz_id);
  ```

- **新增接口**：
  | 接口 | 方法 | 说明 |
  |------|:----:|------|
  | `/api/v1/woa/bizs/{bk_biz_id}/config/cvm/image` | POST | 业务维度镜像查询，返回公共镜像 + 该业务私有镜像 |
  | `/api/v1/cloud/images/{id}/biz_tag` | PUT | 给镜像打业务标签（本地补充 biz_id） |

- **查询逻辑**：
  ```sql
  -- 业务维度镜像查询
  SELECT * FROM image 
  WHERE vendor = 'tcloud-ziyan' 
    AND region = ?
    AND (type = 'PUBLIC_IMAGE' OR (type = 'PRIVATE_IMAGE' AND biz_id = ?))
  ```

- **兼容性**：
  - 无外部 API 破坏性变更，向后兼容
  - 原有接口保持不变，新增业务维度接口
  - 私有镜像默认 `biz_id` 为空，不展示给任何业务
