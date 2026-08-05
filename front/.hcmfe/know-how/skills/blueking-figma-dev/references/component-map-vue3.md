# Vue3 / bkui-vue 组件映射

适用于目标项目已确认使用 bkui-vue 的场景。先用本表定位候选，再从与目标项目版本严格匹配的 `bkui-vue-components` 查询 API；组件 API 以对应组件 reference 为最终依据。

表中的 Figma 名称和中文同义词用于语义检索，不代表真实组件标签或 API。

**映射后禁止直接出码：** 必须先使用候选所属组件库的组件 skill，在其索引中定位并读取目标组件 reference。基础 bkui-vue 组件使用 `bkui-vue-components`；未读文档前，组件名不能作为 props、events、slots 或 methods 的依据。静态 map 未命中时，必须搜索对应组件 skill 完整索引和同义语义；完整索引仍未命中时，列出已检索的 skill、组件和能力缺口并询问用户是否允许手写，获批后才能最小手写。

普通 HTML/CSS 布局和业务组合不属于手写基础组件。

## 基础组件

| 设计语义 | 常见 Figma 图层名/同义词 | 组件候选 | 在组件 Skill 中检索 | 注意事项 |
| --- | --- | --- | --- | --- |
| 菜单 | Menu、菜单、侧边菜单 | Menu | Menu | 区分导航菜单与操作下拉 |
| 图钉/吸附 | Affix、固定、吸顶 | Affix | Affix | 确认滚动容器 |
| 导航框架 | Navigation、导航、主导航 | Navigation | Navigation | 优先项目现有布局封装 |
| 加载 | Loading、加载中、处理中 | Loading | Loading | 区分区域、按钮和增量加载 |
| 按钮 | Button、按钮、主操作、次操作 | Button | Button | 具体主题/尺寸查组件 skill |
| 单选 | Radio、单选、选项 | Radio | Radio | 区分单选组 |
| 多选 | Checkbox、复选、多选 | Checkbox | Checkbox | 区分单项与组 |
| 警告条 | Alert、提示条、警告、错误条 | Alert | Alert | 区分页面异常态 |
| 悬浮导航 | FixedNavbar、悬浮导航 | FixedNavbar | FixedNavbar | 确认固定位置 |
| 返回顶部 | BackTop、Backtop、回到顶部 | BackTop | BackTop | 确认滚动容器 |
| 异常/空态 | Exception、空状态、无权限、错误页 | Exception | Exception | 根据场景查 type/scene |
| 卡片 | Card、卡片、面板 | Card | Card | 优先业务 Card 封装 |
| 徽标 | Badge、角标、数量标记 | Badge | Badge | 区分 Tag |
| 进度条 | Progress、进度、完成度 | Progress | Progress | 区分圆形/线性能力 |
| 开关 | Switcher、Switch、开关 | Switcher | Switcher | 确认禁用和 loading |
| 面包屑 | Breadcrumb、面包屑、路径导航 | Breadcrumb | Breadcrumb | 确认路由来源 |
| 文字链接 | Link、链接、文字按钮 | Link | Link | 导航语义优先使用链接 |
| 折叠面板 | Collapse、折叠、展开收起 | Collapse | Collapse | 确认单开/多开 |
| 步骤条 | Steps、步骤、向导 | Steps | Steps | 区分 Process |
| 流程展示 | Process、流程、节点状态 | Process | Process | 偏流程状态展示 |
| 下拉选择 | Select、选择器、筛选器 | Select | Select | 复杂查询看 SearchSelect |
| 时间轴 | Timeline、时间线、操作记录 | Timeline | Timeline | 确认排序方向 |
| 动画数字 | AnimateNumber、数字动画、滚动数字 | AnimateNumber | AnimateNumber | 静态数字无需使用 |
| 评分 | Rate、评分、星级 | Rate | Rate | 确认可编辑性 |
| 轮播 | Swiper、轮播、走马灯 | Swiper | Swiper | 确认自动播放和分页 |
| 表格 | Table、表格、数据列表 | Table | Table | 无其他依赖证据时先用 bkui-vue Table |
| 输入框 | Input、输入、文本框、搜索框 | Input | Input | 搜索选择看 SearchSelect |
| 下拉菜单 | Dropdown、DropdownMenu、操作菜单 | Dropdown | Dropdown | 区分 Select |
| 表单 | Form、表单、字段组 | Form | Form | 同时查询 FormItem |
| 上传 | Upload、上传、附件 | Upload | Upload | 确认文件限制和状态 |
| 弹出层 | Popover、气泡、帮助提示、详情浮层 | Popover | Popover | 纯文字/溢出提示先看项目现有 tooltip 或 OverflowTitle |
| 消息 | Message、轻提示、操作反馈 | Message | Message | 命令式 API 需查组件 skill |
| 信息框 | InfoBox、信息确认、提示框 | InfoBox | InfoBox | 区分 Dialog/PopConfirm |
| 通知 | Notify、通知、全局通知 | Notify | Notify | 确认位置与持续时间 |
| 文字/工具提示 | Tooltips、Tooltip、文字提示、帮助提示 | Tooltips / OverflowTitle / Popover / 项目 tooltip | Tooltips、OverflowTitle、Popover | 按普通提示、文本溢出、富内容或 click 弹层选择 |
| 树 | Tree、树、层级选择 | Tree | Tree | 大数据树先查虚拟化能力 |
| 颜色选择 | ColorPicker、颜色选择器 | ColorPicker | ColorPicker | 确认格式与透明度 |
| 标签 | Tag、标签、状态标签 | Tag | Tag | 数量角标看 Badge |
| 标签输入 | TagInput、标签输入、多值输入 | TagInput | TagInput | 确认创建与校验 |
| 日期选择 | DatePicker、日期、日期范围 | DatePicker | DatePicker | 项目有 `@blueking/date-picker` 时再比较 |
| 时间选择 | TimePicker、时间、时分秒 | TimePicker | TimePicker | 日期时间组合需确认 |
| 分割线 | Divider、分割线 | Divider | Divider | 简单边框可按项目样式 |
| 选项卡 | Tab、Tabs、标签页 | Tab | Tab | 确认路由/受控状态 |
| 滑块 | Slider、滑块、范围选择 | Slider | Slider | 不要与 Sideslider 混淆 |
| 侧滑面板 | Sideslider、侧滑、抽屉、侧滑详情 | Sideslider | Sideslider | 确认关闭和宽度 |
| 穿梭框 | Transfer、穿梭、左右选择 | Transfer | Transfer | 确认搜索和排序 |
| 代码差异 | Diff、CodeDiff、差异对比 | CodeDiff | CodeDiff | 非代码文本也需确认格式 |
| 虚拟渲染 | VirtualRender、虚拟列表、长列表 | VirtualRender | VirtualRender | 只有性能需求成立时使用 |
| 分页 | Pagination、分页、翻页 | Pagination | Pagination | 确认服务端/客户端分页 |
| 可拉伸布局 | ResizeLayout、分栏拖拽、可调整面板 | ResizeLayout | ResizeLayout | 确认方向和最小尺寸 |
| 栅格布局 | Grid、Row、Col、栅格 | Container | Container / Grid | 先看项目 CSS Grid/Flex 约定 |
| 对话框 | Dialog、Modal、弹窗、模态框 | Dialog | Dialog | 确认焦点和关闭方式 |
| 级联选择 | Cascader、级联、层级选择 | Cascader | Cascader | 区分 TreeSelect 类需求 |
| 查询选择 | SearchSelect、搜索选择、高级筛选、筛选器 | SearchSelect | SearchSelect | 复杂筛选优先候选 |
| 溢出标题 | OverflowTitle、文本省略、悬浮全文 | OverflowTitle | OverflowTitle | 仅文本溢出场景 |
| 气泡确认 | PopConfirm、二次确认、删除确认 | PopConfirm | PopConfirm | 富交互或长文案考虑 Dialog |

## 可选 `@blueking/*` 组件

只有目标项目已安装，且目标区域或相邻业务封装实际使用时才启用。

| 设计语义 | 常见 Figma 图层名/同义词 | 包/组件候选 | API 查阅 | 注意事项 |
| --- | --- | --- | --- | --- |
| 增强表格 | AdvancedTable、列设置、树形表格 | `@blueking/tdesign-ui` PrimaryTable / EnhancedTable | `blueking-tdesign-ui` | 不覆盖 bkui-vue Table 默认候选 |
| 时间范围/时区 | Date Picker、时间范围、时区 | `@blueking/date-picker` | `blueking-date-picker` 的 Vue3 入口 reference | 不要误用 bkui-vue DatePicker 或 Vue2 入口文档 |
| 高级查询选择 | Search Select、高级筛选 | `@blueking/search-select-v3` | `blueking-search-select-v3` 的 Vue3 入口 reference | 不要误用 bkui-vue SearchSelect 或 Vue2 入口文档 |

## API 查阅要求

映射到候选后，只向配套组件 skill 查询本次需要的 API：

- props：状态、尺寸、数据和受控值。
- events：用户操作和状态变化。
- slots：设计定制区域。
- methods：表单校验、聚焦或命令式操作。

未从与目标项目版本严格匹配的组件 skill reference 确认的 API 不得写入代码。
