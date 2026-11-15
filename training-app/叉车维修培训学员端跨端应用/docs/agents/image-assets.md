# 图片资产落位口径

任何页面实施票要动 `<image>` / `static/` / Figma 切图时读本文件。硬口径（包内禁 webp、主包余量、编码）在 `AGENTS.md`「注意事项」与 `docs/adr/0066`，本文件不重复，只讲**怎么落位**。

## 一句话判据

**图跟着结构走，不跟着清单走。**

判据不是「票做没做完」，而是**「这一屏的结构定了没有」**。先补图后改结构 = 图白做；页面没实现就补图 = 无处可放。

## 五档 verdict

每张切图先判档，档决定它什么时候做：

| 档 | 含义 | 什么时候做 |
| --- | --- | --- |
| `UI_ASSET` | 已有插槽且已随包 | 已做完，别重做 |
| `PLACEHOLDER` | 有插槽，但兜底是 emoji / 字母圆圈 / 空灰块 | **可插队**——不改结构，纯换兜底 |
| `UNIMPLEMENTED` | 页面还没实现 | 跟着页面实现票一起做 |
| `UNUSED` | 现网无插槽，补图 = 补 UI = 改结构 | 并入该屏的结构改造批，**不单开换图票** |
| `CONTENT_STATIC` | 内容型静态图（logo、吉祥物） | 随设计稿改版一起换 |

**不要单开「换图票」**——那是「先补图、后改结构」的返工源。唯一例外是 `PLACEHOLDER`，因为它不改结构。

## 落位步骤（每张图）

1. **查映射表**：https://github.com/DriftingLi/FL/issues/2508#issuecomment-6891929930
   25 个切图目录 ↔ 原型屏 node ↔ 现网路由 ↔ 承载票 ↔ 状态，一张表看全。先确认这屏有哪些图、什么档。
2. **确认插槽存在**：`grep '<image'` 数一遍，再看 `v-else` 兜底是什么。没有插槽就别硬塞（那是 `UNUSED`，等结构改造）。
3. **出图**：尺寸 = 容器 CSS 尺寸 × 2（见下「出图口径」）。
4. **改模板**：包内路径写**字面量**（见下「模板写法」）。
5. **契约测试**：先写红再改（见下「契约测试」）。
6. **门 + 主包读数**：`npm run test:unit:changed` + `npm run build:mp-weixin-buildonly` 拿 `mainBytes=`。
7. **验收**：截图对照原型 PNG，结构 + 视觉一致即可，**不做像素级硬门**。

## 出图口径

- **尺寸 = 容器 CSS 尺寸 × 2**。容器 `116rpx` = 58px CSS ⇒ 出 116×116。别拿设计稿原尺寸直接塞（AI 功能那 5 张 256×256 就是没走这条，白占体积）。
- **调色板 PNG，逐张取优** `min(RGBA, 256色)`。
- **≤256 色小图标量化会反涨**（PLTE 开销，实测最狠 7.22×）⇒ 必须逐张实测，不能一律量化。
- **不放大**。原图小于目标尺寸就按原尺寸出。

## 模板写法

```html
<!-- 包内图：路径写字面量，不需要 :webp -->
<image class="icon-img" :src="mode == 'phone' ? '/static/icons/login/phone-active.png' : '/static/icons/login/phone.png'" mode="aspectFit" />

<!-- 网络图：必须 :webp="true" -->
<image :src="item.cover_url" mode="aspectFill" :webp="true" />
```

**为什么包内路径必须写字面量**：守护 `utils/imageWebpAttrContract.test.js` 的豁免判据是 `refersToStaticPath(expr) = /(\/static\/)/.test(expr)`，只看 `:src` **表达式文本**里有没有 `/static/`。路径藏在 computed 后面（`:src="phoneIconPath"`）会被判漏 `:webp` 而红——#2554 踩过。

## 契约测试

仓库已有 `utils/imageWebpAttrContract.test.js`（网络图必须 `:webp`、包内图豁免）、`utils/loginContract.test.js`（模板/样式 sha 钉死）这类契约测试。

落位时按需加断言：
- 兜底消失：不再有 `👤` / `👷` / `authorInitial`，而是 `<image>` 指向 `/static/...`
- 包内路径：`refersToStaticPath` 豁免成立

⚠️ **改模板会让 `TEMPLATE_SHA256` 变红**，必须重算：`readText`（CRLF→LF）→ 取 `<template` 到 `</template>`+10 → sha256 hex。

## 验收

- **截图对照原型 PNG**（`D:\FL\.scratch\figma-proto-export\images\` 或 `design-slices/`），结构 + 视觉一致即可。
- **主包读数**：`mainBytes=` 必须打出来，别拿 dev 产物或旧记录估。`G7a` 只打读数**不判红** ⇒ 破线是静默的。
- 证据形态 A1：PR 正文一行 `跑于 <sha7>` + 门读数行原样。

## 常见坑

| 坑 | 教训 |
| --- | --- |
| 包内 webp | 真机/体验版空白，开发者工具（Chromium）看不出来。**包内一律 PNG** |
| 网络图漏 `:webp` | 后端把 jpg/png/bmp 转 webp 存，封面/头像/验证码全是网络 webp，漏写就是空白 |
| 路径藏 computed 后面 | 守护判不出来，会红。写字面量 |
| 设计稿原尺寸直接塞 | 256×256 显示 40×40，白占体积。按容器 2x 出 |
| 一律量化 | ≤256 色小图标反涨。逐张实测 |
| 全量 `test:unit:gate` | 实测约 80 分钟（40 s/套件 × 118 套件）。日常用 `test:unit:changed`（≈140 s），全量交给 CI |
| 比例不匹配硬塞 | 书封 71×100 塞 70×70 方框会裁切。先看原型确认容器比例，再定改容器还是另出素材 |

## 映射表与源

- **映射表**：#2508 评论 https://github.com/DriftingLi/FL/issues/2508#issuecomment-6891929930
- **切图源**：`D:\FL\.tmp-slices\切图\`（25 子目录 / 190 文件，未跟踪，**别删**）
- **已渲染**：`.tmp-render-tier\`（184 张 1x 调色板 PNG）、`.tmp-render-2x\`（184 张 2x）
- **判定表**：`.tmp-slice-usage.csv`（表头 `dir|name|css|verdict|evidence`）、`.tmp-slices-triage.csv`（尺寸）
- **原始 zip**：`C:\Users\ZHENG\OneDrive\Desktop\01_设计稿\切图优化.zip`（408 条目 / 24,003,953 B）
