# uni-app-x (uvue) CSS 兼容性规则

> 本文件是移动端 `AGENTS.md` 该节的**按需层全文**（#1509 根线式分层）：常驻层只留判据摘要，
> 表格、手法与实测叙述在这里。**引用本文件时引小节名，不引行号。**

本项目使用 uni-app-x (uvue 模式) 编译到 Android/iOS 原生端。uvue 的 CSS 引擎是**原生渲染器**，仅支持 CSS 属性的子集，与 Web CSS 有显著差异。编写 `.uvue` 文件的 `<style>` 时必须遵守：

## 必须使用 `<style lang="scss">`

`.uvue` 文件的 `<style>` 块**必须**声明 `lang="scss"`，否则 SCSS 变量不会被预处理，直接传递给 uvue CSS 引擎会报错。

## 选择器限制（严格）

uvue 原生端**只支持 class 选择器**，以下选择器均不可用：

| 选择器类型 | 示例 | 是否支持 |
|---|---|---|
| class 选择器 | `.className {}` | ✅ 支持 |
| group 选择器 | `.a, .b {}` | ✅ 支持 |
| descendant（后代） | `.parent .child {}` | ⚠️ 支持但有运行时性能损耗，Vapor 不支持 |
| child combinator | `.parent > .child {}` | ⚠️ 支持但有运行时性能损耗 |
| adjacent sibling | `.a + .b {}` | ⚠️ 支持但有运行时性能损耗，Vapor 不支持 |
| **tag 选择器** | `view {}`, `text {}` | ❌ **不支持** |
| **tag + combinator** | `> view + view` | ❌ **不支持** |
| ID 选择器 | `#id {}` | ❌ 不支持 |
| 伪类/伪元素 | `:first-child`, `::before` | ❌ 不支持 |
| 属性选择器 | `[attr] {}` | ❌ 不支持 |

**编写 CSS 时只能使用 `.class-name` 形式的选择器。** 模板中的 `view`、`text` 等标签名不能出现在选择器中。

## 不支持的 CSS 属性与值

| 不支持的语法 | 替代方案 | 说明 |
|---|---|---|
| `gap` / `row-gap` / `column-gap` | 子元素加 margin class | 在 template 的子元素上添加 `ml-N`、`mr-N mb-N` 等 class |
| `text-decoration` | `border-bottom` 模拟 | 用具体颜色值（不支持 `currentColor`） |
| `calc() + env()` | 固定 rpx 值 | 安全区域需通过 `uni.getSystemInfoSync()` 动态获取 |
| `vh` / `vw` 单位 | 固定 rpx 值 | 仅支持 `number` 和 `pixel`（含 `rpx`） |
| `align-items: baseline` | `flex-start` 或 `center` | 仅支持 `center`/`flex-start`/`flex-end`/`stretch` |
| `max-height: 百分比` | 固定 rpx 值 | 仅支持 `number` 和 `pixel` |
| CSS 自定义属性 `var(--xxx)` | 直接写色值或用 SCSS 变量 | uvue 原生端不支持 CSS 变量 |
| `currentColor` | 具体颜色值 | 如 `#2979ff`、`#999999` |
| `display: grid` / `grid-*` | flex 布局 | grid 布局不支持 |
| `transition` / `animation` | uni-app API 动画 | 原生端不支持 CSS 动画 |
| `font-size` / `color` / `text-align` / `font-weight` 写在文字类元素**之外**（合法承载：`<text>` / `<button>` / `<input>` / `<textarea>`，`font-weight` 另多一个 `<loading>`） | 把文字挪进 `<text>` 子元素，让这些声明跟到 `<text>` 的 class 上（容器只留 padding / background / border） | 原生端只把**文字类样式**认给**文字类元素**，落在 `<view>` 上会被渲染层判错并**忽略** ⇒ 设计稿的字号 / 色值 / 字重从未生效过，且**每个非法位各打一条** error（本票两页术前每页 3 条、术后 0 条，#1269 真机实测；污染 ①a 的错误行判据）。真机日志原文：``style property `font-size\|color` is only supported on `<text>\|<button>\|<input>\|<textarea>`. there is an error on `<view class="mode-tab">` ``（#650 T12 的 ①a 实测，术前术后两轮同形）。全仓机检 = `utils/uvueFontCarrierContract.test.js`（#1269 立；只锁同族日志点名的这四个属性，`line-height` / `font-family` 无判据故不锁）；另有一批**已知存量**按「文件 + class + occurrence 数」登记在该文件的 `DEFERRED`（2026-09-24 立锁时 5 位 / 7 条），那张清单**只减不增** —— 同一登记位再加一条也判红，改到那一页时须在同一 PR 里收口并删条目。**5 位 / 7 条已由 #1314 全部收口，该清单现测为空**（写法与 `white-space` 那行「已由 #1113 删除」同形：登记机制照旧，日后新增非法位仍须登记）。**具体是哪几处以那份清单为准，本表不抄**：抄过来就成了第二份真相，而棘轮的用途正是让它变短 |
| `white-space` 写在 `<text>` / `<button>` **之外**的元素上 | 横滑行改用 `flex-direction: row` + 子项 `flex-shrink: 0` 撑出溢出 | 该属性**只在 `<text>` / `<button>` 上有效**；写在 `<scroll-view>` 等元素上会被渲染层判错并**忽略**：真机日志原文 `style property white-space is only supported on <text>\|<button>. there is an error on <scroll-view …>`（2026-09-17 #1081 ①a 实测）。**承载是 `<text>` / `<button>` 的用法合法**，但必须在 `utils/uvueWhiteSpaceContract.test.js` 的 `LEGAL_CARRIER_SITES` 里**登记**（附理由）—— 正例：`pages/profile/personal-info.uvue` 的 `.code-btn-text`（承载是 `<text>`，「获取验证码」倒计时不换行，**不要删**）。存量 6 处非法写法（`favorites` / `records` / `practice-records` 的 `.filter-scroll`、`featured-list` 的 `.filter-scroll`、`ai-feature` 的 `.diag-chips`、`mock-exam` 的 `.palette-scroll`）已由 #1113 删除，同一守护已落锁 |

## `gap` 替换模式

由于 uvue 不支持 tag 选择器，`gap` 替换必须在 **template 的子元素上直接添加 margin class**：

```vue
<template>
  <view class="flex-row">
    <view class="item">A</view>
    <view class="item ml-16">B</view>
    <view class="item ml-16">C</view>
  </view>
</template>

<style lang="scss">
  .flex-row { flex-direction: row; }
  .ml-16 { margin-left: 16rpx; }
</style>
```

- **非换行水平排列**：首个子元素不加 class，后续子元素加 `ml-N`
- **换行排列**：所有子元素加 `mr-N mb-N`
- **v-for 循环**：用 `:class="{ 'base': true, 'ml-N': idx > 0 }"` 条件添加

## UTS 类型系统限制

UTS（uni-app-x 的 TypeScript 变体）不支持以下 TypeScript 语法：
- **交叉类型 + 内联对象字面量**：`type C = A & { field: type }` → 必须展平为独立类型定义
- **联合字面量类型用于运行时强转**：`'student' | 'tutor'` 编译到 Kotlin 后无法用 `as` 强转 → 统一用 `string`

### Kotlin 编译期常见错误（云打包 / 发行模式全量编译会暴露）

| 错误写法 | 报错 | 正确写法 |
|---|---|---|
| `undefined` | 找不到名称 undefined | 空值统一 `null` |
| `String(x)` | None of the following candidates is applicable | `x.toString()` |
| `let x : any = null` | Null cannot be a value of non-null 'Any' | `let x : number \| null = null` 等可空类型 |
| `.catch((e : any) => …)` | None of the following candidates is applicable | `.catch((e) => …)` 或 `.catch((e : any \| null) => …)` |
| `Record<string,string> = {a:1}` | Cannot create an instance of an abstract class | `new Map<string,string>()` + `.set()` |
| 事件回调 `(e : any)` 访问 `e.detail` | Unresolved reference 'detail' | `e as UTSJSONObject` + `e['detail']` 索引 |
| 函数定义引用后声明的变量 | Unresolved reference（无 hoisting） | 变量声明前置到函数之前 |
| 模板对可空 `?:` 字段做 `>`/`<` 比较 | Operator call is prohibited on nullable receiver | `(x ?? 0) > 0` 兜底 |
| `getStorageSync` 返回值传给 `setStorageSync` | Argument type mismatch: Any? vs Any | `if (x != null) setStorageSync(k, x as any)` |

> `any` 在 UTS 编译成 Kotlin 的**非空** `Any`；需要可空时用 `any | null` 或具体可空类型。模板属性访问走宽松路径不报错，但 `<script>` 是严格 Kotlin 检查——报错全在 script，别被模板的"安静"误导。

## 编译验证

修改 `.uvue` 文件后，在 HBuilderX 中重新编译，检查控制台：
- **ERROR** = 阻断编译，必须修复
- **WARNING** = 不阻断但原生端可能不生效（如 `gap` 被静默忽略，布局会坏）
