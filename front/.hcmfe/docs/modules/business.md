# 业务一级视图（容器·待拆解）

> status: drafted · kind: module
> globs: `src/views/business/**`
> ⚠️ 这不是原子模块，而是一级视图 `/business/:bizId` 的**菜单容器**——「按菜单笼统归一」的现状刻画。原子原则要打破它。

## 现状（聚合了什么）

容器内混放了多个本应独立的功能：

- `business-manage.vue` / `business-detail.vue` — 业务管理自身（容器的合法职责）。
- `host/`、`host-inventory/` — 主机 / 主机清单 → 应拆为 host 原子模块。
- `cert-manager/` — 证书 → 应拆为 cert 原子模块。
- `load-balancer/` → 归 [load-balancer](load-balancer.md) 原子模块。
- `resource-plan/` → 归 [resource-plan](resource-plan.md)（资源预测）。
- `rolling-server/` → 归 [rolling-server](rolling-server.md)（滚服）。
- `components/`、`forms/` — 容器内公共件。

## 目标形态

上述功能各自**打平为原子模块**（无论菜单挂在业务/资源/服务哪个视图），本容器最终只保留业务视图入口与业务管理自身。散落对照见 [base](base.md) 的「逻辑特性映射表」与「一级视图容器」表。

## 注意事项

- **只迁不增**：新功能不要继续往容器里堆，应就近落到对应原子模块。
- 改造老代码不删老文件，新建迁移并加 `@deprecated`。
