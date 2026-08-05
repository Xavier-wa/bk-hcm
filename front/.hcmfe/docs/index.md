# hcm 项目文档导航 (Tier A)

> 由 bkdevbuddy docs 维护。动代码前先在这里定位相关模块，再读对应 `modules/<id>.md`。
> 深度随 workflow 逐步补齐（stub → drafted → verified）。

## 基础

- [基础/共享接缝](modules/base.md) — 跨模块共享层：HTTP 客户端、API 封装、全局组件、hooks、store、常量/类型、样式与工具函数等。已拆出 model/menu-route/auth 三个基础能力模块。【架构北极星、逻辑特性→物理落点映射表沉淀在本模块文档，index 仅为工具生成的模块 TOC，动手前先读 base.md】 · `drafted`  (relates: menu-route, auth)

## 模块

- [组件级多云与内外版差异化](modules/component-variant.md) — 系统基础特色能力：单组件同时应对多云（运行时 Factory + vendor 属性映射）与内外版（构建时 .plugin ↔ -internal.plugin 模块替换，由 bk.config.js NormalModuleReplacementPlugin 在 TARGET_MODE=bcc 时切换）。典范：components/account-selector。新代码内外版差异优先用文件级 .plugin 替换而非退场中的 plugin-handler。 · `drafted`  (relates: model, plugin-handler, page-variant)
- [页面级内外版差异化](modules/page-variant.md) — 系统基础特色能力：与 component-variant 同一 .plugin 构建替换机制，但替换整块 UI 片段——Vue SFC(.plugin.vue) 或 TSX 渲染函数(.plugin.tsx)。内版常在外版基础上加流程/分支（如 MOA 校验、bpaas 来源）。典范：security-group single-delete-button、ticket apply-content-render。 · `drafted`  (relates: component-variant, plugin-handler)
- [菜单与路由](modules/menu-route.md) — 横切关注点（从 base 拆出）：三级一级视图（业务/工作台/资源运营）、去中心化路由、菜单与路由解耦、routerAction 统一跳转。关键文件 src/router/**、src/constants/menu-symbol.ts；各模块路由分散在 views/<模块>/route-config.ts。 · `drafted`  (relates: base, auth)
- [权限控制](modules/auth.md) — 横切关注点（从 base 拆出）：视图级权限（路由守卫 + auth.view）与操作级权限（HcmAuth 组件）。关键文件 src/common/auth-service.ts、src/constants/auth-symbols.ts、src/components/auth/**、src/components/permission/**。旧 useVerify/PermissionDialog 方案已废弃。 · `drafted`  (relates: base, menu-route)
- [操作记录](modules/operation-log.md) — 操作审计日志。目标原子模块的正面样板：功能自洽、菜单挂载与模块归属解耦。 · `drafted`
- [资源运营一级视图（容器·待拆解）](modules/resource.md) — 一级视图 /resource/ 的菜单容器，非原子模块，体量最大。现状聚合了账号管理(accountmanage)、回收站(recyclebin-manager)、资源纳管(resource-manage：host/vpc/subnet/安全组/cvm 等多种 IaaS 资源)。目标：账号管理、回收站、各 IaaS 资源类型各自打平为原子模块。详见 base 模块「一级视图容器」表。 · `drafted`
- [业务一级视图（容器·待拆解）](modules/business.md) — 一级视图 /business/:bizId 的菜单容器，非原子模块。现状聚合了业务管理自身 + 主机/证书/负载均衡/资源预测/滚服等功能，是「按菜单笼统归一」的现状刻画。目标：这些功能各自打平为原子模块，容器仅留业务管理。详见 base 模块「一级视图容器」表。 · `drafted`  (relates: load-balancer, resource-plan, rolling-server)
- [自研上云（退场中）](modules/ziyan-scr.md) — 状态: 退场中。历史组织形式，内部混杂 cvm/主机申请/回收/滚动资源等。原则: 只迁不增，新需求就近迁往对应原子模块。 · `drafted`  (relates: rolling-server, resource)
- [工作台一级视图（容器·待拆解）](modules/service.md) — 一级视图 /service/ 的菜单容器，非原子模块。现状聚合了我的申请、我的审批、服务申请 + 资源预测。目标：申请审批(服务单据)成原子模块；resource-plan 已独立。详见 base 模块「一级视图容器」表。 · `drafted`  (relates: resource-plan)
- [云账号管理](modules/cloud-account-manage.md) — 云账号密钥、权限策略/模板、二级/三级账号管理。 · `drafted`
- [负载均衡](modules/load-balancer.md) — CLB/监听器/目标组/设备。功能物理分散在顶层与 business 下，目标收敛为单一原子模块。物理落点见 base 模块逻辑特性映射表。 · `drafted`  (relates: business)
- [账单](modules/bill.md) — 账单与账号账单视图。 · `drafted`
- [单据/工单](modules/ticket.md) — 工单流程，含业务/服务两套入口。 · `drafted`
- [资源选型方案](modules/scheme.md) — 选型方案列表/详情/推荐。 · `drafted`
- [任务管理](modules/task.md) — 异步任务（cvm/clb 等）列表与详情。 · `drafted`
- [交付统计](modules/stats.md) — 交付相关统计报表。 · `drafted`
- [滚服](modules/rolling-server.md) — 滚服：账单/配额/用量。功能物理分散在顶层、business、ziyanScr 下，目标收敛为单一原子模块。物理落点见 base 模块逻辑特性映射表。 · `drafted`  (relates: business, ziyan-scr)
- [资源预测](modules/resource-plan.md) — 资源预测（GPU 等资源用量/容量预测）。物理分散在顶层、business、service 下，目标收敛为单一原子模块。物理落点见 base 模块逻辑特性映射表。 · `drafted`  (relates: business, service)
- [小额绿通](modules/green-channel.md) — 小额绿通相关页面。 · `drafted`
- [智能助手](modules/chatbot.md) — 对话式助手入口。 · `drafted`
- [门户框架/首页/错误页/通知](modules/app-shell.md) — 应用外壳与通用页面：首页布局(home 已废弃)、错误页、全局通知、views 根入口。 · `drafted`
- [模型驱动与页面模式](modules/model.md) — 系统基础特色能力：装饰器字段元数据模型（@Model/@Column + getModel，一次声明多场景投影）、多云 Factory 编码模式、展示/表单/列表三场景通用组件模式。分步 how-to 见 skill page-list/page-form/page-detail。 · `drafted`  (relates: menu-route, auth)
- [内外版版本模式（退场中）](modules/plugin-handler.md) — 状态: 退场中。承载内外版（内部版/外部版）版本模式差异的历史插件式组织形式。原则: 只迁不增，新需求改用现行组件/hooks 方案。 · `drafted`
