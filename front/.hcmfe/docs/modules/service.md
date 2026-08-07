# 工作台一级视图（容器·待拆解）

> status: drafted · kind: module
> globs: `src/views/service/**`
> ⚠️ 这不是原子模块，而是一级视图 `/service/` 的**菜单容器**——「按菜单笼统归一」的现状刻画。原子原则要打破它。

## 现状（聚合了什么）

- `my-apply/` — 我的申请。
- `my-approval/` — 我的审批。
- `service-apply/` — 服务申请。
- `resource-plan/` → 归 [resource-plan](resource-plan.md)（资源预测）。

## 目标形态

申请 / 审批 / 服务申请属同一「服务单据 / 申请审批」域，应**打平为原子模块**（可与 [ticket](ticket.md) 单据域一并考量归属）；resource-plan 已独立。散落对照见 [base](base.md) 的「逻辑特性映射表」与「一级视图容器」表。

## 注意事项

- **只迁不增**：新的申请/审批相关功能就近落到对应原子模块，不要继续堆进容器。
- 改造老代码不删老文件，新建迁移并加 `@deprecated`。
