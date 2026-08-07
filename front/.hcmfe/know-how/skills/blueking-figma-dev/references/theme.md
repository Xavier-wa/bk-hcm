# BlueKing Theme 与 Token 候选

Figma variables 表达设计语义，不证明目标项目存在同名 CSS/Sass Token。本页用于检索和核对候选值，不是项目已接入证明。

## 一、使用前置条件

只有同时满足以下条件才输出 `var(--token)`：

1. 目标项目源码定义该变量。
2. 当前页面渲染上下文加载该变量。
3. 相邻代码实际使用该变量或同一 Theme 体系。
4. Figma 语义与项目 Token 语义一致。

任一条件不成立时，按顺序回退：

1. 项目现有语义 Token、Sass 变量或 utility。
2. 目标目录现有局部样式方式。
3. Figma 精确设计值。

不能仅因名称或数值接近就强行映射。

## 二、Figma 前缀速查

| Figma 变量/语义 | 本页候选区域 |
| --- | --- |
| `Brand-*` | 品牌色 |
| `Success-*` | 成功色 |
| `Warn-*` | 警告色 |
| `Danger-*` | 危险色 |
| `Neutral-*` | 中性色 |
| `text-*` | 语义文字颜色 |
| `bk-mask`、`bk-tooltips` | 遮罩与提示背景 |
| `shadows-*` | 阴影 |
| `font-*`、Typography | 字体候选 |
| `size-*`、Spacing | 间距候选 |
| `radius-*` | 圆角候选 |

## 三、颜色候选

### 品牌色

| Token | Light | Dark |
| --- | --- | --- |
| `Brand-1` | `#1768EF` | `#709BFF` |
| `Brand-2` | `#3A84FF` | `#5D8DFF` |
| `Brand-3` | `#699DF4` | `#4A70C9` |
| `Brand-4` | `#A3C5FD` | `#4264B3` |
| `Brand-5` | `#CDDFFE` | `#334D88` |
| `Brand-6` | `#E1ECFF` | `#2B4173` |
| `Brand-7` | `#F0F5FF` | `#23355D` |

### 成功色

| Token | Light | Dark |
| --- | --- | --- |
| `Success-1` | `#299E56` | `#22A380` |
| `Success-2` | `#2CAF5E` | `#1F8E73` |
| `Success-3` | `#65C389` | `#1E806A` |
| `Success-4` | `#A1E3BA` | `#1C7261` |
| `Success-5` | `#CBF0DA` | `#18574F` |
| `Success-6` | `#DAF6E5` | `#164946` |
| `Success-7` | `#EBFAF0` | `#153B3D` |

### 警告色

| Token | Light | Dark |
| --- | --- | --- |
| `Warn-1` | `#E38B02` | `#FFB646` |
| `Warn-2` | `#F59500` | `#DB9E41` |
| `Warn-3` | `#F8B64F` | `#C38E3E` |
| `Warn-4` | `#F9D090` | `#AB7F3B` |
| `Warn-5` | `#FCE5C0` | `#7C5F35` |
| `Warn-6` | `#FDEED8` | `#644F32` |
| `Warn-7` | `#FDF4E8` | `#4C402F` |

### 危险色

| Token | Light | Dark |
| --- | --- | --- |
| `Danger-1` | `#E71818` | `#F55858` |
| `Danger-2` | `#EA3636` | `#D34E51` |
| `Danger-3` | `#FF5656` | `#BC484C` |
| `Danger-4` | `#F8B4B4` | `#A54247` |
| `Danger-5` | `#FFDDDD` | `#77353D` |
| `Danger-6` | `#FFEBEB` | `#602E38` |
| `Danger-7` | `#FFF0F0` | `#492833` |

### 中性色

| Token | Light | Dark |
| --- | --- | --- |
| `Neutral-1` | `#313238` | `#E5E7EB` |
| `Neutral-2` | `#4D4F56` | `#B0BBD1` |
| `Neutral-3` | `#979BA5` | `#8B98B2` |
| `Neutral-4` | `#C4C6CC` | `#455B7A` |
| `Neutral-5` | `#DCDEE5` | `#394866` |
| `Neutral-6` | `#EAEBF0` | `#283647` |
| `Neutral-7` | `#F0F1F5` | `#222E3D` |
| `Neutral-8` | `#F5F7FA` | `#030712` |
| `Neutral-9` | `#FAFBFD` | `#1B2333` |
| `Neutral-10` | `#FFFFFF` | `#101827` |

`Pure-white` 候选为 `#FFFFFF`，`Pure-black` 候选为 `#000000`。反色文本与随 Theme 变化的背景不要混用同一语义。

## 四、语义颜色、遮罩与阴影

| 语义候选 | Light | Dark | 用途 |
| --- | --- | --- | --- |
| `text-strong` | `Neutral-1` | `Neutral-1` | 强提示/标题 |
| `text-body` | `Neutral-2` | `Neutral-2` | 正文 |
| `text-weak` | `Neutral-3` | `Neutral-3` | 弱提示 |
| `text-disabled` | `Neutral-4` | `Neutral-4` | 失效 |
| `text-invert` | `Pure-white` | `Pure-white` | 反色文本 |
| `bk-mask` | `rgba(0, 0, 0, 0.60)` | `rgba(0, 0, 0, 0.60)` | 模态遮罩 |
| `bk-tooltips` | `rgba(0, 0, 0, 0.80)` | `#3E4152` | 深色提示背景 |
| `shadows-mini` | `0 2px 4px rgba(25, 25, 41, 0.05)` | `0 2px 4px rgba(0, 0, 0, 0.80)` | 卡片正常态 |
| `shadows-small` | `0 2px 4px rgba(0, 0, 0, 0.10)` | `0 2px 4px rgba(0, 0, 0, 0.80)` | 卡片悬浮态 |
| `shadows-middle` | `0 0 10px rgba(0, 0, 0, 0.10)` | `0 0 10px rgba(0, 0, 0, 0.80)` | 下拉菜单/气泡 |
| `shadows-big` | `0 4px 12px rgba(0, 0, 0, 0.20)` | `0 4px 12px rgba(0, 0, 0, 0.80)` | 对话框 |

## 五、字体候选

字体族候选：

- 默认：候选变量 `--bk-font-default`；值为 `-apple-system, BlinkMacSystemFont, "Helvetica Neue", Helvetica, "Segoe UI", Arial, Roboto, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif`
- 代码：候选变量 `--bk-font-code`；值为 `Roboto Mono, Menlo, Monaco, Consolas, Courier, monospace`

| 语义 | 候选检索名 | 字号 | 字重 | 行高 | 字体族 |
| --- | --- | --- | --- | --- | --- |
| Caption 常规 | `--font-caption-regular` | `10px` | `400` | `16px` | 默认 |
| Body S 常规 | `--font-body-s-regular` | `12px` | `400` | `20px` | 默认 |
| Body S 加粗 | `--font-body-s-bold` | `12px` | `700` | `20px` | 默认 |
| Body S 代码 | `--font-body-s-code`；旧资料可能为 `--font-body-s-cold` | `12px` | `400` | `20px` | 代码 |
| Body M 常规 | `--font-body-m-regular` | `13px` | `400` | `20px` | 默认 |
| Body M 加粗 | `--font-body-m-bold` | `13px` | `700` | `20px` | 默认 |
| Body M 代码 | `--font-body-m-code`；旧资料可能为 `--font-body-m-cold` | `13px` | `400` | `20px` | 代码 |
| Body L 常规 | `--font-body-l-regular` | `14px` | `400` | `22px` | 默认 |
| Body L 加粗 | `--font-body-l-bold` | `14px` | `700` | `22px` | 默认 |
| Body L 代码 | `--font-body-l-code`；旧资料可能为 `--font-body-l-cold` | `14px` | `400` | `22px` | 代码 |
| Title S 常规 | `--font-title-s-regular` | `16px` | `400` | `24px` | 默认 |
| Title S 加粗 | `--font-title-s-bold` | `16px` | `700` | `24px` | 默认 |
| Title M 常规 | `--font-title-m-regular` | `20px` | `400` | `28px` | 默认 |
| Title M 加粗 | `--font-title-m-bold` | `20px` | `700` | `28px` | 默认 |
| Title L 常规 | `--font-title-l-regular`；旧资料可能错误重复为 `--font-title-m-regular` | `24px` | `400` | `32px` | 默认 |
| Title L 加粗 | `--font-title-l-bold`；旧资料可能错误重复为 `--font-title-m-bold` | `24px` | `700` | `32px` | 默认 |

旧资料可能把 code 拼作 `cold`，或把 24px Title L 重复命名为 Title M。不要传播旧拼写；以目标项目真实定义为准。

## 六、间距候选

| Token 候选 | 值 | Token 候选 | 值 | Token 候选 | 值 |
| --- | --- | --- | --- | --- | --- |
| `size-1` | `2px` | `size-6` | `16px` | `size-11` | `36px` |
| `size-2` | `4px` | `size-7` | `20px` | `size-12` | `40px` |
| `size-3` | `6px` | `size-8` | `24px` | `size-13` | `48px` |
| `size-4` | `8px` | `size-9` | `28px` | `size-14` | `56px` |
| `size-5` | `12px` | `size-10` | `32px` | `size-15` | `64px` |

Token 名不决定 CSS 属性；`size-6` 可能用于 gap、padding、margin 或尺寸，必须结合 Figma 标注。

## 七、圆角候选

| Token 候选 | 值 | 常见语义 |
| --- | --- | --- |
| `radius-small` | `2px` | 小控件/轻微圆角 |
| `radius-medium` | `4px` | 常规控件/卡片 |
| `radius-large` | `8px` | 大卡片/浮层 |
| `radius-round` | `999px` | 胶囊/绝对圆角 |

## 八、浅色/深色使用约束

- 只有项目已实现 Theme 切换和变量作用域时，才用同名变量自动切换。
- Figma 有 Dark variant 不代表应用已具备暗色能力。
- 项目未接入 Dark 时，先询问是否新增能力；不要仅复制 Dark 值。
- 目标项目变量值与本页候选不同，以目标项目为准。

## 九、落地示例

Figma 标注为 `Brand-2`，项目未找到 `--Brand-2`：

1. 搜索项目已有品牌色 Token。
2. 有语义匹配则使用项目 Token。
3. 没有时按目标目录样式方式写 Figma 精确色值。
4. 不输出未定义的 `var(--Brand-2)`，也不为单个组件新增全局 Token。
