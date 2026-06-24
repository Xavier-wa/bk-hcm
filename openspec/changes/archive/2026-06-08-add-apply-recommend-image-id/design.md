## Context
离线推荐三元组缺镜像，无法支撑接口1 输出完整子单方案。`image_id` 的唯一真源是 `ziyan_cvm_apply_suborder.image_id`（已存在），`ziyan_cvm_device_info` 通过 `suborder_id` 与其关联但当前无 `image_id` 字段。两表同库（均建于 `scripts/sql/0072_..._cvm_apply.sql`）。

## Goals / Non-Goals
- Goals：三表扩展 `image_id`；离线聚合/在线查询以四元组为最小单元。
- Non-Goals：接口1 的推荐与库存校验逻辑（独立子需求）；离线推荐表产出逻辑本身（兄弟需求）；近似机型推荐。

## Decisions
- **Decision: 增量写入透传 `image_id`。** `order.Spec.ImageId` 已有镜像，在 `buildSingleDeviceInfo` → `types.DeviceInfo` → `CreateDeviceInfos` 链路补字段，新交付设备天然带镜像，回填只需覆盖存量。
- **Decision: 空 `image_id` 视为一类候选。** 历史 suborder 可能无镜像，回填后保持 `''`，四元组唯一键含空串自洽（同三元组下"无镜像"独立成一行）。

## Risks / Trade-offs
- **唯一键变更需重建索引** → 迁移在低峰执行；DDL 与回填 DML 分离，DDL 进迁移文件、回填走工具。
- **四元组导致 TopK 候选膨胀**（同机型多镜像拆多行） → 本次不动 `maxRows=5`，上线后观察再调。
- **回填顺序强约束**（先 device_info 再重跑 cron） → 在 tasks 与运维步骤中显式固化顺序。

## Migration Plan
1. 上线 DDL 迁移（三表加列 + 推荐表改唯一键）。
2. 部署带 `image_id` 透传与四元组聚合的新代码。
3. 触发 `/apply_recommend/sync` 重跑离线 cron，推荐表重建带镜像。
- 回滚：代码可回滚；新列默认 `''` 兼容旧逻辑；唯一键回退需反向 DDL。
