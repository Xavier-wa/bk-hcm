# Coding — feat-aiagent-host-apply-data-disk

**TAPD**: [#1069995598137366343](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137366343)
**需求文档**: `front/docs/reqs/主机申领数据盘可选.md`（本地，gitignored）

## 结论摘要

| 项 | 结论 |
|----|------|
| 改动范围 | **纯前端单文件**：调整配置弹窗组件（见下方「文件」） |
| 后端 | **不改**。提单链路已允许空数据盘（见下方「后端现状核查」） |
| 推荐方案 | **不改**。`recommend.go` 两处硬编码继续塞 1 块 500G 盘 |
| A 卡调整入口 | **不动**。仍保持 `暂不支持调整方案` 注释态 |
| 新增接口 | 无 |

## 执行顺序

1. 数据盘行内增删按钮 + 零块空态入口
2. 总块数上限禁用 + hover 提示，数量输入框上限收敛
3. 样式补齐（行内加减号、空态大加号）+ 弹窗加宽至 800
4. 修复下拉选项因内联箭头函数被重复请求（既有缺陷，本次改动使其暴露）
5. 行内字段校验 + 标签说明 tooltip + 容量上下限，对齐 ziyanScr 申领表单
6. 模块文档补充跨栈边界知识 + 选项组件稳定引用约定

> 排序依据：同文件聚合，1→2 有依赖（上限判断依赖新增入口存在）。

## 后端现状核查（本次改动的前提，无需改动后端）

结论：**后端已允许数据盘为空**，故本需求是纯前端能力缺失。核查点：

| 层 | 位置 | 结论 |
|----|------|------|
| 提单校验 | `cmd/woa-server/types/task/scheduler.go` → `ResourceSpec.ValidateDisk()` | 显式允许 `len(DataDisk) == 0`；总块数区间 `[0, 20]` |
| 请求结构 | `pkg/api/woa-server/cvm_apply_recommend.go` | `DataDisk` 无 `required` / `min=1` 约束 |
| 磁盘规格 | `pkg/criteria/enumor/device.go` → `DiskSpec.Validate()` | 只校验单块属性，不对切片长度设限 |
| 持久化 | `scripts/sql/0072_20260115_1755_cvm_apply.sql` | `data_disk JSON DEFAULT NULL`，可为 NULL |
| 推荐组装 | `cmd/woa-server/service/task/recommend.go` | `assembleStaticPlans` / `buildPlanSuborder` 两处**无条件**塞 1 块 500G 高性能云盘 ← 本次**保留不改** |

上限 20 的来源：`pkg/criteria/constant/ziyan.go` 的 `DataDiskTotalNum`，前端 `MAX_DATA_DISK_NUM` 与之对齐。

## 单据 1: aiagent-主机申领条件允许不挂数据盘

**TAPD**: [#1069995598137366343](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137366343)
**文件**: `src/components/chatbot/host-apply-adjust-dialog.vue`（唯一代码改动）；文档 `.hcmfe/docs/modules/chatbot.md`、`.hcmfe/docs/modules/model.md`
**改动点**: 「调整配置」弹窗的数据盘表单项由「只渲染、不可增删」改为「可删至零块 + 可按需添加 + 到 20 块上限禁用并提示」，交互形态参照自研云申领的数据盘编辑器。

### 根因

调整配置弹窗的「数据盘」表单项只用 `v-for` 遍历 `formModel.data_disk` 渲染三个输入（磁盘类型 / 容量 / 数量），行尾**没有任何操作按钮**，表单项本身也未标记 `required`。而 woa-server 推荐方案无条件带 1 块 500G 盘，两者叠加的结果是：用户看得到那块盘，却既删不掉、也加不了。

### 实施要点

1. **脚本层**（`<script setup>`）
   - 新增 `MAX_DATA_DISK_NUM = 20`，注释标明与后端 `constant.DataDiskTotalNum` 对齐。
   - 新增 `createDataDisk()` 工厂，默认 `CLOUD_PREMIUM / 500 / 1`——与推荐方案默认值一致，使「删掉再加」可还原初始配置（澄清 Q2）。
   - 新增 `dataDiskTotalNum` computed：对各行 `disk_num` 求和，作为上限判断依据（**按总块数**而非行数，与后端 `dataDiskTotalNum` 语义一致）。
   - 新增 `handleAddDataDisk` / `handleRemoveDataDisk`，走 `push` / `splice`，参照自研云申领表单里 cvm-data-disk 组件（views/ziyanScr/components/cvm-data-disk/form.vue）的既有增删模式。
   - 类型从 `@/hooks/chatbot/types` 引入 `HostApplyDisk`；图标从 `bkui-vue/lib/icon` 引入 `Plus`。

2. **模板层**——交互形态**对齐自研云申领的数据盘编辑器**（用户指定参照）
   - 每个数据盘行尾内联两个 `text` 按钮：先加号（`bkhcm-icon-plus-circle-shape`）、后减号（`bkhcm-icon-minus-circle-shape`），浅灰圆形图标。
   - 空态：`v-if="formModel.data_disk.length === 0"` 时行内按钮随行一并消失，故单独渲染一个大加号按钮（`Plus`，24px）兜住「零块后还能加回来」的入口。
   - 上限：加号 `:disabled="dataDiskTotalNum >= MAX_DATA_DISK_NUM"`，并挂 `v-bk-tooltips`（`disabled: dataDiskTotalNum < MAX_DATA_DISK_NUM`，即仅到顶时才提示），文案由 `MAX_DATA_DISK_NUM` 插值以免两处漂移。沿用项目「直接在 disabled 按钮上挂 tooltip、不加 wrapper」的既有写法（参考安全组管理页 security-manage.vue 的批量分配按钮）。
   - 数量输入框同步收上限 `:max="MAX_DATA_DISK_NUM - dataDiskTotalNum + Number(disk.disk_num || 0)"`，与参照组件一致——加号只挡「新增行」，数量输入才是绕过上限的真实入口，两处都要拦。

3. **样式层**：`.ha-disk-action`（行内加减号，浅灰 `#c4c6cc`，`.is-disabled` 态降到 `#eaebf0`）、`.ha-disk-add-icon`（空态大加号 24px）。

4. **弹窗宽度**：`640` → `800`。数据盘行比系统盘行多了「数量框 + 两个增删按钮」，容量框仅靠 `flex: 1` + `min-width: 0` 被压到数值完全不可见（实测只剩步进器与 `GB` 后缀）。同时把 `.ha-disk-size` 的 `min-width` 从 `0` 抬到 `160px` 作为下限，防止后续再往该行加控件时同样的症状无声复现。

5. **下拉重复请求（顺带修复的既有缺陷）**：hcm-form-list 内部以 `watchEffect` 追踪 `list` prop（`localList.value = await props.list()`），模板里写成内联箭头函数时每次父组件重渲染都是新的函数引用，effect 判定依赖变化即重新拉取。因此改动容量/数量 → `dataDiskTotalNum` 变化 → 重渲染 → 磁盘类型接口被反复调用。修法是把六个选项工厂提到 `<script setup>` 作用域固化引用（`requireTypeList` / `regionList` / `deviceTypeList` / `imageList` / `diskTypeList` / `resAssignList`）。

   > 级联刷新不受影响：`props.list()` 是在 effect 内**同步**调用的，函数体对 `formModel.region` 的读取仍落在 `watchEffect` 的追踪窗口内（首个 `await` 之前），故地域变更依然会重拉机型与镜像。

6. **行内字段校验 + 标签说明（对齐 ziyanScr 申领表单）**：原先弹窗对 `data_disk` 无任何 `rules`，而输入框可被清空——`Number('')` 静默变成 0，提交后才被后端按 `[DataDiskMinSize, DataDiskMaxSize] = [10, 32000]` 拒单，用户要走到「确认方案」之后才看到报错。补齐三处：
   - `rules.data_disk` 沿用项目既有的同一条校验（`value.length === 0 || value.every(item => item.disk_type && item.disk_size && item.disk_num)`，文案「数据盘信息不能为空」），语义正是「零块合法、有行则三字段必填」；表单项相应挂 `property="data_disk"`。
   - 标签补说明图标 + tooltip「数据盘大小范围为20G-32000G，且为10的倍数」，与自研云申领表单同一份文案。
   - 容量输入框 `:step="10"`，`:min` / `:max` 取自 `CVM_DATA_DISK_INFO`（SSD 云硬盘下限 20G、高性能云盘 10G，本地盘无区间则回退 `0` / `32000`），否则新加的 tooltip 承诺「10 的倍数」而控件仍以 1 为步长、下限 1，UI 自相矛盾。

   > 跨模块引用说明：`CVM_DATA_DISK_INFO` 位于 `views/ziyanScr/components/cvm-data-disk/`，由 components 反向引用 views。这是本 feature 既有做法（规格展示 helper host-apply-display.ts 已从 ziyanScr 取 `getDiskTypesName` / `getZoneCn`），项目内亦有大量同类先例，故未另建常量副本——复制一份会立刻产生「两处上下限漂移」的隐患。

   > 有意未照搬的一点：参照组件在磁盘类型变更时会把容量重置为新类型的下限（`handleDiskTypeChange`）。本弹窗不跟随——用户已填 500G 时切换类型被静默清成 20G 属于意外丢值，而 500G 对两种云盘都合法，重置无必要。

### 交互取舍（记录决策，避免后人回改）

首版实现是「行内只放删除 + 列表下方独立放『添加数据盘』文字按钮」，理由是弹窗宽度仅 640px、数据盘行已有三个控件。**已按用户指定改为参照自研云申领的数据盘编辑器**：行内同时放加减号、零块时塌缩成大加号按钮。取舍结论是**一致性优先于宽松布局**——同一套「类型 + 容量 + 数量」磁盘编辑交互在项目里已有既定形态，用户对它有肌肉记忆，另造一套文字按钮反而是认知成本。宽度上加减号是紧凑图标按钮，实测未挤压三个输入控件。

### 未改动项（有意为之）

| 项 | 原因 |
|----|------|
| hcm-form-list 组件本身（components/form/list.vue） | **用户已确认只修调用侧**。根治需让组件不再追踪函数身份，但它是全站共享组件，会让「依赖新闭包触发重拉」的调用方静默失效，需回归三级账号侧滑的权限模板联动，风险与收益不匹配。改为把约束写进项目规范（见「文档改动」） |
| account-create-sideslider 第 410 行同类写法 | 该处 `:list-generator="getRowPermTemplateListGenerator(row)"` 在模板里直接调用函数，是同一类缺陷。**用户已确认另起单据修**，与数据盘需求无关，混在本分支不利于回溯 |
| 系统盘增删 | 系统盘为必填，不在需求范围 |
| `recommend.go` 默认盘 | 澄清已定：保留默认 500G，只让前端可删 |
| A 卡（推荐方案卡组件 host-apply-recommend-card.vue）调整入口 | 仍保持注释态，开放与否属独立产品决策 |
| `data_disk` 的类型定义 | `HostApplySuborder.data_disk` 本就是可选数组，无需改类型 |

### 需求映射

| 验收项 | 落点 |
|--------|------|
| AC-001 删除后保存为空且不报必填 | 行内减号按钮；`rules.data_disk` 对空数组直接返回 `true`，故零块不触发校验失败 |
| AC-002 / AC-003 添加行与默认值 | 行内加号 / 空态大加号 → `handleAddDataDisk` + `createDataDisk` |
| AC-004 上限禁用与提示 | `dataDiskTotalNum` + 加号 `:disabled` + `v-bk-tooltips` + 数量框 `:max` |
| AC-005 重开仍为零块 | 沿用既有 `watch(visible)` 回填，`createState` 取 `suborder.data_disk ?? []`，不注入默认盘 |
| AC-006 空数组下游展示 | 既有规格展示 helper（host-apply-display.ts）与预提单表格已有空值处理，本次无需改动 |
| AC-007 草稿不被跨标签覆盖 | 依赖预提单卡既有 `edited` 本地态保护，本次**未触碰**该逻辑 |
| AC-008 agent 不补默认盘 | 后端/agent 侧行为，澄清 Q4 已确认，前端无对应改动 |

## 文档改动

### model 模块：选项组件的 list 必须传稳定引用

`.hcmfe/docs/modules/model.md` 的「关键约定（务必遵守）」补一条：hcm-form-list 的 `list` / `list-generator` 必须传稳定函数引用，内联箭头函数会因每次重渲染换引用而重复拉接口；并写明「固化引用不会破坏级联刷新」的机理，避免后人以为要改回去。同时已 `docs_promote` 一条 `rule` 候选（`promo-97a97e8c`，pending 待评审）。

> 记录动机：这是本次排查里最反直觉的一点。用户先只反馈磁盘类型下拉重复请求，再补充「所有下拉都有、resize 也触发」，两次现象同一根因——症状分散、从表象根本归不到「函数身份」上。

### chatbot 模块：数据盘可为空

`.hcmfe/docs/modules/chatbot.md` 的「关键边界」补一条**跨栈**知识：后端允许数据盘总块数为 0（`ValidateDisk` / `data_disk` 可 NULL / 下发前有 `len()` 守卫），但 woa-server 推荐方案固定塞 1 块 500G，因此调整弹窗**必须**提供增删行能力，否则用户无法申领不挂盘的机器；上限 20 对齐后端 `DataDiskTotalNum`；空数组在预提单表格 / 确认提交卡 / 规格展示处渲染为 `--` 或隐藏该行。

> 记录动机：这条边界横跨前后端，只看前端代码无法得知「空数组是安全的」，只看后端也无法得知「前端拿不到删除入口」。

## 提交策略

单单据单提交（与 `git-commit` skill 默认一致）。
