# 资源运营一级视图（容器·待拆解）

> status: drafted · kind: module
> globs: `src/views/resource/**`
> ⚠️ 这不是原子模块，而是一级视图 `/resource/` 的**菜单容器**（体量最大）——「按菜单笼统归一」的现状刻画。原子原则要打破它。

## 现状（聚合了什么）

- `accountmanage/` — 账号管理 → 应拆为账号管理原子模块。
- `recyclebin-manager/` — 回收站 → 应拆为 recyclebin 原子模块。
- `resource-manage/` — 资源纳管：内部含 host/vpc/subnet/安全组/cvm 等**多种 IaaS 资源类型**（`resource-manage/children/detail/*`），每种资源类型都是一个潜在原子模块。
- 安全组列表页 `resource-manage/children/manage/security-manage.vue` 由业务视角与资源接入共用。三个 tab 都用 `ResourceSearchSelect`（`option-common.ts` 只登记字段和 children）；`type` / `filterRules` 与默认回填由页面挂到 URL `filter` + `useSearchQs`。业务视角安全组 tab 通过 `exclude` 去掉使用业务/管理业务。GCP 防火墙条件不含云厂商，与资源接入一致。
- `NoPermission.tsx` — 旧无权限页（已废弃，见 auth 模块，改用路由守卫 + 权限页）。

## 目标形态

账号管理、回收站、各 IaaS 资源类型各自**打平为原子模块**；资源纳管本身可能保留为「多资源类型」的聚合入口，但各资源类型的字段/列表/详情应走 [model](model.md) 的多云/多资源 Factory 模式。散落对照见 [base](base.md) 的「逻辑特性映射表」与「一级视图容器」表。

## 注意事项

- **只迁不增**：新资源/新功能不要继续无序堆进容器。
- IaaS 资源的列表/详情/表单遵循 [model](model.md) 的字段模型 + 通用组件模式。
- 改造老代码不删老文件，新建迁移并加 `@deprecated`。
