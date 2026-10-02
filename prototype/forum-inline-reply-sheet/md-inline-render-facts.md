# Markdown 行内渲染能力 · 一手文档事实矩阵（研究笔记 / 临时）

取证日期 2026-10-02。只读一手文档（DCloud uni-app x 官方文档、微信开放文档、库自身仓库/源码）。
版本号取文档「兼容性」表原值；`x` = 文档标 x（不支持）。

## 主矩阵

| # | 能力 | App 端（uvue / app-android） | mp-weixin 端 | 依据 |
| --- | --- | --- | --- | --- |
| 1 | `<text>` 嵌 `<text>` | 可以且只能嵌 text；子不继承父样式；子的 position/display/width/height/margin/padding 与 text-align/lines/white-space/text-overflow 不生效 | 「text 组件内只支持 text 嵌套」；内联文本只能用 text | uvue text 组件页 https://doc.dcloud.net.cn/uni-app-x/component/text.html（兼容性：Web4.0/微信4.41/Android3.9/iOS4.11）；微信 https://developers.weixin.qq.com/miniprogram/dev/component/text.html |
| 1b | 嵌套子节点布局 | 「text组件内部为inline」（子节点随父文本流内联，非各自成块） | text 内联是 webview 默认；Skyline 下 html tag 映射成 text/span/view | https://doc.dcloud.net.cn/uni-app-x/css/ ；微信 rich-text 页 Skyline 说明2 |
| 2a | font-weight | 支持 Android 3.9+；不支持继承；部分自定义字体不支持具体数值 | 走 WXSS | https://doc.dcloud.net.cn/uni-app-x/css/font-weight.html |
| 2b | font-style | 支持 Android 3.9+；不支持继承 | WXSS | …/css/font-style.html |
| 2c | font-family | 支持 Android 3.9+；不支持逗号回退列表（仅一个字体）；app-android 自定义字体仅 ttf/otf | WXSS（Skyline 表列 monospace 关键字） | …/css/font-family.html ；Skyline …/framework/runtime/skyline/wxss.html |
| 2d | color | 支持 Android 3.9+（适用 text/button/input/textarea） | WXSS | …/css/color.html |
| 2e | background-color | 支持 Android 3.9+；官方嵌套示例在子 text 上写 background-color:yellow | WXSS | …/css/background-color.html ；text 页示例「嵌套1」 |
| 2f | text-decoration（简写） | **不支持**：兼容表 Android x/iOS x/HarmonyOS x；tips「app平台暂不支持 text-decoration 简写样式，仅支持 text-decoration-line」 | 编译到小程序「可以支持 web 的全部 css」；Skyline：简写「支持解析但以展开属性为准；当前仅支持设置一种类型」 | …/css/text-decoration.html ；…/uni-app-x/css/ ；Skyline 样式表 |
| 2g | text-decoration-line | 支持 Android 3.9+；「App平台仅支持 underline 和 line-through」；不支持继承 | WXSS/Skyline 表：none/underline/overline/line-through，「仅作用于 text 节点」 | …/css/text-decoration-line.html ；Skyline 样式表 |
| 2h | border-radius | 支持 Android 3.9+（子 text 是否生效：文档未载明，未列入失效清单） | WXSS | …/css/border-radius.html |
| 2i | padding / margin / width | 根 text 支持 Android 3.9+；**嵌套子 text 不生效**（明写 margin/padding/width） | WXSS | …/css/padding.html ；text 页限制2 |
| 2j | display | 属性值仅列 flex / none；uvue 默认 flex（「W3C 默认值为：inline」）；子 text 的 display 不生效 | webview 下 CSS 可用；uni-app x 编译到小程序默认重置为 flex，可 enableUcssReset:false 关闭 | …/css/display.html ；…/uni-app-x/css/ ；…/uni-app-x/mp/index.html |
| 3 | CSS 总览/差集页 | 「uni-app x 在 app平台实现了 web css 的子集」(ucss)；三条常见区别：仅 flex+绝对定位 / 只能 class 选择器 / 样式不继承；含「样式清单」「css样式重置」「不支持拍平的 CSS 属性」（后者列 text-decoration 四兄弟） | 「当 uni-app x 编译到 web、小程序等平台时，可以支持 web 的全部 css」 | https://doc.dcloud.net.cn/uni-app-x/css/ |
| 4 | text/子节点绑点击 | 「HBuilderX4.51版本起 text组件嵌套时，子组件支持点击事件响应。之前版本如有这方面需求，请改用 rich-text」；Vapor 下子 text 不支持 hover 点击态 | 微信 text 无「屏蔽事件」条款（rich-text 才有） | text 组件页；微信 rich-text 页 tip2 |
| 5 | rich-text 实现状态 | **App 端已实现**：Android 3.9+/iOS 4.11，蒸汽模式有 mode=native（C 实现）；标签白名单 br,p,ul,li,span,strong,i,big,small,a[href],u,del,h1-h6,img[src]；span 样式=color,background-color,text-decoration；「text-decoration仅支持line-through」；「仅在 app-android 平台 VDOM 模式下且配置 mode=native 时受上述表格限制」；@itemclick 给 `href`；「不可以嵌套组件」 | 微信小程序 4.41；DCloud 明写「小程序的rich-text功能要弱一些，不支持rich-text中子内容的点击事件，如有这类需求，在小程序平台需要条件编译使用mp-html」 | https://doc.dcloud.net.cn/uni-app-x/component/rich-text.html |
| 5b | 微信 rich-text 白名单 | — | 基础库 1.4.0；「全局支持class和style属性，不支持id属性」；含 a(无属性),b,code,del,em,i,mark,pre,s,small,span,strong,sub,sup,u,img,table,td,th,tr…；「rich-text 组件内屏蔽所有节点的事件」→ a 不可点；「如果使用了不受信任的HTML节点，该节点及其所有子节点将会被移除」；user-select 2.24.0 会使节点变 block | https://developers.weixin.qq.com/miniprogram/dev/component/rich-text.html |
| 6 | 编译到小程序的语义 | — | 「小程序端 uvue 文件会被编译为同名的 js、json、wxml/axml、wxss/acss 文件」；小程序端仍做 ucss 样式重置（可关）；Element API 映射到 wxs/sjs | https://doc.dcloud.net.cn/uni-app-x/mp/index.html |
| 7 | 微信 text 细节 | — | 基础库 1.0.0；decode 为 **WebView 特有**属性（1.4.0），可解析 &nbsp;&lt;&gt;&amp;&apos;&ensp;&emsp;；user-select 2.12.1 → inline-block；「基础库低于 2.1.0 时，text 内嵌的 text style 设置可能不会生效」；渲染框架 Skyline+WebView | 微信 text 页 |
| 9 | 打开外链 | — | 「可打开关联的公众号的文章，其它网页需登录小程序管理后台配置业务域名」；「个人类型的小程序暂不支持使用」；「网页内 iframe 的域名也需要配置到域名白名单」；每页仅一个 web-view 且铺满覆盖 | https://developers.weixin.qq.com/miniprogram/dev/component/web-view.html |
| 10 | 键盘/textarea | uni.onKeyboardHeightChange：Web x，Android 4.71/iOS 4.71 | 微信小程序 **4.41 支持**；uni-app x textarea 组件（微信 4.41）有 adjust-position（属性级分端兼容列空白）；微信 textarea adjust-position 1.9.90、bind:keyboardheightchange 2.7.0；「keyboardheightchange事件可能会多次触发，开发者对于相同的height值应该忽略掉」；「textarea 的 blur 事件会晚于页面上的 tap 事件」 | https://doc.dcloud.net.cn/uni-app-x/api/keyboard.html ；…/component/textarea.html ；https://developers.weixin.qq.com/miniprogram/dev/component/textarea.html |

## 官方示例里 `/* #ifdef WEB */` 包住 text-decoration（A2f 的旁证）

https://doc.dcloud.net.cn/uni-app-x/component/text.html 示例样式出现 3 处：

```
font-weight: bold;
font-style: italic;
/* #ifdef WEB */
text-decoration: underline;
/* #endif */
```

同页「嵌套1」示例则在 App 生效路径上用 `text-decoration-line:underline`（未包 #ifdef）。

## 社区做法（非一手，仅参照）

- mp-html https://github.com/jin-yufeng/mp-html ：「≈25KB，9KB gzipped」；更新日志 v2.5.2 (20251214)；仓库 API pushed_at 2026-04-19、star 3750、MIT。概况原文：「其自带的 rich-text 组件支持的标签少且屏蔽所有事件，难以实际应用」。承载面（读自身源码 dist/mp-weixin/node/node.wxml）：递归 WXML `<template is="el">` + 手写 5 层 `wx:for` + 超深用 `<node>` 自定义组件递归；行内标签集由 wxs `isInline` 判定（abbr,b,big,code,del,em,i,ins,label,q,small,span,strong,sub,sup）；文本走 `<text decode>`，`a` 走 `<view style="display:inline" catchtap="linkTap">`，其余分支 fallback `<rich-text nodes="{{[n]}}">`。链接点击=linkTap；`copy-link` 默认 true「是否允许外部链接被点击时自动复制」；表格=table + `scroll-table`；代码高亮=highlight 插件；markdown=markdown 插件。
- towxml https://github.com/sbfkcel/towxml ：「可将 HTML、Markdown 转为微信小程序 WXML 的渲染库」；3.0 支持代码高亮/表格/删除线/事件绑定/按需构建；API pushed_at 2026-04-14、star 2898。

## 文档空白（只能真机/真工具判）

1. App 端**嵌套子 text** 上 background-color / color / font-* / border-radius 的实际渲染与「是否随父文本流内联换行」——文档只给「排版类样式不生效」的清单，未给正面对子节点生效样式的完整清单。
2. App 端子 text 的 padding/行高内边距盒（底色块能否包住文字、是否被裁）——文档未载明。
3. app-android 等宽字体可得性：系统是否自带可用 monospace、ttf 注册后行内是否生效——文档未载明（只写「支持ttf和otf」）。
4. 子 text 点击命中区（是否含空白、是否与 hover 冲突）——文档未载明。
5. 微信 WebView 渲染下 text/text-align/text-decoration 的逐属性矩阵：微信新版 WXSS 页无组件级支持表（仅 Skyline 页有表）→ WebView 侧「文档未载明」，只说受各系统 webview 版本影响。
6. rich-text nodes 的 **style 属性子集**：白名单只给 tag 与 tag 属性，未给内联 style 允许哪些属性 → 文档未载明。
7. 小程序端「用户贴的外链」官方标准做法（复制链接+toast）——无任何官方文档表述；只有 web-view 业务域名硬约束。
8. uni-app x textarea `adjust-position` 的分端兼容（属性表兼容性列为空）——文档未载明。
9. 「UGC 不要用 HTML 字符串直接渲染」的官方安全口径——DCloud 与微信两侧均**查无原文**；机制层只有微信白名单+移除不受信任节点，UGC 审核另有微信「内容安全」接口 https://developers.weixin.qq.com/miniprogram/dev/api-backend/open-api/sec-check/security.msgSecCheck.html 。

## 两端口径不一致（决定退化是否按端分档）

- `text-decoration` 简写：App 端 x（须用 `text-decoration-line`）；小程序/web 全 CSS 可用 → **必须按端分档**。
- 删除线：App 端只能 `text-decoration-line: line-through`（overline 不支持）；rich-text 侧「text-decoration仅支持line-through」。
- 样式继承：App 端父子不继承（每条样式须逐节点写满）；小程序端（wxml/wxss）按 CSS 继承 → 同一份 runs 在两端必须都显式带样式才一致。
- 盒类样式（padding/margin/width/display）：根 text 可用、**嵌套子 text 不生效**（App），小程序端 WXSS 正常生效 → 行内代码「底色+圆角+内边距」的观感两端会分叉。
- 点击：App 端 4.51+ 子 text 可点（且 Vapor 下子 text 无 hover 点击态）；微信端 text 无屏蔽条款，但 rich-text 明确「屏蔽所有节点的事件」→ 若任一端走 rich-text，链接点击只在 App 端有。
- uvue css 默认：`display` 默认 flex（W3C 是 inline）、`white-space` uvue-app=keep / uvue-web=pre-line / w3c=normal → 换行与空白折叠两端不同。

---
---

# 结论（主会话 · 2026-10-02）：三问三答

> 本文件是 `/research` 的唯一产物：上半部 = 一手文档事实矩阵，下半部 = 对本仓三个问题的判定。
> 仓内引用一律给「文件:行号」。

## 问 1：用 HTML 验证，能不能「明确格式」？

**分两层，答案相反。**

| 层 | HTML 原型能不能定 | 依据 |
| --- | --- | --- |
| **格式表达的逻辑**（哪些记号 → 哪种 run → 摘要怎么剥 → 未声明语法落哪） | **能**，且已经定完（`.scratch/prototype/_verify-forum-inline.js` 44 条断言全绿） | 本仓判据口径：「读者能不能读出原意」，不是「两端是否一模一样」（根 `docs/adr/ADR-0046-内容渲染口径.md:29`） |
| **平台画不画得出来**（双端 CSS 支持面） | **不能** —— 这一半由一手文档 + 真机定，不由浏览器定 | 既有先例原文：「H5 端未验。**本票判据的对象是 uvue 原生渲染器**」（`docs/verification/uvue-font-carrier/1269/README.md:121`）、「未验：H5 端与小程序端」（同目录 `SUMMARY.md:22`） |

⚠️ **本轮最有用的一处反转**：我上一轮把 4 格划成「只能真机判」，一手文档其实**已经答了 2 格**——

- `text-decoration` 简写在 App 端**不支持**（兼容表 Android x / iOS x），但 **`text-decoration-line` 长形态支持**（Android 3.9+），且「App平台仅支持 underline 和 line-through」⇒ ADR-0025:314 那句「大概率不生效」**可以收窄**：不是"能不能"，而是"长形态在嵌套子 text 上的实际出图"。
- 子 `<text>` 上 `margin/padding/width/display` **不生效**是**文档明写**（不是推测）⇒ 行内代码「底色 + 字符空格」的退化口径从"猜最坏"升级为"按文档定死"。

**剩余真空白 = 6 格**（见上「文档空白」1/2/3/5/6/8 条），其中真正会咬票一的只有两条：**app-android 等宽字体可得性**（`font-family` 不支持逗号回退、自定义只认 ttf/otf）与 **子 text 点击命中区**。

## 问 2：微信小程序 + App 移动端「能接受的方式」是什么？

**结论：现在的 ⑩ 方案（解析器出 runs + 嵌套 `<text>` 承载样式）是双端唯一同时成立的形态**，但它的**四条写法约束**必须先钉死，否则会在真机上返工：

| # | 约束 | 一手依据 |
| --- | --- | --- |
| C1 | **每条 run 自带全量样式**，不靠继承 | App 端「子不继承父样式」（uvue text 组件页）；小程序端编译成 wxss 后**按 CSS 继承**（`uni-app-x/mp/index.html`）⇒ 取严的交集才两端一致 |
| C2 | 修饰线一律写 **`text-decoration-line`**（长形态），禁 `text-decoration` 简写 | `uni-app-x/css/text-decoration.html`：「app平台暂不支持 text-decoration 简写样式，仅支持 text-decoration-line」；官方示例自己就用 `/* #ifdef WEB */` 包着简写 |
| C3 | **行内代码胶囊不用 `padding`**，用 `background-color` + 字符空格；圆角不指望 | text 页明写子 text 的 `margin/padding/width` 不生效 |
| C4 | **等宽不依赖 `font-family` 回退列表**，Android 侧按「拿不到等宽 ⇒ 只掉样式不掉内容」设计 | `uni-app-x/css/font-family.html`：「不支持使用分隔符（,）…仅支持设置一个字体」；app-android 自定义字体「支持ttf和otf，不支持woff/woff2/可变字体」 |

**双端能力差里最硬的一条（决定链接的验收形状）**：

- App 端：子 `<text>` 可绑点击，**HBuilderX 4.51 起**（本机 5.23 满足）。
- 微信端：`<text>` **没有**「屏蔽事件」条款（屏蔽条款只在 `rich-text`），但**也正面没有**"嵌套子 text 可点"的承诺 ⇒ **文档未载明**，只能真机判。
- ⇒ ⑩-8 的「App 点开 / 小程序复制」这个按端分叉**站得住**，但方向要补一句：**小程序端可能连"复制"这个点击都拿不到**，那才是它真正的落空面。

**两条被证据推翻的既有口径**（都在移动端 ADR-0025）：

1. **⑩-2「`rich-text` 因此被否」的理由不成立**。事实：rich-text **App 端已实现**（Android 3.9+，蒸汽模式 `mode=native` 是 C 全新实现），标签白名单含 `strong / i / del / u / a[href] / span`，还提供 `@itemclick` 回 `href` ⇒ "只允许嵌 text"推不出"rich-text 不可用"。**结论仍然保留**，但要换成四条真理由：微信端 rich-text「**屏蔽所有节点的事件**」（`a` 不可点）+ 「内部为 block」+「不可以嵌套组件」+「nodes 的 style 子集文档未载明」+ 不吃 HTML 字符串就不给 UGC 新开注入通道。**改理由、不改决定。**
2. **⑩-8「小程序退化为复制链接 + toast」的依据层级要标出来**：微信官方**没有**任何"用户贴的外链该怎么处理"的表述，只有 `web-view` 业务域名硬约束（「其它网页需登录小程序管理后台配置业务域名」「个人类型小程序暂不支持使用」）。「点击外链 → 自动复制」是**社区库做法**（mp-html `copy-link` 默认 true），不是平台规范。

**第三方渲染库：不引入，但要知道为什么。**

- 本仓 `towxml / mp-html / nodes=` **全树 0 命中**（从未引入）。
- 两者都是**微信小程序原生 WXML 递归模板**思路（mp-html 承载面 = 递归 `<template is="el">` + 5 层 `wx:for` + 超深走自定义组件递归 + `wxs` 判行内标签集），**在 uni-app x 的双端形态上没有对应物**，且 towxml 只解小程序一端。
- 体积：mp-html 自称「≈25KB / 9KB gzipped」。⚠️ **本仓没有「小程序主包体积阈值」这条判据**（`主包 / 分包 / subPackages` 全树 0 命中）⇒ 将来真要引入，**先补阈值判据再谈**，否则引入决定无法验收。

## 问 3：较好实践（可直接抄进票一 AC 的 8 条）

1. **样式判据锁「有日志/有读数的属性」，不锁「推理出来的」**——本仓已确立这条口径：「只锁**有真机日志判据的四个属性**；`line-height`/`font-family` **不锁**——没有它们被判错的日志，**写了就是替渲染器编规则**」（`docs/verification/uvue-font-carrier/1269/README.md:30`）。⇒ 票一要加的按文本判守护应该是 **`text-decoration` 简写零命中** + **子 text 上不写 padding/margin**，形态照 `utils/uvueWhiteSpaceContract.test.js:53`（`LEGAL_CARRIERS` + `LEGAL_CARRIER_SITES` + 计数守恒）与同族 `uvueFontCarrierContract.test.js`。
2. **runs 是唯一事实源**：渲染与摘要都从同一次分词取叶子（本轮实测：两处分道正是 `~~` 泄漏的成因）。
3. **降级方向一律「记号消失、样式不出」**，绝不让内容消失——本轮所有落空格（等宽、下划线、删除线、胶囊内边距、小程序点击）都套这一条，与 ADR-0046:22-29 的「降级必须可读」同轴。
4. **按端分档要落到「判据形状」而不只是「样式形状」**：App 侧能出 a11y bounds / 像素读数（1269 先例：从 bounds 反推字号生效、认出蓝字+加粗）；**小程序侧结构上不能逐样式断言**——`page.$()` / `page.data()` 挂起超时、导航恒报 `Uncaught [object Object]` 记 SKIP（`docs/adr/0008-移动端验收门与证据.md:162,165,168`）⇒ 小程序端样式判据**只能整页截图 + 人眼**，别在 AC 里许诺"小程序端机检样式"。
5. **成对取证**：声明表加一行而渲染没变 ⇒ 用例必红；「只跑通过的那一次，不算验收」（0008:92）。
6. **跨端结论必须逐端取证**——本仓有现成的反例账：ADR-0067:30 那条 cookie 结论「已被 #1389 的真机读数推翻…**它的射程就是浏览器这一面**」。这正是「不能拿 HTML 结论外推到双端」的同族血账。
7. **键盘与浮层的既有事实**（票二）：`uni.onKeyboardHeightChange` Android **4.71** / 微信小程序 **4.41** 都**有**（Web 才没有）⇒ 票二 §6 那格从"API 可能不存在"收窄为"存在但配合未验"。微信侧另给两条**必须写进实现**的告警：「keyboardheightchange事件可能会多次触发，开发者对于相同的height值应该忽略掉」（去抖）、以及「textarea 的 blur 事件会晚于页面上的 tap 事件」（遮罩收起与点击竞态）。
8. **`white-space` / `display` 默认值两端不同**（uvue-app `white-space=keep`、`display=flex`；w3c 是 `normal`/`inline`）⇒ 行内 run 的**换行与空白折叠**在两端口径不同，别把 Web 上看到的换行行为当真机预期。

## 主结论核实（主会话亲读一手页，2026-10-02）

子代理返回后，我亲自回源核了**最要紧的一条**（它推翻了我上一轮划的待验格）：

- `text-decoration.html`：兼容性表 **Web 4.0 / Android x / iOS x / HarmonyOS x**；tips 原文「**app平台暂不支持 text-decoration 简写样式，仅支持 text-decoration-line 设置修饰线类型**」；适用组件 `text`、`button`。
- `text-decoration-line.html`：兼容性表 **Web 4.0 / Android 3.9 / iOS 4.11 / HarmonyOS 4.61**（Vapor 拍平 Android 5.21）；属性值表只列 **underline / line-through / overline / none**；适用组件 `text`、`button`；官方示例直接在 `<text style="text-decoration-line: underline;">` 上写、**未包 `#ifdef WEB`**。

⇒ 「App 端画不出下划线/删除线」这个担忧**在文档层面已否定**：写长形态就支持（Android 3.9 起，本机 5.23 满足）。
⇒ 我上一轮原型里那条「样式不出」的最坏演示（App 受限模式）现在应读作**兜底口径**，不再是预期结果。
⇒ 仍未证的只剩：**长形态作用在嵌套子 `<text>`（而非根 text）上时的实际出图** —— 文档给的是根 text 示例，子节点正面生效清单未载明（空白 1）。这一格留 ①a。

其余条目（C1–C4 的依据、`rich-text` 双端差异、微信 `text`/`web-view` 条款、键盘 API 版本）仍为子代理读数，URL 已逐条内嵌在上半部矩阵，**引用时请自行打开比对**——本仓口径：「只跑通过的那一次，不算验收」（移动端 `docs/adr/0008-移动端验收门与证据.md:92`）。

## 这张研究把票一/票二的「待验格」从 4 格收窄到 2 格

| 原 handoff §6 的格 | 现状 |
| --- | --- |
| App 端 `text-decoration` 是否生效 | **文档已答**（简写 x / 长形态支持 underline+line-through）⇒ 收窄为「长形态在子 text 上实际出图」 |
| 嵌套子 text 样式继承 / 盒模型不生效 | **文档已答**（正面明写）⇒ 不再是待验格，转为 C1/C3 两条写法约束 |
| app-android 等宽字体可得性 | **仍是真空白**（真机判） |
| 子 text 点击命中 + 小程序端嵌套 text 能否点 | **仍是真空白**（真机判；且这是 ⑩-8 唯一悬着的半边） |
| `uni.onKeyboardHeightChange` + fixed 面板配合 | **API 存在性已由文档确认双端都有** ⇒ 剩「配合」一格（真机判），微信侧另需按第 7 条做去抖与 blur 竞态处理 |

## 归档

- 一手来源已逐条内嵌 URL（上半部表格「依据」列）。
- 原型产物：`.scratch/prototype/forum-inline-and-reply-sheet.html`（+ `_verify-forum-inline.js` / `_verify.out` / `verdict-…20261002.md`），全在 gitignore 内，**未提交任何东西**。
- 建议下一步：把「两条被推翻的口径」与「C1–C4 四条写法约束」回写进移动端 `docs/adr/0025`（⑩-2 改理由、§6 收窄待验格），并把 8 条实践拆进两张 issue 的 AC——**回写 ADR 需要点头，本会话没动文档**。
