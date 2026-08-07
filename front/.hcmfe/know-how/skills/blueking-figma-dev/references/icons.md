# BlueKing Icon 映射

目标项目决定 Icon 来源。BlueKing 项目可能使用 bkui-vue Icon、Vue2 iconfont、其他 Icon 包或项目自有 SVG。

## 一、适用范围

- 本页完整候选清单面向 bkui-vue / Vue3。
- Vue3 必须使用与目标项目版本严格匹配的 `bkui-vue-components` Icon reference，并以其记录作为最终 export 和 API 依据。
- Vue2 不直接使用本清单；必须使用与目标项目版本严格匹配的 `bk-magicbox-vue-components` Icon reference，并遵循项目资源约定。

## 二、选型顺序

1. 检查相邻 imports、业务封装、iconfont 和资源 barrel。
2. 复用造型与语义都匹配的项目现有 Icon。
3. 项目使用 bkui-vue Icon 时，用 Figma 图层名/语义在本页找候选。
4. 使用 `bkui-vue-components` Icon reference 确认最终 export 和 API。
5. Vue2 使用 `bk-magicbox-vue-components` Icon reference 确认组件能力和 API，并按项目 iconfont、`bk-icon`、SVG 组件或资源目录约定接入。
6. 找不到准确候选时，从 Figma 获取真实 SVG/图片并按项目规范接入。

Figma 图层名只是检索词，不保证等于 export 名。

## 三、常见语义候选

| 语义 | 候选 |
| --- | --- |
| 展开/收起 | AngleDown、AngleUp、AngleDownLine、AngleUpFill、CollapseLeft |
| 左右/返回/进入 | AngleLeft、AngleRight、LeftShape、RightShape、LeftTurnLine、RightTurnLine |
| 双向移动 | AngleDoubleLeft、AngleDoubleRight、ArrowsLeft、ArrowsRight、Transfer |
| 新增 | Plus |
| 关闭/删除 | Close、CloseLine、Del |
| 编辑 | EditLine |
| 搜索/筛选 | Search、Funnel |
| 复制/分享 | Copy、CopyShape、Share |
| 固定 | FixLine、FixShape |
| 放大/缩小/全屏 | EnlargeLine、NarrowLine、FilliscreenLine、UnfullScreen |
| 可见/隐藏 | Eye、Unvisible |
| 成功/完成 | Success、Done |
| 错误 | Error、ExclamationCircleShape |
| 警告 | Warn |
| 信息/帮助 | Info、InfoLine、Help、HelpFill、HelpDocumentFill |
| 加载 | Loading、Spinner、SwitcherLoading |
| 上传 | Upload |
| 播放/音视频 | PlayShape、AudioFill、VideoFill |
| 文件 | DocFill、TextFile、TextFill、TextAll |
| Office/PDF | ExcelFill、PptFill、PdfFill |
| 图片 | ImageFill、ImgError、ImgPlacehoulder |
| 文件夹/归档 | Folder、FolderOpen、FolderShape、FolderShapeOpen、ArchiveFill |
| 数据/代码 | DataShape、Code |
| 更多 | Ellipsis |
| 品牌/社交 | Bk、Qq、Weixin、WeixinPro |

同一语义有多个候选时，必须对照 Figma 实际形状和项目现有使用。

## 四、完整候选清单

以下名称来自 bkui-vue Icon 候选快照，最终以 `bkui-vue-components` Icon reference 为准：

`FilliscreenLine`、`GragFill`、`ImgPlacehoulder` 等名称可能是上游历史拼写；不要自行改名，也不要假设仍存在，必须核对对应版本 Icon reference。

```text
AngleDoubleDownLine
AngleDoubleLeft
AngleDoubleLeftLine
AngleDoubleRight
AngleDoubleRightLine
AngleDoubleUpLine
AngleDown
AngleDownFill
AngleDownLine
AngleLeft
AngleRight
AngleUp
AngleUpFill
ArchiveFill
ArrowsLeft
ArrowsRight
Assistant
AudioFill
Bk
Circle
Close
CloseLine
Code
CogShape
CollapseLeft
Copy
CopyShape
DataShape
Del
DocFill
Done
DownShape
DownSmall
EditLine
Ellipsis
EnlargeLine
Error
ExcelFill
ExclamationCircleShape
Eye
FilliscreenLine
FixLine
FixShape
Folder
FolderOpen
FolderShape
FolderShapeOpen
Funnel
GragFill
Help
HelpDocumentFill
HelpFill
ImageFill
ImgError
ImgPlacehoulder
Info
InfoLine
LeftShape
LeftTurnLine
Loading
NarrowLine
Original
PdfFill
PlayShape
Plus
PptFill
Qq
RightShape
RightTurnLine
Search
Share
Spinner
Success
SwitcherLoading
TextAll
TextFile
TextFill
Transfer
TreeApplicationShape
UnfullScreen
Unvisible
UpShape
Upload
VideoFill
Warn
Weixin
WeixinPro
```

## 五、缺失处理

- 不使用 emoji、字符、近似 CSS 图形或占位 SVG。
- 从 Figma 导出真实资源，保留 viewBox、比例和必要 path。
- 按项目现有命名、目录、组件封装和 barrel export 接入。
- 装饰 Icon 对辅助技术隐藏；可点击 Icon 使用按钮语义和可访问名称。
- 保持项目现有尺寸、颜色继承和 tree-shaking 方式。
