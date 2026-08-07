# Vue2 / bk-magic-vue 组件映射

适用于目标项目已确认使用 bk-magic-vue 的场景。先用本表定位候选，再从与目标项目版本严格匹配的 `bk-magicbox-vue-components` 查询 API；组件 API 以对应组件 reference 为最终依据。

Vue2 项目的 Icon 与 Token 体系差异较大：本表只映射组件，不默认套用 Vue3 Icon 清单或 Theme Token。

**映射后禁止直接出码：** 必须先使用 `bk-magicbox-vue-components`，在其组件索引中定位并读取目标组件 reference，确认 props、events、slots、methods、类型、默认值和示例。静态 map 未命中时，必须搜索该 skill 完整索引和同义语义；完整索引仍未命中时，列出已检索的 skill、组件和能力缺口并询问用户是否允许手写，获批后才能最小手写。

普通 HTML/CSS 布局和业务组合不属于手写基础组件。

| 设计语义 | 常见 Figma 图层名/同义词 | 组件候选 | 在组件 Skill 中检索 | 注意事项 |
| --- | --- | --- | --- | --- |
| 按钮 | Button、按钮、主操作、次操作 | bkButton | Button | API 只查 Vue2 组件 skill |
| 文字链接 | Link、链接、文字按钮 | bkLink | Link | 导航语义需确认 |
| 过渡动画 | Transition、过渡、展开动画 | bkTransition | Transition | 简单 CSS 动画按项目约定 |
| 栅格 | Grid、Row、Col、栅格 | bkRow / bkCol | Grid | 先看项目 Flex/Grid 用法 |
| 可拉伸布局 | ResizeLayout、可调整面板、拖拽分栏 | bkResizeLayout | ResizeLayout | 确认方向和尺寸限制 |
| 导航 | Navigation、导航、主导航 | bkNavigation | Navigation | 优先项目布局封装 |
| 选项卡 | Tab、Tabs、标签页 | bkTab | Tab | 确认路由/受控状态 |
| 步骤条 | Steps、步骤、向导 | bkSteps | Steps | 区分 Process |
| 流程展示 | Process、流程、节点状态 | bkProcess | Process | 偏流程状态展示 |
| 面包屑 | Breadcrumb、面包屑、路径导航 | bkBreadcrumb | Breadcrumb | 确认路由来源 |
| 分割线 | Divider、分割线 | bkDivider | Divider | 简单边框可按项目样式 |
| 悬浮导航 | FixedNavbar、悬浮导航 | bkFixedNavbar | FixedNavbar | 确认固定位置 |
| 返回顶部 | BackTop、回到顶部 | bkBackTop | BackTop | 确认滚动容器 |
| 图钉/吸附 | Affix、固定、吸顶 | bkAffix | Affix | 确认滚动容器 |
| 输入框 | Input、输入、文本框、搜索框 | bkInput | Input | 复杂筛选看 bkSearchSelect |
| 单选 | Radio、单选、选项 | bkRadio | Radio | 区分单选组 |
| 多选 | Checkbox、复选、多选 | bkCheckbox | Checkbox | 区分单项与组 |
| 下拉选择 | Select、选择器、筛选器 | bkSelect | Select | 复杂查询看 SearchSelect |
| 级联选择 | Cascade、Cascader、级联 | bkCascade | Cascade | 确认数据结构 |
| 开关 | Switcher、Switch、开关 | bkSwitcher | Switcher | 确认禁用和 loading |
| 颜色选择 | ColorPicker、颜色选择器 | bkColorPicker | ColorPicker | 确认格式和透明度 |
| 日期选择 | DatePicker、日期、日期范围 | bkDatePicker | DatePicker | 确认日期/范围 |
| 时间选择 | TimePicker、时间、时分秒 | bkTimePicker | TimePicker | 日期时间组合需确认 |
| 标签输入 | TagInput、标签输入、多值输入 | bkTagInput | TagInput | 确认创建与校验 |
| 上传 | Upload、上传、附件 | bkUpload | Upload | 确认文件限制 |
| 查询选择 | SearchSelect、搜索选择、高级筛选、筛选器 | bkSearchSelect | SearchSelect | 复杂筛选候选 |
| 滑块 | Slider、滑块、范围选择 | bkSlider | Slider | 不要与 Sideslider 混淆 |
| 穿梭框 | Transfer、穿梭、左右选择 | bkTransfer | Transfer | 确认搜索和排序 |
| 评分 | Rate、评分、星级 | bkRate | Rate | 确认可编辑性 |
| 组合表单项 | ComposeFormItem、组合字段、复合输入 | bkComposeFormItem | ComposeFormItem | 普通字段使用 FormItem |
| 表单 | Form、表单、字段组 | bkForm | Form | 同时查询 FormItem |
| 动画数字 | AnimateNumber、数字动画、滚动数字 | bkAnimateNumber | AnimateNumber | 静态数字无需使用 |
| 徽标 | Badge、角标、数量标记 | bkBadge | Badge | 区分 Tag |
| 折叠面板 | Collapse、折叠、展开收起 | bkCollapse | Collapse | 确认单开/多开 |
| 差异对比 | Diff、代码差异、文本对比 | bkDiff | Diff | 确认内容格式 |
| 下拉菜单 | DropdownMenu、Dropdown、操作菜单 | bkDropdownMenu | DropdownMenu | 区分 Select |
| 轮播 | Swiper、轮播、走马灯 | bkSwiper | Swiper | 确认自动播放 |
| 分页 | Pagination、分页、翻页 | bkPagination | Pagination | 确认服务端/客户端 |
| 圆形进度 | RoundProgress、圆形进度、环形进度 | bkRoundProgress | RoundProgress | 普通进度看 bkProgress |
| 进度条 | Progress、进度、完成度 | bkProgress | Progress | 区分圆形进度 |
| 时间轴 | Timeline、时间线、操作记录 | bkTimeline | Timeline | 确认排序方向 |
| 树 | Tree、树、层级选择 | bkTree | Tree | 大数据树看 bkBigTree |
| 大数据树 | BigTree、大树、海量节点树 | bkBigTree | BigTree | 只有规模需求成立时使用 |
| 表格 | Table、表格、数据列表 | bkTable | Table | API 只查 Vue2 组件 skill |
| 标签 | Tag、标签、状态标签 | bkTag | Tag | 数量角标看 Badge |
| 虚拟滚动 | VirtualScroll、虚拟列表、长列表 | bkVirtualScroll | VirtualScroll | 只有性能需求成立时使用 |
| 图片缩放 | ZoomImage、缩放图片、查看大图 | bkZoomImage | ZoomImage | 区分预览组件 |
| 图片预览 | Image、图片、预览图片 | bkImage | Image | 确认失败占位 |
| 警告条 | Alert、提示条、警告、错误条 | bkAlert | Alert | 区分页面异常态 |
| 异常/空态 | Exception、空状态、无权限、错误页 | bkException | Exception | 根据场景确认类型 |
| 加载 | Loading、加载中、处理中 | bkLoading | Loading | 区分容器、按钮和增量加载 |
| 信息框 | InfoBox、信息确认、提示框 | bkInfoBox | InfoBox | 区分 Dialog |
| 消息 | Message、轻提示、操作反馈 | bkMessage | Message | 命令式 API 需查 skill |
| 通知 | Notify、通知、全局通知 | bkNotify | Notify | 确认位置与时长 |
| 工具提示 | Tooltips、Tooltip、文字提示、帮助提示 | bkTooltips | Tooltips | 通常为指令，必须查目标版本 |
| 弹出层 | Popover、气泡、富内容提示 | bkPopover | Popover | 区分纯文字提示 |
| 气泡确认 | Popconfirm、二次确认、删除确认 | bkPopconfirm | Popconfirm | 长文案/复杂交互考虑 Dialog |
| 对话框 | Dialog、Modal、弹窗、模态框 | bkDialog | Dialog | 确认焦点和关闭方式 |
| 侧滑面板 | Sideslider、侧滑、抽屉、侧滑详情 | bkSideslider | Sideslider | 不要与 Slider 混淆 |
| 卡片 | Card、卡片、面板 | bkCard | Card | 优先业务封装 |
| 加载中 | Spin、旋转加载、局部加载 | bkSpin | Spin | 与 bkLoading 的使用边界查组件 skill |

## 可选扩展组件

只有目标项目已安装，且目标区域或相邻业务封装实际使用时才启用。

| 设计语义 | 常见 Figma 图层名/同义词 | 包候选 | API 查阅 | 注意事项 |
| --- | --- | --- | --- | --- |
| 高级查询选择 | Search Select、高级筛选 | `@blueking/search-select-v3` | `blueking-search-select-v3` 的 Vue2 入口 reference | 不要误用 `bkSearchSelect` 或 Vue3 入口文档 |
| 时间范围/时区 | Date Picker、时间范围、时区 | `@blueking/date-picker` | `blueking-date-picker` 的 Vue2 入口 reference | 不要误用 `bkDatePicker` 或 Vue3 入口文档 |

## Icon 处理

不要使用 `icons.md` 的 Vue3 bkui-vue 清单直接映射。按顺序检查：

1. 使用与目标项目版本匹配的 `bk-magicbox-vue-components` Icon reference 确认组件能力和 API。
2. 检查目标项目 iconfont、`bk-icon` 实际类型、SVG 组件和资源目录。
3. 复用相邻业务 Icon 封装。
4. 没有可用项目资源时，从 Figma 获取真实 SVG/图片。

任何图标名称和 API 都以 `bk-magicbox-vue-components` Icon reference 和目标项目资源约定为依据。

## Token 处理

Vue2 不默认具备统一 BlueKing CSS Variable 体系。优先使用目标项目已有 Sass/CSS 变量；不存在时使用设计值，不输出未经定义的 Vue3 Token。
