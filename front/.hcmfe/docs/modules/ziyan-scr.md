# 自研上云（退场中）

> status: stub · kind: module
> globs: `src/views/ziyanScr/**`

状态: 退场中。历史组织形式，内部混杂 cvm/主机申请/回收/滚动资源等。原则: 只迁不增，新需求就近迁往对应原子模块。

## 职责

自研上云（退场中）历史模块，内部混杂 cvm/主机申请/回收/滚动资源等。原则: 只迁不增，新需求就近迁往对应原子模块。

## 关键流程 / 注意事项

- **CVM 机型配置管理**（`src/views/ziyanScr/cvm-model/`）：列表页 + 「创建新机型」弹窗（`CreateDevice/index.tsx`）。
  - 创建表单字段：地域/园区/实例族/机型/机型类型/技术分类/机型分类/核心类型/机型代次/CPU/内存/GPU卡数/GPU卡类型/技术分类资源量。
  - GPU卡类型（`gpu_type`）支持默认枚举 + 自定义输入，枚举来自 `GET /api/v1/woa/meta/gpu_type/list`（`src/api/scrApi/index.tsx` 的 `getGpuTypeList`）。
  - 技术分类资源量（`tech_class_res_amt`）为数值字段（>=0）。
  - 创建接口 `POST /api/v1/woa/config/createmany/config/cvm/device`，`device_types` 数组元素携带 `gpu_type`、`tech_class_res_amt`。
